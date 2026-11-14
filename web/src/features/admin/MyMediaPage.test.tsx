import type { MyMediaSetting } from '@slouchcat/api-client';
import { screen, waitFor, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { json, mockApi, problem, renderApp, session, signedIn } from '@/test/api';
import { adminShell, renderAdminPage } from '@/test/admin';

// «Meine Medien» (concept 21 P-11, variant A; O-20, O-21): built like the start page rows: a
// position per entry, what it holds, its state and one box «Ausblenden» for the side navigation and
// one for the start page; the tab «Vorschau» shows both places with the draft. The server stores
// the order; the page only edits it.

const defaultEntries: MyMediaSetting[] = [
  {
    id: 'library:1',
    key: 'library',
    libraryId: 1,
    libraryName: 'Filme',
    libraryKind: 'video',
    count: 1828,
  },
  {
    id: 'library:3',
    key: 'library',
    libraryId: 3,
    libraryName: 'Hörbücher',
    libraryKind: 'audiobooks',
    count: 89,
  },
  { id: 'radio', key: 'radio', count: 3 },
  { id: 'collections', key: 'collections', count: 333 },
  { id: 'picks', key: 'picks', count: 1 },
].map((entry, position) => ({
  ...entry,
  position,
  hideSidebar: false,
  hideHome: entry.key === 'collections' || entry.key === 'picks',
})) as MyMediaSetting[];

const names = ['Filme', 'Hörbücher', 'Radio', 'Sammlungen', 'User-Empfehlungen'];

const table = () => screen.getByRole('table', { name: 'Einträge von «Meine Medien»' });
const rowOf = (name: string) =>
  within(table())
    .getByRole('rowheader', { name: new RegExp(`^${name}`) })
    .closest('tr')!;
const rowNames = () =>
  within(table())
    .getAllByRole('rowheader')
    .map((cell) => cell.querySelector('span')?.firstChild?.textContent?.trim());
const rank = (name: string) => screen.getByRole('combobox', { name: `Platz für ${name}` });
const saveBar = () => screen.getByRole('region', { name: 'Speichern' });
const box = (name: string, place: 'sidebar' | 'home') =>
  within(table()).getByRole('checkbox', {
    name:
      place === 'sidebar'
        ? `${name} in der Seitenleiste ausblenden`
        : `${name} auf der Startseite ausblenden`,
  });

/** A server that keeps what it is sent, with the library, its kind and the count again. */
function storingRoutes(initial: MyMediaSetting[] = defaultEntries) {
  let stored = initial;
  return {
    ...signedIn,
    ...adminShell,
    'GET /admin/my-media': () => json(stored),
    'PUT /admin/my-media': (call: { body: unknown }) => {
      const before = stored;
      stored = (call.body as MyMediaSetting[]).map((entry) => ({
        ...before.find((old) => old.id === entry.id),
        ...entry,
      }));
      return json(stored);
    },
  };
}

describe('admin «Meine Medien» (concept 21 P-11)', () => {
  afterEach(() => {
    vi.restoreAllMocks();
    vi.unstubAllGlobals();
  });

  it('lists every entry with its kind, content and state, places and hides them per place, saves all', async () => {
    const { calls } = mockApi(storingRoutes());
    const { nav } = await renderAdminPage('/admin/my-media');
    const user = userEvent.setup();

    await screen.findByRole('table', { name: 'Einträge von «Meine Medien»' });
    expect(screen.getByRole('heading', { level: 1, name: 'Meine Medien' })).toBeVisible();
    expect(nav.querySelector('[aria-current="page"]')).toHaveTextContent('Meine Medien');
    expect(rowNames()).toEqual(names);
    expect(
      within(table())
        .getAllByRole('columnheader')
        .map((cell) => cell.textContent),
    ).toEqual(['Platz', 'Eintrag', 'Inhalt', 'Zustand', 'Seitenleiste', 'Startseite']);
    // The kind as a chip, what each holds with its noun, the state from both boxes (O-21).
    expect(rowOf('Filme')).toHaveTextContent('Bibliothek');
    expect(rowOf('Filme')).toHaveTextContent('1828 Titel');
    expect(rowOf('Hörbücher')).toHaveTextContent('89 Hörbücher');
    expect(rowOf('Radio')).toHaveTextContent('3 Sender');
    expect(rowOf('Sammlungen')).toHaveTextContent('333 Sammlungen');
    expect(rowOf('User-Empfehlungen')).toHaveTextContent('Empfehlungen');
    expect(rowOf('User-Empfehlungen')).toHaveTextContent('1 Titel');
    expect(rowOf('Filme')).toHaveTextContent('sichtbar');
    expect(rowOf('Sammlungen')).toHaveTextContent('nur Seitenleiste');
    expect(box('Sammlungen', 'home')).toBeChecked();
    expect(box('Sammlungen', 'sidebar')).not.toBeChecked();
    expect(rank('Radio')).toHaveAccessibleDescription('Die anderen Einträge rücken nach.');
    expect(within(saveBar()).getByText('Gespeichert')).toBeVisible();

    // Radio to the top: the others move down; the select keeps the focus.
    await user.selectOptions(rank('Radio'), 'Platz 1');
    expect(rowNames()).toEqual(['Radio', 'Filme', 'Hörbücher', 'Sammlungen', 'User-Empfehlungen']);
    await waitFor(() => expect(rank('Radio')).toHaveFocus());
    expect(screen.getByRole('status')).toHaveTextContent(
      'Radio steht jetzt auf Platz 1. Speichere, damit es gilt.',
    );
    // Hörbücher only on the start page, then in neither place; collections on the start page too.
    await user.click(box('Hörbücher', 'sidebar'));
    expect(rowOf('Hörbücher')).toHaveTextContent('nur Startseite');
    await user.click(box('Hörbücher', 'home'));
    expect(rowOf('Hörbücher')).toHaveTextContent('ausgeblendet');
    await user.click(box('Sammlungen', 'home'));
    expect(rowOf('Sammlungen')).toHaveTextContent('sichtbar');
    expect(within(saveBar()).getByText('Ungespeicherte Änderungen')).toBeVisible();
    expect(calls.some((call) => call.method === 'PUT')).toBe(false);

    await user.click(within(saveBar()).getByRole('button', { name: 'Speichern' }));
    expect(await screen.findByText('«Meine Medien» ist gespeichert.')).toBeVisible();
    const body = calls.find((call) => call.method === 'PUT')?.body as MyMediaSetting[];
    expect(body.map((entry) => entry.id)).toEqual([
      'radio',
      'library:1',
      'library:3',
      'collections',
      'picks',
    ]);
    expect(body.map((entry) => entry.position)).toEqual([0, 1, 2, 3, 4]);
    expect(body.map((entry) => [entry.hideSidebar, entry.hideHome])).toEqual([
      [false, false],
      [false, false],
      [true, true],
      [false, false],
      [false, true],
    ]);
    expect(within(saveBar()).getByText('Gespeichert')).toBeVisible();
  });

  it('the preview shows the side navigation and the tiles with the draft, each without what is hidden there', async () => {
    mockApi(storingRoutes());
    renderApp('/admin/my-media');
    const user = userEvent.setup();
    await screen.findByRole('table', { name: 'Einträge von «Meine Medien»' });
    await user.click(box('Filme', 'sidebar'));
    await user.click(screen.getByRole('tab', { name: /^Vorschau/ }));
    const sidebar = screen.getByRole('region', { name: 'Vorschau der Seitenleiste' });
    const home = screen.getByRole('region', { name: 'Vorschau der Startseite' });
    const items = (region: HTMLElement) =>
      within(region)
        .getAllByRole('listitem')
        .map((item) => item.firstChild?.textContent);
    expect(items(sidebar)).toEqual(['Hörbücher', 'Radio', 'Sammlungen', 'User-Empfehlungen']);
    expect(items(home)).toEqual(['Filme', 'Hörbücher', 'Radio']);
    expect(within(home).getByText('1828 Titel')).toBeVisible();
    // Back on the order, the draft stands.
    await user.click(screen.getByRole('tab', { name: /^Reihenfolge/ }));
    expect(box('Filme', 'sidebar')).toBeChecked();
  });

  it('«Verwerfen» asks, then brings back what is saved', async () => {
    const { calls } = mockApi(storingRoutes());
    renderApp('/admin/my-media');
    const user = userEvent.setup();
    await screen.findByRole('table', { name: 'Einträge von «Meine Medien»' });
    await user.selectOptions(rank('User-Empfehlungen'), 'Platz 1');
    await user.click(box('Radio', 'home'));
    await user.click(within(saveBar()).getByRole('button', { name: 'Verwerfen' }));
    const dialog = await screen.findByRole('alertdialog', { name: 'Änderungen verwerfen?' });
    await user.click(within(dialog).getByRole('button', { name: 'Verwerfen' }));
    await waitFor(() => expect(dialog).not.toBeInTheDocument());
    expect(rowNames()).toEqual(names);
    expect(box('Radio', 'home')).not.toBeChecked();
    expect(screen.getByRole('status')).toHaveTextContent('Die Änderungen sind verworfen.');
    await waitFor(() =>
      expect(screen.getByRole('heading', { level: 1, name: 'Meine Medien' })).toHaveFocus(),
    );
    expect(calls.some((call) => call.method === 'PUT')).toBe(false);
  });

  it('shows the server’s refusal and keeps the draft', async () => {
    mockApi({
      ...storingRoutes(),
      'PUT /admin/my-media': () =>
        problem(422, 'validation_failed', { errors: [{ field: 'rows', code: 'invalid_value' }] }),
    });
    renderApp('/admin/my-media');
    const user = userEvent.setup();
    await screen.findByRole('table', { name: 'Einträge von «Meine Medien»' });
    await user.click(box('Radio', 'sidebar'));
    await user.click(within(saveBar()).getByRole('button', { name: 'Speichern' }));
    expect(await screen.findByRole('alert')).toBeVisible();
    expect(box('Radio', 'sidebar')).toBeChecked();
  });

  it('is only for the administrator’s primary profile', async () => {
    mockApi({
      'GET /auth/session': () =>
        json({ ...session, profile: { ...session.profile, id: 2, isPrimary: false } }),
    });
    renderApp('/admin/my-media');
    expect(await screen.findByText('Nur für Administratoren')).toBeVisible();
    expect(screen.queryByRole('table')).toBeNull();
  });
});
