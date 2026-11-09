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

  it('shows the name and the visible credits with their roles', async () => {
    mockApi({ ...signedIn, 'GET /people/3': () => json(person) });
    renderApp('/people/3');

    expect(
      await screen.findByRole('heading', { level: 1, name: 'Sigourney Weaver' }),
    ).toBeVisible();
    const row = screen.getByRole('region', { name: 'Filme und Serien' });
    const film = within(row).getByRole('link', { name: /Alien/ });
    expect(film).toHaveAttribute('href', `/items/${ids.alien}`);
    expect(within(film).getByText('als Ellen Ripley')).toBeVisible();
    expect(within(row).getByText('Regie')).toBeVisible();
    // Each credit carries the watchlist button on its cover (concept 17 D-9), after its link.
    const mark = within(row).getByRole('button', { name: 'Alien merken' });
    expect(mark).toHaveAttribute('aria-pressed', 'false');
    expect(film.compareDocumentPosition(mark) & Node.DOCUMENT_POSITION_FOLLOWING).toBeTruthy();
    expect(within(row).getByRole('button', { name: 'Slouchcat Test Show merken' })).toBeVisible();
    expect(document.title).toBe('Sigourney Weaver · Slouchcat');
  });

  it('shows the portrait left of the name, and the initial without one or when it fails', async () => {
    const portrait = { url: '/people/3/image?v=a&p=1', width: 185, height: 278 };
    mockApi({
      ...signedIn,
      'GET /people/3': () => json({ ...person, image: portrait }),
      'GET /people/4': () => json({ ...person, id: 4, name: 'Tom Skerritt' }),
    });
    const { router } = renderApp('/people/3');
    const heading = await screen.findByRole('heading', { level: 1, name: 'Sigourney Weaver' });
    const head = heading.parentElement;
    const image = head?.querySelector('img');
    expect(image?.getAttribute('src')).toContain('/people/3/image?v=a&p=1');
    // The file is gone: the initial takes the portrait's place.
    if (image) fireEvent.error(image);
    expect(head?.querySelector('img')).toBeNull();
    expect(head).toHaveTextContent('S');

    // A person without a portrait shows the initial from the start.
    await router.navigate({ to: '/people/$personId', params: { personId: '4' } });
    const other = await screen.findByRole('heading', { level: 1, name: 'Tom Skerritt' });
    expect(other.parentElement?.querySelector('img')).toBeNull();
    expect(other.parentElement).toHaveTextContent('T');
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
