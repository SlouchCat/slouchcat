import { act, screen, within } from '@testing-library/react';
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

  it('prefixes image URLs with the API base and loads only the first row eagerly', async () => {
    mockApi({ ...signedIn, 'GET /home': () => json(homePage) });
    const { container } = renderApp('/');
    await screen.findByRole('heading', { name: 'Weiterschauen' });

    const backdrop = container.querySelector('img[fetchpriority="high"]');
    expect(backdrop).toHaveAttribute(
      'src',
      `/api/v1/items/${ids.heat}/images/backdrop/w1280?v=def`,
    );

    const continueRow = screen.getByRole('region', { name: 'Weiterschauen' });
    const alienCard = within(continueRow).getByRole('link', { name: /Alien/ });
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
