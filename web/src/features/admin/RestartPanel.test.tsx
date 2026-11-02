import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { act, render, screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { json, mockApi } from '@/test/api';
import { RestartPanel } from './RestartPanel';

// Concept 11: the panel is shared by the restore and the restart. It shows the caller's texts,
// waits for a new server process and then calls onBack once; after 90 s without one it gives up
// until «Erneut prüfen».

const health = (uptimeSeconds: number) => json({ status: 'ok', version: '1.7.0', uptimeSeconds });

function renderPanel(acceptedAt: number, onBack: () => void) {
  const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  render(
    <QueryClientProvider client={queryClient}>
      <RestartPanel acceptedAt={acceptedAt} title="Titel" body="Text" onBack={onBack} />
    </QueryClientProvider>,
  );
}

describe('RestartPanel', () => {
  afterEach(() => {
    vi.useRealTimers();
    vi.unstubAllGlobals();
  });

  it('shows the given texts and calls onBack once the new server answers', async () => {
    mockApi({ 'GET /healthz': () => health(0) });
    const onBack = vi.fn();
    renderPanel(Date.now() - 5000, onBack);
    const heading = screen.getByRole('heading', { level: 2, name: 'Titel' });
    await waitFor(() => expect(heading).toHaveFocus());
    expect(screen.getByRole('region', { name: 'Titel' })).toBeVisible();
    expect(screen.getByText('Text')).toBeVisible();
    await waitFor(() => expect(onBack).toHaveBeenCalledTimes(1));
    expect(screen.getByRole('status')).toHaveTextContent('Der Server ist zurück.');
  });

  it('gives up after 90 s without the new server; «Erneut prüfen» waits again', async () => {
    vi.useFakeTimers({ shouldAdvanceTime: true });
    let down = true;
    const { calls } = mockApi({
      'GET /healthz': () => {
        if (down) throw new TypeError('Failed to fetch');
        return health(0);
      },
    });
    const onBack = vi.fn();
    renderPanel(Date.now(), onBack);
    const heading = screen.getByRole('heading', { level: 2, name: 'Titel' });
    const polls = () => calls.filter((call) => call.path === '/healthz').length;

    await act(() => vi.advanceTimersByTimeAsync(88_000));
    expect(screen.queryByText('Der Server kommt nicht zurück')).not.toBeInTheDocument();
    await act(() => vi.advanceTimersByTimeAsync(2_000));
    expect(await screen.findByText('Der Server kommt nicht zurück')).toBeVisible();
    expect(screen.getByRole('status')).toHaveTextContent(
      'Seit 90 Sekunden antwortet kein neuer Server.',
    );
    const stopped = polls();
    await act(() => vi.advanceTimersByTimeAsync(10_000));
    expect(polls()).toBe(stopped);
    expect(onBack).not.toHaveBeenCalled();

    down = false;
    const user = userEvent.setup({ advanceTimers: (ms) => vi.advanceTimersByTime(ms) });
    await user.click(screen.getByRole('button', { name: 'Erneut prüfen' }));
    await waitFor(() => expect(onBack).toHaveBeenCalledTimes(1));
    expect(heading).toHaveFocus();
    expect(screen.queryByText('Der Server kommt nicht zurück')).not.toBeInTheDocument();
  });
});
