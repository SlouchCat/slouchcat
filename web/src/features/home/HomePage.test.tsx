import type { Health } from '@slouchcat/api-client';
import { createMemoryHistory } from '@tanstack/react-router';
import { QueryClient } from '@tanstack/react-query';
import { render, screen } from '@testing-library/react';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { App } from '@/app/App';
import { createAppRouter } from '@/app/router';

function renderHome() {
  const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  const router = createAppRouter(createMemoryHistory({ initialEntries: ['/'] }));
  return render(<App router={router} queryClient={queryClient} />);
}

describe('home route', () => {
  afterEach(() => {
    vi.unstubAllGlobals();
  });

  it('shows the app name, a reachable server and its version', async () => {
    const health: Health = { status: 'ok', version: '0.1.0+abc1234', uptimeSeconds: 5 };
    const fetchMock = vi
      .fn<(request: Request) => Promise<Response>>()
      .mockImplementation(() => Promise.resolve(Response.json(health)));
    vi.stubGlobal('fetch', fetchMock);

    renderHome();

    expect(await screen.findByRole('heading', { level: 1, name: 'Slouchcat' })).toBeVisible();
    expect(await screen.findByText('Server erreichbar')).toBeVisible();
    expect(screen.getByText('Version 0.1.0+abc1234')).toBeVisible();
    expect(new URL(fetchMock.mock.calls[0]![0].url).pathname).toBe('/api/v1/healthz');
  });

  it('reports an unreachable server', async () => {
    vi.stubGlobal('fetch', vi.fn().mockRejectedValue(new TypeError('Failed to fetch')));

    renderHome();

    expect(await screen.findByText('Server nicht erreichbar')).toBeVisible();
    expect(screen.queryByText(/^Version/)).not.toBeInTheDocument();
  });
});
