import type { Trailer } from '@slouchcat/api-client';
import { act, fireEvent, screen, waitFor, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { afterEach, describe, expect, it, vi } from 'vitest';
import {
  alienVersions,
  episode2Detail,
  extraDetail,
  ids,
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
    const play = screen.getByRole('link', { name: 'Weiterschauen ab 00:42:13' });
    expect(play).toHaveAttribute('href', `/play/${ids.alien}?file=mf-1`);

    await user.click(sd);
    expect(sd).toHaveAttribute('aria-pressed', 'true');
    expect(play).toHaveAttribute('href', `/play/${ids.alien}?file=mf-2`);
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

  it('offline closes an open trailer dialog for good and moves the focus to "Zurück"', async () => {
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
    await waitFor(() => expect(screen.getByRole('button', { name: 'Zurück' })).toHaveFocus());

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

  it('shows "Verwalten" only for the main account with its primary profile', async () => {
    mockApi({ ...signedIn, [`GET /items/${ids.alien}`]: () => json(alienVersions) });
    const { unmount } = renderApp(`/items/${ids.alien}`);
    expect(await screen.findByRole('link', { name: 'Verwalten' })).toHaveAttribute(
      'href',
      `/admin/items/${ids.alien}`,
    );
    unmount();

    mockApi({
      'GET /auth/session': () =>
        json({ ...session, profile: { ...session.profile, id: 2, isPrimary: false } }),
      [`GET /items/${ids.alien}`]: () => json(alienVersions),
    });
    renderApp(`/items/${ids.alien}`);
    await screen.findByRole('heading', { level: 1, name: 'Alien' });
    expect(screen.queryByRole('link', { name: 'Verwalten' })).not.toBeInTheDocument();
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

  it('series: season chips, the episodes of the chosen season, extras and cast', async () => {
    const { calls } = mockApi(seriesRoutes);
    renderApp(`/items/${seriesIds.show}`);
    const user = userEvent.setup();

    await screen.findByRole('heading', { level: 1, name: 'Slouchcat Test Show' });
    // A series has no play button of its own.
    expect(screen.queryByRole('link', { name: /Abspielen/ })).not.toBeInTheDocument();
    const seasons = screen.getByRole('group', { name: 'Staffel wählen' });
    expect(within(seasons).getByRole('button', { name: 'Season 1' })).toHaveAttribute(
      'aria-pressed',
      'true',
    );

    const list = await screen.findByRole('list', { name: 'Folgen' });
    const pilot = within(list).getByRole('link', { name: /Pilot/ });
    expect(pilot).toHaveAttribute('href', `/items/${seriesIds.e1}`);
    expect(within(pilot).getByText('S01E01')).toBeVisible();
    expect(within(pilot).getByText('45 min')).toBeVisible();
    expect(within(pilot).getByRole('progressbar', { name: '22 % gesehen' })).toBeVisible();
    expect(within(list).getByText('S01E03–E04')).toBeVisible();

    await user.click(within(seasons).getByRole('button', { name: 'Specials' }));
    expect(await screen.findByText('S00E01')).toBeVisible();
    expect(calls.some((call) => call.path === `/items/${seriesIds.specials}`)).toBe(true);

    expect(screen.getByRole('region', { name: 'Extras' })).toBeVisible();
    const cast = screen.getByRole('region', { name: 'Besetzung' });
    expect(within(cast).getByText('als Kommissarin Weber')).toBeVisible();
    expect(within(cast).getByText('Idee')).toBeVisible();
    expect(document.title).toBe('Slouchcat Test Show · Slouchcat');
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

    const seasons = await screen.findByRole('group', { name: 'Staffel wählen' });
    await screen.findByRole('list', { name: 'Folgen' });
    await user.click(within(seasons).getByRole('button', { name: 'Specials' }));
    // The episodes of the previous season do not stay on screen while the next one loads.
    expect(screen.queryByRole('list', { name: 'Folgen' })).not.toBeInTheDocument();
    expect(document.querySelector('[aria-busy="true"]')).not.toBeNull();

    answer(problem(500, 'internal'));
    const retry = await screen.findByRole('button', { name: 'Erneut versuchen' });
    fail = false;
    await user.click(retry);
    expect(await screen.findByText('S00E01')).toBeVisible();
  });

  it('season: its episodes, a link to the series, the series in the tab title', async () => {
    mockApi(seriesRoutes);
    renderApp(`/items/${seriesIds.season1}`);
    await screen.findByRole('heading', { level: 1, name: 'Season 1' });
    expect(screen.getByRole('link', { name: /S01E02/ })).toHaveAttribute(
      'href',
      `/items/${seriesIds.e2}`,
    );
    expect(screen.getByRole('link', { name: 'Zur Serie' })).toHaveAttribute(
      'href',
      `/items/${seriesIds.show}`,
    );
    await waitFor(() => expect(document.title).toBe('Season 1 – Slouchcat Test Show · Slouchcat'));
  });

  it('episode: code and series, play, next episode and series links', async () => {
    mockApi({ ...signedIn, [`GET /items/${seriesIds.e2}`]: () => json(episode2Detail) });
    renderApp(`/items/${seriesIds.e2}`);
    await screen.findByRole('heading', { level: 1, name: 'Die lange Nacht' });
    expect(screen.getByText('S01E02 · Slouchcat Test Show')).toBeVisible();
    expect(screen.getByRole('link', { name: 'Abspielen' })).toHaveAttribute(
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
