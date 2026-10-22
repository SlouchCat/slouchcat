import type { Dashboard, JobSummary, SystemInfo } from '@slouchcat/api-client';
import { act, fireEvent, screen, waitFor, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { json, mockApi, problem, renderApp, session, signedIn, type Routes } from '@/test/api';

// The administration's overview (A-2, A-3): the dashboard asked every 5 s, failed jobs queued
// again or cancelled, a card for every admin page; the admin navigation above every /admin page
// and one "Verwaltung" entry in the header.

const failedJob: JobSummary = {
  id: 7,
  type: 'metadata.match',
  state: 'failed',
  attempts: 5,
  maxAttempts: 5,
  lastError: 'tmdb: HTTP 503',
  updatedAt: '2026-10-21T09:15:00Z',
};

const dashboard = (overrides: Partial<Dashboard> = {}): Dashboard => ({
  streams: [
    {
      id: 'ps-1',
      accountEmail: 'owner@example.test',
      profileName: 'Marco',
      itemId: '01JB8Z6Q4M7N9P2R3S5T7V9W1X',
      itemTitle: 'Heat',
      mediaFileId: 'mf-1',
      mode: 'transcode',
      hardware: true,
      startedAt: '2026-10-21T09:00:00Z',
      lastActivityAt: '2026-10-21T09:10:00Z',
    },
  ],
  jobs: { queued: 12, running: 1, failed: 1, recentFailed: [failedJob] },
  storage: {
    config: { path: '/config', totalBytes: 100e9, freeBytes: 60e9 },
    cache: { path: '/cache', totalBytes: 500e9, freeBytes: 450e9 },
  },
  integrations: [
    { key: 'tmdb', state: 'configured' },
    { key: 'trakt', state: 'error', detail: 'HTTP 503', checkedAt: '2026-10-21T09:00:00Z' },
    { key: 'email', state: 'missing' },
    { key: 'updateCheck', state: 'disabled' },
  ],
  // M9: two integrations, one of them silent for a week (I-8).
  webhooks: { total: 2, eventsLast24h: 3, silent: ['Sonarr'] },
  errors: [
    {
      at: '2026-10-21T09:15:00Z',
      source: 'metadata.match',
      message: 'match failed: tmdb: HTTP 503',
      jobId: 7,
      jobType: 'metadata.match',
    },
    { at: '2026-10-21T09:00:00Z', source: 'trakt', message: 'trakt refresh: HTTP 503' },
  ],
  update: {
    enabled: true,
    currentVersion: '0.9.0',
    latestVersion: '0.9.1',
    available: true,
    url: 'https://github.com/SlouchCat/slouchcat-releases/releases/tag/v0.9.1',
  },
  ...overrides,
});

const system: SystemInfo = {
  version: '0.9.0+abc1234',
  startedAt: '2026-10-21T08:00:00Z',
  gpu: {
    backend: 'qsv',
    hardwareEncode: true,
    tonemapper: 'vpp_qsv',
    hwTonemap: true,
    composeOverride: 'intel',
    devices: { dri: true, nvidia: false },
  },
  inotify: { maxUserWatches: 65536, watchedDirs: 58982, usagePercent: 90, warn: true },
  configFs: 'ext4',
  cacheFs: 'ext4',
};

const routes = (extra: Routes = {}): Routes => ({
  ...signedIn,
  'GET /admin/dashboard': () => json(dashboard()),
  'GET /admin/system': () => json(system),
  ...extra,
});

describe('admin overview (A-2, A-3)', () => {
  afterEach(() => {
    vi.useRealTimers();
    vi.unstubAllGlobals();
  });

  it('shows streams, jobs, storage, integrations, server and errors', async () => {
    mockApi(routes());
    renderApp('/admin');
    expect(await screen.findByRole('heading', { level: 1, name: 'Verwaltung' })).toBeVisible();

    const streams = await screen.findByRole('region', { name: 'Streams' });
    expect(within(streams).getByText('Laufende Streams')).toBeVisible();
    expect(within(streams).getByText('1')).toBeVisible();
    expect(within(streams).getByText('Marco: Heat · Transcode (HW)')).toBeVisible();
    expect(within(streams).getByRole('link', { name: 'Alle Streams' })).toHaveAttribute(
      'href',
      '/admin/streams',
    );

    const jobs = screen.getByRole('region', { name: 'Hintergrundjobs' });
    // Each counter is a term with its number (StatTile).
    expect(
      within(jobs)
        .getAllByRole('term')
        .map((term) => [term.textContent, term.nextElementSibling?.textContent]),
    ).toEqual([
      ['Wartend', '12'],
      ['Laufend', '1'],
      ['Fehlgeschlagen', '1'],
    ]);
    const failed = screen.getByRole('table', { name: 'Fehlgeschlagene Jobs' });
    const row = within(failed).getByRole('rowheader', { name: 'metadata.match' }).closest('tr')!;
    expect(within(row).getByText('5 von 5')).toBeVisible();
    expect(within(row).getByText('tmdb: HTTP 503')).toBeVisible();

    const storage = screen.getByRole('region', { name: 'Speicher' });
    const config = within(storage).getByRole('meter', { name: 'Konfiguration (/config)' });
    expect(config).toHaveAttribute('aria-valuenow', '40');
    // Intl puts a no-break space between number and unit.
    expect(config).toHaveAttribute(
      'aria-valuetext',
      expect.stringMatching(/^40\sGB von 100\sGB belegt$/),
    );
    expect(within(storage).getByRole('meter', { name: 'Cache (/cache)' })).toHaveAttribute(
      'aria-valuenow',
      '10',
    );
    expect(within(storage).getByText('Frei: 450 GB von 500 GB')).toBeVisible();

    const integrations = screen.getByRole('region', { name: 'Integrationen' });
    expect(within(integrations).getByText('TMDB: Eingerichtet')).toBeVisible();
    expect(within(integrations).getByText('Trakt: Fehler')).toBeVisible();
    expect(within(integrations).getByText('HTTP 503')).toBeVisible();
    expect(within(integrations).getByText('E-Mail: Nicht eingerichtet')).toBeVisible();
    expect(within(integrations).getByText('Update-Prüfung: Ausgeschaltet')).toBeVisible();
    expect(
      within(integrations).getByRole('link', { name: 'Integrationen einrichten' }),
    ).toHaveAttribute('href', '/admin/integrations');

    const server = await screen.findByRole('region', { name: 'Server' });
    expect(within(server).getByText('0.9.0+abc1234')).toBeVisible();
    expect(within(server).getByText('Intel Quick Sync (QSV)')).toBeVisible();
    expect(within(server).getByText('58.982 von 65.536 (90 %)')).toBeVisible();
    expect(within(server).getByText('Das inotify-Limit ist knapp')).toBeVisible();
    expect(
      within(server).getByRole('group', { name: 'Befehl für das inotify-Limit' }),
    ).toHaveTextContent('sysctl fs.inotify.max_user_watches=524288');

    const errors = screen.getByRole('list', { name: 'Fehler aus dem Server-Log' });
    // The list itself is not live: a poll announces a short count instead (below).
    expect(errors).not.toHaveAttribute('aria-live');
    const entries = within(errors).getAllByRole('listitem');
    expect(entries).toHaveLength(2);
    expect(entries[0]).toHaveTextContent('match failed: tmdb: HTTP 503');
    expect(entries[0]).toHaveTextContent('Job 7 (metadata.match)');
    expect(entries[1]).toHaveTextContent('trakt refresh: HTTP 503');
  });

  it('announces a newer version with its release notes (A-5)', async () => {
    mockApi(routes());
    renderApp('/admin');
    const notice = await screen.findByText('Version 0.9.1 ist verfügbar');
    const box = notice.closest('[role="status"]')!;
    expect(box).toHaveTextContent('Du verwendest 0.9.0.');
    expect(
      within(box as HTMLElement).getByRole('link', { name: /Versionshinweise/ }),
    ).toHaveAttribute(
      'href',
      'https://github.com/SlouchCat/slouchcat-releases/releases/tag/v0.9.1',
    );
  });

  it('shows no update hint, an empty stream list and no failed jobs when all is quiet', async () => {
    mockApi(
      routes({
        'GET /admin/dashboard': () =>
          json(
            dashboard({
              streams: [],
              jobs: { queued: 0, running: 0, failed: 0, recentFailed: [] },
              errors: [],
              update: { enabled: true, currentVersion: '0.9.0', available: false },
            }),
          ),
      }),
    );
    renderApp('/admin');
    expect(await screen.findByText('Gerade läuft nichts.')).toBeVisible();
    expect(screen.getByText('Keine Fehler seit dem Start.')).toBeVisible();
    expect(screen.queryByText(/ist verfügbar/)).not.toBeInTheDocument();
    expect(screen.queryByRole('table', { name: 'Fehlgeschlagene Jobs' })).not.toBeInTheDocument();
    expect(screen.getByRole('list', { name: 'Fehler aus dem Server-Log' })).toBeEmptyDOMElement();
    // The live region of the count is there before the first error comes.
    const errors = screen.getByRole('region', { name: 'Fehler' });
    expect(within(errors).getByRole('status')).toBeEmptyDOMElement();
  });

  it('asks again every 5 s; new errors go on top and are announced as a count', async () => {
    vi.useFakeTimers({ shouldAdvanceTime: true });
    const older = { at: '2026-10-21T09:20:00Z', source: 'library', message: 'scan aborted' };
    let current = dashboard({ errors: [] });
    const { calls } = mockApi(routes({ 'GET /admin/dashboard': () => json(current) }));
    renderApp('/admin');
    await screen.findByRole('region', { name: 'Hintergrundjobs' });
    const tile = screen.getByRole('region', { name: 'Fehler' });
    const status = within(tile).getByRole('status');
    const polls = () => calls.filter((call) => call.path === '/admin/dashboard').length;
    expect(polls()).toBe(1);
    current = dashboard({ errors: [older] });
    await act(() => vi.advanceTimersByTimeAsync(5000));
    await waitFor(() => expect(polls()).toBe(2));
    const errors = screen.getByRole('list', { name: 'Fehler aus dem Server-Log' });
    const first = await within(errors).findByText('scan aborted');
    expect(first).toBeVisible();
    expect(status).toHaveTextContent(/^1 neuer Fehler$/);

    // Two more on top (newest first): the older entry keeps its node, only the count is read.
    current = dashboard({
      errors: [
        { at: '2026-10-21T09:22:00Z', source: 'transcode', message: 'ffmpeg exited' },
        { at: '2026-10-21T09:21:00Z', source: 'library', message: 'scan aborted' },
        older,
      ],
    });
    await act(() => vi.advanceTimersByTimeAsync(5000));
    await waitFor(() => expect(within(errors).getAllByRole('listitem')).toHaveLength(3));
    expect(status).toHaveTextContent(/^2 neue Fehler$/);
    expect(within(errors).getAllByRole('listitem')[2]).toContainElement(first);

    // A poll without news keeps the last count and the nodes.
    await act(() => vi.advanceTimersByTimeAsync(5000));
    await waitFor(() => expect(polls()).toBe(4));
    expect(status).toHaveTextContent(/^2 neue Fehler$/);
    expect(within(errors).getAllByRole('listitem')[2]).toContainElement(first);
  });

  it('queues a failed job again; focus goes to the jobs', async () => {
    let listed = dashboard();
    const { calls } = mockApi(
      routes({
        'GET /admin/dashboard': () => json(listed),
        'POST /admin/jobs/7/retry': () => {
          listed = dashboard({ jobs: { queued: 13, running: 1, failed: 0, recentFailed: [] } });
          return json({ ...failedJob, state: 'queued', attempts: 0 });
        },
      }),
    );
    renderApp('/admin');
    const user = userEvent.setup();
    await user.click(
      await screen.findByRole('button', { name: 'Job metadata.match (7) erneut einreihen' }),
    );
    await waitFor(() =>
      expect(
        calls.some((call) => call.method === 'POST' && call.path === '/admin/jobs/7/retry'),
      ).toBe(true),
    );
    expect(await screen.findByText('Job metadata.match wartet wieder.')).toBeVisible();
    await waitFor(() =>
      expect(screen.queryByRole('table', { name: 'Fehlgeschlagene Jobs' })).toBeNull(),
    );
    expect(screen.getByRole('heading', { name: 'Hintergrundjobs' })).toHaveFocus();
  });

  it('cancels a failed job after the confirmation; a refusal shows its message', async () => {
    let refuse = true;
    const { calls } = mockApi(
      routes({
        'DELETE /admin/jobs/7': () =>
          refuse ? problem(404, 'not_found') : new Response(null, { status: 204 }),
      }),
    );
    renderApp('/admin');
    const user = userEvent.setup();
    const cancel = await screen.findByRole('button', { name: 'Job metadata.match (7) abbrechen' });
    await user.click(cancel);
    let dialog = await screen.findByRole('alertdialog', { name: 'Job metadata.match abbrechen?' });
    await user.click(within(dialog).getByRole('button', { name: 'Job abbrechen' }));
    expect(await screen.findByRole('alert')).toHaveTextContent('Nicht gefunden.');
    // «Erneut einreihen» is refused as well (no route: 404).
    await user.click(
      screen.getByRole('button', { name: 'Job metadata.match (7) erneut einreihen' }),
    );
    await waitFor(() =>
      expect(calls.filter((call) => call.path === '/admin/jobs/7/retry')).toHaveLength(1),
    );
    expect(await screen.findByRole('alert')).toHaveTextContent('Nicht gefunden.');

    refuse = false;
    await user.click(screen.getByRole('button', { name: 'Job metadata.match (7) abbrechen' }));
    dialog = await screen.findByRole('alertdialog', { name: 'Job metadata.match abbrechen?' });
    await user.click(within(dialog).getByRole('button', { name: 'Job abbrechen' }));
    expect(await screen.findByText('Job metadata.match abgebrochen.')).toBeVisible();
    // The next action clears the old refusal, also the one of the other button.
    expect(screen.queryByRole('alert')).not.toBeInTheDocument();
    expect(
      calls.filter((call) => call.method === 'DELETE' && call.path === '/admin/jobs/7'),
    ).toHaveLength(2);
  });

  it('a failing dashboard request offers a retry', async () => {
    let fail = true;
    mockApi(
      routes({
        'GET /admin/dashboard': () => (fail ? problem(500, 'internal') : json(dashboard())),
      }),
    );
    renderApp('/admin');
    const user = userEvent.setup();
    expect(await screen.findByText('Etwas ist schiefgelaufen. Versuche es erneut.')).toBeVisible();
    fail = false;
    await user.click(screen.getByRole('button', { name: 'Erneut versuchen' }));
    expect(await screen.findByRole('region', { name: 'Hintergrundjobs' })).toBeVisible();
  });

  it('links every admin page as a card, in the groups of the navigation (A-2)', async () => {
    mockApi(routes());
    renderApp('/admin');
    const manage = await screen.findByRole('region', { name: 'Alle Verwaltungsseiten' });
    const cards = (group: string) =>
      within(within(manage).getByRole('region', { name: group }))
        .getAllByRole('link')
        .map((link) => [
          document.getElementById(link.getAttribute('aria-labelledby') ?? '')?.textContent,
          link.getAttribute('href'),
        ]);
    expect(cards('Bibliothek')).toEqual([
      ['Bibliotheken', '/admin/libraries'],
      ['Podcasts', '/admin/podcasts'],
      ['Pflege', '/admin/maintenance'],
    ]);
    expect(cards('Konten')).toEqual([
      ['Konten', '/admin/accounts'],
      ['Zugriff', '/admin/access'],
      ['Profile', '/settings/profiles'],
      ['Geräte', '/admin/devices'],
      ['Streams', '/admin/streams'],
    ]);
    expect(cards('Kuratierung')).toEqual([
      ["Cat's Empfehlungen", '/admin/picks'],
      ['Sammlungen', '/admin/collections'],
      ['Startseite', '/admin/home-rows'],
      ['Ankündigungen', '/admin/announcements'],
    ]);
    expect(cards('System')).toEqual([
      ['Integrationen', '/admin/integrations'],
      ['Benachrichtigungen', '/settings/notifications'],
      ['Statistiken', '/admin/stats'],
      ['Sicherung', '/admin/backup'],
    ]);
    // The title names a card, its line describes it.
    const integrations = within(manage).getByRole('link', { name: 'Integrationen' });
    expect(integrations).toHaveAccessibleDescription(
      'Schlüssel, E-Mail, Telegram, Web-Push und Update-Prüfung.',
    );
  });
});

describe('admin navigation (A-2)', () => {
  afterEach(() => vi.unstubAllGlobals());

  it('puts the admin pages above every /admin page; ← → Home End move, one Tab stop', async () => {
    mockApi({ ...signedIn, 'GET /admin/devices': () => json([]) });
    renderApp('/admin/devices');
    const nav = await screen.findByRole('navigation', { name: 'Verwaltung' });
    const links = within(nav).getAllByRole('link');
    expect(links.map((link) => link.textContent)).toEqual([
      'Übersicht',
      'Bibliotheken',
      'Podcasts',
      'Pflege',
      'Konten',
      'Zugriff',
      'Geräte',
      'Streams',
      "Cat's Empfehlungen",
      'Sammlungen',
      'Startseite',
      'Ankündigungen',
      'Integrationen',
      'Statistiken',
      'Sicherung',
    ]);
    expect(within(nav).getByRole('list', { name: 'Konten' })).toBeVisible();
    const devices = within(nav).getByRole('link', { name: 'Geräte' });
    expect(devices).toHaveAttribute('aria-current', 'page');
    // Roving tabIndex: only the current page's link is in the Tab order.
    expect(links.filter((link) => link.tabIndex === 0)).toEqual([devices]);

    devices.focus();
    fireEvent.keyDown(devices, { key: 'ArrowRight' });
    const streams = within(nav).getByRole('link', { name: 'Streams' });
    expect(streams).toHaveFocus();
    expect(streams.tabIndex).toBe(0);
    expect(devices.tabIndex).toBe(-1);
    fireEvent.keyDown(streams, { key: 'ArrowLeft' });
    expect(devices).toHaveFocus();
    fireEvent.keyDown(devices, { key: 'End' });
    expect(within(nav).getByRole('link', { name: 'Sicherung' })).toHaveFocus();
    fireEvent.keyDown(document.activeElement!, { key: 'ArrowRight' });
    expect(within(nav).getByRole('link', { name: 'Übersicht' })).toHaveFocus();
    fireEvent.keyDown(document.activeElement!, { key: 'ArrowLeft' });
    expect(within(nav).getByRole('link', { name: 'Sicherung' })).toHaveFocus();
    fireEvent.keyDown(document.activeElement!, { key: 'Home' });
    expect(within(nav).getByRole('link', { name: 'Übersicht' })).toHaveFocus();
    // Leaving the row puts the Tab stop back on the current page.
    act(() => (document.activeElement as HTMLElement).blur());
    expect(devices.tabIndex).toBe(0);
  });

  it('Enter follows a link; maintenance stays current on a title page', async () => {
    mockApi({
      ...signedIn,
      'GET /admin/devices': () => json([]),
      'GET /admin/streams': () => json([]),
    });
    const { router } = renderApp('/admin/devices');
    const user = userEvent.setup();
    const nav = await screen.findByRole('navigation', { name: 'Verwaltung' });
    await user.click(within(nav).getByRole('link', { name: 'Streams' }));
    await waitFor(() => expect(router.state.location.pathname).toBe('/admin/streams'));
    expect(within(nav).getByRole('link', { name: 'Streams' })).toHaveAttribute(
      'aria-current',
      'page',
    );
  });

  it('collapses the header to one "Verwaltung" entry, current on every /admin page', async () => {
    mockApi({ ...signedIn, 'GET /admin/devices': () => json([]) });
    renderApp('/admin/devices');
    const header = await screen.findByRole('navigation', { name: 'Hauptnavigation' });
    expect(within(header).getByRole('link', { name: 'Verwaltung' })).toHaveAttribute(
      'aria-current',
      'page',
    );
    expect(within(header).queryByRole('link', { name: 'Geräte' })).not.toBeInTheDocument();
    expect(within(header).getByRole('link', { name: 'Profile' })).toBeVisible();
  });

  it('a profile that is not the main account primary gets the notice without the navigation', async () => {
    const guest = {
      ...session,
      account: { ...session.account, role: 'guest' as const },
    };
    const { calls } = mockApi({ 'GET /auth/session': () => json(guest) });
    renderApp('/admin');
    expect(await screen.findByText('Nur für das Hauptkonto')).toBeVisible();
    expect(screen.queryByRole('navigation', { name: 'Verwaltung' })).not.toBeInTheDocument();
    expect(
      within(screen.getByRole('navigation', { name: 'Hauptnavigation' })).queryByRole('link', {
        name: 'Verwaltung',
      }),
    ).not.toBeInTheDocument();
    expect(calls.some((call) => call.path.startsWith('/admin/'))).toBe(false);
  });
});

describe('update badge (A-5)', () => {
  afterEach(() => vi.unstubAllGlobals());

  it('shows the main account a badge with the version that links to the overview', async () => {
    mockApi({
      ...routes(),
      'GET /auth/session': () =>
        json({ ...session, updateAvailable: { version: '0.9.1', url: 'https://example.test' } }),
    });
    const { router } = renderApp('/admin/devices');
    const user = userEvent.setup();
    const badge = await screen.findByRole('link', { name: 'Update auf Version 0.9.1 verfügbar' });
    expect(badge).toHaveTextContent('Update');
    await user.click(badge);
    await waitFor(() => expect(router.state.location.pathname).toBe('/admin'));
  });

  it('shows no badge without a newer version or for another profile', async () => {
    const other = {
      ...session,
      profile: { ...session.profile, id: 2, name: 'Lena', isPrimary: false },
      updateAvailable: { version: '0.9.1', url: 'https://example.test' },
    };
    mockApi({ 'GET /auth/session': () => json(other), 'GET /home': () => json({ rows: [] }) });
    renderApp('/');
    await screen.findByRole('navigation', { name: 'Hauptnavigation' });
    expect(screen.queryByRole('link', { name: /^Update/ })).not.toBeInTheDocument();
  });
});
