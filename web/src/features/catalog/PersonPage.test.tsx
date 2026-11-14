import type { Person } from '@slouchcat/api-client';
import { fireEvent, screen, waitFor, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { alien, homePage, ids, seriesIds } from '@/test/fixtures';
import { json, mockApi, problem, renderApp, signedIn } from '@/test/api';

const person: Person = {
  id: 3,
  name: 'Sigourney Weaver',
  credits: [
    { item: alien, role: 'cast', character: 'Ellen Ripley' },
    {
      item: {
        id: seriesIds.show,
        kind: 'series',
        title: 'Slouchcat Test Show',
        inWatchlist: false,
        year: 2024,
        images: {},
      },
      role: 'director',
    },
  ],
};

describe('person page', () => {
  afterEach(() => vi.unstubAllGlobals());

  it('says "nicht gefunden" in the tab title only once that is certain', async () => {
    let answer: (response: Response) => void = () => undefined;
    mockApi({
      ...signedIn,
      'GET /people/3': () =>
        new Promise<Response>((resolve) => {
          answer = resolve;
        }),
    });
    renderApp('/people/3');
    await waitFor(() => expect(document.querySelector('[aria-busy="true"]')).not.toBeNull());
    expect(document.title).toBe('Slouchcat');
    answer(problem(404, 'not_found'));
    await waitFor(() => expect(document.title).toBe('Person nicht gefunden · Slouchcat'));
  });

  it('shows the name and one card per title with kind, year and roles', async () => {
    mockApi({
      ...signedIn,
      'GET /people/3': () =>
        json({
          ...person,
          credits: [...person.credits, { item: alien, role: 'writer' }],
        }),
    });
    renderApp('/people/3');

    expect(
      await screen.findByRole('heading', { level: 1, name: 'Sigourney Weaver' }),
    ).toBeVisible();
    expect(screen.getByRole('heading', { level: 2, name: 'Filme und Serien · 2' })).toBeVisible();
    const grid = screen.getByRole('list', { name: 'Filme und Serien' });
    // Alien as actress and writer: one card with both roles.
    const film = within(grid).getByRole('link', { name: /Alien/ });
    expect(film).toHaveAttribute('href', `/items/${ids.alien}`);
    expect(within(film).getByText('Film · 1979 · als Ellen Ripley, Drehbuch')).toBeVisible();
    expect(within(grid).getByText('Serie · 2024 · Regie')).toBeVisible();
    expect(within(grid).getAllByRole('link')).toHaveLength(2);
    // Each title carries the watchlist button on its cover (concept 17 D-9), after its link.
    const mark = within(grid).getByRole('button', { name: 'Alien merken' });
    expect(mark).toHaveAttribute('aria-pressed', 'false');
    expect(film.compareDocumentPosition(mark) & Node.DOCUMENT_POSITION_FOLLOWING).toBeTruthy();
    expect(within(grid).getByRole('button', { name: 'Slouchcat Test Show merken' })).toBeVisible();
    // Without facts from TMDB only the count of the library stands beside the name.
    expect(screen.getByText('In deiner Bibliothek')).toBeVisible();
    expect(screen.getByText('2 Titel')).toBeVisible();
    expect(screen.queryByText('Bekannt für')).toBeNull();
    expect(document.title).toBe('Sigourney Weaver · Slouchcat');
  });

  it('shows the facts from TMDB: department, birthday with age, place, death, n of m', async () => {
    vi.useFakeTimers({ toFake: ['Date'], now: new Date('2026-11-14T12:00:00Z') });
    mockApi({
      ...signedIn,
      'GET /people/3': () =>
        json({
          ...person,
          knownForDepartment: 'Acting',
          birthday: '1949-10-08',
          placeOfBirth: 'Manhattan, New York City, USA',
          totalCredits: 130,
        }),
      'GET /people/4': () =>
        json({
          ...person,
          id: 4,
          name: 'Ian Holm',
          knownForDepartment: 'Sound',
          birthday: '1931-09-12',
          deathday: '2020-06-19',
          totalCredits: 1,
        }),
      'GET /people/5': () =>
        json({ ...person, id: 5, name: 'Somebody', knownForDepartment: 'Puppetry' }),
    });
    const { router } = renderApp('/people/3');
    await screen.findByRole('heading', { level: 1, name: 'Sigourney Weaver' });
    const fact = (term: string) => screen.getByText(term).nextElementSibling?.textContent;
    expect(fact('Bekannt für')).toBe('Schauspiel');
    expect(fact('Geboren')).toBe('8. Oktober 1949 (77)');
    expect(fact('Geburtsort')).toBe('Manhattan, New York City, USA');
    expect(fact('In deiner Bibliothek')).toBe('2 von 130 Titeln');
    expect(screen.queryByText('Gestorben')).toBeNull();

    // A person who died: the age at death, no age at the birthday; m is never below n.
    await router.navigate({ to: '/people/$personId', params: { personId: '4' } });
    await screen.findByRole('heading', { level: 1, name: 'Ian Holm' });
    expect(fact('Bekannt für')).toBe('Ton');
    expect(fact('Geboren')).toBe('12. September 1931');
    expect(fact('Gestorben')).toBe('19. Juni 2020 (mit 88)');
    expect(fact('In deiner Bibliothek')).toBe('2 von 2 Titeln');

    // A department the locale does not know shows as TMDB names it.
    await router.navigate({ to: '/people/$personId', params: { personId: '5' } });
    await screen.findByRole('heading', { level: 1, name: 'Somebody' });
    expect(fact('Bekannt für')).toBe('Puppetry');
    vi.useRealTimers();
  });

  it('clamps the biography with «Mehr lesen» and marks the English fallback', async () => {
    // Three lines cut the text: the paragraph is taller than its box while closed.
    const height = vi
      .spyOn(HTMLElement.prototype, 'scrollHeight', 'get')
      .mockImplementation(function (this: HTMLElement) {
        return this.tagName === 'P' && this.lang === 'en' ? 200 : 0;
      });
    mockApi({
      ...signedIn,
      'GET /people/3': () =>
        json({
          ...person,
          biography: 'Sigourney Weaver is an American actress.',
          biographyLang: 'en',
          biographyFallback: true,
        }),
      'GET /people/4': () =>
        json({
          ...person,
          id: 4,
          name: 'Tom Skerritt',
          biography: 'Kurze Biografie.',
          biographyLang: 'de',
        }),
    });
    const { router } = renderApp('/people/3');
    const text = await screen.findByText('Sigourney Weaver is an American actress.');
    expect(text).toHaveAttribute('lang', 'en');
    expect(screen.getByText('Biografie von TMDB, nur auf Englisch vorhanden')).toBeVisible();
    const more = screen.getByRole('button', { name: 'Mehr lesen' });
    expect(more).toHaveAttribute('aria-expanded', 'false');
    expect(more).toHaveAttribute('aria-controls', text.id);
    const user = userEvent.setup();
    await user.click(more);
    const less = screen.getByRole('button', { name: 'Weniger' });
    expect(less).toHaveAttribute('aria-expanded', 'true');
    await user.click(less);
    expect(screen.getByRole('button', { name: 'Mehr lesen' })).toBeVisible();

    // A short biography in the main language: no button, no hint.
    await router.navigate({ to: '/people/$personId', params: { personId: '4' } });
    expect(await screen.findByText('Kurze Biografie.')).toHaveAttribute('lang', 'de');
    expect(screen.queryByRole('button', { name: 'Mehr lesen' })).toBeNull();
    expect(screen.queryByText('Biografie von TMDB, nur auf Englisch vorhanden')).toBeNull();
    height.mockRestore();
  });

  it('shows the 2:3 portrait, and the initial without one or when it fails', async () => {
    const portrait = { url: '/people/3/image?v=a&p=1', width: 300, height: 450 };
    mockApi({
      ...signedIn,
      'GET /people/3': () => json({ ...person, image: portrait }),
      'GET /people/4': () => json({ ...person, id: 4, name: 'Tom Skerritt' }),
    });
    const { router } = renderApp('/people/3');
    const heading = await screen.findByRole('heading', { level: 1, name: 'Sigourney Weaver' });
    const head = heading.parentElement?.parentElement;
    const image = head?.querySelector('img');
    expect(image?.getAttribute('src')).toContain('/people/3/image?v=a&p=1');
    // The band behind the head is the same portrait, blurred (ui/Backdrop).
    expect(document.querySelectorAll('main img[src*="/people/3/image"]')).toHaveLength(2);
    // The file is gone: the initial takes the portrait's place.
    if (image) fireEvent.error(image);
    expect(head?.querySelector('img')).toBeNull();
    expect(head).toHaveTextContent('S');

    // A person without a portrait shows the initial from the start and no band.
    await router.navigate({ to: '/people/$personId', params: { personId: '4' } });
    const other = await screen.findByRole('heading', { level: 1, name: 'Tom Skerritt' });
    expect(other.parentElement?.parentElement?.querySelector('img')).toBeNull();
    expect(other.parentElement?.parentElement).toHaveTextContent('T');
    expect(document.querySelector('main img[src*="/people/"]')).toBeNull();
  });

  it('answers a person without visible credits (404) with the not-found notice', async () => {
    mockApi({ ...signedIn, 'GET /people/*': () => problem(404, 'not_found') });
    renderApp('/people/9');
    expect(await screen.findByText('Person nicht gefunden')).toBeVisible();
    expect(screen.getByText('Zu dieser Person gibt es hier keine Titel.')).toBeVisible();
    expect(screen.getByRole('link', { name: 'Zur Startseite' })).toHaveAttribute('href', '/');
  });

  it('treats an id that is no number as not found without asking the server', async () => {
    const { calls } = mockApi({ ...signedIn });
    renderApp('/people/abc');
    expect(await screen.findByText('Person nicht gefunden')).toBeVisible();
    expect(calls.some((call) => call.path.startsWith('/people'))).toBe(false);
  });

  it('goes back with Escape', async () => {
    mockApi({
      ...signedIn,
      'GET /people/3': () => json(person),
      'GET /home': () => json(homePage),
    });
    const { router } = renderApp('/people/3');
    await screen.findByRole('heading', { level: 1, name: 'Sigourney Weaver' });
    await userEvent.setup().keyboard('{Escape}');
    await waitFor(() => expect(router.state.location.pathname).toBe('/'));
  });
});
