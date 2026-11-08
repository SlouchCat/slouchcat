import type { Trailer } from '@slouchcat/api-client';
import { act, fireEvent, screen, waitFor, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { afterEach, describe, expect, it, vi } from 'vitest';
import {
  alienVersions,
  audioIds,
  audiobookDetail,
  episode2Detail,
  episodes,
  extraDetail,
  ids,
  podcastDetail,
  podcastEpisodeDetail,
  podcastEpisodes,
  season1Detail,
  seriesIds,
  showDetail,
  specialsDetail,
} from '@/test/fixtures';
import { json, mockApi, problem, renderApp, session, signedIn } from '@/test/api';

const trailers: Trailer[] = [
  {
    id: 1,
    lang: 'de',
    youtubeId: 'abcdefghijk',
    embedUrl: 'https://www.youtube-nocookie.com/embed/abcdefghijk',
    source: 'manual',
    official: false,
  },
  {
    id: 2,
    lang: 'en',
    youtubeId: 'zyxwvutsrqp',
    embedUrl: 'https://www.youtube-nocookie.com/embed/zyxwvutsrqp',
    source: 'tmdb',
    official: true,
  },
];

describe('detail page: film with versions, extras, cast and trailers', () => {
  afterEach(() => {
    vi.restoreAllMocks();
    vi.unstubAllGlobals();
  });

  it('offers the versions; the play link carries the chosen file', async () => {
    mockApi({ ...signedIn, [`GET /items/${ids.alien}`]: () => json(alienVersions) });
    renderApp(`/items/${ids.alien}`);
    const user = userEvent.setup();

    const versions = await screen.findByRole('group', { name: 'Versionen' });
    const hd = within(versions).getByRole('button', { name: '1080p' });
    const sd = within(versions).getByRole('button', { name: '720p' });
    // The primary version (first in the list) is chosen.
    expect(hd).toHaveAttribute('aria-pressed', 'true');
    const play = screen.getByRole('link', { name: /^Fortsetzen bei 0:42:13/ });
    expect(play).toHaveAttribute('href', `/play/${ids.alien}?file=mf-1`);
    const restart = screen.getByRole('link', { name: 'Von vorne' });
    expect(restart).toHaveAttribute('href', `/play/${ids.alien}?at=0&file=mf-1`);

    await user.click(sd);
    expect(sd).toHaveAttribute('aria-pressed', 'true');
    expect(play).toHaveAttribute('href', `/play/${ids.alien}?file=mf-2`);
    expect(restart).toHaveAttribute('href', `/play/${ids.alien}?at=0&file=mf-2`);
    const file = screen.getByRole('region', { name: 'Datei' });
    expect(within(file).getByText('1280 × 720 · 720p')).toBeVisible();
  });

  it('lists the extras with their kind (playable) and the cast with links to the person', async () => {
    mockApi({ ...signedIn, [`GET /items/${ids.alien}`]: () => json(alienVersions) });
    renderApp(`/items/${ids.alien}`);

    const extras = await screen.findByRole('region', { name: 'Extras' });
    const trailer = within(extras).getByRole('link', { name: /Alien Trailer/ });
    expect(trailer).toHaveAttribute('href', '/play/01JB8Z6Q4M7N9P2R3S5T7V9W5A');
    expect(within(trailer).getByText('Trailer')).toBeVisible();
    expect(within(extras).getByText('Hinter den Kulissen')).toBeVisible();

    const cast = screen.getByRole('region', { name: 'Besetzung' });
    const ripley = within(cast).getByRole('link', { name: /Sigourney Weaver/ });
    expect(ripley).toHaveAttribute('href', '/people/3');
    expect(within(ripley).getByText('als Ellen Ripley')).toBeVisible();
  });

  it('opens the trailer dialog: languages, youtube-nocookie frame, Escape closes', async () => {
    const { calls } = mockApi({
      ...signedIn,
      [`GET /items/${ids.alien}`]: () => json(alienVersions),
      [`GET /items/${ids.alien}/trailers`]: () => json(trailers),
    });
    const { router } = renderApp(`/items/${ids.alien}`);
    const user = userEvent.setup();

    const button = await screen.findByRole('button', { name: 'Trailer' });
    await user.click(button);
    const dialog = await screen.findByRole('dialog', { name: 'Trailer: Alien' });
    const frame = await within(dialog).findByTitle('Trailer von Alien (YouTube)');
    expect(frame.getAttribute('src')).toBe(
      'https://www.youtube-nocookie.com/embed/abcdefghijk?autoplay=1',
    );
    expect(frame).toHaveAttribute('allow', 'autoplay; encrypted-media; picture-in-picture');
    expect(frame).toHaveAttribute('referrerpolicy', 'strict-origin-when-cross-origin');
    expect(calls.some((call) => call.path === `/items/${ids.alien}/trailers`)).toBe(true);

    // Profile language first, the other one a radio away.
    expect(within(dialog).getByRole('radio', { name: 'Deutsch' })).toBeChecked();
    await user.click(within(dialog).getByRole('radio', { name: 'English' }));
    expect(within(dialog).getByTitle('Trailer von Alien (YouTube)').getAttribute('src')).toContain(
      'youtube-nocookie.com/embed/zyxwvutsrqp',
    );

    await user.keyboard('{Escape}');
    await waitFor(() => expect(screen.queryByRole('dialog')).not.toBeInTheDocument());
    // Escape closed the dialog only, the page stays.
    expect(router.state.location.pathname).toBe(`/items/${ids.alien}`);
    await waitFor(() => expect(button).toHaveFocus());
  });

  it('Backspace in the trailer dialog closes the dialog, not the page (C-4)', async () => {
    mockApi({
      ...signedIn,
      [`GET /items/${ids.alien}`]: () => json(alienVersions),
      [`GET /items/${ids.alien}/trailers`]: () => json(trailers),
    });
    const { router } = renderApp(`/items/${ids.alien}`);
    const user = userEvent.setup();

    const button = await screen.findByRole('button', { name: 'Trailer' });
    await user.click(button);
    let dialog = await screen.findByRole('dialog', { name: 'Trailer: Alien' });
    await within(dialog).findByTitle('Trailer von Alien (YouTube)');
    // On the dialog's own button: react-aria does not consume Backspace there.
    const close = within(dialog).getByRole('button', { name: 'Schliessen' });
    close.focus();
    fireEvent.keyDown(close, { key: 'Backspace' });
    await waitFor(() => expect(screen.queryByRole('dialog')).not.toBeInTheDocument());
    expect(router.state.location.pathname).toBe(`/items/${ids.alien}`);
    await waitFor(() => expect(button).toHaveFocus());

    // On the dialog element itself (where react-aria puts the focus after Enter).
    await user.click(button);
    dialog = await screen.findByRole('dialog', { name: 'Trailer: Alien' });
    fireEvent.keyDown(dialog, { key: 'Backspace' });
    await waitFor(() => expect(screen.queryByRole('dialog')).not.toBeInTheDocument());
    expect(router.state.location.pathname).toBe(`/items/${ids.alien}`);
  });

  it('offline closes an open trailer dialog for good and moves the focus to the big button', async () => {
    const online = vi.spyOn(navigator, 'onLine', 'get').mockReturnValue(true);
    mockApi({
      ...signedIn,
      [`GET /items/${ids.alien}`]: () => json(alienVersions),
      [`GET /items/${ids.alien}/trailers`]: () => json(trailers),
    });
    renderApp(`/items/${ids.alien}`);
    const user = userEvent.setup();

    await user.click(await screen.findByRole('button', { name: 'Trailer' }));
    await screen.findByRole('dialog', { name: 'Trailer: Alien' });

    online.mockReturnValue(false);
    act(() => {
      window.dispatchEvent(new Event('offline'));
    });
    expect(screen.queryByRole('dialog')).not.toBeInTheDocument();
    await waitFor(() =>
      expect(screen.getByRole('link', { name: /^Fortsetzen bei/ })).toHaveFocus(),
    );

    // Back online: the button returns, the dialog does not open (and play) by itself.
    online.mockReturnValue(true);
    act(() => {
      window.dispatchEvent(new Event('online'));
    });
    expect(screen.getByRole('button', { name: 'Trailer' })).toBeVisible();
    expect(screen.queryByRole('dialog')).not.toBeInTheDocument();
  });

  it('hides the trailer button while offline (W-10) and without trailers', async () => {
    const online = vi.spyOn(navigator, 'onLine', 'get').mockReturnValue(true);
    mockApi({ ...signedIn, [`GET /items/${ids.alien}`]: () => json(alienVersions) });
    renderApp(`/items/${ids.alien}`);
    expect(await screen.findByRole('button', { name: 'Trailer' })).toBeVisible();

    online.mockReturnValue(false);
    act(() => {
      window.dispatchEvent(new Event('offline'));
    });
    expect(screen.queryByRole('button', { name: 'Trailer' })).not.toBeInTheDocument();
    online.mockReturnValue(true);
    act(() => {
      window.dispatchEvent(new Event('online'));
    });
    expect(screen.getByRole('button', { name: 'Trailer' })).toBeVisible();
  });

  it('shows the pen only for the main account with its primary profile', async () => {
    mockApi({ ...signedIn, [`GET /items/${ids.alien}`]: () => json(alienVersions) });
    const { unmount } = renderApp(`/items/${ids.alien}`);
    expect(await screen.findByRole('button', { name: 'Metadaten bearbeiten' })).toBeVisible();
    // A film has the editor; the link to its administration page is in the editor's head.
    expect(screen.queryByRole('link', { name: 'Verwalten' })).not.toBeInTheDocument();
    unmount();

    mockApi({
      'GET /auth/session': () =>
        json({ ...session, profile: { ...session.profile, id: 2, isPrimary: false } }),
      [`GET /items/${ids.alien}`]: () => json(alienVersions),
    });
    renderApp(`/items/${ids.alien}`);
    await screen.findByRole('heading', { level: 1, name: 'Alien' });
    expect(screen.queryByRole('button', { name: 'Metadaten bearbeiten' })).not.toBeInTheDocument();
  });
});

describe('detail page: series, season, episode, extra', () => {
  afterEach(() => vi.unstubAllGlobals());

  const seriesRoutes = {
    ...signedIn,
    [`GET /items/${seriesIds.show}`]: () => json(showDetail),
    [`GET /items/${seriesIds.season1}`]: () => json(season1Detail),
    [`GET /items/${seriesIds.specials}`]: () => json(specialsDetail),
  };

  it('series: the big button by nextUp, the season shelf, the chosen season and its episodes', async () => {
    const { calls } = mockApi(seriesRoutes);
    renderApp(`/items/${seriesIds.show}`);
    const user = userEvent.setup();

    await screen.findByRole('heading', { level: 1, name: 'Slouchcat Test Show' });
    // The newest activity is the started pilot (concept 17 3.3): resume it, or start it over.
    expect(
      screen.getByRole('link', { name: 'Weiterschauen S01E01 · Pilot · noch 35 min' }),
    ).toHaveAttribute('href', `/play/${seriesIds.e1}`);
    expect(screen.getByRole('link', { name: 'Von vorne' })).toHaveAttribute(
      'href',
      `/play/${seriesIds.e1}?at=0`,
    );
    expect(screen.getByRole('button', { name: 'Favorit' })).toBeVisible();

    // The shelf (E-13): the season of the nextUp episode is chosen, with its watched episodes.
    const shelf = screen.getByRole('region', { name: 'Staffeln' });
    const season1 = within(shelf).getByRole('button', { name: /Season 1/ });
    expect(season1).toHaveAttribute('aria-current', 'true');
    expect(within(season1).getByText('2024 · 3 Folgen · 1 gesehen')).toBeVisible();
    expect(
      within(season1).getByRole('progressbar', { name: '1 von 3 Folgen gesehen' }),
    ).toBeVisible();
    const specials = within(shelf).getByRole('button', { name: /Specials/ });
    expect(specials).not.toHaveAttribute('aria-current');
    expect(specials).toHaveAttribute('tabindex', '-1');

    // The chosen season: its text and the episodes as a list (E-14).
    expect(await screen.findByText('Weber übernimmt ihren ersten Fall.')).toBeVisible();
    const list = screen.getByRole('list', { name: 'Folgen' });
    const pilot = within(list).getByRole('link', { name: /Pilot/ });
    expect(pilot).toHaveAttribute('href', `/items/${seriesIds.e1}`);
    expect(pilot).toHaveAccessibleDescription('Ein Anruf in der Nacht führt Weber an den Hafen.');
    expect(within(pilot).getByText('S01E01')).toBeVisible();
    expect(within(pilot).getByText('Weiterschauen')).toBeVisible();
    expect(within(pilot).getByText('45 min')).toBeVisible();
    expect(within(pilot).getByRole('progressbar', { name: '22 % gesehen' })).toBeVisible();
    const double = within(list).getByRole('link', { name: /S01E03–E04/ });
    expect(within(double).getByRole('img', { name: 'Gesehen' })).toBeVisible();
    expect(
      within(list).getByRole('link', { name: /Die lange Nacht/ }),
    ).not.toHaveAccessibleDescription();

    await user.click(specials);
    expect(specials).toHaveAttribute('aria-current', 'true');
    expect(await screen.findByText('S00E01')).toBeVisible();
    expect(calls.some((call) => call.path === `/items/${seriesIds.specials}`)).toBe(true);
    // The nextUp episode is not in this season: no episode is marked (m-3).
    const specialsList = screen.getByRole('list', { name: 'Folgen' });
    expect(within(specialsList).queryByText('Weiterschauen')).not.toBeInTheDocument();
    expect(within(specialsList).queryByText('Als Nächstes')).not.toBeInTheDocument();

    expect(screen.getByRole('region', { name: 'Extras' })).toBeVisible();
    const cast = screen.getByRole('region', { name: 'Besetzung' });
    expect(within(cast).getByText('als Kommissarin Weber')).toBeVisible();
    expect(within(cast).getByText('Idee')).toBeVisible();
    expect(document.title).toBe('Slouchcat Test Show · Slouchcat');
  });

  it.each([
    ['start', 'Abspielen S01E01 · Pilot', `/play/${seriesIds.e1}`],
    ['next', 'Nächste Folge S01E01 · Pilot', `/play/${seriesIds.e1}`],
    ['rewatch', 'Nochmals ansehen Alles gesehen · ab S01E01', `/play/${seriesIds.e1}?at=0`],
  ] as const)(
    'series: nextUp %s names its episode on the big button',
    async (reason, name, href) => {
      mockApi({
        ...seriesRoutes,
        [`GET /items/${seriesIds.show}`]: () =>
          json({
            ...showDetail,
            nextUp: { reason, episode: { ...episodes.e1, progress: undefined } },
          }),
      });
      renderApp(`/items/${seriesIds.show}`);
      expect(await screen.findByRole('link', { name })).toHaveAttribute('href', href);
      // Only resuming offers "Von vorne".
      expect(screen.queryByRole('link', { name: 'Von vorne' })).not.toBeInTheDocument();
    },
  );

  it('series: no big button without a visible episode', async () => {
    mockApi({
      ...seriesRoutes,
      [`GET /items/${seriesIds.show}`]: () => json({ ...showDetail, nextUp: undefined }),
    });
    renderApp(`/items/${seriesIds.show}`);
    await screen.findByRole('heading', { level: 1, name: 'Slouchcat Test Show' });
    expect(
      screen.queryByRole('link', { name: /^(Abspielen|Weiterschauen)/ }),
    ).not.toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Zurück' })).toBeVisible();
  });

  it('series: a season switch shows the loading state, then an error with a retry', async () => {
    let answer: (response: Response) => void = () => undefined;
    let fail = true;
    mockApi({
      ...seriesRoutes,
      [`GET /items/${seriesIds.specials}`]: () => {
        if (!fail) return json(specialsDetail);
        return new Promise<Response>((resolve) => {
          answer = resolve;
        });
      },
    });
    renderApp(`/items/${seriesIds.show}`);
    const user = userEvent.setup();

    const shelf = await screen.findByRole('region', { name: 'Staffeln' });
    await screen.findByRole('list', { name: 'Folgen' });
    await user.click(within(shelf).getByRole('button', { name: /Specials/ }));
    // The episodes of the previous season do not stay on screen while the next one loads.
    expect(screen.queryByRole('list', { name: 'Folgen' })).not.toBeInTheDocument();
    expect(document.querySelector('[aria-busy="true"]')).not.toBeNull();

    answer(problem(500, 'internal'));
    const retry = await screen.findByRole('button', { name: 'Erneut versuchen' });
    fail = false;
    await user.click(retry);
    expect(await screen.findByText('S00E01')).toBeVisible();
  });

  it('season: the series as a link over its title, facts, big button and its episodes', async () => {
    mockApi(seriesRoutes);
    renderApp(`/items/${seriesIds.season1}`);
    await screen.findByRole('heading', { level: 1, name: 'Season 1' });
    expect(screen.getByRole('link', { name: 'Slouchcat Test Show' })).toHaveAttribute(
      'href',
      `/items/${seriesIds.show}`,
    );
    expect(screen.getByText('2024 · 3 Folgen · 1 gesehen')).toBeVisible();
    expect(screen.getByText('Weber übernimmt ihren ersten Fall.')).toBeVisible();
    expect(
      screen.getByRole('link', { name: 'Weiterschauen S01E01 · Pilot · noch 35 min' }),
    ).toHaveAttribute('href', `/play/${seriesIds.e1}`);
    // No watchlist and no favourite for a season (E-15).
    expect(screen.queryByRole('button', { name: 'Merken' })).not.toBeInTheDocument();
    expect(screen.queryByRole('button', { name: 'Favorit' })).not.toBeInTheDocument();
    const list = screen.getByRole('list', { name: 'Folgen' });
    expect(within(list).getByRole('link', { name: /S01E02/ })).toHaveAttribute(
      'href',
      `/items/${seriesIds.e2}`,
    );
    expect(within(list).getByText('Weiterschauen')).toBeVisible();
    await waitFor(() => expect(document.title).toBe('Season 1 – Slouchcat Test Show · Slouchcat'));
  });

  it('episode: code and series, play, next episode and series links', async () => {
    mockApi({ ...signedIn, [`GET /items/${seriesIds.e2}`]: () => json(episode2Detail) });
    renderApp(`/items/${seriesIds.e2}`);
    await screen.findByRole('heading', { level: 1, name: 'Die lange Nacht' });
    expect(screen.getByText('S01E02 · Slouchcat Test Show')).toBeVisible();
    expect(screen.getByRole('link', { name: 'Abspielen 45 min' })).toHaveAttribute(
      'href',
      `/play/${seriesIds.e2}`,
    );
    expect(screen.getByRole('link', { name: /^Nächste Folge/ })).toHaveAttribute(
      'href',
      `/items/${seriesIds.e34}`,
    );
    expect(screen.getByText('S01E03–E04 · Episode 3')).toBeVisible();
    expect(screen.getByRole('link', { name: 'Zur Serie' })).toHaveAttribute(
      'href',
      `/items/${seriesIds.show}`,
    );
    await waitFor(() =>
      expect(document.title).toBe('S01E02 Die lange Nacht – Slouchcat Test Show · Slouchcat'),
    );
  });

  it('extra: play and a link to the main title', async () => {
    mockApi({ ...signedIn, [`GET /items/${seriesIds.extra}`]: () => json(extraDetail) });
    renderApp(`/items/${seriesIds.extra}`);
    await screen.findByRole('heading', { level: 1, name: 'Making Of' });
    expect(screen.getByText('Featurette')).toBeVisible();
    expect(screen.getByRole('link', { name: 'Abspielen' })).toHaveAttribute(
      'href',
      `/play/${seriesIds.extra}`,
    );
    expect(screen.getByRole('link', { name: 'Zum Haupttitel' })).toHaveAttribute(
      'href',
      `/items/${seriesIds.show}`,
    );
  });
});

describe('detail page: audiobook, podcast, podcast episode (M6)', () => {
  afterEach(() => vi.unstubAllGlobals());

  it('audiobook: series and volume, author, narrator, duration, resume and the chapters', async () => {
    mockApi({ ...signedIn, [`GET /items/${audioIds.book}`]: () => json(audiobookDetail) });
    renderApp(`/items/${audioIds.book}`);

    await screen.findByRole('heading', { level: 1, name: 'Slouchcat Test Audiobook M4B' });
    expect(screen.getByText('Slouchcat Test Audio Series · Band 1')).toBeVisible();
    expect(screen.getByText('von Slouchcat Test Author')).toBeVisible();
    expect(screen.getByText('Gelesen von Slouchcat Test Narrator')).toBeVisible();
    expect(screen.getByText('2024')).toBeVisible();
    expect(screen.getByText('1 min')).toBeVisible();
    expect(screen.queryByText('FSK 0')).not.toBeInTheDocument();
    // The book plays as a whole (no version), from where the profile stopped.
    expect(screen.getByRole('link', { name: /^Weiterhören bei 0:12 · noch / })).toHaveAttribute(
      'href',
      `/play/${audioIds.book}`,
    );
    expect(screen.getByRole('link', { name: 'Von vorne' })).toHaveAttribute(
      'href',
      `/play/${audioIds.book}?at=0`,
    );
    // A square cover, no file section (the files are the book's parts).
    expect(
      document.querySelector(`img[src*="/items/${audioIds.book}/images/poster"][height="400"]`),
    ).not.toBeNull();
    expect(screen.queryByRole('region', { name: 'Datei' })).not.toBeInTheDocument();

    // H-1: every chapter starts the player at its start.
    const chapters = screen.getByRole('list', { name: 'Kapitel' });
    expect(within(chapters).getAllByRole('link')).toHaveLength(12);
    expect(
      within(chapters).getByRole('link', { name: 'Kapitel 3 ab 0:10 abspielen' }),
    ).toHaveAttribute('href', `/play/${audioIds.book}?at=10000`);
    expect(
      within(chapters).getByRole('link', { name: 'Kapitel 12 ab 0:55 abspielen' }),
    ).toHaveAttribute('href', `/play/${audioIds.book}?at=55000`);
  });

  it('audiobook: the volume is a number of the UI language', async () => {
    mockApi({
      ...signedIn,
      [`GET /items/${audioIds.book}`]: () => json({ ...audiobookDetail, seriesIndex: 2.5 }),
    });
    renderApp(`/items/${audioIds.book}`);
    expect(await screen.findByText('Slouchcat Test Audio Series · Band 2,5')).toBeVisible();
  });

  it('podcast: episodes newest first with date, duration and progress; more on demand; no play', async () => {
    const { fetchMock } = mockApi({
      ...signedIn,
      [`GET /items/${audioIds.podcast}`]: () => json(podcastDetail),
      [`GET /items/${audioIds.podcast}/episodes`]: () =>
        json({ episodes: [podcastEpisodes.ep1], total: 3, offset: 2, limit: 50 }),
    });
    renderApp(`/items/${audioIds.podcast}`);
    const user = userEvent.setup();

    await screen.findByRole('heading', { level: 1, name: 'Slouchcat Test Podcast' });
    expect(screen.getByText('von Slouchcat Test Author')).toBeVisible();
    expect(screen.getByText('Ein Test-Podcast aus Sinustoenen.')).toBeVisible();
    // The container plays through its episodes.
    expect(
      screen.queryByRole('link', { name: /^(Abspielen|Weiterhören)/ }),
    ).not.toBeInTheDocument();

    const list = screen.getByRole('list', { name: 'Episoden' });
    const links = within(list)
      .getAllByRole('link', { name: /^Slouchcat Test Episode \d/ })
      .filter((link) => !link.getAttribute('aria-label'));
    expect(links.map((link) => link.textContent)).toEqual([
      expect.stringContaining('Slouchcat Test Episode 3'),
      expect.stringContaining('Slouchcat Test Episode 2'),
    ]);
    expect(links[0]).toHaveAttribute('href', `/items/${audioIds.ep3}`);
    expect(within(list).getByText('20.10.2025 · 12 s')).toBeVisible();
    expect(within(list).getByText('13.10.2025 · 10 s')).toBeVisible();
    expect(within(list).getByRole('progressbar', { name: '40 % gehört' })).toBeVisible();
    expect(
      within(list).getByRole('link', { name: 'Slouchcat Test Episode 2 abspielen' }),
    ).toHaveAttribute('href', `/play/${audioIds.ep2}`);

    await user.click(screen.getByRole('button', { name: 'Mehr laden' }));
    expect(await within(list).findByText('Slouchcat Test Episode 1')).toBeVisible();
    // Focus moves to the first episode of the page (the button goes away with the last page).
    await waitFor(() =>
      expect(
        within(list)
          .getAllByRole('link', { name: /^Slouchcat Test Episode 1/ })
          .find((link) => !link.getAttribute('aria-label')),
      ).toHaveFocus(),
    );
    expect(within(list).getByText('06.10.2025 · 8 s')).toBeVisible();
    const page = fetchMock.mock.calls
      .map(([request]) => new URL(request.url))
      .find((url) => url.pathname.endsWith('/episodes'));
    expect(page?.searchParams.get('offset')).toBe('2');
    expect(page?.searchParams.get('limit')).toBe('50');
    // All three are there: nothing more to load.
    await waitFor(() =>
      expect(screen.queryByRole('button', { name: 'Mehr laden' })).not.toBeInTheDocument(),
    );
  });

  it('podcast: a failed page keeps the button as a retry, focused while it loads', async () => {
    let fail = true;
    mockApi({
      ...signedIn,
      [`GET /items/${audioIds.podcast}`]: () => json(podcastDetail),
      [`GET /items/${audioIds.podcast}/episodes`]: () =>
        fail
          ? problem(500, 'internal')
          : json({ episodes: [podcastEpisodes.ep1], total: 3, offset: 2, limit: 50 }),
    });
    renderApp(`/items/${audioIds.podcast}`);
    const user = userEvent.setup();

    const more = await screen.findByRole('button', { name: 'Mehr laden' });
    await user.click(more);
    const retry = await screen.findByRole('button', { name: 'Erneut versuchen' });
    expect(retry).toHaveFocus();
    expect(retry).toBeEnabled();

    fail = false;
    await user.keyboard('{Enter}');
    const list = screen.getByRole('list', { name: 'Episoden' });
    expect(await within(list).findByText('Slouchcat Test Episode 1')).toBeVisible();
    await waitFor(() =>
      expect(
        screen.queryByRole('button', { name: /^(Mehr laden|Erneut versuchen)$/ }),
      ).not.toBeInTheDocument(),
    );
  });

  it('podcast episode: podcast, date, duration, description, play and a link to the podcast', async () => {
    mockApi({ ...signedIn, [`GET /items/${audioIds.ep2}`]: () => json(podcastEpisodeDetail) });
    renderApp(`/items/${audioIds.ep2}`);

    await screen.findByRole('heading', { level: 1, name: 'Slouchcat Test Episode 2' });
    expect(screen.getByText('Slouchcat Test Podcast')).toBeVisible();
    expect(screen.getByText('13.10.2025')).toBeVisible();
    expect(screen.getByText('10 s')).toBeVisible();
    expect(screen.getByText('Zweite Episode, zehn Sekunden.')).toBeVisible();
    // Without a file of its own the episode streams from its feed through the server.
    expect(screen.getByRole('link', { name: /^Weiterhören bei 0:04 · noch / })).toHaveAttribute(
      'href',
      `/play/${audioIds.ep2}`,
    );
    expect(screen.getByRole('link', { name: 'Zum Podcast' })).toHaveAttribute(
      'href',
      `/items/${audioIds.podcast}`,
    );
    await waitFor(() =>
      expect(document.title).toBe('Slouchcat Test Episode 2 – Slouchcat Test Podcast · Slouchcat'),
    );
  });
});
