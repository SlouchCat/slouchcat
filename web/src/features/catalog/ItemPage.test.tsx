import { screen, waitFor, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { afterEach, describe, expect, it, vi } from 'vitest';
import {
  alien,
  alienCollection,
  alienDetail,
  homePage,
  ids,
  season1Detail,
  seriesIds,
  showDetail,
} from '@/test/fixtures';
import { json, mockApi, problem, renderApp, session, signedIn } from '@/test/api';

describe('detail page', () => {
  afterEach(() => vi.unstubAllGlobals());

  it('shows the title facts and a resume button at the stored position', async () => {
    mockApi({ ...signedIn, [`GET /items/${ids.alien}`]: () => json(alienDetail) });
    renderApp(`/items/${ids.alien}`);

    expect(await screen.findByRole('heading', { level: 1, name: 'Alien' })).toBeVisible();
    // The title has an age rating (16); viewers do not see it (concept 17 F-10).
    expect(screen.queryByText('FSK 16')).not.toBeInTheDocument();
    expect(screen.getByText('Horror, Science-Fiction')).toBeVisible();
    expect(screen.getByText('Im Weltraum hört dich niemand schreien.')).toBeVisible();
    // The big button (concept 17 E-10): where the profile stopped and the time left.
    const play = screen.getByRole('link', { name: 'Fortsetzen bei 0:42:13 · noch 1 h 15 min' });
    expect(play).toHaveAttribute('href', `/play/${ids.alien}`);
    // While resuming, "Von vorne" starts at 0 (E-11).
    expect(screen.getByRole('link', { name: 'Von vorne' })).toHaveAttribute(
      'href',
      `/play/${ids.alien}?at=0`,
    );
    expect(screen.getByRole('button', { name: 'Zurück' })).toBeVisible();
    // Same original title as the title: not repeated.
    expect(screen.queryByText(/Originaltitel/)).not.toBeInTheDocument();

    const file = screen.getByRole('region', { name: 'Datei' });
    expect(within(file).getByText('MP4')).toBeVisible();
    expect(within(file).getByText('1920 × 1080 · 1080p')).toBeVisible();
    expect(within(file).getByText('H264')).toBeVisible();
    expect(within(file).getByText('4,2 GB')).toBeVisible();
  });

  it('offers "Abspielen" without progress and marks unavailable files', async () => {
    const [file] = alienDetail.mediaFiles;
    mockApi({
      ...signedIn,
      [`GET /items/${ids.alien}`]: () =>
        json({
          ...alienDetail,
          progress: undefined,
          originalTitle: 'Alien – Das unheimliche Wesen',
          mediaFiles: [{ ...file, available: false }],
        }),
    });
    renderApp(`/items/${ids.alien}`);
    // Without progress the second line is the runtime; without a playable file it is disabled.
    expect(await screen.findByRole('button', { name: 'Abspielen 1 h 57 min' })).toBeDisabled();
    expect(screen.queryByRole('link', { name: 'Von vorne' })).not.toBeInTheDocument();
    expect(screen.getByText('nicht verfügbar')).toBeVisible();
    expect(screen.getByText('Originaltitel: Alien – Das unheimliche Wesen')).toBeVisible();
  });

  it('offers "Nochmals ansehen" from the start for a finished title', async () => {
    mockApi({
      ...signedIn,
      [`GET /items/${ids.alien}`]: () =>
        json({
          ...alienDetail,
          progress: { ...alienDetail.progress!, completed: true, percent: 100 },
        }),
    });
    renderApp(`/items/${ids.alien}`);
    expect(
      await screen.findByRole('link', { name: 'Nochmals ansehen Gesehen am 18.10.2026' }),
    ).toHaveAttribute('href', `/play/${ids.alien}?at=0`);
    expect(screen.queryByRole('link', { name: 'Von vorne' })).not.toBeInTheDocument();
  });

  it('shows its collection right after the head: members in order, this title ringed', async () => {
    mockApi({
      ...signedIn,
      [`GET /items/${ids.alien}`]: () => json({ ...alienDetail, collections: [alienCollection] }),
    });
    renderApp(`/items/${ids.alien}`);
    const shelf = await screen.findByRole('region', { name: 'Weltraum-Reihe' });
    expect(within(shelf).getByText('3 Titel')).toBeVisible();
    expect(
      within(shelf).getByRole('link', { name: 'Alle zeigen: Weltraum-Reihe' }),
    ).toHaveAttribute('href', `/collections/${alienCollection.id}`);
    const list = within(shelf).getByRole('list', { name: 'Weltraum-Reihe' });
    const cards = within(list).getAllByRole('link');
    expect(cards.map((card) => card.getAttribute('href'))).toEqual([
      `/items/${ids.alien}`,
      `/items/${ids.heat}`,
      `/items/${ids.arrival}`,
    ]);
    expect(cards[0]).toHaveAttribute('aria-current', 'true');
    expect(cards[1]).not.toHaveAttribute('aria-current');
    expect(within(cards[0]!).getByRole('progressbar', { name: '36 % gesehen' })).toBeVisible();
    expect(within(cards[1]!).getByRole('img', { name: 'Gesehen' })).toBeVisible();
    expect(within(cards[2]!).queryByRole('img', { name: 'Gesehen' })).not.toBeInTheDocument();
    // Before the file section (E-9).
    const file = screen.getByRole('region', { name: 'Datei' });
    expect(shelf.compareDocumentPosition(file) & Node.DOCUMENT_POSITION_FOLLOWING).toBeTruthy();
  });

  it('checks a watched series in its collection shelf (contract 1.22.0)', async () => {
    const member = (seriesId: string, title: string, watched: boolean) => ({
      id: seriesId,
      kind: 'series' as const,
      title,
      inWatchlist: false,
      images: {},
      watched,
    });
    mockApi({
      ...signedIn,
      [`GET /items/${ids.alien}`]: () =>
        json({
          ...alienDetail,
          collections: [
            {
              ...alienCollection,
              members: [
                alien,
                member(seriesIds.show, 'Ganz gesehen', true),
                member(seriesIds.season1, 'Halb gesehen', false),
              ],
            },
          ],
        }),
    });
    renderApp(`/items/${ids.alien}`);
    const shelf = await screen.findByRole('region', { name: 'Weltraum-Reihe' });
    const list = within(shelf).getByRole('list', { name: 'Weltraum-Reihe' });
    const card = (name: RegExp) => within(list).getByRole('link', { name });
    expect(within(card(/Ganz gesehen/)).getByRole('img', { name: 'Gesehen' })).toBeVisible();
    expect(within(card(/Halb gesehen/)).queryByRole('img', { name: 'Gesehen' })).toBeNull();
  });

  it('tints backdrop and poster with the title accent and offers every image size', async () => {
    const image = (kind: string, size: string, width: number) => ({
      url: `/items/${ids.alien}/images/${kind}/${size}?v=x`,
      width,
      height: Math.round(width * (kind === 'poster' ? 1.5 : 0.5625)),
    });
    mockApi({
      ...signedIn,
      [`GET /items/${ids.alien}`]: () =>
        json({
          ...alienDetail,
          accent: '#c0392b',
          images: {
            poster: image('poster', 'w400', 400),
            posterLarge: image('poster', 'w800', 800),
            backdropSmall: image('backdrop', 'w780', 780),
            backdrop: image('backdrop', 'w1280', 1280),
            backdropLarge: image('backdrop', 'w1920', 1920),
          },
        }),
    });
    const { container } = renderApp(`/items/${ids.alien}`);
    await screen.findByRole('heading', { level: 1, name: 'Alien' });

    const base = `/api/v1/items/${ids.alien}/images`;
    const backdrop = container.querySelector('img[fetchpriority="high"]');
    expect(backdrop).toHaveAttribute(
      'srcset',
      `${base}/backdrop/w780?v=x 780w, ${base}/backdrop/w1280?v=x 1280w, ${base}/backdrop/w1920?v=x 1920w`,
    );
    const tinted = [...container.querySelectorAll<HTMLElement>('[style]')].filter(
      (element) => element.style.getPropertyValue('--sc-item-accent') === '#c0392b',
    );
    // The backdrop (tint over the scrim) and the poster column (glow).
    expect(tinted).toHaveLength(2);
    const poster = container.querySelector(`img[src="${base}/poster/w400?v=x"]`);
    expect(poster).toHaveAttribute(
      'srcset',
      `${base}/poster/w400?v=x 400w, ${base}/poster/w800?v=x 800w`,
    );
  });

  it('shows a not-found message on 404', async () => {
    mockApi({ ...signedIn, 'GET /items/*': () => problem(404, 'not_found') });
    renderApp(`/items/${ids.heat}`);
    expect(await screen.findByText('Titel nicht gefunden')).toBeVisible();
    expect(screen.getByRole('link', { name: 'Zur Startseite' })).toHaveAttribute('href', '/');
  });

  it('goes back to the start page with Escape when opened directly', async () => {
    mockApi({
      ...signedIn,
      [`GET /items/${ids.alien}`]: () => json(alienDetail),
      'GET /home': () => json(homePage),
    });
    const { router } = renderApp(`/items/${ids.alien}`);
    await screen.findByRole('heading', { level: 1, name: 'Alien' });
    await userEvent.setup().keyboard('{Escape}');
    await waitFor(() => expect(router.state.location.pathname).toBe('/'));
  });

  it('adds a title to the watchlist and takes it off again', async () => {
    let inWatchlist = false;
    const { calls } = mockApi({
      ...signedIn,
      [`GET /items/${ids.alien}`]: () => json({ ...alienDetail, inWatchlist }),
      [`PUT /items/${ids.alien}/watchlist`]: () => {
        inWatchlist = true;
        return new Response(null, { status: 204 });
      },
      [`DELETE /items/${ids.alien}/watchlist`]: () => {
        inWatchlist = false;
        return new Response(null, { status: 204 });
      },
    });
    renderApp(`/items/${ids.alien}`);
    const user = userEvent.setup();
    await user.click(await screen.findByRole('button', { name: 'Merken' }));
    await user.click(await screen.findByRole('button', { name: 'Gemerkt' }));
    expect(await screen.findByRole('button', { name: 'Merken' })).toBeVisible();
    expect(
      calls
        .filter((call) => call.path === `/items/${ids.alien}/watchlist`)
        .map((call) => call.method),
    ).toEqual(['PUT', 'DELETE']);
    // A film is no favourite: only series and podcasts (E-8).
    expect(screen.queryByRole('button', { name: 'Favorit' })).not.toBeInTheDocument();
  });

  it('marks a series as favourite and shows its collections', async () => {
    let favorite = false;
    const collectionId = '01JB8Z6Q4M7N9P2R3S5T7V9C01';
    mockApi({
      ...signedIn,
      [`GET /items/${seriesIds.show}`]: () =>
        json({
          ...showDetail,
          favorite,
          collections: [{ id: collectionId, name: 'Test-Sammlung', count: 2 }],
        }),
      [`GET /items/${seriesIds.season1}`]: () => json(season1Detail),
      [`PUT /items/${seriesIds.show}/favorite`]: () => {
        favorite = true;
        return new Response(null, { status: 204 });
      },
    });
    renderApp(`/items/${seriesIds.show}`);
    const button = await screen.findByRole('button', { name: 'Favorit' });
    expect(button).toHaveAttribute('aria-pressed', 'false');
    await userEvent.setup().click(button);
    await waitFor(() =>
      expect(screen.getByRole('button', { name: 'Favorit' })).toHaveAttribute(
        'aria-pressed',
        'true',
      ),
    );
    // Without members (an older server) the shelf still names the collection and links it.
    const collection = screen.getByRole('region', { name: 'Test-Sammlung' });
    expect(within(collection).getByText('2 Titel')).toBeVisible();
    expect(
      within(collection).getByRole('link', { name: 'Alle zeigen: Test-Sammlung' }),
    ).toHaveAttribute('href', `/collections/${collectionId}`);
  });

  it('links the owner to "Zu den User-Empfehlungen", never other profiles', async () => {
    mockApi({ ...signedIn, [`GET /items/${ids.alien}`]: () => json(alienDetail) });
    const view = renderApp(`/items/${ids.alien}`);
    expect(await screen.findByRole('link', { name: 'Zu den User-Empfehlungen' })).toHaveAttribute(
      'href',
      `/admin/picks?add=${ids.alien}`,
    );
    view.unmount();
    vi.unstubAllGlobals();

    mockApi({
      'GET /auth/session': () =>
        json({ ...session, profile: { ...session.profile, isPrimary: false } }),
      [`GET /items/${ids.alien}`]: () => json(alienDetail),
    });
    renderApp(`/items/${ids.alien}`);
    await screen.findByRole('heading', { level: 1, name: 'Alien' });
    expect(
      screen.queryByRole('link', { name: 'Zu den User-Empfehlungen' }),
    ).not.toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Merken' })).toBeVisible();
  });

  it('offers no watchlist button on a season', async () => {
    mockApi({ ...signedIn, [`GET /items/${seriesIds.season1}`]: () => json(season1Detail) });
    renderApp(`/items/${seriesIds.season1}`);
    await screen.findByRole('heading', { level: 1 });
    expect(screen.queryByRole('button', { name: 'Merken' })).not.toBeInTheDocument();
  });
});
