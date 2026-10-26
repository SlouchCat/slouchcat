import type { ItemRef, Trailer } from '@slouchcat/api-client';
import { act, fireEvent, screen, waitFor, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { alien, arrival, heat, homePage, ids } from '@/test/fixtures';
import { json, mockApi, renderApp, signedIn } from '@/test/api';

describe('start page', () => {
  afterEach(() => {
    vi.useRealTimers();
    vi.unstubAllGlobals();
  });

  it('shows the stage the server names, then "Weiterschauen" and "Neu hinzugefügt" in server order', async () => {
    mockApi({ ...signedIn, 'GET /home': () => json(homePage) });
    renderApp('/');

    const hero = await screen.findByRole('region', { name: 'Spotlight' });
    expect(hero).toHaveAttribute('aria-roledescription', 'Karussell');
    expect(within(hero).getByRole('heading', { name: 'Heat' })).toBeVisible();
    expect(within(hero).getByRole('link', { name: 'Abspielen' })).toHaveAttribute(
      'href',
      `/play/${ids.heat}`,
    );
    expect(within(hero).getByRole('link', { name: 'Details' })).toHaveAttribute(
      'href',
      `/items/${ids.heat}`,
    );
    expect(within(hero).getByText('FSK 16')).toBeVisible();

    const headings = screen
      .getAllByRole('heading', { level: 2 })
      .map((heading) => heading.textContent)
      .filter((text) => text === 'Weiterschauen' || text === 'Neu hinzugefügt');
    expect(headings).toEqual(['Weiterschauen', 'Neu hinzugefügt']);
  });

  it('opens a title through the router; modified clicks keep the browser default', async () => {
    mockApi({ ...signedIn, 'GET /home': () => json(homePage) });
    const { router } = renderApp('/');
    const recentRow = await screen.findByRole('region', { name: 'Neu hinzugefügt' });
    const card = within(recentRow).getByRole('link', { name: /Heat/ });
    expect(card).toHaveAttribute('href', `/items/${ids.heat}`);

    // Ctrl+click (new tab) is left to the browser: not prevented, no app navigation.
    expect(fireEvent.click(card, { ctrlKey: true })).toBe(true);
    expect(router.state.location.pathname).toBe('/');

    expect(fireEvent.click(card)).toBe(false);
    await waitFor(() => expect(router.state.location.pathname).toBe(`/items/${ids.heat}`));
  });

  it('"/" opens the search, unless a widget already took the key', async () => {
    mockApi({ ...signedIn, 'GET /home': () => json(homePage) });
    const { router } = renderApp('/');
    await screen.findByRole('region', { name: 'Neu hinzugefügt' });
    const slash = () =>
      document.body.dispatchEvent(
        new KeyboardEvent('keydown', { key: '/', bubbles: true, cancelable: true }),
      );

    const takeKey = (event: KeyboardEvent) => event.preventDefault();
    document.body.addEventListener('keydown', takeKey);
    slash();
    document.body.removeEventListener('keydown', takeKey);
    await new Promise((resolve) => setTimeout(resolve, 50));
    expect(router.state.location.pathname).toBe('/');

    slash();
    await waitFor(() => expect(router.state.location.pathname).toBe('/search'));
  });

  it('prefixes image URLs with the API base and loads only the first row eagerly', async () => {
    mockApi({ ...signedIn, 'GET /home': () => json(homePage) });
    const { container } = renderApp('/');
    await screen.findByRole('heading', { name: 'Weiterschauen' });

    const backdrop = container.querySelector('img[fetchpriority="high"]');
    expect(backdrop).toHaveAttribute('src', `/api/v1/items/${ids.heat}/images/backdrop/w780?v=ghi`);

    // Posters wait until the hero artwork is on screen; their frames are already there.
    const continueRow = screen.getByRole('region', { name: 'Weiterschauen' });
    const alienCard = within(continueRow).getByRole('link', { name: /Alien/ });
    expect(container.querySelectorAll('img')).toHaveLength(1);
    fireEvent.load(backdrop!);
    await waitFor(() => expect(alienCard.querySelector('img')).not.toBeNull());
    const poster = alienCard.querySelector('img');
    expect(poster).toHaveAttribute('src', `/api/v1/items/${ids.alien}/images/poster/w500?v=abc`);
    expect(poster).toHaveAttribute('loading', 'eager');

    const recentRow = screen.getByRole('region', { name: 'Neu hinzugefügt' });
    const heatPoster = within(recentRow).getByRole('link', { name: /Heat/ }).querySelector('img');
    expect(heatPoster).toHaveAttribute('loading', 'lazy');
    // Arrival has no poster: the lettered fallback tile, no broken image.
    expect(
      within(recentRow)
        .getByRole('link', { name: /Arrival/ })
        .querySelector('img'),
    ).toBeNull();
  });

  it('offers the w200 and w400 posters and hands the title accent to the card', async () => {
    const poster = (size: string) => ({
      url: `/items/${ids.heat}/images/poster/${size}?v=abc`,
      width: size === 'w200' ? 200 : 400,
      height: size === 'w200' ? 300 : 600,
    });
    const tinted: ItemRef = {
      ...heat,
      accent: '#3a7bd5',
      images: { poster: poster('w400'), posterSmall: poster('w200'), posterLarge: poster('w800') },
    };
    mockApi({
      ...signedIn,
      'GET /home': () => json({ rows: [{ key: 'recent', items: [tinted, arrival] }] }),
    });
    renderApp('/');
    const row = await screen.findByRole('region', { name: 'Neu hinzugefügt' });

    const card = within(row).getByRole('link', { name: /Heat/ });
    expect(card.style.getPropertyValue('--sc-item-accent')).toBe('#3a7bd5');
    const base = `/api/v1/items/${ids.heat}/images/poster`;
    const image = card.querySelector('img');
    expect(image).toHaveAttribute('src', `${base}/w400?v=abc`);
    // w800 stays out of the cards: a 3x phone would pick it and the start page would get heavier.
    expect(image).toHaveAttribute('srcset', `${base}/w200?v=abc 200w, ${base}/w400?v=abc 400w`);
    expect(image).toHaveAttribute('sizes');

    // No accent from the server: no property, the CSS falls back to the tokens.
    const plain = within(row).getByRole('link', { name: /Arrival/ });
    expect(plain.style.getPropertyValue('--sc-item-accent')).toBe('');
  });

  it('asks for the session and the start page in parallel, never for /setup', async () => {
    const { calls } = mockApi({ ...signedIn, 'GET /home': () => json(homePage) });
    renderApp('/');
    await screen.findByRole('heading', { name: 'Weiterschauen' });
    const paths = calls.map((call) => call.path);
    // The start page is requested before the session answered, i.e. not after the guard.
    expect(paths.slice(0, 2)).toEqual(['/home', '/auth/session']);
    expect(paths.filter((path) => path === '/home')).toHaveLength(1);
    expect(paths).not.toContain('/setup');
  });

  it('loads the active backdrop first, in w780 for small screens, and the next one after it', async () => {
    const wideOnly: ItemRef = {
      ...arrival,
      images: {
        backdrop: {
          url: `/items/${ids.arrival}/images/backdrop/w1280?v=jkl`,
          width: 1280,
          height: 720,
        },
      },
    };
    mockApi({
      ...signedIn,
      'GET /home': () =>
        json({
          heroRow: 'spotlight',
          rows: [{ key: 'spotlight', items: [heat, alien, wideOnly] }],
        }),
    });
    const { container } = renderApp('/');
    const hero = await screen.findByRole('region', { name: 'Spotlight' });
    // The band's artwork; the poster strip's images are counted apart.
    const images = () => [...container.querySelectorAll('.art img')];
    const strip = () => [...within(hero).getByRole('list').querySelectorAll('img')];

    expect(images()).toHaveLength(1);
    expect(strip()).toHaveLength(0);
    const [first] = images();
    const base = `/api/v1/items/${ids.heat}/images/backdrop`;
    expect(first).toHaveAttribute('src', `${base}/w780?v=ghi`);
    expect(first).toHaveAttribute('srcset', `${base}/w780?v=ghi 780w, ${base}/w1280?v=def 1280w`);
    expect(first).toHaveAttribute('sizes', '100vw');
    expect(first).toHaveAttribute('loading', 'eager');
    expect(first).toHaveAttribute('fetchpriority', 'high');
    expect(first).toHaveAttribute('width', '1280');
    expect(first).toHaveAttribute('height', '720');

    // Once the first image is painted (two frames after load), the next slide joins with low
    // priority; never before, as the server may have preloaded the first one already.
    fireEvent.load(first!);
    expect(images()).toHaveLength(1);
    await waitFor(() => expect(images()).toHaveLength(2));
    expect(images()[1]).toHaveAttribute(
      'src',
      `/api/v1/items/${ids.alien}/images/backdrop/w780?v=ghi`,
    );
    expect(images()[1]).toHaveAttribute('loading', 'lazy');
    expect(images()[1]).toHaveAttribute('fetchpriority', 'low');
    // The strip's posters join with it (P-10); a slide without a poster keeps the empty frame.
    expect(strip().map((image) => image.getAttribute('src'))).toEqual([
      `/api/v1/items/${ids.heat}/images/poster/w500?v=abc`,
      `/api/v1/items/${ids.alien}/images/poster/w500?v=abc`,
    ]);
    for (const image of strip()) {
      expect(image).toHaveAttribute('loading', 'lazy');
      expect(image).not.toHaveAttribute('fetchpriority');
    }

    // A slide without the small size falls back to w1280 without srcset.
    fireEvent.click(within(hero).getByRole('button', { name: 'Arrival' }));
    const third = images().find((image) => image.getAttribute('src')?.includes(ids.arrival));
    expect(third).toHaveAttribute(
      'src',
      `/api/v1/items/${ids.arrival}/images/backdrop/w1280?v=jkl`,
    );
    expect(third).not.toHaveAttribute('srcset');
    expect(third).toHaveAttribute('fetchpriority', 'high');
  });

  it('loads the first six posters of the first row eagerly and the rest lazily', async () => {
    const items = Array.from({ length: 8 }, (_, index) => ({
      ...alien,
      id: `${ids.alien}${index}`,
    }));
    mockApi({ ...signedIn, 'GET /home': () => json({ rows: [{ key: 'recent', items }] }) });
    const { container } = renderApp('/');
    await screen.findByRole('heading', { name: 'Neu hinzugefügt' });
    const loading = [...container.querySelectorAll('img')].map((image) =>
      image.getAttribute('loading'),
    );
    expect(loading).toEqual([...Array<string>(6).fill('eager'), 'lazy', 'lazy']);
    for (const image of container.querySelectorAll('img')) {
      expect(image).toHaveAttribute('decoding', 'async');
    }
  });

  it('fetches the first card with high priority when the page has no stage', async () => {
    mockApi({
      ...signedIn,
      'GET /home': () =>
        json({
          rows: [
            { key: 'recent', items: [alien, heat] },
            { key: 'watchlist', items: [heat] },
          ],
        }),
    });
    const { container } = renderApp('/');
    await screen.findByRole('heading', { name: 'Neu hinzugefügt' });
    // The first card's poster is the page's first image, preloaded by the server too (P-10).
    const priorities = [...container.querySelectorAll('img')].map((image) =>
      image.getAttribute('fetchpriority'),
    );
    expect(priorities).toEqual(['high', null, null]);
  });

  describe('the poster prepaint (P-10)', () => {
    afterEach(() => document.getElementById('poster-prepaint')?.remove());

    /** The first cards the server paints into the shell when there is no stage (api/preload.go). */
    function prepaint() {
      const element = document.createElement('div');
      element.id = 'poster-prepaint';
      document.body.append(element);
      return element;
    }

    it('without a stage gives way only once the first card shows its poster', async () => {
      const painted = prepaint();
      mockApi({
        ...signedIn,
        'GET /home': () => json({ rows: [{ key: 'recent', items: [alien, heat] }] }),
      });
      const { container } = renderApp('/');
      await screen.findByRole('heading', { name: 'Neu hinzugefügt' });
      const [first, second] = container.querySelectorAll('img');
      expect(first).toHaveAttribute('fetchpriority', 'high');

      // Another card's poster does not count; the first card's does, once it is painted.
      fireEvent.load(second!);
      await act(() => new Promise((resolve) => requestAnimationFrame(() => resolve(undefined))));
      expect(painted.dataset.done).toBeUndefined();
      fireEvent.load(first!);
      await waitFor(() => expect(painted.dataset.done).toBe(''));
    });

    it('gives way when the poster of the first card fails', async () => {
      const painted = prepaint();
      mockApi({
        ...signedIn,
        'GET /home': () => json({ rows: [{ key: 'recent', items: [alien] }] }),
      });
      const { container } = renderApp('/');
      await screen.findByRole('heading', { name: 'Neu hinzugefügt' });
      expect(painted.dataset.done).toBeUndefined();
      fireEvent.error(container.querySelector('img')!);
      await waitFor(() => expect(painted.dataset.done).toBe(''));
    });

    it.each([
      { first: 'a stage on top', home: homePage },
      {
        first: 'a stage further down',
        home: {
          heroRow: 'spotlight',
          rows: [
            { key: 'recent', items: [alien] },
            { key: 'spotlight', items: [heat] },
          ],
        },
      },
      {
        first: 'a title without poster',
        home: { rows: [{ key: 'recent', items: [{ ...alien, images: {} }] }] },
      },
      {
        first: 'genres',
        home: {
          rows: [
            { key: 'genres', items: [], genres: [{ name: 'Drama', count: 2 }] },
            { key: 'recent', items: [alien] },
          ],
        },
      },
    ])('gives way at once with $first', async ({ home }) => {
      const painted = prepaint();
      mockApi({ ...signedIn, 'GET /home': () => json(home) });
      renderApp('/');
      await screen.findByRole('region', { name: 'Neu hinzugefügt' });
      await waitFor(() => expect(painted.dataset.done).toBe(''));
    });
  });

  it('fetches no card with high priority when the page has a stage', async () => {
    mockApi({ ...signedIn, 'GET /home': () => json(homePage) });
    const { container } = renderApp('/');
    await screen.findByRole('region', { name: 'Spotlight' });
    fireEvent.load(container.querySelector('.art img')!);
    // The band goes first: once its artwork is shown the cards join, none of them with priority.
    const cards = () =>
      ['Weiterschauen', 'Neu hinzugefügt'].flatMap((name) => [
        ...screen.getByRole('region', { name }).querySelectorAll('img'),
      ]);
    await waitFor(() => expect(cards()).toHaveLength(2));
    for (const image of cards()) expect(image).not.toHaveAttribute('fetchpriority');
  });

  it('shows watch progress and the remaining time on "Weiterschauen" cards', async () => {
    mockApi({ ...signedIn, 'GET /home': () => json(homePage) });
    renderApp('/');
    const row = await screen.findByRole('region', { name: 'Weiterschauen' });
    expect(within(row).getByRole('progressbar', { name: '36 % gesehen' })).toHaveAttribute(
      'aria-valuenow',
      '36',
    );
    expect(within(row).getByText('Noch 1 h 15 min')).toBeVisible();
  });

  it('never renders an empty row and works without a stage', async () => {
    mockApi({
      ...signedIn,
      'GET /home': () =>
        json({
          rows: [
            { key: 'continue', items: [] },
            { key: 'recent', items: [heat, alien] },
          ],
        }),
    });
    renderApp('/');
    expect(await screen.findByRole('heading', { name: 'Neu hinzugefügt' })).toBeVisible();
    expect(screen.queryByRole('heading', { name: 'Weiterschauen' })).not.toBeInTheDocument();
    expect(document.querySelector('[aria-roledescription="Karussell"]')).toBeNull();
  });

  it('shows an empty state when the server returns no rows', async () => {
    mockApi({ ...signedIn, 'GET /home': () => json({ rows: [] }) });
    renderApp('/');
    expect(await screen.findByRole('heading', { name: 'Hier ist es noch leer' })).toBeVisible();
    expect(screen.getByRole('link', { name: 'Bibliotheken ansehen' })).toHaveAttribute(
      'href',
      '/admin/libraries',
    );
  });

  it('rotates the carousel every 8 s', async () => {
    vi.useFakeTimers({ shouldAdvanceTime: true });
    mockApi({
      ...signedIn,
      'GET /home': () =>
        json({ heroRow: 'spotlight', rows: [{ key: 'spotlight', items: [heat, arrival] }] }),
    });
    renderApp('/');
    const hero = await screen.findByRole('region', { name: 'Spotlight' });
    expect(within(hero).getByRole('heading', { name: 'Heat' })).toBeVisible();
    await act(() => vi.advanceTimersByTimeAsync(8000));
    expect(within(hero).getByRole('heading', { name: 'Arrival' })).toBeVisible();
    // The strip marks the active slide's poster, named after its title.
    expect(within(hero).getByRole('button', { name: 'Arrival' })).toHaveAttribute(
      'aria-current',
      'true',
    );
    expect(within(hero).getByRole('button', { name: 'Heat' })).not.toHaveAttribute('aria-current');
  });

  it('shows the header with profile name and the admin link for the owner', async () => {
    mockApi({ ...signedIn, 'GET /home': () => json(homePage) });
    renderApp('/');
    const nav = await screen.findByRole('navigation', { name: 'Hauptnavigation' });
    expect(within(nav).getByRole('link', { name: 'Start' })).toHaveAttribute(
      'aria-current',
      'page',
    );
    // M8: the administration is one entry; its pages have their own navigation.
    expect(within(nav).getByRole('link', { name: 'Verwaltung' })).toHaveAttribute('href', '/admin');
    expect(within(nav).queryByRole('link', { name: 'Bibliotheken' })).not.toBeInTheDocument();
    expect(screen.getByText('Marco')).toBeVisible();
    expect(screen.getByRole('button', { name: 'Abmelden' })).toBeVisible();
  });

  it('renders every row kind in server order: titles, collections and genre chips (E-1, E-6)', async () => {
    // Without `heroRow` there is no stage: every row is a poster row, the picks and spotlight too.
    const collectionId = '01JB8Z6Q4M7N9P2R3S5T7V9C01';
    mockApi({
      ...signedIn,
      'GET /home': () =>
        json({
          rows: [
            { key: 'picks', items: [{ ...heat, note: 'Unbedingt im Kino-Modus schauen.' }] },
            { key: 'continue', items: [alien] },
            { key: 'recent', items: [arrival] },
            { key: 'watchlist', items: [heat] },
            {
              key: 'collections',
              items: [],
              collections: [{ id: collectionId, name: 'Alien-Reihe', count: 3, cover: alien }],
            },
            { key: 'genres', items: [], genres: [{ name: 'Science-Fiction', count: 12 }] },
            { key: 'spotlight', items: [arrival] },
            { key: 'released', items: [alien] },
          ],
        }),
    });
    renderApp('/');
    const picks = await screen.findByRole('region', { name: "Cat's Empfehlungen" });
    expect(within(picks).getByRole('link', { name: /Heat/ })).toHaveTextContent(
      'Unbedingt im Kino-Modus schauen.',
    );

    const headings = screen
      .getAllByRole('heading', { level: 2 })
      .map((heading) => heading.textContent);
    expect(headings).toEqual([
      "Cat's Empfehlungen",
      'Weiterschauen',
      'Neu hinzugefügt',
      'Merkliste',
      'Sammlungen',
      'Genres',
      'Spotlight',
      'Kürzlich veröffentlicht',
    ]);

    const collections = screen.getByRole('region', { name: 'Sammlungen' });
    const card = within(collections).getByRole('link', { name: /Alien-Reihe/ });
    expect(card).toHaveAttribute('href', `/collections/${collectionId}`);
    expect(within(card).getByText('3 Titel')).toBeVisible();

    const genres = screen.getByRole('region', { name: 'Genres' });
    expect(within(genres).getByRole('link', { name: /Science-Fiction/ })).toHaveAttribute(
      'href',
      '/search?genre=Science-Fiction',
    );
    expect(
      within(screen.getByRole('region', { name: 'Spotlight' })).getByRole('link', {
        name: /Arrival/,
      }),
    ).toHaveAttribute('href', `/items/${ids.arrival}`);
  });

  it('drops a collections or genres row without entries', async () => {
    mockApi({
      ...signedIn,
      'GET /home': () =>
        json({
          rows: [
            { key: 'collections', items: [], collections: [] },
            { key: 'genres', items: [] },
            { key: 'recent', items: [heat] },
          ],
        }),
    });
    renderApp('/');
    expect(await screen.findByRole('heading', { name: 'Neu hinzugefügt' })).toBeVisible();
    expect(screen.queryByRole('heading', { name: 'Sammlungen' })).not.toBeInTheDocument();
    expect(screen.queryByRole('heading', { name: 'Genres' })).not.toBeInTheDocument();
  });

  it('opens the search with "/" and links search and watchlist in the header', async () => {
    mockApi({ ...signedIn, 'GET /home': () => json(homePage), 'GET /genres': () => json([]) });
    const { router } = renderApp('/');
    const nav = await screen.findByRole('navigation', { name: 'Hauptnavigation' });
    expect(within(nav).getByRole('link', { name: 'Suche' })).toHaveAttribute('href', '/search');
    expect(within(nav).getByRole('link', { name: 'Merkliste' })).toHaveAttribute(
      'href',
      '/watchlist',
    );
    await screen.findByRole('heading', { name: 'Weiterschauen' });
    fireEvent.keyDown(document.body, { key: '/' });
    await waitFor(() => expect(router.state.location.pathname).toBe('/search'));
    await waitFor(() =>
      expect(screen.getByRole('searchbox', { name: 'Suchbegriff' })).toHaveFocus(),
    );
  });

  describe('the stage (heroRow, M11)', () => {
    const note = 'Unbedingt im Kino-Modus schauen.';
    const carousels = () => document.querySelectorAll('[aria-roledescription="Karussell"]');
    /** The sections of the rows block, top to bottom: the stage and the poster rows. */
    const sections = () => [...document.querySelectorAll('main section')];
    const rowsBlock = () => document.querySelector('main .rows');

    afterEach(() => document.getElementById('hero-prepaint')?.remove());

    /** The image the server paints into the shell when the stage is on top (api/preload.go). */
    function prepaint() {
      const element = document.createElement('div');
      element.id = 'hero-prepaint';
      document.body.append(element);
      return element;
    }

    it('comes from the server: the named row is the band, the picks a poster row with their note', async () => {
      mockApi({
        ...signedIn,
        'GET /home': () =>
          json({
            heroRow: 'spotlight',
            rows: [
              { key: 'spotlight', items: [arrival] },
              { key: 'picks', items: [{ ...heat, note }] },
              { key: 'recent', items: [alien] },
            ],
          }),
      });
      renderApp('/');
      const stage = await screen.findByRole('region', { name: 'Spotlight' });
      expect(stage).toHaveAttribute('aria-roledescription', 'Karussell');
      expect(within(stage).getByRole('heading', { name: 'Arrival' })).toBeVisible();
      expect(carousels()).toHaveLength(1);

      const picks = screen.getByRole('region', { name: "Cat's Empfehlungen" });
      expect(picks).not.toHaveAttribute('aria-roledescription');
      expect(within(picks).getByRole('link', { name: /Heat/ })).toHaveTextContent(note);
    });

    it('is missing without heroRow: every row is a poster row, the picks too', async () => {
      mockApi({
        ...signedIn,
        'GET /home': () =>
          json({
            rows: [
              { key: 'picks', items: [{ ...heat, note }] },
              { key: 'recent', items: [alien, arrival] },
            ],
          }),
      });
      renderApp('/');
      const picks = await screen.findByRole('region', { name: "Cat's Empfehlungen" });
      expect(carousels()).toHaveLength(0);
      expect(within(picks).getByRole('link', { name: /Heat/ })).toHaveTextContent(note);
      expect(sections().map((section) => section.hasAttribute('aria-labelledby'))).toEqual([
        true,
        true,
      ]);

      // Spacing without a stage: the rows block starts below the header.
      expect(rowsBlock()).toHaveClass('padded');
    });

    it('stands at its place: on position 2 below two poster rows, with the normal gaps', async () => {
      mockApi({
        ...signedIn,
        'GET /home': () =>
          json({
            heroRow: 'spotlight',
            rows: [
              { key: 'picks', items: [heat] },
              { key: 'recent', items: [alien] },
              { key: 'spotlight', items: [arrival] },
              { key: 'watchlist', items: [heat] },
            ],
          }),
      });
      renderApp('/');
      const stage = await screen.findByRole('region', { name: 'Spotlight' });
      expect(sections().indexOf(stage)).toBe(2);
      expect(stage).toHaveAttribute('aria-roledescription', 'Karussell');

      // The rows block starts below the header; the framed band and the rows around it all stand
      // in it with the normal gap.
      expect(rowsBlock()).toHaveClass('padded');
      expect(stage.parentElement).toBe(rowsBlock());
      expect(screen.getByRole('region', { name: 'Neu hinzugefügt' }).parentElement).toBe(
        rowsBlock(),
      );
      expect(screen.getByRole('region', { name: 'Merkliste' }).parentElement).toBe(rowsBlock());

      // Not the page's first image: the posters of the first row do not wait for the band's art.
      const firstPoster = within(screen.getByRole('region', { name: "Cat's Empfehlungen" }))
        .getByRole('link', { name: /Heat/ })
        .querySelector('img');
      expect(firstPoster).toHaveAttribute('loading', 'eager');
    });

    it('on top dismisses the prepaint only once its own art is shown', async () => {
      const painted = prepaint();
      mockApi({ ...signedIn, 'GET /home': () => json(homePage) });
      const { container } = renderApp('/');
      const stage = await screen.findByRole('region', { name: 'Spotlight' });
      // The framed band starts below the header like any row; #hero-prepaint paints there.
      expect(rowsBlock()).toHaveClass('padded');
      expect(stage.parentElement).toBe(rowsBlock());

      const backdrop = container.querySelector('img[fetchpriority="high"]');
      expect(backdrop).toHaveAttribute(
        'src',
        `/api/v1/items/${ids.heat}/images/backdrop/w780?v=ghi`,
      );
      expect(painted.dataset.done).toBeUndefined();
      fireEvent.load(backdrop!);
      await waitFor(() => expect(painted.dataset.done).toBe(''));
    });

    describe('the trailer button (step 8, W-10)', () => {
      const trailers: Trailer[] = [
        {
          id: 1,
          lang: 'de',
          youtubeId: 'abcdefghijk',
          embedUrl: 'https://www.youtube-nocookie.com/embed/abcdefghijk',
          source: 'manual',
          official: false,
        },
      ];
      const stagePage = (first: ItemRef) => ({
        heroRow: 'spotlight',
        rows: [{ key: 'spotlight', items: [first, arrival] }],
      });

      it.each([
        { hasTrailer: true, buttons: ['Abspielen', 'Details', 'Trailer'] },
        { hasTrailer: false, buttons: ['Abspielen', 'Details'] },
        { hasTrailer: undefined, buttons: ['Abspielen', 'Details'] },
      ])('hasTrailer $hasTrailer: the stage shows $buttons', async ({ hasTrailer, buttons }) => {
        mockApi({ ...signedIn, 'GET /home': () => json(stagePage({ ...heat, hasTrailer })) });
        renderApp('/');
        const stage = await screen.findByRole('region', { name: 'Spotlight' });
        expect(within(stage).getByRole('heading', { name: 'Heat' })).toBeVisible();
        // The action buttons in their order; the strip's posters are named after their titles.
        const actions = [...stage.querySelectorAll('a, button')]
          .map((element) => element.textContent)
          .filter((name) => ['Abspielen', 'Details', 'Trailer'].includes(name ?? ''));
        expect(actions).toEqual(buttons);
      });

      it('opens the trailer dialog of the slide; Escape closes it and returns the focus', async () => {
        const { calls } = mockApi({
          ...signedIn,
          'GET /home': () => json(stagePage({ ...heat, hasTrailer: true })),
          [`GET /items/${ids.heat}/trailers`]: () => json(trailers),
        });
        const { router } = renderApp('/');
        const user = userEvent.setup();
        const stage = await screen.findByRole('region', { name: 'Spotlight' });

        const button = within(stage).getByRole('button', { name: 'Trailer' });
        await user.click(button);
        const dialog = await screen.findByRole('dialog', { name: 'Trailer: Heat' });
        const frame = await within(dialog).findByTitle('Trailer von Heat (YouTube)');
        expect(frame.getAttribute('src')).toBe(
          'https://www.youtube-nocookie.com/embed/abcdefghijk?autoplay=1',
        );
        expect(calls.some((call) => call.path === `/items/${ids.heat}/trailers`)).toBe(true);
        // The privacy note of the detail page's dialog stays.
        expect(
          within(dialog).getByText(
            'Der Trailer wird von YouTube abgespielt (youtube-nocookie.com).',
          ),
        ).toBeVisible();

        await user.keyboard('{Escape}');
        await waitFor(() => expect(screen.queryByRole('dialog')).not.toBeInTheDocument());
        expect(router.state.location.pathname).toBe('/');
        await waitFor(() => expect(button).toHaveFocus());
      });

      it('holds the stage on its slide while the dialog is open', async () => {
        vi.useFakeTimers({ shouldAdvanceTime: true });
        mockApi({
          ...signedIn,
          'GET /home': () => json(stagePage({ ...heat, hasTrailer: true })),
          [`GET /items/${ids.heat}/trailers`]: () => json(trailers),
        });
        renderApp('/');
        const stage = await screen.findByRole('region', { name: 'Spotlight' });
        // A click without focus: only the open dialog holds the carousel.
        fireEvent.click(within(stage).getByRole('button', { name: 'Trailer' }));
        const dialog = await screen.findByRole('dialog', { name: 'Trailer: Heat' });
        await act(() => vi.advanceTimersByTimeAsync(16_000));
        // The modal dialog hides the page from the accessibility tree meanwhile.
        expect(within(stage).getByRole('heading', { name: 'Heat', hidden: true })).toBeVisible();

        fireEvent.click(within(dialog).getByRole('button', { name: 'Schliessen' }));
        await waitFor(() => expect(screen.queryByRole('dialog')).not.toBeInTheDocument());
        // The dialog restores the focus a frame after closing; with the focus off the stage it
        // moves on again.
        await act(() => vi.advanceTimersByTimeAsync(100));
        (document.activeElement as HTMLElement | null)?.blur();
        await act(() => vi.advanceTimersByTimeAsync(8000));
        expect(within(stage).getByRole('heading', { name: 'Arrival' })).toBeVisible();
      });
    });

    it.each([
      {
        stage: 'further down',
        home: {
          heroRow: 'spotlight',
          rows: [
            { key: 'recent', items: [alien] },
            { key: 'spotlight', items: [heat] },
          ],
        },
      },
      { stage: 'missing', home: { rows: [{ key: 'recent', items: [alien] }] } },
    ])('$stage leaves no prepaint standing', async ({ home }) => {
      const painted = prepaint();
      mockApi({ ...signedIn, 'GET /home': () => json(home) });
      renderApp('/');
      await screen.findByRole('region', { name: 'Neu hinzugefügt' });
      await waitFor(() => expect(painted.dataset.done).toBe(''));
    });
  });
});
