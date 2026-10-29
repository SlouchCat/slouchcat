import type { PathCheck, SystemInfo } from '@slouchcat/api-client';
import { screen, waitFor, within } from '@testing-library/react';
import userEvent, { type UserEvent } from '@testing-library/user-event';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { homePage } from '@/test/fixtures';
import { json, mockApi, problem, renderApp, session, type Routes } from '@/test/api';

/** The state of a server without GPU in the container and a tight inotify limit. */
const system = (overrides: Partial<SystemInfo> = {}): SystemInfo => ({
  version: '0.9.0+abc1234',
  startedAt: '2026-10-21T08:00:00Z',
  gpu: {
    backend: 'software',
    hardwareEncode: false,
    tonemapper: 'cpu',
    hwTonemap: false,
    composeOverride: 'nvidia',
    devices: { dri: false, nvidia: true },
  },
  inotify: { maxUserWatches: 8192, watchedDirs: 0, usagePercent: 0, warn: true },
  configFs: 'ext4',
  cacheFs: 'tmpfs',
  ...overrides,
});

const localPath: PathCheck = { ok: true, fsType: 'ext4', watchMode: 'inotify' };

const setupRequired: Routes = {
  'GET /auth/session': () => problem(401, 'unauthenticated'),
  'GET /setup': () => json({ setupRequired: true, system: system() }),
  'GET /healthz': () => json({ status: 'ok', version: '0.2.0', uptimeSeconds: 1 }),
  'POST /setup/path-check': () => json(localPath),
};

async function fillWizard(user: UserEvent, { tmdb = '' } = {}) {
  await user.type(await screen.findByLabelText(/E-Mail-Adresse/), 'owner@example.test');
  await user.type(screen.getByLabelText('Passwort'), 'correct horse battery');
  await user.type(screen.getByLabelText('Passwort wiederholen'), 'correct horse battery');
  await user.click(screen.getByRole('radio', { name: 'Video' }));
  await user.type(screen.getByLabelText('Ordner'), '/media/Filme');
  if (tmdb) await user.type(screen.getByLabelText('API-Lesezugriffs-Token'), tmdb);
}

