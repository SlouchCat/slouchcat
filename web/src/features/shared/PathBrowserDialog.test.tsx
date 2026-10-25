import type { BrowseRoot, DirectoryListing } from '@slouchcat/api-client';
import { screen, waitFor, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { json, mockApi, renderApp, signedIn } from '@/test/api';
import { library } from '@/test/fixtures';

const roots: BrowseRoot[] = [
  { id: 'r1', path: '/media', source: 'configured', browsable: true, fsType: 'ext4' },
];

const mediaDir: DirectoryListing = {
  rootId: 'r1',
  rel: '',
  path: '/media',
  atRoot: true,
  fsType: 'ext4',
  watch: 'inotify',
  entries: [
    { name: 'Filme', rel: 'Filme', path: '/media/Filme' },
    { name: 'Serien', rel: 'Serien', path: '/media/Serien' },
  ],
};

const serienDir: DirectoryListing = {
  rootId: 'r1',
  rel: 'Serien',
  path: '/media/Serien',
  parentRel: '',
  atRoot: false,
  fsType: 'ext4',
  watch: 'inotify',
  entries: [{ name: 'Fallout', rel: 'Serien/Fallout', path: '/media/Serien/Fallout' }],
};

/** The folder picker as the library form opens it. */
function mount() {
  mockApi({
    ...signedIn,
    'GET /admin/libraries': () => json([library()]),
    'GET /admin/fs/roots': () => json(roots),
    'POST /admin/fs/browse': (call) => {
      const body = call.body as { rel?: string };
      return json(body.rel === 'Serien' ? serienDir : mediaDir);
    },
  });
  return renderApp('/admin/libraries');
}

const dialog = () => screen.getByRole('dialog', { name: 'Ordner auswählen' });

/** Opens the picker of the "add library" form and waits for the roots. */
async function open(user: ReturnType<typeof userEvent.setup>) {
  const page = await screen.findByRole('heading', { level: 1, name: 'Bibliotheken' });
  expect(page).toBeVisible();
  await user.click(screen.getAllByRole('button', { name: 'Durchsuchen …' })[0]!);
  await waitFor(() => expect(within(dialog()).getAllByRole('option').length).toBeGreaterThan(0));
}

describe('path browser dialog (Emby shape, idea 02 E18)', () => {
  afterEach(() => {
    vi.unstubAllGlobals();
  });

  it('writes the marked row into the path field and hands that value on with OK', async () => {
    mount();
    const user = userEvent.setup();
    await open(user);

    const field = within(dialog()).getByLabelText('Ordner');
    expect(field).toHaveValue('');

    await user.click(within(dialog()).getByRole('option', { name: /\/media/ }));
    await waitFor(() => expect(field).toHaveValue('/media'));

    await user.click(within(dialog()).getByRole('button', { name: 'OK' }));
    await waitFor(() => expect(screen.queryByRole('dialog')).toBeNull());
    // The library form's own folder field now carries it.
    const form = screen.getByRole('region', { name: 'Bibliothek hinzufügen' });
    expect(within(form).getByLabelText('Ordner')).toHaveValue('/media');
  });

  it('takes the list to a path that was typed', async () => {
    mount();
    const user = userEvent.setup();
    await open(user);

    const field = within(dialog()).getByLabelText('Ordner');
    await user.clear(field);
    await user.type(field, '/media/Serien');
    await user.click(within(dialog()).getByRole('button', { name: 'Gehe zu' }));

    await waitFor(() =>
      expect(within(dialog()).getByRole('option', { name: /Fallout/ })).toBeVisible(),
    );
    // What was typed survives the jump, so OK still hands on exactly that folder.
    expect(field).toHaveValue('/media/Serien');
  });

  it('keeps a path below no root and says the server decides about it', async () => {
    mount();
    const user = userEvent.setup();
    await open(user);

    const field = within(dialog()).getByLabelText('Ordner');
    await user.type(field, '/etc/secrets');
    await user.click(within(dialog()).getByRole('button', { name: 'Gehe zu' }));

    expect(
      await within(dialog()).findByText(/liegt unter keinem freigegebenen Ordner/),
    ).toBeVisible();
    expect(field).toHaveValue('/etc/secrets');
    // The dialog refuses nothing: OK hands the typed path on, the server answers it.
    await user.click(within(dialog()).getByRole('button', { name: 'OK' }));
    await waitFor(() => expect(screen.queryByRole('dialog')).toBeNull());
    const form = screen.getByRole('region', { name: 'Bibliothek hinzufügen' });
    expect(within(form).getByLabelText('Ordner')).toHaveValue('/etc/secrets');
  });

  it('forgets what was typed when the dialog is closed without OK', async () => {
    mount();
    const user = userEvent.setup();
    await open(user);

    await user.type(within(dialog()).getByLabelText('Ordner'), '/media/Filme');
    await user.click(within(dialog()).getByRole('button', { name: 'Abbrechen' }));
    await waitFor(() => expect(screen.queryByRole('dialog')).toBeNull());

    await open(user);
    expect(within(dialog()).getByLabelText('Ordner')).toHaveValue('');
  });
});
