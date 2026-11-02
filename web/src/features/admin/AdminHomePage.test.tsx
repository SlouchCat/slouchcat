import type { Dashboard, SystemInfo } from '@slouchcat/api-client';
import { act, fireEvent, screen, waitFor, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { afterEach, describe, expect, it, vi } from 'vitest';
import i18n from '@/i18n';
import { json, mockApi, problem, renderApp, session, signedIn, type Routes } from '@/test/api';
import { dashboardFixture, renderAdminPage } from '@/test/admin';
import { activitySentence } from './AdminHomePage';
import { reloadPage } from './restart';

// A restart that went through reloads the page; the test only notes it.
vi.mock('./restart', async (original) => ({
  ...(await original<typeof import('./restart')>()),
  reloadPage: vi.fn(),
}));

// The administration (concept 11, A-2, A-3): the sidebar in the shell's column on every /admin
// page (20 entries, numbers after F-7, one Tab stop, ↑ ↓ Home End), the sheet on a phone, and the
// overview with eight equal cards asked every 5 s, the hints, the collapsed server log and the
// restart.

const dashboard = (overrides: Partial<Dashboard> = {}): Dashboard =>
  dashboardFixture({
    streams: [
      {
        id: 'ps-1',
        accountEmail: 'owner@example.test',
        profileName: 'Marco',
        device: { id: 'd-1', name: 'Wohnzimmer' },
        itemId: '01JB8Z6Q4M7N9P2R3S5T7V9W1X',
        itemTitle: 'Heat',
        mediaFileId: 'mf-1',
        mode: 'transcode',
        hardware: true,
        startedAt: '2026-10-21T09:00:00Z',
        lastActivityAt: '2026-10-21T09:10:00Z',
        positionMs: 1_800_000,
        durationMs: 7_200_000,
      },
    ],
    jobs: {
      queued: 12,
      running: 1,
      failed: 1,
      recentFailed: [],
      active: [
        {
          id: 40,
          type: 'probe.keyframes',
          state: 'running',
          priority: 0,
          attempts: 1,
          maxAttempts: 3,
          updatedAt: '2026-10-21T09:14:00Z',
          subject: 'Oppenheimer (2023)',
          skipped: false,
          progress: { done: 12, total: 40 },
          estimatedSecondsLeft: 180,
          estimate: 'measured',
        },
      ],
      failedByType: [
        {
          type: 'metadata.match',
          count: 1,
          lastError: 'tmdb: HTTP 503',
          lastFailedAt: '2026-10-21T09:15:00Z',
        },
      ],
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
      development: false,
      available: true,
      url: 'https://github.com/SlouchCat/slouchcat-releases/releases/tag/v0.9.1',
    },
    counts: { titles: 1200, accounts: 3, maintenanceOpen: 4 },
    activity: [
      {
        id: 1,
        at: '2026-10-21T09:12:00Z',
        kind: 'playback_started',
        profileName: 'Marco',
        accountEmail: 'owner@example.test',
        itemTitle: 'Alien',
        detail: {},
      },
    ],
    ...overrides,
  });

const quiet = (overrides: Partial<Dashboard> = {}): Dashboard =>
  dashboard({
    streams: [],
    jobs: { queued: 0, running: 0, failed: 0, recentFailed: [], active: [], failedByType: [] },
    webhooks: { total: 0, eventsLast24h: 0, silent: [] },
    errors: [],
    update: { enabled: true, currentVersion: '0.9.0', development: false, available: false },
    counts: { titles: 1200, accounts: 3, maintenanceOpen: 0 },
    activity: [],
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

/** The cards of the overview by their headings, in the order of the grid. */
const cardTitles = () =>
  screen
    .getAllByRole('heading', { level: 2 })
    .map((heading) => heading.textContent)
    .filter((title) => title !== 'Server-Protokoll');

const termsOf = (region: HTMLElement) =>
  within(region)
    .getAllByRole('term')
    .map((term) => [term.textContent, term.nextElementSibling?.textContent]);

describe('admin overview (concept 11, A-3)', () => {
  afterEach(() => {
    vi.useRealTimers();
    vi.unstubAllGlobals();
  });

  it('shows eight cards in two columns: server, streams, tasks, activity, integrations, webhooks, storage, hints', async () => {
    mockApi(routes());
    renderApp('/admin');
    expect(await screen.findByRole('heading', { level: 1, name: 'Verwaltung' })).toBeVisible();
    expect(screen.getByText('Live, alle 5 Sekunden')).toBeVisible();
    const server = await screen.findByRole('region', { name: 'Server' });
    expect(cardTitles()).toEqual([
      'Server',
      'Jetzt läuft',
      'Aufgaben',
      'Letzte Aktivität',
      'Integrationen',
      'Webhooks',
      'Speicher',
      'Hinweise',
    ]);

    expect(await within(server).findByText('0.9.0+abc1234')).toBeVisible();
    expect(within(server).getByText('Intel Quick Sync (QSV)')).toBeVisible();
    expect(within(server).getByText('58.982 von 65.536 (90 %)')).toBeVisible();
    expect(within(server).getByText('Version 0.9.1 verfügbar')).toBeVisible();
    expect(within(server).getByRole('button', { name: 'Nach Updates suchen' })).toBeVisible();
    expect(within(server).getByRole('button', { name: 'Neu starten' })).toBeVisible();
    // The warnings with their commands moved to the hints.
    expect(within(server).queryByText('Das inotify-Limit ist knapp')).not.toBeInTheDocument();

    const streams = screen.getByRole('region', { name: 'Jetzt läuft' });
    expect(within(streams).getByText('Heat')).toBeVisible();
    expect(within(streams).getByText('Marco · Wohnzimmer')).toBeVisible();
    expect(within(streams).getByText('Transcode (HW)')).toBeVisible();
    expect(within(streams).getByRole('progressbar', { name: 'Fortschritt Heat' })).toHaveAttribute(
      'aria-valuenow',
      '25',
    );
    expect(within(streams).getByText('0:30:00 / 2:00:00')).toBeVisible();
    expect(within(streams).getByRole('link', { name: 'Alle Streams' })).toHaveAttribute(
      'href',
      '/admin/streams',
    );

    const tasks = screen.getByRole('region', { name: 'Aufgaben' });
    expect(termsOf(tasks)).toEqual([
      ['Wartend', '12'],
      ['Laufend', '1'],
      ['Fehlgeschlagen', '1'],
    ]);
    const running = within(tasks).getByRole('list', { name: 'Laufende Aufgaben' });
    expect(within(running).getByText('Keyframe-Index · Oppenheimer (2023)')).toBeVisible();
    expect(
      within(running).getByRole('progressbar', { name: 'Fortschritt Keyframe-Index' }),
    ).toHaveAttribute('aria-valuenow', '30');
    expect(within(running).getByText('12 von 40')).toBeVisible();
    expect(within(running).getByText('noch ca. 3 min')).toBeVisible();
    const failed = within(tasks).getByRole('list', { name: 'Fehlgeschlagen je Jobart' });
    expect(within(failed).getByText('Titel zuordnen: 1 fehlgeschlagen')).toBeVisible();
    expect(within(failed).getByText('tmdb: HTTP 503')).toBeVisible();
    expect(within(tasks).getByRole('link', { name: 'Hintergrundjobs' })).toHaveAttribute(
      'href',
      '/admin/jobs',
    );
    // Concept 12 (JOB-8, 11.5 point 11): the failures of every type are a tab of the jobs page.
    expect(within(tasks).getByRole('link', { name: 'Fehlschläge ansehen' })).toHaveAttribute(
      'href',
      '/admin/jobs?tab=failed',
    );
    // F-2: no table with «Erneut» and «Abbrechen» on the overview any more.
    expect(screen.queryByRole('table')).not.toBeInTheDocument();

    const activity = screen.getByRole('list', { name: 'Letzte Aktivität' });
    expect(within(activity).getByRole('listitem')).toHaveTextContent('Marco hat «Alien» gestartet');

    const integrations = screen.getByRole('region', { name: 'Integrationen' });
    expect(within(integrations).getByText('1 von 4 eingerichtet')).toBeInTheDocument();
    expect(within(integrations).getByText('TMDB: Eingerichtet')).toBeVisible();
    expect(within(integrations).getByText('Trakt: Fehler')).toBeVisible();
    expect(within(integrations).getByText('HTTP 503')).toBeVisible();
    expect(within(integrations).getByText('E-Mail: Nicht eingerichtet')).toBeVisible();
    expect(within(integrations).getByText('Update-Prüfung: Ausgeschaltet')).toBeVisible();
    expect(
      within(integrations).getByRole('link', { name: 'Integrationen einrichten' }),
    ).toHaveAttribute('href', '/admin/integrations');

    const webhooks = screen.getByRole('region', { name: 'Webhooks' });
    expect(
      within(webhooks).getByText(
        '3 Meldungen im Protokoll der letzten 24 Stunden, von 2 Integrationen.',
      ),
    ).toBeVisible();
    expect(within(webhooks).getByText('Seit sieben Tagen ohne Meldung: Sonarr')).toBeVisible();
    expect(within(webhooks).getByRole('link', { name: 'Webhooks verwalten' })).toBeVisible();

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

    // The old overview's parts are gone: intro, cards of every page.
    expect(screen.queryByText(/an einem Ort/)).not.toBeInTheDocument();
    expect(
      screen.queryByRole('region', { name: 'Alle Verwaltungsseiten' }),
    ).not.toBeInTheDocument();
  });

  it('lists what needs doing, most urgent first, and announces only their number', async () => {
    mockApi(
      routes({
        'GET /admin/dashboard': () =>
          json(
            dashboard({
              jobs: { ...dashboard().jobs, failed: 265, paused: 2 },
              attention: {
                abortedScans: [{ libraryId: 1, name: 'Filme', reason: 'loss_over_threshold' }],
              },
              browse: { lostSlots: 1, slots: 4 },
              storage: {
                config: { path: '/config', totalBytes: 100e9, freeBytes: 5e9 },
                cache: { path: '/cache', totalBytes: 500e9, freeBytes: 450e9 },
              },
            }),
          ),
        'GET /admin/system': () =>
          json({ ...system, gpu: { ...system.gpu, hardwareEncode: false, backend: 'software' } }),
      }),
    );
    renderApp('/admin');
    const hints = await screen.findByRole('region', { name: 'Hinweise' });
    await waitFor(() => expect(within(hints).getAllByRole('listitem')).toHaveLength(8));
    expect(
      within(hints)
        .getAllByRole('listitem')
        .map((item) => item.querySelector('span > span:last-child')?.textContent),
    ).toEqual([
      '265 Hintergrundjobs fehlgeschlagen',
      '2 Jobarten pausiert',
      'Scan der Bibliothek «Filme» abgebrochen',
      'Ordner-Browser blockiert',
      'Kein Grafikchip erkannt',
      'Das inotify-Limit ist knapp',
      '/config ist zu über 90 % belegt',
      'Version 0.9.1 ist verfügbar',
    ]);
    // The failures hint opens the tab «Fehlschläge», the paused hint the page.
    expect(
      within(hints)
        .getAllByRole('link', { name: 'Zu den Hintergrundjobs' })
        .map((link) => link.getAttribute('href')),
    ).toEqual(['/admin/jobs?tab=failed', '/admin/jobs']);
    expect(within(hints).getByRole('link', { name: 'Zu den Bibliotheken' })).toHaveAttribute(
      'href',
      '/admin/libraries',
    );
    expect(
      within(hints).getByText(
        'Mehr als 20 % der Dateien fehlen. Zum Schutz wurde nichts gelöscht.',
      ),
    ).toBeVisible();
    expect(
      within(hints).getByRole('group', { name: 'Befehl für das inotify-Limit' }),
    ).toHaveTextContent('sysctl fs.inotify.max_user_watches=524288');
    expect(
      within(hints).getByRole('group', { name: 'Compose-Befehl für Intel' }),
    ).toHaveTextContent('docker compose -f compose.yaml -f compose.intel.yaml up -d');
    expect(within(hints).getByRole('link', { name: /Versionshinweise/ })).toHaveAttribute(
      'href',
      'https://github.com/SlouchCat/slouchcat-releases/releases/tag/v0.9.1',
    );
    // Integration errors and silent webhooks stay in their own cards (G-8).
    expect(within(hints).queryByText(/Trakt/)).not.toBeInTheDocument();
    expect(within(hints).queryByText(/Sonarr/)).not.toBeInTheDocument();
    // The head's number is the live region (the command blocks have their own for «kopiert»).
    expect(within(hints).getByText('8 Hinweise').closest('[role="status"]')).toHaveTextContent(
      '8 Hinweise',
    );
  });

  it('names a development build as a hint instead of offering an update for it', async () => {
    mockApi(
      routes({
        'GET /admin/dashboard': () =>
          json(
            quiet({
              update: {
                enabled: true,
                currentVersion: 'spotlight-marker-f92c09f',
                latestVersion: '1.1.0',
                development: true,
                available: false,
                url: 'https://github.com/SlouchCat/slouchcat/releases/tag/v1.1.0',
              },
            }),
          ),
      }),
    );
    renderApp('/admin');
    const hints = await screen.findByRole('region', { name: 'Hinweise' });
    expect(await within(hints).findByText('Du verwendest einen Entwicklungsstand')).toBeVisible();
    expect(hints).toHaveTextContent(
      'spotlight-marker-f92c09f ist kein veröffentlichtes Release, darum gibt es keinen Vergleich und keinen Update-Hinweis. Zuletzt veröffentlicht: 1.1.0.',
    );
    expect(within(hints).getByRole('link', { name: /Versionshinweise/ })).toHaveAttribute(
      'href',
      'https://github.com/SlouchCat/slouchcat/releases/tag/v1.1.0',
    );
    expect(screen.queryByText(/ist verfügbar/)).not.toBeInTheDocument();
    expect(
      within(screen.getByRole('region', { name: 'Server' })).getByText('Entwicklungsstand'),
    ).toBeVisible();
  });

  it('when all is quiet: nothing plays, the next run, no hints, no errors, no activity', async () => {
    mockApi(
      routes({
        'GET /admin/dashboard': () =>
          json(
            quiet({
              jobs: {
                queued: 0,
                running: 0,
                failed: 0,
                recentFailed: [],
                active: [],
                failedByType: [],
                nextRun: { type: 'podcast.poll', at: '2026-10-21T12:00:00Z' },
              },
            }),
          ),
        'GET /admin/system': () => json({ ...system, inotify: { ...system.inotify, warn: false } }),
      }),
    );
    renderApp('/admin');
    expect(await screen.findByText('Gerade läuft nichts.')).toBeVisible();
    expect(
      screen.getByText(/^Gerade läuft keine Aufgabe\. Als Nächstes: Podcast-Feeds prüfen, /),
    ).toBeVisible();
    expect(screen.getByText('Noch keine Aktivität.')).toBeVisible();
    expect(screen.getByText('Kein Webhook eingerichtet.')).toBeVisible();
    const hints = screen.getByRole('region', { name: 'Hinweise' });
    await waitFor(() => expect(within(hints).getByText('Keine Hinweise.')).toBeVisible());
    expect(within(hints).getByRole('status')).toHaveTextContent('0 Hinweise');
    const log = screen.getByRole('region', { name: 'Server-Protokoll' });
    expect(log).toHaveTextContent('Server-Protokoll · keine Fehler');
    expect(within(log).getByText('Keine Fehler seit dem Start.')).not.toBeVisible();
    // The live region of the count is there before the first error comes.
    expect(within(log).getByRole('status')).toBeEmptyDOMElement();
  });

  it('builds a sentence for every kind of the activity log, and a general one for a new kind', async () => {
    const at = '2026-10-21T09:00:00Z';
    mockApi(
      routes({
        'GET /admin/dashboard': () =>
          json(
            quiet({
              activity: [
                {
                  id: 1,
                  at,
                  kind: 'playback_completed',
                  profileName: 'Lena',
                  itemTitle: 'Dune',
                  detail: {},
                },
                {
                  id: 2,
                  at,
                  kind: 'playback_stopped',
                  profileName: 'Lena',
                  itemTitle: 'Dune',
                  detail: { by: 'revoked' },
                },
                { id: 3, at, kind: 'scan_done', libraryName: 'Filme', detail: { newTitles: 3 } },
                {
                  id: 4,
                  at,
                  kind: 'scan_aborted',
                  libraryName: 'Serien',
                  detail: { reason: 'cancelled' },
                },
                { id: 5, at, kind: 'job_failed', detail: { jobType: 'probe.keyframes', count: 4 } },
                { id: 6, at, kind: 'telescope_aligned', detail: {} },
              ],
            }),
          ),
      }),
    );
    renderApp('/admin');
    const feed = await screen.findByRole('list', { name: 'Letzte Aktivität' });
    expect(
      within(feed)
        .getAllByRole('listitem')
        .map((item) => item.firstElementChild?.nextElementSibling?.textContent),
    ).toEqual([
      'Lena hat «Dune» fertig geschaut',
      'Wiedergabe von «Dune» (Lena) beendet, Gerät vergessen',
      'Scan von «Filme» fertig: 3 neue Titel',
      'Scan von «Serien» von Hand abgebrochen',
      'Keyframe-Index: 4 Jobs fehlgeschlagen',
      'Ereignis auf dem Server',
    ]);
    expect(within(feed).getAllByRole('listitem')[0]!.querySelector('time')).toHaveAttribute(
      'datetime',
      at,
    );
  });

  it('builds the sentence of every kind and branch from the locale', () => {
    const t = i18n.t.bind(i18n);
    const at = '2026-10-21T09:00:00Z';
    const sentence = (kind: string, fields: Partial<Dashboard['activity'][number]> = {}) =>
      activitySentence(t, { id: 1, at, kind, detail: {}, ...fields }, 'de');
    const viewer = { profileName: 'Lena', itemTitle: 'Dune' };
    expect(sentence('playback_started', viewer)).toBe('Lena hat «Dune» gestartet');
    expect(sentence('playback_completed', viewer)).toBe('Lena hat «Dune» fertig geschaut');
    expect(sentence('playback_stopped', { ...viewer, detail: { by: 'admin' } })).toBe(
      'Wiedergabe von «Dune» (Lena) beendet',
    );
    expect(sentence('playback_stopped', { ...viewer, detail: { by: 'revoked' } })).toBe(
      'Wiedergabe von «Dune» (Lena) beendet, Gerät vergessen',
    );
    // A deleted profile or title keeps the sentence whole.
    expect(sentence('playback_started')).toBe('Ein Profil hat «einen Titel» gestartet');
    const library = { libraryName: 'Filme' };
    expect(sentence('scan_started', { ...library, detail: { trigger: 'manual' } })).toBe(
      'Scan von «Filme» gestartet',
    );
    expect(sentence('scan_done', { ...library, detail: { newTitles: 1 } })).toBe(
      'Scan von «Filme» fertig: 1 neuer Titel',
    );
    expect(sentence('scan_done', library)).toBe('Scan von «Filme» fertig: 0 neue Titel');
    expect(sentence('scan_failed', { ...library, detail: { reason: 'error' } })).toBe(
      'Scan von «Filme» fehlgeschlagen',
    );
    expect(sentence('scan_aborted', { ...library, detail: { reason: 'root_missing' } })).toBe(
      'Scan von «Filme» vom Verlustschutz abgebrochen',
    );
    expect(sentence('scan_aborted', { ...library, detail: { reason: 'cancelled' } })).toBe(
      'Scan von «Filme» von Hand abgebrochen',
    );
    expect(sentence('scan_started')).toBe('Scan von «eine Bibliothek» gestartet');
    expect(sentence('job_failed', { detail: { jobType: 'probe.keyframes', count: 1 } })).toBe(
      'Keyframe-Index: 1 Job fehlgeschlagen',
    );
    expect(sentence('job_failed', { detail: { jobType: 'new.kind', count: 2 } })).toBe(
      'new.kind: 2 Jobs fehlgeschlagen',
    );
    expect(sentence('backup_created')).toBe('Sicherung erstellt');
    expect(sentence('backup_created', { detail: { sizeBytes: 2e9 } })).toMatch(
      /^Sicherung erstellt \(2\sGB\)$/,
    );
    expect(sentence('backup_restored')).toBe('Sicherung zurückgeholt');
    expect(sentence('server_started')).toBe('Server gestartet');
    expect(sentence('server_started', { detail: { version: '1.7.0' } })).toBe(
      'Server gestartet, Version 1.7.0',
    );
    expect(sentence('server_restart')).toBe('Server neu gestartet');
    expect(sentence('server_restart', { accountEmail: 'owner@example.test' })).toBe(
      'Neustart durch owner@example.test',
    );
    expect(sentence('telescope_aligned', viewer)).toBe('Ereignis auf dem Server');
  });

  it('keeps the server log collapsed; a poll every 5 s announces new errors as a count', async () => {
    vi.useFakeTimers({ shouldAdvanceTime: true });
    const older = { at: '2026-10-21T09:20:00Z', source: 'library', message: 'scan aborted' };
    let current = quiet();
    const { calls } = mockApi(routes({ 'GET /admin/dashboard': () => json(current) }));
    renderApp('/admin');
    const log = await screen.findByRole('region', { name: 'Server-Protokoll' });
    const status = within(log).getByRole('status');
    const polls = () => calls.filter((call) => call.path === '/admin/dashboard').length;
    await waitFor(() => expect(polls()).toBeGreaterThanOrEqual(1));
    const before = polls();
    current = quiet({ errors: [older] });
    await act(() => vi.advanceTimersByTimeAsync(5000));
    await waitFor(() => expect(polls()).toBeGreaterThan(before));
    const errors = within(log).getByRole('list', {
      hidden: true,
      name: 'Fehler aus dem Server-Log',
    });
    const first = await within(errors).findByText('scan aborted');
    // Collapsed: the list is there but not shown; its count is read out all the same.
    expect(first).not.toBeVisible();
    expect(status).toHaveTextContent(/^1 neuer Fehler$/);
    expect(log).toHaveTextContent('Server-Protokoll · 1 Fehler');

    // Two more on top (newest first): the older entry keeps its node, only the count is read.
    current = quiet({
      errors: [
        { at: '2026-10-21T09:22:00Z', source: 'transcode', message: 'ffmpeg exited' },
        { at: '2026-10-21T09:21:00Z', source: 'library', message: 'scan aborted' },
        older,
      ],
    });
    await act(() => vi.advanceTimersByTimeAsync(5000));
    await waitFor(() => expect(errors.querySelectorAll('li')).toHaveLength(3));
    expect(status).toHaveTextContent(/^2 neue Fehler$/);
    expect(errors.querySelectorAll('li')[2]).toContainElement(first);

    // Opened, the list shows; the list itself is not live.
    const user = userEvent.setup({ advanceTimers: (ms) => vi.advanceTimersByTime(ms) });
    await user.click(within(log).getByText('Server-Protokoll'));
    expect(first).toBeVisible();
    expect(errors).not.toHaveAttribute('aria-live');
    expect(errors.querySelectorAll('li')[0]).toHaveTextContent('ffmpeg exited');
  });

  it('restart: the dialog names the streams; «Abbrechen» keeps the server running', async () => {
    const { calls } = mockApi(routes());
    renderApp('/admin');
    const user = userEvent.setup();
    const server = await screen.findByRole('region', { name: 'Server' });
    const restart = within(server).getByRole('button', { name: 'Neu starten' });
    act(() => restart.focus());
    await user.keyboard('{Enter}');
    const dialog = await screen.findByRole('alertdialog', { name: 'Server neu starten?' });
    expect(dialog).toHaveTextContent(
      '1 laufender Stream bricht ab. Laufende Downloads für unterwegs werden unterbrochen.',
    );
    await user.click(within(dialog).getByRole('button', { name: 'Abbrechen' }));
    await waitFor(() => expect(screen.queryByRole('alertdialog')).not.toBeInTheDocument());
    await waitFor(() => expect(restart).toHaveFocus());
    expect(calls.some((call) => call.path === '/admin/server/restart')).toBe(false);
  });

  it('restart without streams: the waiting panel replaces the cards, title and sidebar stay', async () => {
    const { calls } = mockApi(
      routes({
        'GET /admin/dashboard': () => json(quiet()),
        'POST /admin/server/restart': () => json({ restartAt: '2026-10-21T09:30:01Z' }, 202),
        // The old process still answers: the panel keeps waiting.
        'GET /healthz': () => json({ status: 'ok', version: '0.9.0', uptimeSeconds: 99_999 }),
      }),
    );
    renderApp('/admin');
    const user = userEvent.setup();
    await user.click(await screen.findByRole('button', { name: 'Neu starten' }));
    const dialog = await screen.findByRole('alertdialog', { name: 'Server neu starten?' });
    expect(dialog).toHaveTextContent(
      'Gerade läuft nichts. Laufende Downloads für unterwegs werden unterbrochen.',
    );
    await user.click(within(dialog).getByRole('button', { name: 'Neu starten' }));

    const heading = await screen.findByRole('heading', { level: 2, name: 'Server startet neu' });
    await waitFor(() => expect(heading).toHaveFocus());
    expect(
      calls.filter((call) => call.method === 'POST' && call.path === '/admin/server/restart'),
    ).toHaveLength(1);
    expect(screen.getByRole('status')).toHaveTextContent('Warte auf den Server …');
    expect(screen.queryByRole('region', { name: 'Server' })).not.toBeInTheDocument();
    expect(screen.queryByRole('region', { name: 'Hinweise' })).not.toBeInTheDocument();
    expect(screen.getByRole('heading', { level: 1, name: 'Verwaltung' })).toBeVisible();
    expect(screen.getByRole('navigation', { name: 'Verwaltung' })).toBeVisible();
  });

  it('a restart that went through reloads the page once the new server answers', async () => {
    vi.mocked(reloadPage).mockClear();
    let restarted = false;
    let answers = 0;
    mockApi(
      routes({
        'GET /admin/dashboard': () => json(quiet()),
        'POST /admin/server/restart': () => {
          restarted = true;
          return json({ restartAt: '2026-10-21T09:30:01Z' }, 202);
        },
        // Before the restart and right after the 202 the old process answers; then the new one.
        'GET /healthz': () => {
          if (restarted) answers += 1;
          const old = !restarted || answers === 1;
          return json({ status: 'ok', version: '0.9.0', uptimeSeconds: old ? 99_999 : 0 });
        },
      }),
    );
    renderApp('/admin');
    const user = userEvent.setup();
    await user.click(await screen.findByRole('button', { name: 'Neu starten' }));
    const dialog = await screen.findByRole('alertdialog', { name: 'Server neu starten?' });
    await user.click(within(dialog).getByRole('button', { name: 'Neu starten' }));
    expect(await screen.findByRole('heading', { name: 'Server startet neu' })).toBeVisible();
    expect(reloadPage).not.toHaveBeenCalled();
    await waitFor(() => expect(reloadPage).toHaveBeenCalledTimes(1), { timeout: 5000 });
    expect(screen.getByRole('status')).toHaveTextContent('Der Server ist zurück.');
  });

  it('a restart already on its way shows the waiting panel instead of an error', async () => {
    mockApi(routes({ 'POST /admin/server/restart': () => problem(409, 'restart_pending') }));
    renderApp('/admin');
    const user = userEvent.setup();
    await user.click(await screen.findByRole('button', { name: 'Neu starten' }));
    const dialog = await screen.findByRole('alertdialog', { name: 'Server neu starten?' });
    await user.click(within(dialog).getByRole('button', { name: 'Neu starten' }));
    expect(await screen.findByRole('heading', { name: 'Server startet neu' })).toBeVisible();
    expect(screen.queryByRole('alert')).not.toBeInTheDocument();
    expect(screen.queryByRole('region', { name: 'Server' })).not.toBeInTheDocument();
  });

  it('a refused restart says why and keeps the cards', async () => {
    mockApi(routes({ 'POST /admin/server/restart': () => problem(409, 'backup_busy') }));
    renderApp('/admin');
    const user = userEvent.setup();
    await user.click(await screen.findByRole('button', { name: 'Neu starten' }));
    const dialog = await screen.findByRole('alertdialog', { name: 'Server neu starten?' });
    await user.click(within(dialog).getByRole('button', { name: 'Neu starten' }));
    expect(await screen.findByRole('alert')).toHaveTextContent(
      'Gerade läuft eine Sicherung oder Wiederherstellung.',
    );
    expect(screen.getByRole('region', { name: 'Server' })).toBeVisible();
  });

  it('«Nach Updates suchen» asks the server and reads the dashboard again; a refusal shows', async () => {
    let enabled = true;
    const { calls } = mockApi(
      routes({
        'POST /admin/update/check': () =>
          enabled ? json(dashboard().update) : problem(422, 'update_check_disabled'),
      }),
    );
    renderApp('/admin');
    const user = userEvent.setup();
    const check = await screen.findByRole('button', { name: 'Nach Updates suchen' });
    const polls = () => calls.filter((call) => call.path === '/admin/dashboard').length;
    const before = polls();
    await user.click(check);
    expect(await screen.findByText('Nach Updates gesucht.')).toBeVisible();
    await waitFor(() => expect(polls()).toBeGreaterThan(before));
    enabled = false;
    await user.click(check);
    expect(await screen.findByRole('alert')).toHaveTextContent(
      'Die Update-Prüfung ist ausgeschaltet.',
    );
    expect(screen.queryByText('Nach Updates gesucht.')).not.toBeInTheDocument();
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
    // The head no longer claims to be live.
    expect(screen.getByText('Keine Antwort, neuer Versuch alle 5 Sekunden')).toBeVisible();
    expect(screen.queryByText('Live, alle 5 Sekunden')).not.toBeInTheDocument();
    fail = false;
    await user.click(screen.getByRole('button', { name: 'Erneut versuchen' }));
    expect(await screen.findByRole('region', { name: 'Aufgaben' })).toBeVisible();
  });
});

/** Every entry of the sidebar, in its order, with its path (11.1: 20 entries). */
const SIDEBAR = [
  ['Zurück zur App', '/'],
  ['Übersicht', '/admin'],
  ['Bibliotheken', '/admin/libraries'],
  ['Podcasts', '/admin/podcasts'],
  ['Pflege', '/admin/maintenance'],
  ['Metadaten', '/admin/metadata'],
  ['Konten', '/admin/accounts'],
  ['Profile', '/admin/accounts?item=me#meine-profile'],
  ['Geräte', '/admin/devices'],
  ['Streams', '/admin/streams'],
  ['Spotlight', '/admin/spotlight'],
  ["Cat's Empfehlungen", '/admin/picks'],
  ['Sammlungen', '/admin/collections'],
  ['Startseite', '/admin/home-rows'],
  ['Ankündigungen', '/admin/announcements'],
  ['Hintergrundjobs', '/admin/jobs'],
  ['Integrationen', '/admin/integrations'],
  ['Benachrichtigungen', '/settings/notifications'],
  ['Statistiken', '/admin/stats'],
  ['Sicherung', '/admin/backup'],
] as const;

describe('admin sidebar (A-2, concept 11)', () => {
  afterEach(() => {
    vi.unstubAllGlobals();
    localStorage.clear();
  });

  it('stands in the column with 20 entries and their numbers; ↑ ↓ Home End move, one Tab stop', async () => {
    mockApi(routes({ 'GET /admin/devices': () => json([]) }));
    const { nav } = await renderAdminPage('/admin/devices');
    const links = within(nav).getAllByRole('link');
    expect(links.map((link) => link.getAttribute('href'))).toEqual(SIDEBAR.map(([, href]) => href));
    expect(links.map((link) => link.querySelector('span')?.textContent)).toEqual(
      SIDEBAR.map(([name]) => name),
    );
    for (const group of ['Bibliothek', 'Konten', 'Kuratierung', 'System']) {
      expect(within(nav).getByRole('list', { name: group })).toBeVisible();
    }
    // F-7: few muted counts, coloured pills for what needs a look, each with a spoken name.
    expect(
      await within(nav).findByRole('link', { name: 'Bibliotheken, 1.200 Titel' }),
    ).toHaveTextContent('1.200');
    expect(
      within(nav).getByRole('link', { name: 'Pflege, 4 Titel brauchen Pflege' }),
    ).toBeVisible();
    expect(within(nav).getByRole('link', { name: 'Konten, 3 Konten' })).toBeVisible();
    expect(within(nav).getByRole('link', { name: 'Streams, 1 läuft' })).toBeVisible();
    expect(
      within(nav).getByRole('link', { name: 'Hintergrundjobs, 1 fehlgeschlagen' }),
    ).toBeVisible();
    expect(within(nav).getByRole('link', { name: 'Integrationen, 1 Webhook still' })).toBeVisible();
    expect(within(nav).getByRole('link', { name: 'Podcasts' })).toBeVisible();

    const devices = within(nav).getByRole('link', { name: 'Geräte' });
    expect(devices).toHaveAttribute('aria-current', 'page');
    // Roving tabIndex: only the current page's link is in the Tab order.
    expect(links.filter((link) => link.tabIndex === 0)).toEqual([devices]);

    devices.focus();
    fireEvent.keyDown(devices, { key: 'ArrowDown' });
    const streams = within(nav).getByRole('link', { name: 'Streams, 1 läuft' });
    expect(streams).toHaveFocus();
    expect(streams.tabIndex).toBe(0);
    expect(devices.tabIndex).toBe(-1);
    fireEvent.keyDown(streams, { key: 'ArrowUp' });
    expect(devices).toHaveFocus();
    fireEvent.keyDown(devices, { key: 'End' });
    expect(within(nav).getByRole('link', { name: 'Sicherung' })).toHaveFocus();
    // The arrows wrap, like the old navigation.
    fireEvent.keyDown(document.activeElement!, { key: 'ArrowDown' });
    expect(within(nav).getByRole('link', { name: 'Zurück zur App' })).toHaveFocus();
    fireEvent.keyDown(document.activeElement!, { key: 'ArrowUp' });
    expect(within(nav).getByRole('link', { name: 'Sicherung' })).toHaveFocus();
    fireEvent.keyDown(document.activeElement!, { key: 'Home' });
    expect(within(nav).getByRole('link', { name: 'Zurück zur App' })).toHaveFocus();
    // Leaving the column puts the Tab stop back on the current page.
    act(() => (document.activeElement as HTMLElement).blur());
    expect(devices.tabIndex).toBe(0);
  });

  it('shows the entries without numbers while the dashboard cannot be read', async () => {
    const { calls } = mockApi({ ...signedIn, 'GET /admin/devices': () => json([]) });
    const { nav, queryClient } = await renderAdminPage('/admin/devices');
    // The dashboard was asked and answered with an error before the entries are checked.
    await waitFor(() => expect(calls.some((call) => call.path === '/admin/dashboard')).toBe(true));
    await waitFor(() =>
      expect(queryClient.getQueryState(['admin', 'dashboard'])?.status).toBe('error'),
    );
    expect(within(nav).getAllByRole('link')).toHaveLength(20);
    expect(within(nav).getByRole('link', { name: 'Bibliotheken' })).toHaveTextContent(
      /^Bibliotheken$/,
    );
    expect(within(nav).getByRole('link', { name: 'Integrationen' })).toBeVisible();
  });

  it('counts running jobs and set-up integrations when nothing failed or went silent', async () => {
    mockApi(
      routes({
        'GET /admin/dashboard': () => json(quiet({ jobs: { ...quiet().jobs, running: 2 } })),
        'GET /admin/devices': () => json([]),
      }),
    );
    const { nav } = await renderAdminPage('/admin/devices');
    expect(
      await within(nav).findByRole('link', { name: 'Hintergrundjobs, 2 laufen' }),
    ).toHaveTextContent('2');
    expect(
      within(nav).getByRole('link', { name: 'Integrationen, 1 von 4 eingerichtet' }),
    ).toHaveTextContent('1 / 4');
    expect(within(nav).getByRole('link', { name: 'Pflege' })).toBeVisible();
    expect(within(nav).getByRole('link', { name: 'Streams' })).toBeVisible();
  });

  it('Enter follows a link and the sidebar stays; maintenance stays current on a title page', async () => {
    mockApi(
      routes({
        'GET /admin/devices': () => json([]),
        'GET /admin/streams': () => json([]),
      }),
    );
    const { nav, router } = await renderAdminPage('/admin/devices');
    const user = userEvent.setup();
    await user.click(within(nav).getByRole('link', { name: /^Streams/ }));
    await waitFor(() => expect(router.state.location.pathname).toBe('/admin/streams'));
    expect(within(nav).getByRole('link', { name: /^Streams/ })).toHaveAttribute(
      'aria-current',
      'page',
    );
    // The shell keeps the same sidebar between the admin pages.
    expect(screen.getByRole('navigation', { name: 'Verwaltung' })).toBe(nav);
    await act(() => router.navigate({ to: '/admin/items/$itemId', params: { itemId: 'x1' } }));
    expect(within(nav).getByRole('link', { name: /^Pflege/ })).toHaveAttribute(
      'aria-current',
      'page',
    );
  });

  it('«Profile» opens the own account in «Konten»; `item=me` makes it the current entry (KP-1)', async () => {
    mockApi(
      routes({
        'GET /admin/devices': () => json([]),
        // The own account (the session's id 1), so `item=me` names a row.
        'GET /admin/accounts': () =>
          json([
            {
              id: 1,
              email: 'owner@example.test',
              role: 'admin',
              isOwner: true,
              maxRating: 18,
              allLibraries: true,
              libraryIds: [],
              profiles: [],
              createdAt: '2026-10-01T10:00:00Z',
              limits: {},
              channels: [],
            },
          ]),
        'GET /admin/invitations': () => json([]),
        'GET /libraries': () => json([]),
        'GET /profiles': () => json([]),
      }),
    );
    const { nav, router } = await renderAdminPage('/admin/devices');
    const user = userEvent.setup();
    const profiles = within(nav).getByRole('link', { name: /^Profile/ });
    await user.click(profiles);
    await waitFor(() => expect(router.state.location.pathname).toBe('/admin/accounts'));
    expect(router.state.location.search).toEqual({ item: 'me' });
    expect(router.state.location.hash).toBe('meine-profile');
    expect(profiles).toHaveAttribute('aria-current', 'page');
    const accounts = within(nav).getByRole('link', { name: /^Konten/ });
    expect(accounts).not.toHaveAttribute('aria-current');
    // The same page without the item is «Konten».
    await user.click(accounts);
    await waitFor(() => expect(router.state.location.search).toEqual({}));
    expect(accounts).toHaveAttribute('aria-current', 'page');
    expect(profiles).not.toHaveAttribute('aria-current');
  });

  it('replaces the app column on /admin/*, also when it is hidden; the menu opens the app drawer', async () => {
    localStorage.setItem('slouchcat.sidenav', 'hidden');
    mockApi(
      routes({ 'GET /admin/devices': () => json([]), 'GET /home': () => json({ rows: [] }) }),
    );
    const { router } = await renderAdminPage('/admin/devices');
    expect(document.documentElement.dataset.shell).toBe('admin');
    expect(screen.queryByRole('navigation', { name: 'Hauptnavigation' })).not.toBeInTheDocument();
    const user = userEvent.setup();
    await user.click(screen.getByRole('button', { name: 'Menü' }));
    const drawer = await screen.findByRole('dialog', { name: 'Menü' });
    expect(within(drawer).getByRole('link', { name: 'Verwaltung' })).toHaveAttribute(
      'aria-current',
      'page',
    );
    // Showing the app's column would change nothing here.
    expect(
      within(drawer).queryByRole('button', { name: 'Seitenleiste einblenden' }),
    ).not.toBeInTheDocument();
    await user.keyboard('{Escape}');

    // Outside /admin the app's own shell is back.
    await act(() => router.navigate({ to: '/' }));
    await waitFor(() =>
      expect(screen.queryByRole('navigation', { name: 'Verwaltung' })).not.toBeInTheDocument(),
    );
    expect(document.documentElement.dataset.shell).toBeUndefined();
  });

  it('on a phone: the button «Verwaltung · page» opens the entries as a sheet', async () => {
    vi.stubGlobal('matchMedia', (query: string) => ({
      matches: false,
      media: query,
      addEventListener: () => undefined,
      removeEventListener: () => undefined,
    }));
    mockApi(routes({ 'GET /admin/devices': () => json([]) }));
    renderApp('/admin/devices');
    const user = userEvent.setup();
    const button = await screen.findByRole('button', { name: 'Verwaltung · Geräte' });
    expect(screen.queryByRole('navigation', { name: 'Verwaltung' })).not.toBeInTheDocument();
    await user.click(button);
    const sheet = await screen.findByRole('dialog', { name: 'Seiten der Verwaltung' });
    const nav = within(sheet).getByRole('navigation', { name: 'Verwaltung' });
    expect(within(nav).getAllByRole('link')).toHaveLength(20);
    expect(within(nav).getByRole('link', { name: 'Geräte' })).toHaveFocus();
    await user.keyboard('{Escape}');
    await waitFor(() => expect(screen.queryByRole('dialog')).not.toBeInTheDocument());
    expect(button).toHaveFocus();
  });

  it('a profile that is not the main account primary gets the notice without the sidebar', async () => {
    const guest = {
      ...session,
      account: { ...session.account, role: 'user' as const, isOwner: false },
    };
    const { calls } = mockApi({ 'GET /auth/session': () => json(guest) });
    renderApp('/admin');
    expect(await screen.findByText('Nur für Administratoren')).toBeVisible();
    expect(screen.queryByRole('navigation', { name: 'Verwaltung' })).not.toBeInTheDocument();
    expect(
      within(screen.getByRole('navigation', { name: 'Hauptnavigation' })).queryByRole('link', {
        name: 'Verwaltung',
      }),
    ).not.toBeInTheDocument();
    expect(document.documentElement.dataset.shell).toBeUndefined();
    expect(calls.some((call) => call.path.startsWith('/admin/'))).toBe(false);
  });
});

describe('update badge (A-5)', () => {
  afterEach(() => vi.unstubAllGlobals());

  it("shows the main account the badge on the sidebar's «Übersicht», linking to the overview", async () => {
    mockApi({
      ...routes({ 'GET /admin/devices': () => json([]) }),
      'GET /auth/session': () =>
        json({ ...session, updateAvailable: { version: '0.9.1', url: 'https://example.test' } }),
    });
    const { nav, router } = await renderAdminPage('/admin/devices');
    const user = userEvent.setup();
    const badge = within(nav).getByRole('link', {
      name: 'Übersicht, Update auf Version 0.9.1 verfügbar',
    });
    expect(badge).toHaveTextContent('Update');
    await user.click(badge);
    await waitFor(() => expect(router.state.location.pathname).toBe('/admin'));
  });

  it("outside the administration the app column's «Verwaltung» keeps the badge", async () => {
    mockApi({
      'GET /auth/session': () =>
        json({ ...session, updateAvailable: { version: '0.9.1', url: 'https://example.test' } }),
      'GET /home': () => json({ rows: [] }),
    });
    renderApp('/');
    const nav = await screen.findByRole('navigation', { name: 'Hauptnavigation' });
    expect(
      within(nav).getByRole('link', { name: 'Verwaltung, Update auf Version 0.9.1 verfügbar' }),
    ).toHaveAttribute('href', '/admin');
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
