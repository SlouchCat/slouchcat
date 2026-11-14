import { screen, within } from '@testing-library/react';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { json, mockApi, problem, renderApp, signedIn } from '@/test/api';

// The side navigation's group «Meine Medien» (concept 21 P-9, P-11): the server's entries in its
// order, «Radio» moved there from the top group with the number of favourites as «N Sender»,
// «Sammlungen» with the number of collections; what the administration hid is not sent.

const entries = [
  { id: 'radio', key: 'radio', count: 3 },
  {
    id: 'library:1',
    key: 'library',
    count: 12,
    library: { id: 1, name: 'Filme', kind: 'video', contents: ['movie'], itemCount: 12 },
  },
  { id: 'collections', key: 'collections', count: 333 },
  { id: 'picks', key: 'picks', count: 1 },
];

const nav = () => screen.getByRole('navigation', { name: 'Hauptnavigation' });
const navShown = () => screen.findByRole('navigation', { name: 'Hauptnavigation' });

describe('the side navigation’s «Meine Medien» (concept 21 P-9)', () => {
  afterEach(() => vi.unstubAllGlobals());

  it('lists the entries in the stored order, Radio with its stations, no Radio on top', async () => {
    mockApi({
      ...signedIn,
      'GET /home': () => json({ rows: [] }),
      'GET /my-media': () => json(entries),
    });
    renderApp('/');
    const media = await within(await navShown()).findByRole('list', { name: 'Meine Medien' });
    await within(media).findByRole('link', { name: 'Radio, 3 Sender' });
    expect(
      within(media)
        .getAllByRole('link')
        .map((link) => [link.getAttribute('aria-label'), link.getAttribute('href')]),
    ).toEqual([
      ['Radio, 3 Sender', '/radio'],
      ['Filme, 12 Titel', '/libraries/1'],
      ['Sammlungen, 333 Sammlungen', '/collections'],
      ['User-Empfehlungen, 1 Titel', '/picks'],
    ]);
    // The pill of Radio says «3 Sender», the others their number.
    expect(within(media).getByRole('link', { name: 'Radio, 3 Sender' })).toHaveTextContent(
      'Radio3 Sender',
    );
    expect(within(media).getByRole('link', { name: /^Sammlungen/ })).toHaveTextContent(
      'Sammlungen333',
    );
    // The top group lost «Radio»: Start, Suche, Merkliste, Gesehen (and Downloads where possible).
    const top = within(nav()).getAllByRole('list')[0]!;
    expect(within(top).queryByRole('link', { name: /Radio/ })).toBeNull();
    expect(within(top).getByRole('link', { name: 'Gesehen' })).toBeVisible();
  });

  it('marks the entry of the current page: «Sammlungen» on a collection', async () => {
    mockApi({
      ...signedIn,
      'GET /my-media': () => json(entries),
      'GET /collections': () => json([]),
    });
    renderApp('/collections');
    const link = await within(await navShown()).findByRole('link', { name: /^Sammlungen, / });
    expect(link).toHaveAttribute('aria-current', 'page');
  });

  it('says so when the entries cannot be read, and the rest stands', async () => {
    mockApi({
      ...signedIn,
      'GET /home': () => json({ rows: [] }),
      'GET /my-media': () => problem(500, 'internal'),
    });
    renderApp('/');
    expect(
      await within(await navShown()).findByText('Offline: Bibliotheken nicht verfügbar'),
    ).toBeVisible();
    expect(within(nav()).getByRole('link', { name: 'Start' })).toBeVisible();
  });
});
