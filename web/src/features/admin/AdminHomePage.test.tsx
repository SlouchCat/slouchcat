import type { Dashboard, SystemInfo } from '@slouchcat/api-client';
import { act, fireEvent, screen, waitFor, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { json, mockApi, problem, renderApp, session, signedIn, type Routes } from '@/test/api';
import { dashboardFixture, renderAdminPage } from '@/test/admin';

// Swiss numbers as the app formats German (de-CH; concept 22 section 4.4).
const swiss = (value: number) => new Intl.NumberFormat('de-CH').format(value);

// The administration (concept 11, A-2, A-3; redesign, build plan 1 to 3): the sidebar in the
// shell's column on every /admin page (numbers after F-7, one Tab stop, ↑ ↓ Home End), the sheet
// on a phone, and the overview asked every 5 s: the band, four key figures, what needs attention,
// what is active, the areas and the server in short. The server's functions in full are tested on
// System › Server (ServerPage.test.tsx).

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
        kind: 'movie',
        source: 'file',
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
      updaterConfigured: false,
      updaterAvailable: false,
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
    integrations: [
      { key: 'tmdb', state: 'configured' },
      { key: 'email', state: 'missing' },
    ],
    webhooks: { total: 0, eventsLast24h: 0, silent: [] },
    errors: [],
    update: {
      enabled: true,
      currentVersion: '0.9.0',
      development: false,
      available: false,
      updaterConfigured: false,
      updaterAvailable: false,
    },
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
    candidates: [],
  },
  inotify: { maxUserWatches: 65536, watchedDirs: 58982, usagePercent: 90, warn: true },
  configFs: 'ext4',
  timeZone: 'Europe/Zurich',
  cacheFs: 'ext4',
};

const routes = (extra: Routes = {}): Routes => ({
  ...signedIn,
  'GET /admin/dashboard': () => json(dashboard()),
  'GET /admin/system': () => json(system),
  ...extra,
});

/** The attention card of the overview. */
const attentionCard = () => screen.findByRole('region', { name: 'Was benötigt Aufmerksamkeit?' });

/** The titles of the hints in a region, in their order (not the failures listed under one). */
const hintTitles = (region: HTMLElement) =>
  Array.from(region.querySelectorAll('li > span:first-child > span:last-child')).map(
    (title) => title.textContent,
  );

const termsOf = (region: HTMLElement) =>
  within(region)
    .getAllByRole('term')
    .map((term) => [term.textContent, term.nextElementSibling?.textContent]);

