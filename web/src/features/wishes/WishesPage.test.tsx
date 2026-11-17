import type { components } from '@slouchcat/api-client';
import { screen, waitFor, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { json, mockApi, renderApp, session } from '@/test/api';

type Wish = components['schemas']['Wish'];

const rights = { enabled: true, canRequest: true, movies: true, shows: true };
const withWishes = (wishes = rights) => ({
  'GET /auth/session': () => json({ ...session, wishes }),
});

const base: Pick<
  Wish,
  | 'origin'
  | 'role'
  | 'canWithdraw'
  | 'canRequestAgain'
  | 'createdAt'
  | 'updatedAt'
  | 'events'
  | 'seasons'
> = {
  origin: 'slouchcat',
  role: 'owner',
  canWithdraw: false,
  canRequestAgain: false,
  createdAt: '2026-11-10T09:00:00Z',
  updatedAt: '2026-11-12T09:00:00Z',
  events: [],
  seasons: [],
};

const pending: Wish = {
  ...base,
  id: 1,
  media: 'movie',
  tmdbId: 9001,
  title: 'Glasvogel',
  year: 2026,
  state: 'pending',
  canWithdraw: true,
  events: [
    { at: '2026-11-12T09:00:00Z', kind: 'submitted' },
    { at: '2026-11-12T08:59:00Z', kind: 'created' },
  ],
};
const partial: Wish = {
  ...base,
  id: 2,
  media: 'show',
  tmdbId: 9100,
  title: 'Nebelhafen',
  state: 'partial',
  role: 'joined',
  seasons: [
    { number: 1, state: 'available' },
    { number: 2, state: 'processing' },
  ],
};
const failed: Wish = {
  ...base,
  id: 3,
  media: 'movie',
  tmdbId: 9002,
  title: 'Kupfermond',
  state: 'failed',
  failReason: 'quota',
  canRequestAgain: true,
};

const counts = { open: 2, pending: 1, processing: 0, partial: 1, available: 0, closed: 1 };

const queryOf = (fetchMock: ReturnType<typeof mockApi>['fetchMock']) =>
  fetchMock.mock.calls
    .map(([request]) => new URL(request.url))
    .filter((url) => url.pathname === '/api/v1/wishes' && url.searchParams.get('limit') !== '1')
    .map((url) => url.searchParams.get('state'));

describe('«Meine Wünsche» (concept 25, 5.3)', () => {
  afterEach(() => vi.unstubAllGlobals());

  it('lists the wishes with state, seasons, history and actions; the chips filter by state', async () => {
    const { fetchMock } = mockApi({
      ...withWishes(),
      'GET /wishes': () => json({ items: [pending, partial, failed], total: 3, counts }),
      'GET /wishes/quota': () =>
        json({
          unlimited: false,
          movie: { limit: 10, days: 7, used: 3, queued: 0, remaining: 7 },
          at: '2026-11-17T08:00:00Z',
        }),
    });
    renderApp('/wishes');
    expect(await screen.findByRole('heading', { level: 1, name: 'Meine Wünsche' })).toBeVisible();
    expect(await screen.findByText('2 offene Wünsche')).toBeVisible();
    expect(
      await screen.findByText(
        'Limit: Filme: 3 von 10 in 7 Tagen · noch 7 frei · Staffeln ohne Limit',
      ),
    ).toBeVisible();

    const glass = screen.getByRole('article', { name: /Glasvogel/ });
    expect(within(glass).getByText('Freigabe ausstehend')).toBeVisible();
    expect(within(glass).getByRole('link', { name: 'Zum Titel' })).toHaveAttribute(
      'href',
      '/titles/movie/9001',
    );
    expect(within(glass).getByRole('button', { name: 'Glasvogel: zurückziehen' })).toBeVisible();
    // The history is folded, its last step in the toggle.
    const history = within(glass).getByRole('button', { name: 'Verlauf: An Seerr übermittelt' });
    expect(history).toHaveAttribute('aria-expanded', 'false');
    const user = userEvent.setup();
    await user.click(history);
    expect(within(glass).getByRole('list', { name: 'Verlauf' })).toBeVisible();

    const fog = screen.getByRole('article', { name: /Nebelhafen/ });
    expect(within(fog).getByText('Staffeln 1, 2 · du wünschst mit')).toBeVisible();
    const copper = screen.getByRole('article', { name: /Kupfermond/ });
    expect(
      within(copper).getByText('Limit erreicht: Seerr hat den Wunsch nicht angenommen.'),
    ).toBeVisible();
    expect(
      within(copper).getByRole('button', { name: 'Kupfermond erneut anfordern' }),
    ).toBeVisible();

    // The chips with their counts; «Offen» goes into the URL and the request.
    await user.click(screen.getByRole('button', { name: 'Offen (2)' }));
    await waitFor(() => expect(queryOf(fetchMock)).toContain('open'));
    expect(document.body.textContent).not.toContain('{{');
  });

  it('a limit never read from Seerr (no `at`) claims nothing, not «ohne Limit»', async () => {
    mockApi({
      ...withWishes(),
      'GET /wishes': () => json({ items: [pending], total: 1, counts }),
      'GET /wishes/quota': () => json({ unlimited: false }),
    });
    renderApp('/wishes');
    expect(await screen.findByRole('article', { name: /Glasvogel/ })).toBeVisible();
    expect(screen.queryByText(/ohne Limit|^Limit:/)).toBeNull();
  });

  it('asks before it withdraws, then calls withdrawWish', async () => {
    let gone = false;
    const { calls } = mockApi({
      ...withWishes(),
      'GET /wishes': () =>
        json({
          items: gone ? [] : [pending],
          total: gone ? 0 : 1,
          counts: { ...counts, open: gone ? 0 : 1 },
        }),
      'GET /wishes/quota': () => json({ unlimited: true }),
      'DELETE /wishes/1': () => {
        gone = true;
        return new Response(null, { status: 204 });
      },
    });
    renderApp('/wishes');
    const user = userEvent.setup();
    await user.click(await screen.findByRole('button', { name: 'Glasvogel: zurückziehen' }));
    const dialog = await screen.findByRole('alertdialog', { name: 'Wunsch zurückziehen?' });
    await user.click(within(dialog).getByRole('button', { name: 'Zurückziehen' }));
    await waitFor(() => expect(calls.some((call) => call.method === 'DELETE')).toBe(true));
    expect(await screen.findByText('Noch keine Wünsche.')).toBeVisible();
  });

  it('the side navigation shows «Wünsche» with the open count, only while the wishes are on', async () => {
    mockApi({
      ...withWishes(),
      'GET /wishes': () => json({ items: [pending], total: 1, counts }),
      'GET /wishes/quota': () => json({ unlimited: true }),
    });
    const { unmount } = renderApp('/wishes');
    const nav = await screen.findByRole('navigation', { name: 'Hauptnavigation' });
    expect(
      await within(nav).findByRole('link', { name: 'Wünsche, 2 offene Wünsche' }),
    ).toHaveAttribute('aria-current', 'page');
    unmount();
    vi.unstubAllGlobals();

    mockApi({ ...withWishes({ ...rights, enabled: false }) });
    renderApp('/watched');
    const other = await screen.findByRole('navigation', { name: 'Hauptnavigation' });
    expect(await within(other).findByRole('link', { name: 'Gesehen' })).toBeVisible();
    expect(within(other).queryByRole('link', { name: /Wünsche/ })).toBeNull();
  });
});
