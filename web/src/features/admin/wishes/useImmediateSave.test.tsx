import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { act, renderHook, waitFor } from '@testing-library/react';
import type { ReactNode } from 'react';
import { describe, expect, it, vi } from 'vitest';
import { useImmediateSave } from './useImmediateSave';

function deferred<T>() {
  let resolve!: (value: T) => void;
  let reject!: (error: unknown) => void;
  const promise = new Promise<T>((yes, no) => {
    resolve = yes;
    reject = no;
  });
  return { promise, resolve, reject };
}

function wrapper({ children }: { children: ReactNode }) {
  return <QueryClientProvider client={new QueryClient()}>{children}</QueryClientProvider>;
}

describe('useImmediateSave', () => {
  it('sends two changes of one control one after the other and keeps only the last answer', async () => {
    const answers = [deferred<string>(), deferred<string>()];
    const save = vi.fn((patch: string) => answers[patch === 'A' ? 0 : 1]!.promise);
    const saved = vi.fn((data: string) => `saved ${data}`);
    const { result } = renderHook(() => useImmediateSave({ save, saved }), { wrapper });

    act(() => result.current.change('switch', 'A'));
    act(() => result.current.change('switch', 'B'));
    expect(result.current.pending.get('switch')).toBe('B');
    await waitFor(() => expect(save).toHaveBeenCalledTimes(1));

    // A's answer arrives while B waits: the control keeps showing B.
    await act(async () => {
      answers[0]!.resolve('A');
      await Promise.resolve();
    });
    await waitFor(() => expect(save).toHaveBeenCalledTimes(2));
    expect(result.current.pending.get('switch')).toBe('B');
    expect(saved).not.toHaveBeenCalled();

    await act(async () => {
      answers[1]!.resolve('B');
      await Promise.resolve();
    });
    await waitFor(() => expect(result.current.pending.has('switch')).toBe(false));
    expect(saved).toHaveBeenCalledTimes(1);
    expect(saved).toHaveBeenCalledWith('B', 'B', 'switch');
    expect(result.current.failure).toBeNull();
  });

  it('a refusal of an earlier change does not stand beside a later one that went through', async () => {
    const answers = [deferred<string>(), deferred<string>()];
    const save = vi.fn((patch: string) => answers[patch === 'A' ? 0 : 1]!.promise);
    const { result } = renderHook(() => useImmediateSave({ save, saved: () => 'ok' }), {
      wrapper,
    });

    act(() => result.current.change('choice', 'A'));
    act(() => result.current.change('choice', 'B'));
    await act(async () => {
      answers[0]!.reject(new Error('refused'));
      await Promise.resolve();
    });
    await waitFor(() => expect(save).toHaveBeenCalledTimes(2));
    expect(result.current.failure).toBeNull();
    await act(async () => {
      answers[1]!.resolve('B');
      await Promise.resolve();
    });
    await waitFor(() => expect(result.current.pending.size).toBe(0));
    expect(result.current.failure).toBeNull();
    expect(result.current.message?.text).toBe('ok');
  });

  it('two controls do not wait for each other', async () => {
    const hanging = deferred<string>();
    const save = vi.fn((patch: string) =>
      patch === 'slow' ? hanging.promise : Promise.resolve(patch),
    );
    const { result } = renderHook(() => useImmediateSave({ save, saved: (data) => data }), {
      wrapper,
    });
    act(() => result.current.change('one', 'slow'));
    act(() => result.current.change('two', 'fast'));
    await waitFor(() => expect(result.current.pending.has('two')).toBe(false));
    expect(result.current.pending.get('one')).toBe('slow');
  });
});
