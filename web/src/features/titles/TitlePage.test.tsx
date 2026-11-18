import type { components } from '@slouchcat/api-client';
import { screen, waitFor, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { json, mockApi, problem, renderApp, session } from '@/test/api';
import { heat } from '@/test/fixtures';

type ExternalTitle = components['schemas']['ExternalTitle'];
type Wish = components['schemas']['Wish'];

const withWishes = {
  'GET /auth/session': () =>
    json({ ...session, wishes: { enabled: true, canRequest: true, movies: true, shows: true } }),
};

const film: ExternalTitle = {
  media: 'movie',
  tmdbId: 9001,
  title: 'Glasvogel',
  year: 2026,
  images: {
    poster: {
      url: '/api/v1/titles/movie/9001/images/poster/w400?v=1&p=1',
      width: 400,
      height: 600,
    },
  },
  hasTrailer: false,
  canRequest: true,
  genres: ['Drama'],
  seasons: [],
  trailers: [],
  overview: 'Ein Vogel aus Glas.',
  runtimeMinutes: 101,
  request: {
    canRequest: true,
    autoApprove: false,
    quota: {
      unlimited: false,
      movie: { limit: 10, days: 7, used: 3, queued: 0, remaining: 7 },
      at: '2026-11-17T08:00:00Z',
    },
  },
};

const series: ExternalTitle = {
  ...film,
  media: 'show',
  tmdbId: 9100,
  title: 'Nebelhafen',
  runtimeMinutes: undefined,
  seasons: [
    { number: 1, episodeCount: 8, state: 'missing' },
    { number: 2, episodeCount: 10, state: 'requested' },
    { number: 3, episodeCount: 10, state: 'missing' },
  ],
  request: {
    canRequest: true,
    autoApprove: true,
    quota: {
      unlimited: false,
      at: '2026-11-17T08:00:00Z',
      tv: {
        limit: 5,
        days: 7,
        used: 4,
        queued: 0,
        remaining: 1,
        nextFreeAt: '2026-11-20T10:00:00Z',
      },
    },
  },
};

const wishOf = (title: ExternalTitle, extra: Partial<Wish> = {}): Wish => ({
  id: 41,
  media: title.media,
  tmdbId: title.tmdbId,
  title: title.title,
  state: 'pending',
  origin: 'slouchcat',
  role: 'owner',
  seasons: [],
  canWithdraw: true,
  canRequestAgain: false,
  createdAt: '2026-11-17T09:00:00Z',
  updatedAt: '2026-11-17T09:00:00Z',
  events: [{ at: '2026-11-17T09:00:00Z', kind: 'created' }],
  ...extra,
});

const counts = { open: 1, pending: 1, processing: 0, partial: 0, available: 0, closed: 0 };

describe('preview page of a title outside the library (concept 25, 5.1)', () => {
  afterEach(() => vi.unstubAllGlobals());

  it('requests a film in the dialog; the big button turns into the state', async () => {
    let wished = false;
    const { calls } = mockApi({
      ...withWishes,
      'GET /titles/movie/9001': () =>
        json(wished ? { ...film, wish: { id: 41, state: 'pending' } } : film),
      'GET /wishes/quota': () => json(film.request.quota),
      'GET /wishes': () =>
        json({ items: wished ? [wishOf(film)] : [], total: wished ? 1 : 0, counts }),
      'POST /wishes': () => {
        wished = true;
        return json(wishOf(film), 201);
      },
    });
    renderApp('/titles/movie/9001');
    expect(await screen.findByRole('heading', { level: 1, name: 'Glasvogel' })).toBeVisible();
    expect(screen.getByText('Noch nicht in der Bibliothek')).toBeVisible();
    // No age rating for viewers (concept 17 F-10).
    expect(screen.getByText(/2026 · 1 h 41 min/)).toBeVisible();

    const user = userEvent.setup();
    await user.click(screen.getByRole('button', { name: 'Film anfordern' }));
    const dialog = await screen.findByRole('dialog', { name: 'Film anfordern: Glasvogel' });
    expect(within(dialog).getByText(/Braucht eine Freigabe/)).toBeVisible();
    expect(
      await within(dialog).findByText('Filme: 3 von 10 in 7 Tagen · noch 7 frei'),
    ).toBeVisible();
    await user.click(within(dialog).getByRole('button', { name: 'Anfordern' }));

    await waitFor(() => expect(dialog).not.toBeInTheDocument());
    expect(calls.find((call) => call.method === 'POST')?.body).toEqual({
      media: 'movie',
      tmdbId: 9001,
      source: 'detail',
    });
    expect(await screen.findByRole('link', { name: 'Freigabe ausstehend' })).toHaveAttribute(
      'href',
      '/wishes',
    );
    expect(await screen.findByRole('heading', { name: 'Dein Wunsch' })).toBeVisible();
    expect(screen.getByRole('button', { name: 'Zurückziehen' })).toBeVisible();
    expect(document.body.textContent).not.toContain('{{');
  });

  it('offers the seasons with their states and refuses more than the limit', async () => {
    const { calls } = mockApi({
      ...withWishes,
      'GET /titles/show/9100': () => json(series),
      'GET /titles/show/9100/seasons': () =>
        json({
          source: 'seerr',
          seasons: [
            { number: 1, episodeCount: 8, state: 'available' },
            { number: 2, episodeCount: 10, state: 'requested' },
            { number: 3, episodeCount: 10, state: 'missing' },
            { number: 4, episodeCount: 6, state: 'missing' },
          ],
        }),
      'GET /wishes/quota': () => json(series.request.quota),
      'POST /wishes': () => json(wishOf(series), 201),
    });
    renderApp('/titles/show/9100');
    const user = userEvent.setup();
    await user.click(await screen.findByRole('button', { name: 'Serie anfordern' }));
    const dialog = await screen.findByRole('dialog', { name: 'Serie anfordern: Nebelhafen' });
    expect(await within(dialog).findByRole('checkbox', { name: 'Staffel 1' })).toBeDisabled();
    expect(
      within(dialog).getByRole('checkbox', { name: 'Alle fehlenden Staffeln (3)' }),
    ).toBeChecked();
    // Two new seasons, one place left: refused with the date of the next free place.
    expect(
      within(dialog).getByText(/Noch 1 von 5 Staffeln frei. Wähle höchstens 1 Staffel./),
    ).toBeVisible();
    expect(within(dialog).getByRole('button', { name: '3 Staffeln anfordern' })).toBeDisabled();

    // Season 4 off: one new season and one co-wish fit.
    await user.click(within(dialog).getByRole('checkbox', { name: 'Staffel 4' }));
    const submit = within(dialog).getByRole('button', { name: '2 Staffeln anfordern' });
    expect(submit).toBeEnabled();
    await user.click(submit);
    await waitFor(() =>
      expect(calls.find((call) => call.method === 'POST')?.body).toEqual({
        media: 'show',
        tmdbId: 9100,
        source: 'detail',
        seasons: [2, 3],
      }),
    );
  });

  it('shows a refused limit from the server in the dialog and keeps it open', async () => {
    mockApi({
      ...withWishes,
      'GET /titles/movie/9001': () => json(film),
      'GET /wishes/quota': () => json(film.request.quota),
      'POST /wishes': () =>
        problem(409, 'quota_exceeded', {
          quota: {
            unlimited: false,
            movie: {
              limit: 10,
              days: 7,
              used: 10,
              queued: 0,
              remaining: 0,
              nextFreeAt: '2026-11-20T10:00:00Z',
            },
          },
          needed: 1,
        } as never),
    });
    renderApp('/titles/movie/9001');
    const user = userEvent.setup();
    await user.click(await screen.findByRole('button', { name: 'Film anfordern' }));
    const dialog = await screen.findByRole('dialog');
    await user.click(within(dialog).getByRole('button', { name: 'Anfordern' }));
    expect(
      await within(dialog).findByText(
        'Das Limit ist erreicht: 10 Filme in 7 Tagen. Der nächste Platz wird am 20.11.2026 frei.',
      ),
    ).toBeVisible();
    expect(within(dialog).getByRole('button', { name: 'Anfordern' })).toBeDisabled();
  });

  it('says why the server refused the limit when the stored one still fits (M1)', async () => {
    mockApi({
      ...withWishes,
      'GET /titles/movie/9001': () => json(film),
      'GET /wishes/quota': () => json(film.request.quota),
      'POST /wishes': () => problem(409, 'quota_exceeded', { needed: 1 } as never),
    });
    renderApp('/titles/movie/9001');
    const user = userEvent.setup();
    await user.click(await screen.findByRole('button', { name: 'Film anfordern' }));
    const dialog = await screen.findByRole('dialog');
    await user.click(within(dialog).getByRole('button', { name: 'Anfordern' }));
    expect(await within(dialog).findByText('Das Limit reicht dafür nicht.')).toBeVisible();
  });

  it('keeps the result of a request when the dialog closed on the way (m2)', async () => {
    let answer: (response: Response) => void = () => undefined;
    mockApi({
      ...withWishes,
      'GET /titles/movie/9001': () => json(film),
      'GET /wishes/quota': () => json(film.request.quota),
      'GET /wishes': () => json({ items: [], total: 0, counts }),
      'POST /wishes': () =>
        new Promise<Response>((resolve) => {
          answer = resolve;
        }),
    });
    renderApp('/titles/movie/9001');
    const user = userEvent.setup();
    await user.click(await screen.findByRole('button', { name: 'Film anfordern' }));
    const dialog = await screen.findByRole('dialog');
    await user.click(within(dialog).getByRole('button', { name: 'Anfordern' }));
    await user.keyboard('{Escape}');
    await waitFor(() => expect(dialog).not.toBeInTheDocument());
    answer(json(wishOf(film), 201));
    expect(
      await screen.findByText('Angefordert. Den Stand siehst du unter «Meine Wünsche».'),
    ).toBeVisible();
  });

  it('opens the library title on 409 already_available and says «nicht gefunden» on 404', async () => {
    mockApi({
      ...withWishes,
      'GET /titles/movie/9001': () => problem(409, 'already_available', { item: heat } as never),
      'GET /titles/movie/404': () => problem(404, 'not_found'),
      [`GET /items/${heat.id}`]: () => json({ ...heat, mediaFiles: [], genres: [] }),
    });
    const { router } = renderApp('/titles/movie/9001');
    await waitFor(() => expect(router.state.location.pathname).toBe(`/items/${heat.id}`));

    renderApp('/titles/movie/404');
    expect(await screen.findByText('Titel nicht gefunden')).toBeVisible();
  });

  it('names the reason when the profile cannot request and the last closed wish', async () => {
    mockApi({
      ...withWishes,
      'GET /titles/movie/9001': () =>
        json({
          ...film,
          canRequest: false,
          request: { canRequest: false, reason: 'profile_not_allowed', autoApprove: false },
          lastWish: { id: 7, state: 'declined', at: '2026-11-12T10:00:00Z' },
        }),
    });
    renderApp('/titles/movie/9001');
    expect(await screen.findByText('Anfordern ist für dieses Profil nicht möglich')).toBeVisible();
    expect(screen.queryByRole('button', { name: 'Film anfordern' })).not.toBeInTheDocument();
    expect(screen.getByText('Dein letzter Wunsch: Abgelehnt')).toBeVisible();
    expect(screen.getByText(/12\.11\.2026 · Seerr nennt keinen Grund/)).toBeVisible();
  });
  it('shows the download progress of the own wish on its card (E-7)', async () => {
    const readAt = '2026-11-17T09:00:00Z';
    mockApi({
      ...withWishes,
      'GET /titles/movie/9001': () =>
        json({
          ...film,
          wish: {
            id: 41,
            state: 'processing',
            progress: { stage: 'download', percent: 60, etaSeconds: 300, readAt },
          },
        }),
      'GET /wishes': () =>
        json({ items: [wishOf(film, { state: 'processing' })], total: 1, counts }),
    });
    renderApp('/titles/movie/9001');
    const card = await screen.findByRole('region', { name: 'Dein Wunsch' });
    expect(within(card).getByText('In Beschaffung')).toBeVisible();
    expect(within(card).getByText('Wird heruntergeladen · 60 % · noch 5 min')).toBeVisible();
    expect(within(card).getByRole('progressbar')).toHaveAttribute('aria-valuenow', '60');
  });
  it('joins a film another profile wished with «Benachrichtige mich»; only the state without the right (F-7)', async () => {
    let joined = false;
    const shared = { state: 'pending' as const, canJoin: true };
    const { calls } = mockApi({
      ...withWishes,
      'GET /titles/movie/9001': () =>
        json(
          joined
            ? { ...film, wish: { id: 41, state: 'pending' } }
            : { ...film, request: { ...film.request, shared } },
        ),
      'GET /wishes': () =>
        json({
          items: joined ? [wishOf(film, { role: 'joined' })] : [],
          total: joined ? 1 : 0,
          counts,
        }),
      'POST /wishes': () => {
        joined = true;
        return json(wishOf(film, { role: 'joined' }), 201);
      },
    });
    renderApp('/titles/movie/9001');
    expect(await screen.findByText('Bereits angefordert · Freigabe ausstehend')).toBeVisible();
    expect(screen.queryByRole('button', { name: 'Film anfordern' })).toBeNull();
    const user = userEvent.setup();
    await user.click(screen.getByRole('button', { name: 'Benachrichtige mich' }));
    await waitFor(() =>
      expect(calls.find((call) => call.method === 'POST')?.body).toEqual({
        media: 'movie',
        tmdbId: 9001,
        source: 'detail',
      }),
    );
    expect(await screen.findByRole('link', { name: 'Freigabe ausstehend' })).toBeVisible();
    expect(screen.getByText('Du wirst benachrichtigt, sobald der Titel da ist.')).toBeVisible();
    const card = await screen.findByRole('region', { name: 'Dein Wunsch' });
    expect(within(card).getByText(/du wünschst mit/)).toBeVisible();
    expect(document.body.textContent).not.toContain('{{');
  });

  it("shows only the state of another profile's wish when it may not join", async () => {
    mockApi({
      ...withWishes,
      'GET /titles/movie/9001': () =>
        json({
          ...film,
          canRequest: false,
          request: {
            ...film.request,
            canRequest: false,
            reason: 'status_only',
            shared: { state: 'processing', stateDetail: 'importing', canJoin: false },
          },
        }),
    });
    renderApp('/titles/movie/9001');
    expect(
      await screen.findByText('Bereits angefordert · In Beschaffung · wird eingelesen'),
    ).toBeVisible();
    expect(screen.getByText(/Sobald er abspielbar ist, findest du ihn/)).toBeVisible();
    expect(screen.queryByRole('button', { name: 'Benachrichtige mich' })).toBeNull();
    expect(screen.queryByRole('button', { name: 'Film anfordern' })).toBeNull();
  });
});