describe('admin overview (concept 11, A-3; build plan 3)', () => {
  afterEach(() => {
    vi.useRealTimers();
    vi.unstubAllGlobals();
  });

  it('shows the band, four key figures, what needs attention, what is active, the areas and the server in short', async () => {
    mockApi(routes());
    renderApp('/admin');
    expect(await screen.findByRole('heading', { level: 1, name: 'Alles im Blick' })).toBeVisible();
    expect(screen.getByText('Übersicht', { selector: 'p' })).toBeVisible();
    expect(
      screen.getByText('Wichtiges zuerst. Alles Weitere erreichst du über die Bereiche.'),
    ).toBeVisible();
    expect(screen.getByText('Live, alle 5 Sekunden')).toBeVisible();
    // The overview has no area tabs.
    expect(screen.queryByRole('navigation', { name: /^Seiten im Bereich/ })).toBeNull();

    // What needs attention, most urgent first: the failed jobs with their types, the integration
    // with its reason, the silent webhook, the warning of the system, the new version.
    const attention = await attentionCard();
    await waitFor(() =>
      expect(hintTitles(attention)).toEqual([
        '1 Hintergrundjob fehlgeschlagen',
        'Trakt: Fehler',
        'Seit sieben Tagen ohne Meldung: Sonarr',
        'Das inotify-Limit ist knapp',
        'Version 0.9.1 ist verfügbar',
      ]),
    );
    expect(screen.getByText('5 Punkte benötigen Aufmerksamkeit')).toBeVisible();
    const failed = within(attention).getByRole('list', { name: 'Fehlgeschlagen je Jobart' });
    expect(within(failed).getByText('Titel zuordnen: 1 fehlgeschlagen')).toBeVisible();
    expect(within(failed).getByText('tmdb: HTTP 503')).toBeVisible();
    expect(within(attention).getByText('HTTP 503')).toBeVisible();
    expect(
      within(attention).getByRole('link', { name: 'Integrationen einrichten' }),
    ).toHaveAttribute('href', '/admin/integrations');
    // The silent webhooks lead to the webhooks' own view of the integrations page.
    expect(within(attention).getByRole('link', { name: 'Webhooks verwalten' })).toHaveAttribute(
      'href',
      '/admin/integrations?item=webhooks',
    );
    expect(
      within(attention).getByRole('group', { name: 'Befehl für das inotify-Limit' }),
    ).toHaveTextContent('sysctl fs.inotify.max_user_watches=524288');

    // Four key figures, each a link to its page; storage with both paths (F-4).
    const metrics = screen.getByRole('region', { name: 'Kennzahlen' });
    expect(
      within(metrics)
        .getAllByRole('link')
        .map((link) => [link.textContent, link.getAttribute('href')]),
    ).toEqual([
      ['Aktive Streams1Gerade verbunden', '/admin/streams'],
      ['Laufende Jobs112 wartend', '/admin/jobs'],
      ['Titel zur Pflege4Offene Fälle in der Pflege', '/admin/maintenance'],
      ['Speicherplatz40 % · 10 %Belegt: Konfiguration · Cache', '/admin/server'],
    ]);

    const active = screen.getByRole('region', { name: 'Gerade aktiv' });
    const running = within(active).getByRole('list', { name: 'Laufende Aufgaben' });
    expect(
      within(running).getByText('Keyframe-Index (ganze Datei lesen) · Oppenheimer (2023)'),
    ).toBeVisible();
    expect(
      within(running).getByRole('progressbar', {
        name: 'Fortschritt Keyframe-Index (ganze Datei lesen)',
      }),
    ).toHaveAttribute('aria-valuenow', '30');
    expect(within(running).getByText('12 von 40')).toBeVisible();
    expect(within(running).getByText('noch ca. 3 min')).toBeVisible();
    const streams = within(active).getByRole('list', { name: 'Jetzt läuft' });
    expect(within(streams).getByText('Heat')).toBeVisible();
    expect(within(streams).getByText('Marco · Wohnzimmer')).toBeVisible();
    expect(within(streams).getByText('Transcode (HW)')).toBeVisible();
    expect(within(streams).getByRole('progressbar', { name: 'Fortschritt Heat' })).toHaveAttribute(
      'aria-valuenow',
      '25',
    );
    expect(within(streams).getByText('0:30:00 / 2:00:00')).toBeVisible();
    expect(within(active).getByRole('link', { name: 'Hintergrundjobs' })).toHaveAttribute(
      'href',
      '/admin/jobs',
    );
    // Concept 12 (JOB-8, 11.5 point 11): the failures of every type are a tab of the jobs page.
    expect(within(active).getByRole('link', { name: 'Fehlschläge ansehen' })).toHaveAttribute(
      'href',
      '/admin/jobs?tab=failed',
    );
    expect(within(active).getByRole('link', { name: 'Alle Streams' })).toHaveAttribute(
      'href',
      '/admin/streams',
    );
    // F-2: no table with «Erneut» and «Abbrechen» on the overview.
    expect(screen.queryByRole('table')).not.toBeInTheDocument();

    // The areas, each to its first page.
    const quick = screen.getByRole('region', { name: 'Direkt zu deinen Aufgaben' });
    expect(
      within(quick)
        .getAllByRole('link')
        .map((link) => link.getAttribute('href')),
    ).toEqual([
      '/admin/libraries',
      '/admin/accounts',
      '/admin/home-rows',
      '/admin/jobs',
      '/admin/stats',
    ]);
    expect(within(quick).getByRole('link', { name: 'System' })).toHaveAccessibleDescription(
      'Statistiken, Sicherung, Server',
    );

    // The server in short; everything in full on System › Server.
    const summary = screen.getByRole('region', { name: 'Server & letzte Aktivitäten' });
    expect(within(summary).getAllByRole('link', { name: 'Serverdetails' })[0]).toHaveAttribute(
      'href',
      '/admin/server',
    );
    const state = within(summary).getByRole('region', { name: 'Serverzustand' });
    await waitFor(() =>
      expect(termsOf(state)).toEqual([
        ['Version', '0.9.0+abc1234'],
        ['Läuft seit', expect.any(String)],
        ['Updates', 'Version 0.9.1 verfügbar'],
        ['Integrationen', '1 von 4 eingerichtet'],
        ['Webhooks', '3 Meldungen im Protokoll der letzten 24 Stunden, von 2 Integrationen.'],
        ['Server-Protokoll', '2 Fehler'],
      ]),
    );
    expect(within(state).getByRole('link', { name: 'Server-Protokoll ansehen' })).toHaveAttribute(
      'href',
      '/admin/server',
    );
    const activity = within(summary).getByRole('list', { name: 'Letzte Aktivität' });
    expect(within(activity).getByRole('listitem')).toHaveTextContent('Marco hat «Alien» gestartet');
    // The server's functions are not on the overview any more.
    expect(screen.queryByRole('button', { name: 'Neu starten' })).not.toBeInTheDocument();
    expect(screen.queryByRole('button', { name: 'Nach Updates suchen' })).not.toBeInTheDocument();
  });

  it('«Ansehen» in the band moves the focus to what needs attention', async () => {
    mockApi(routes());
    renderApp('/admin');
    const user = userEvent.setup();
    await user.click(await screen.findByRole('button', { name: 'Ansehen' }));
    expect(screen.getByRole('group', { name: 'Was benötigt Aufmerksamkeit?' })).toHaveFocus();
  });

  it('«Aktualisieren» asks the dashboard again at once', async () => {
    const { calls } = mockApi(routes());
    renderApp('/admin');
    await attentionCard();
    const polls = () => calls.filter((call) => call.path === '/admin/dashboard').length;
    const before = polls();
    const user = userEvent.setup();
    await user.click(screen.getByRole('button', { name: 'Aktualisieren' }));
    await waitFor(() => expect(polls()).toBeGreaterThan(before));
  });

  it('a running update install is a point of attention that leads to the server', async () => {
    mockApi(
      routes({
        'GET /admin/dashboard': () =>
          json(
            quiet({
              update: {
                ...quiet().update,
                install: { phase: 'pulling', from: '0.9.0', to: '0.9.1', migrated: false },
              },
            }),
          ),
        'GET /admin/system': () => json({ ...system, inotify: { ...system.inotify, warn: false } }),
      }),
    );
    renderApp('/admin');
    const attention = await attentionCard();
    await waitFor(() => expect(hintTitles(attention)).toEqual(['Update auf 0.9.1 läuft']));
    expect(within(attention).getByRole('link', { name: 'Serverdetails' })).toHaveAttribute(
      'href',
      '/admin/server',
    );
  });

  it('says once that ratings are minimum ages now and closes the hint on «Verstanden» (concept 22)', async () => {
    let notice = true;
    const { calls } = mockApi(
      routes({
        'GET /admin/dashboard': () =>
          json(
            dashboard({
              attention: {
                abortedScans: [],
                rejectedKeys: [],
                ...(notice ? { minimumAgeNotice: true } : {}),
                ratingsRereading: true,
              },
            }),
          ),
        'DELETE /admin/dashboard/minimum-age-notice': () => {
          notice = false;
          return new Response(null, { status: 204 });
        },
      }),
    );
    renderApp('/admin');
    const hints = await attentionCard();
    expect(await within(hints).findByText('Freigaben sind jetzt Mindestalter')).toBeVisible();
    expect(within(hints).getByText(/PG-13 ist «ab 13» statt FSK 12/)).toBeVisible();
    expect(within(hints).getByText('Freigaben werden neu gelesen')).toBeVisible();
    const user = userEvent.setup();
    await user.click(within(hints).getByRole('button', { name: 'Verstanden' }));
    await waitFor(() =>
      expect(within(hints).queryByText('Freigaben sind jetzt Mindestalter')).toBeNull(),
    );
    expect(calls.filter((call) => call.method === 'DELETE')).toHaveLength(1);
    // The reading goes on: its hint stays until the job is done.
    expect(within(hints).getByText('Freigaben werden neu gelesen')).toBeVisible();
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
                rejectedKeys: ['tvdb'],
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
    const hints = await attentionCard();
    await waitFor(() => expect(hintTitles(hints)).toHaveLength(11));
    expect(hintTitles(hints)).toEqual([
      '265 Hintergrundjobs fehlgeschlagen',
      '2 Jobarten pausiert',
      'TVDB-Schlüssel abgewiesen',
      'Trakt: Fehler',
      'Seit sieben Tagen ohne Meldung: Sonarr',
      'Scan der Bibliothek «Filme» abgebrochen',
      'Ordner-Browser blockiert',
      'Kein Grafikchip erkannt',
      'Das inotify-Limit ist knapp',
      '/config ist zu über 90 % belegt',
      'Version 0.9.1 ist verfügbar',
    ]);
    expect(screen.getByText('11 Punkte benötigen Aufmerksamkeit')).toBeVisible();
    // The failures hint opens the tab «Fehlschläge», the paused hint the page.
    expect(
      within(hints)
        .getAllByRole('link', { name: 'Zu den Hintergrundjobs' })
        .map((link) => link.getAttribute('href')),
    ).toEqual(['/admin/jobs?tab=failed', '/admin/jobs']);
    // A rejected provider key (concept 21 P-2) leads to the keys.
    expect(within(hints).getByRole('link', { name: 'Zu den Schlüsseln' })).toHaveAttribute(
      'href',
      '/admin/integrations?item=keys',
    );
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
    // The head's number is the live region (the command blocks have their own for «kopiert»).
    expect(within(hints).getByText('11 Hinweise').closest('[role="status"]')).toHaveTextContent(
      '11 Hinweise',
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
                updaterConfigured: false,
                updaterAvailable: false,
              },
            }),
          ),
      }),
    );
    renderApp('/admin');
    const hints = await attentionCard();
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
      within(screen.getByRole('region', { name: 'Serverzustand' })).getByText('Entwicklungsstand'),
    ).toBeVisible();
  });

  it('when all is quiet: nothing plays, the next run, all is well, no errors, no activity', async () => {
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
    expect(screen.getByText('keine Fehler')).toBeVisible();
    const hints = await attentionCard();
    await waitFor(() => expect(within(hints).getByText('Alles in Ordnung.')).toBeVisible());
    expect(within(hints).getByRole('status')).toHaveTextContent('0 Hinweise');
    expect(screen.queryByText(/benötigen? Aufmerksamkeit$/)).not.toBeInTheDocument();
    expect(screen.queryByRole('button', { name: 'Ansehen' })).not.toBeInTheDocument();
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
    expect(await screen.findByRole('region', { name: 'Gerade aktiv' })).toBeVisible();
  });
});