describe('setup wizard', () => {
  afterEach(() => vi.unstubAllGlobals());

  it('sends everything to the guard first: setup required redirects to /setup', async () => {
    mockApi(setupRequired);
    const { router } = renderApp('/');
    expect(
      await screen.findByRole('heading', { level: 1, name: 'Willkommen bei Slouchcat' }),
    ).toBeVisible();
    expect(router.state.location.pathname).toBe('/setup');
    // The four steps of A-1.
    expect(screen.getByRole('heading', { name: /Administratorkonto/ })).toBeVisible();
    expect(screen.getByRole('heading', { name: /Erste Bibliothek/ })).toBeVisible();
    expect(screen.getByRole('heading', { name: /Metadaten von TMDB/ })).toBeVisible();
    expect(screen.getByRole('heading', { name: /Grafikchip und Dateisystem/ })).toBeVisible();
  });

  it('checks password length and repetition in the client', async () => {
    const { calls } = mockApi(setupRequired);
    renderApp('/setup');
    const user = userEvent.setup();
    await user.type(await screen.findByLabelText('Passwort'), 'short');
    await user.type(screen.getByLabelText('Passwort wiederholen'), 'other');
    await user.click(screen.getByRole('button', { name: 'Einrichtung abschliessen' }));
    expect(
      await screen.findByText('Mindestens 10 Zeichen.', { selector: '[slot="errorMessage"]' }),
    ).toBeVisible();
    expect(screen.getByText('Die Passwörter stimmen nicht überein.')).toBeVisible();
    expect(calls.some((call) => call.method === 'POST')).toBe(false);
  });

  it('posts the request and opens the start page on 201', async () => {
    // Like the server: no session before the wizard ran, the session cookie after it.
    let done = false;
    const { calls } = mockApi({
      ...setupRequired,
      'POST /setup': () => {
        done = true;
        return json(session, 201);
      },
      'GET /auth/session': () => (done ? json(session) : problem(401, 'unauthenticated')),
      'GET /home': () => json(homePage),
    });
    const { router } = renderApp('/setup');
    const user = userEvent.setup();
    await fillWizard(user, { tmdb: 'tmdb-token' });
    await user.click(screen.getByRole('button', { name: 'Einrichtung abschliessen' }));

    await waitFor(() => expect(router.state.location.pathname).toBe('/'));
    const post = calls.find((call) => call.method === 'POST' && call.path === '/setup');
    expect(post?.body).toEqual({
      email: 'owner@example.test',
      password: 'correct horse battery',
      library: {
        name: 'Filme',
        kind: 'video',
        roots: [{ path: '/media/Filme', content: 'movie', providers: ['tmdb'] }],
      },
      tmdbApiKey: 'tmdb-token',
      language: 'de',
    });
  });

  it('maps library_path_invalid of the first folder to the path field', async () => {
    mockApi({
      ...setupRequired,
      'POST /setup': () =>
        problem(422, 'validation_failed', {
          errors: [{ field: 'library.roots[0].path', code: 'library_path_invalid' }],
        }),
    });
    renderApp('/setup');
    const user = userEvent.setup();
    await fillWizard(user);
    await user.click(screen.getByRole('button', { name: 'Einrichtung abschliessen' }));

    const path = screen.getByLabelText('Ordner');
    await waitFor(() => expect(path).toHaveAttribute('aria-invalid', 'true'));
    expect(path).toHaveAccessibleDescription(/Diesen Ordner gibt es auf dem Server nicht/);
  });

  it.each([
    ['tmdb_key_invalid', 'TMDB hat den Schlüssel abgelehnt.'],
    ['tmdb_unreachable', 'TMDB ist gerade nicht erreichbar.'],
  ])('maps %s to the TMDB field', async (code, message) => {
    mockApi({ ...setupRequired, 'POST /setup': () => problem(422, code) });
    renderApp('/setup');
    const user = userEvent.setup();
    await fillWizard(user, { tmdb: 'bad' });
    await user.click(screen.getByRole('button', { name: 'Einrichtung abschliessen' }));
    const field = screen.getByLabelText('API-Lesezugriffs-Token');
    await waitFor(() => expect(field).toHaveAccessibleDescription(new RegExp(message)));
  });

  it('lists validation_failed field codes at their inputs', async () => {
    mockApi({
      ...setupRequired,
      'POST /setup': () =>
        problem(422, 'validation_failed', {
          errors: [
            { field: 'email', code: 'invalid_email' },
            { field: 'library.name', code: 'too_long' },
          ],
        }),
    });
    renderApp('/setup');
    const user = userEvent.setup();
    await fillWizard(user);
    await user.click(screen.getByRole('button', { name: 'Einrichtung abschliessen' }));

    expect(await screen.findByRole('alert')).toHaveTextContent(
      'Bitte prüfe die markierten Felder.',
    );
    expect(screen.getByLabelText(/E-Mail-Adresse/)).toHaveAccessibleDescription(
      /Keine gültige E-Mail-Adresse/,
    );
    expect(screen.getByLabelText('Name')).toHaveAccessibleDescription(/Zu lang/);
  });

  it('checks the folder on blur: file system, watch mode and the network share warning', async () => {
    const { calls } = mockApi({
      ...setupRequired,
      'POST /setup/path-check': () =>
        json({ ok: true, fsType: 'cifs', watchMode: 'poll', warning: 'network_share' }),
    });
    renderApp('/setup');
    const user = userEvent.setup();
    await user.type(await screen.findByLabelText('Ordner'), '/media/nas');
    await user.tab();

    const step = screen.getByRole('region', { name: /Erste Bibliothek/ });
    expect(
      await within(step).findByText('Dateisystem cifs · Änderungen werden regelmässig abgefragt'),
    ).toBeVisible();
    expect(within(step).getByText('Netzfreigabe erkannt')).toBeVisible();
    expect(within(step).getByText(/Webhooks aus Radarr und Sonarr/)).toBeVisible();
    // Tab moved to "Ordner prüfen": the same value is not asked again.
    expect(screen.getByRole('button', { name: 'Ordner prüfen' })).toHaveFocus();
    await user.keyboard('{Enter}');
    const checks = calls.filter((call) => call.path === '/setup/path-check');
    expect(checks).toHaveLength(1);
    expect(checks[0]?.body).toEqual({ path: '/media/nas' });
  });

  it('shows the FUSE hint and maps the problems of a folder to the field', async () => {
    let answer: PathCheck = {
      ok: true,
      fsType: 'fuse.mergerfs',
      watchMode: 'inotify_poll',
      warning: 'fuse_unknown',
    };
    mockApi({ ...setupRequired, 'POST /setup/path-check': () => json(answer) });
    renderApp('/setup');
    const user = userEvent.setup();
    const path = await screen.findByLabelText('Ordner');
    await user.type(path, '/mnt/user/Filme');
    await user.click(screen.getByRole('button', { name: 'Ordner prüfen' }));
    expect(await screen.findByText('Unbekanntes Dateisystem')).toBeVisible();
    expect(
      screen.getByText(
        'Dateisystem fuse.mergerfs · Sofort erkannt, zusätzlich regelmässig abgefragt',
      ),
    ).toBeVisible();

    answer = { ok: false, problem: 'missing' };
    await user.type(path, '/fehlt');
    await user.click(screen.getByRole('button', { name: 'Ordner prüfen' }));
    await waitFor(() => expect(path).toHaveAttribute('aria-invalid', 'true'));
    expect(path).toHaveAccessibleDescription(/Diesen Ordner gibt es auf dem Server nicht/);
    expect(screen.queryByText('Unbekanntes Dateisystem')).not.toBeInTheDocument();

    answer = { ok: false, problem: 'not_absolute' };
    await user.clear(path);
    await user.type(path, 'Filme');
    await user.click(screen.getByRole('button', { name: 'Ordner prüfen' }));
    await waitFor(() =>
      expect(path).toHaveAccessibleDescription(/Gib einen absoluten Pfad an, er beginnt mit \//),
    );
  });

  it('asks for a folder before checking an empty one', async () => {
    const { calls } = mockApi(setupRequired);
    renderApp('/setup');
    const user = userEvent.setup();
    await user.click(await screen.findByRole('button', { name: 'Ordner prüfen' }));
    const path = screen.getByLabelText('Ordner');
    await waitFor(() => expect(path).toHaveAccessibleDescription(/Pflichtfeld/));
    expect(calls.some((call) => call.path === '/setup/path-check')).toBe(false);
  });

  it.each([
    [{ ok: true }, 'Gültig: TMDB hat den Schlüssel angenommen.', false],
    [{ ok: false, problem: 'invalid' }, 'Ungültig: TMDB hat den Schlüssel abgelehnt.', true],
    [{ ok: false, problem: 'unreachable' }, 'Nicht erreichbar: TMDB antwortet gerade nicht.', true],
  ])('checks the TMDB key: %o', async (answer, message, invalid) => {
    const { calls } = mockApi({ ...setupRequired, 'POST /setup/tmdb-check': () => json(answer) });
    const { queryClient } = renderApp('/setup');
    const user = userEvent.setup();
    const field = await screen.findByLabelText('API-Lesezugriffs-Token');
    await user.type(field, 'tmdb-token');
    await user.click(screen.getByRole('button', { name: 'Schlüssel prüfen' }));
    expect(calls.find((call) => call.path === '/setup/tmdb-check')?.body).toEqual({
      tmdbApiKey: 'tmdb-token',
    });
    if (invalid) {
      await waitFor(() => expect(field).toHaveAttribute('aria-invalid', 'true'));
      expect(field).toHaveAccessibleDescription(new RegExp(message));
    } else {
      expect(await screen.findByText(message)).toBeVisible();
      expect(field).not.toHaveAttribute('aria-invalid');
    }
    // The key is a secret: it does not stay in the mutation cache.
    await waitFor(() => expect(queryClient.getMutationCache().getAll()).toHaveLength(0));
    // A changed key hides the answer.
    await user.type(field, 'x');
    await waitFor(() => expect(screen.queryByText(new RegExp(message))).not.toBeInTheDocument());
  });

  it('without a GPU shows the Compose override command and the sysctl command to copy', async () => {
    mockApi(setupRequired);
    renderApp('/setup');
    const user = userEvent.setup();
    const step = await screen.findByRole('region', { name: /Grafikchip und Dateisystem/ });
    expect(within(step).getByText('Kein Grafikchip erkannt')).toBeVisible();
    expect(within(step).getByText('Prozessor (Software)')).toBeVisible();
    expect(within(step).getByText('8.192')).toBeVisible();
    expect(within(step).getByText('tmpfs')).toBeVisible();
    const compose = within(step).getByRole('group', { name: 'Compose-Befehl für NVIDIA' });
    expect(compose).toHaveTextContent(
      'docker compose -f compose.yaml -f compose.nvidia.yaml up -d',
    );
    const sysctl = within(step).getByRole('group', { name: 'Befehl für das inotify-Limit' });
    expect(sysctl).toHaveTextContent('sysctl fs.inotify.max_user_watches=524288');
    expect(within(step).getByText('Das inotify-Limit ist knapp')).toBeVisible();

    const writeText = vi.spyOn(navigator.clipboard, 'writeText');
    await user.click(within(compose).getByRole('button', { name: 'Kopieren' }));
    expect(writeText).toHaveBeenCalledWith(
      'docker compose -f compose.yaml -f compose.nvidia.yaml up -d',
    );
    expect(await within(compose).findByText('Befehl kopiert.')).toBeVisible();

    // Without a clipboard (plain HTTP) the command is selected for Ctrl+C.
    writeText.mockRejectedValue(new Error('not allowed'));
    await user.click(within(sysctl).getByRole('button', { name: 'Kopieren' }));
    expect(await within(sysctl).findByText(/Kopieren nicht möglich/)).toBeVisible();
    expect(window.getSelection()?.toString()).toBe('sysctl fs.inotify.max_user_watches=524288');
  });

  it('offers one command per maker when no GPU device is visible', async () => {
    const none = system();
    mockApi({
      ...setupRequired,
      'GET /setup': () =>
        json({
          setupRequired: true,
          system: {
            ...none,
            gpu: {
              ...none.gpu,
              composeOverride: undefined,
              devices: { dri: false, nvidia: false },
            },
            inotify: { ...none.inotify, maxUserWatches: 524288, warn: false },
          },
        }),
    });
    renderApp('/setup');
    const step = await screen.findByRole('region', { name: /Grafikchip und Dateisystem/ });
    expect(
      within(step)
        .getAllByRole('group')
        .map((group) => group.getAttribute('aria-label')),
    ).toEqual(['Compose-Befehl für NVIDIA', 'Compose-Befehl für Intel', 'Compose-Befehl für AMD']);
    expect(within(step).queryByText('Das inotify-Limit ist knapp')).not.toBeInTheDocument();
  });

  it('names the detected backend and its tonemapper; nothing to copy', async () => {
    mockApi({
      ...setupRequired,
      'GET /setup': () =>
        json({
          setupRequired: true,
          system: system({
            gpu: {
              backend: 'nvenc',
              hardwareEncode: true,
              tonemapper: 'cuda',
              hwTonemap: true,
              ffmpegVersion: 'ffmpeg version 7.1.1',
              composeOverride: 'nvidia',
              devices: { dri: false, nvidia: true },
            },
            inotify: { maxUserWatches: 524288, watchedDirs: 0, usagePercent: 0, warn: false },
          }),
        }),
    });
    renderApp('/setup');
    const step = await screen.findByRole('region', { name: /Grafikchip und Dateisystem/ });
    expect(within(step).getByText('NVIDIA NVENC')).toBeVisible();
    expect(within(step).getByText('CUDA auf dem Grafikchip')).toBeVisible();
    expect(within(step).getByText('ffmpeg version 7.1.1')).toBeVisible();
    expect(within(step).getByText(/Grafikchip erkannt/)).toBeVisible();
    expect(within(step).queryByRole('button', { name: 'Kopieren' })).not.toBeInTheDocument();
  });

  it('says so when the server sent no system state', async () => {
    mockApi({ ...setupRequired, 'GET /setup': () => json({ setupRequired: true }) });
    renderApp('/setup');
    expect(
      await screen.findByText('Der Server hat keine Angaben zur Hardware geliefert.'),
    ).toBeVisible();
  });
});
