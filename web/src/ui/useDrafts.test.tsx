import { act, renderHook } from '@testing-library/react';
import { describe, expect, it } from 'vitest';
import { useDrafts } from './useDrafts';

const server: Record<string, { name: string }> = { a: { name: 'Alpha' }, b: { name: 'Beta' } };
const originalOf = (id: string) => server[id];

describe('useDrafts (concept 12, S-17)', () => {
  it('holds drafts per id, reports dirty per id and in total, discards per id', () => {
    const { result } = renderHook(() => useDrafts(originalOf));
    expect(result.current.dirty).toBe(false);
    expect(result.current.valueOf('a')).toEqual({ name: 'Alpha' });

    act(() => result.current.set('a', { name: 'Alfa' }));
    act(() => result.current.set('b', (current) => ({ ...current, name: 'Bet' })));
    expect(result.current.valueOf('a')).toEqual({ name: 'Alfa' });
    expect(result.current.isDirty('a')).toBe(true);
    expect(result.current.isDirty('b')).toBe(true);
    expect(result.current.dirtyIds).toEqual(['a', 'b']);
    expect(result.current.dirty).toBe(true);

    act(() => result.current.discard('a'));
    expect(result.current.valueOf('a')).toEqual({ name: 'Alpha' });
    expect(result.current.isDirty('a')).toBe(false);
    expect(result.current.dirty).toBe(true);

    act(() => result.current.discardAll());
    expect(result.current.dirty).toBe(false);
  });

  it('drops a draft that is changed back to the original', () => {
    const { result } = renderHook(() => useDrafts(originalOf));
    act(() => result.current.set('a', { name: 'Alfa' }));
    act(() => result.current.set('a', { name: 'Alpha' }));
    expect(result.current.isDirty('a')).toBe(false);
    expect(result.current.dirty).toBe(false);
  });
});