/** The column (build plan 2.2): back, the overview and the five areas, each to its first page. */
const SIDEBAR = [
  ['Zurück zur App', '/'],
  ['Übersicht', '/admin'],
  ['Medien', '/admin/libraries'],
  ['Nutzer & Geräte', '/admin/accounts'],
  ['Startseite', '/admin/home-rows'],
  ['Automatisierung', '/admin/jobs'],
  ['System', '/admin/stats'],
] as const;

/**
 * Every page of the administration in the order of the sheet on a phone (build plan 1, 2.3):
 * back, the overview, then the five areas with all their pages (11.1: 19 pages; concept 16 adds
 * «Radio», concept 21 «Meine Medien», the redesign «Hörbuch-Quellen»).
 */
const SHEET = [
  ['Zurück zur App', '/'],
  ['Übersicht', '/admin'],
  ['Bibliotheken', '/admin/libraries'],
  ['Metadaten', '/admin/metadata'],
  ['Pflege', '/admin/maintenance'],
  ['Podcasts', '/admin/podcasts'],
  ['Radio', '/admin/radio'],
  ['Hörbuch-Quellen', '/admin/metadata/audiobooks'],
  ['Konten', '/admin/accounts'],
  ['Geräte', '/admin/devices'],
  ['Streams', '/admin/streams'],
  ['Startseite', '/admin/home-rows'],
  ['Spotlight', '/admin/spotlight'],
  ['User-Empfehlungen', '/admin/picks'],
  ['Sammlungen', '/admin/collections'],
  ['Meine Medien', '/admin/my-media'],
  ['Ankündigungen', '/admin/announcements'],
  ['Hintergrundjobs', '/admin/jobs'],
  ['Integrationen', '/admin/integrations'],
  ['Benachrichtigungen', '/admin/notifications'],
  ['Statistiken', '/admin/stats'],
  ['Sicherung', '/admin/backup'],
  ['Server', '/admin/server'],
] as const;

