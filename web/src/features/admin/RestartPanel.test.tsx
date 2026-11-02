import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { render, screen, waitFor } from '@testing-library/react';
import { describe, expect, it, vi } from 'vitest';
import { json, mockApi } from '@/test/api';
import { RestartPanel } from './RestartPanel';

// Concept 11: the panel is shared by the restore and the restart. It shows the caller's texts,
// waits for a new server process and then calls onBack once.
describe('RestartPanel', () => {
  it('shows the given texts and calls onBack once the new server answers', async () => {
    mockApi({ 'GET /healthz': () => json({ status: 'ok', version: '1.7.0', uptimeSeconds: 0 }) });
    const onBack = vi.fn();
    const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } });
    render(
      <QueryClientProvider client={queryClient}>
        <RestartPanel acceptedAt={Date.now() - 5000} title="Titel" body="Text" onBack={onBack} />
      </QueryClientProvider>,
    );
    const heading = screen.getByRole('heading', { level: 2, name: 'Titel' });
    await waitFor(() => expect(heading).toHaveFocus());
    expect(screen.getByText('Text')).toBeVisible();
    await waitFor(() => expect(onBack).toHaveBeenCalledTimes(1));
    expect(screen.getByRole('status')).toHaveTextContent('Der Server ist zurück.');
  });
});
