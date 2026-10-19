import type { ItemRef } from '@slouchcat/api-client';
import { act, fireEvent, screen, waitFor, within } from '@testing-library/react';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { alien, arrival, heat, homePage, ids } from '@/test/fixtures';
import { json, mockApi, renderApp, signedIn } from '@/test/api';

describe('start page', () => {
  afterEach(() => {
    vi.useRealTimers();
    vi.unstubAllGlobals();
  });

  it('shows the carousel, then "Weiterschauen" and "Neu hinzugefügt" in server order', async () => {
    mockApi({ ...signedIn, 'GET /home': () => json(homePage) });
    renderApp('/');

    const hero = await screen.findByRole('region', { name: "Cat's Empfehlungen" });
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
      'GET /home': () => json({ rows: [{ key: 'picks', items: [heat, alien, wideOnly] }] }),
    });
    const { container } = renderApp('/');
    const hero = await screen.findByRole('region', { name: "Cat's Empfehlungen" });
    const images = () => [...container.querySelectorAll('img')];

    expect(images()).toHaveLength(1);
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

    // A slide without the small size falls back to w1280 without srcset.
    fireEvent.click(within(hero).getByRole('button', { name: 'Empfehlung 3 von 3 anzeigen' }));
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

  it('never renders an empty row and works without picks', async () => {
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
    expect(screen.queryByRole('region', { name: "Cat's Empfehlungen" })).not.toBeInTheDocument();
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
      'GET /home': () => json({ rows: [{ key: 'picks', items: [heat, arrival] }] }),
    });
    renderApp('/');
    const hero = await screen.findByRole('region', { name: "Cat's Empfehlungen" });
    expect(within(hero).getByRole('heading', { name: 'Heat' })).toBeVisible();
    await act(() => vi.advanceTimersByTimeAsync(8000));
    expect(within(hero).getByRole('heading', { name: 'Arrival' })).toBeVisible();
    expect(
      within(hero).getByRole('button', { name: 'Empfehlung 2 von 2 anzeigen' }),
    ).toHaveAttribute('aria-current', 'true');
  });

  it('shows the header with profile name and the admin link for the owner', async () => {
    mockApi({ ...signedIn, 'GET /home': () => json(homePage) });
    renderApp('/');
    const nav = await screen.findByRole('navigation', { name: 'Hauptnavigation' });
    expect(within(nav).getByRole('link', { name: 'Start' })).toHaveAttribute(
      'aria-current',
      'page',
    );
    expect(within(nav).getByRole('link', { name: 'Bibliotheken' })).toBeVisible();
    expect(screen.getByText('Marco')).toBeVisible();
    expect(screen.getByRole('button', { name: 'Abmelden' })).toBeVisible();
  });
});