/** The tabs of the page's area (build plan 2.5). */
const areaTabs = (area: string) =>
  screen.getByRole('navigation', { name: `Seiten im Bereich ${area}` });

/** A phone: no wide screen, so the admin pages have the sheet instead of the column. */
const stubPhone = () =>
  vi.stubGlobal('matchMedia', (query: string) => ({
    matches: false,
    media: query,
    addEventListener: () => undefined,
    removeEventListener: () => undefined,
  }));

describe('admin sidebar (A-2, concept 11; build plan 1, 2)', () => {
  afterEach(() => {
    vi.unstubAllGlobals();
    localStorage.clear();
  });

  it('stands in the column with 7 entries and the areas’ pills; ↑ ↓ Home End move, one Tab stop', async () => {
    mockApi(routes({ 'GET /admin/devices': () => json([]) }));
    const { nav } = await renderAdminPage('/admin/devices');
    const links = within(nav).getAllByRole('link');
    expect(links.map((link) => link.getAttribute('href'))).toEqual(SIDEBAR.map(([, href]) => href));
    expect(links.map((link) => link.querySelector('span')?.textContent)).toEqual(
      SIDEBAR.map(([name]) => name),
    );
    expect(within(nav).getByRole('list', { name: 'Verwaltung' })).toBeVisible();
    // Build plan 1: an area shows only what needs a look, with the spoken name of the number.
    expect(
      await within(nav).findByRole('link', { name: 'Medien, 4 Titel brauchen Pflege' }),
    ).toHaveTextContent('4');
    expect(
      within(nav).getByRole('link', { name: 'Automatisierung, 1 fehlgeschlagen' }),
    ).toBeVisible();
    expect(within(nav).getByRole('link', { name: 'System' })).toBeVisible();
    // The active profile with its role at the foot of the column.
    expect(screen.getByText('Server-Admin')).toBeVisible();

    const people = within(nav).getByRole('link', { name: 'Nutzer & Geräte, 1 läuft' });
    expect(people).toHaveAttribute('aria-current', 'page');
    // Roving tabIndex: only the current area's link is in the Tab order.
    expect(links.filter((link) => link.tabIndex === 0)).toEqual([people]);

    people.focus();
    fireEvent.keyDown(people, { key: 'ArrowDown' });
    const home = within(nav).getByRole('link', { name: 'Startseite' });
    expect(home).toHaveFocus();
    expect(home.tabIndex).toBe(0);
    expect(people.tabIndex).toBe(-1);
    fireEvent.keyDown(home, { key: 'ArrowUp' });
    expect(people).toHaveFocus();
    fireEvent.keyDown(people, { key: 'End' });
    expect(within(nav).getByRole('link', { name: 'System' })).toHaveFocus();
    // The arrows wrap, like the old navigation.
    fireEvent.keyDown(document.activeElement!, { key: 'ArrowDown' });
    expect(within(nav).getByRole('link', { name: 'Zurück zur App' })).toHaveFocus();
    fireEvent.keyDown(document.activeElement!, { key: 'ArrowUp' });
    expect(within(nav).getByRole('link', { name: 'System' })).toHaveFocus();
    fireEvent.keyDown(document.activeElement!, { key: 'Home' });
    expect(within(nav).getByRole('link', { name: 'Zurück zur App' })).toHaveFocus();
    // Leaving the column puts the Tab stop back on the current page.
    act(() => (document.activeElement as HTMLElement).blur());
    expect(people.tabIndex).toBe(0);
  });

  it('puts the area’s pages as tabs above the page, with their numbers', async () => {
    mockApi(routes({ 'GET /admin/devices': () => json([]) }));
    await renderAdminPage('/admin/devices');
    const tabs = await screen.findByRole('navigation', {
      name: 'Seiten im Bereich Nutzer & Geräte',
    });
    expect(
      within(tabs)
        .getAllByRole('link')
        .map((link) => link.getAttribute('href')),
    ).toEqual(['/admin/accounts', '/admin/devices', '/admin/streams']);
    expect(await within(tabs).findByRole('link', { name: 'Konten, 3 Konten' })).toHaveTextContent(
      'Konten3',
    );
    expect(within(tabs).getByRole('link', { name: 'Geräte' })).toHaveAttribute(
      'aria-current',
      'page',
    );
    expect(within(tabs).getByRole('link', { name: 'Streams, 1 läuft' })).toHaveTextContent(
      'Streams1',
    );
    // The line over the title and the breadcrumb of the header say where the page stands.
    expect(screen.getByText('Nutzer & Geräte / Geräte')).toBeVisible();
    const crumbs = screen.getByRole('navigation', { name: 'Pfad' });
    expect(within(crumbs).getByRole('link', { name: 'Verwaltung' })).toHaveAttribute(
      'href',
      '/admin',
    );
    expect(within(crumbs).getByRole('link', { name: 'Nutzer & Geräte' })).toHaveAttribute(
      'href',
      '/admin/accounts',
    );
    expect(within(crumbs).getByText('Geräte')).toHaveAttribute('aria-current', 'page');
  });

  it('shows the entries without numbers while the dashboard cannot be read', async () => {
    const { calls } = mockApi({ ...signedIn, 'GET /admin/devices': () => json([]) });
    const { nav, queryClient } = await renderAdminPage('/admin/devices');
    // The dashboard was asked and answered with an error before the entries are checked.
    await waitFor(() => expect(calls.some((call) => call.path === '/admin/dashboard')).toBe(true));
    await waitFor(() =>
      expect(queryClient.getQueryState(['admin', 'dashboard'])?.status).toBe('error'),
    );
    expect(within(nav).getAllByRole('link')).toHaveLength(SIDEBAR.length);
    expect(within(nav).getByRole('link', { name: 'Medien' })).toHaveTextContent(/^Medien$/);
    expect(
      within(areaTabs('Nutzer & Geräte')).getByRole('link', { name: 'Konten' }),
    ).toHaveTextContent(/^Konten$/);
  });

  it('counts running jobs and set-up integrations when nothing failed or went silent', async () => {
    stubPhone();
    mockApi(
      routes({
        'GET /admin/dashboard': () => json(quiet({ jobs: { ...quiet().jobs, running: 2 } })),
        'GET /admin/devices': () => json([]),
      }),
    );
    renderApp('/admin/devices');
    const user = userEvent.setup();
    await user.click(
      await screen.findByRole('button', { name: 'Verwaltung · Nutzer & Geräte / Geräte' }),
    );
    const sheet = await screen.findByRole('dialog', { name: 'Seiten der Verwaltung' });
    expect(
      await within(sheet).findByRole('link', { name: 'Hintergrundjobs, 2 laufen' }),
    ).toHaveTextContent('2');
    expect(
      within(sheet).getByRole('link', { name: 'Integrationen, 1 von 2 eingerichtet' }),
    ).toHaveTextContent('1 / 2');
    expect(within(sheet).getByRole('link', { name: 'Pflege' })).toBeVisible();
    expect(within(sheet).getByRole('link', { name: 'Streams' })).toBeVisible();
  });

  it('Enter follows a tab and the sidebar stays; maintenance stays current on a title page', async () => {
    mockApi(
      routes({
        'GET /admin/devices': () => json([]),
        'GET /admin/streams': () => json([]),
      }),
    );
    const { nav, router } = await renderAdminPage('/admin/devices');
    const user = userEvent.setup();
    await user.click(
      await within(areaTabs('Nutzer & Geräte')).findByRole('link', { name: /^Streams/ }),
    );
    await waitFor(() => expect(router.state.location.pathname).toBe('/admin/streams'));
    expect(
      await within(areaTabs('Nutzer & Geräte')).findByRole('link', { name: /^Streams/ }),
    ).toHaveAttribute('aria-current', 'page');
    expect(within(nav).getByRole('link', { name: /^Nutzer & Geräte/ })).toHaveAttribute(
      'aria-current',
      'page',
    );
    // The shell keeps the same sidebar between the admin pages.
    expect(screen.getByRole('navigation', { name: 'Verwaltung' })).toBe(nav);
    await act(() => router.navigate({ to: '/admin/items/$itemId', params: { itemId: 'x1' } }));
    expect(within(nav).getByRole('link', { name: /^Medien/ })).toHaveAttribute(
      'aria-current',
      'page',
    );
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

  it('on a phone: the button «Verwaltung › area / page» opens every page as a sheet', async () => {
    stubPhone();
    mockApi(routes({ 'GET /admin/devices': () => json([]) }));
    renderApp('/admin/devices');
    const user = userEvent.setup();
    const button = await screen.findByRole('button', {
      name: 'Verwaltung · Nutzer & Geräte / Geräte',
    });
    expect(screen.queryByRole('navigation', { name: 'Verwaltung' })).not.toBeInTheDocument();
    await user.click(button);
    const sheet = await screen.findByRole('dialog', { name: 'Seiten der Verwaltung' });
    const nav = within(sheet).getByRole('navigation', { name: 'Verwaltung' });
    const links = within(nav).getAllByRole('link');
    expect(links.map((link) => link.getAttribute('href'))).toEqual(SHEET.map(([, href]) => href));
    expect(links.map((link) => link.querySelector('span')?.textContent)).toEqual(
      SHEET.map(([name]) => name),
    );
    for (const area of ['Medien', 'Nutzer & Geräte', 'Startseite', 'Automatisierung', 'System']) {
      expect(within(nav).getByRole('list', { name: area })).toBeVisible();
    }
    // F-7: the pages keep their numbers, each with a spoken name.
    expect(
      await within(nav).findByRole('link', { name: `Bibliotheken, ${swiss(1200)} Titel` }),
    ).toHaveTextContent(swiss(1200));
    expect(
      within(nav).getByRole('link', { name: 'Pflege, 4 Titel brauchen Pflege' }),
    ).toBeVisible();
    expect(within(nav).getByRole('link', { name: 'Konten, 3 Konten' })).toBeVisible();
    expect(within(nav).getByRole('link', { name: 'Streams, 1 läuft' })).toBeVisible();
    expect(
      within(nav).getByRole('link', { name: 'Hintergrundjobs, 1 fehlgeschlagen' }),
    ).toBeVisible();
    expect(within(nav).getByRole('link', { name: 'Integrationen, 1 Webhook still' })).toBeVisible();
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
    // The same badge on the area «System» and its tab «Server» (build plan 1).
    expect(
      within(nav).getByRole('link', { name: 'System, Update auf Version 0.9.1 verfügbar' }),
    ).toHaveTextContent('Update');
    await user.click(badge);
    await waitFor(() => expect(router.state.location.pathname).toBe('/admin'));
  });

  it('puts the badge on the tab «Server» of the area «System»', async () => {
    mockApi({
      ...routes(),
      'GET /auth/session': () =>
        json({ ...session, updateAvailable: { version: '0.9.1', url: 'https://example.test' } }),
    });
    renderApp('/admin/server');
    const tabs = await screen.findByRole('navigation', { name: 'Seiten im Bereich System' });
    const server = within(tabs).getByRole('link', {
      name: 'Server, Update auf Version 0.9.1 verfügbar',
    });
    expect(server).toHaveAttribute('href', '/admin/server');
    expect(server).toHaveTextContent('Update');
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
