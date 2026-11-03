import type { JobOverview, JobRun, JobSummary, JobTypeInfo } from '@slouchcat/api-client';
import { act, screen, waitFor, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { dashboardFixture } from '@/test/admin';
import { json, mockApi, problem, renderApp, session, signedIn, type Routes } from '@/test/api';

// The page "Hintergrundjobs" (concept 09; concept 12 variant C): counters, «Läuft gerade» with
// progress and remaining time, one tab per group with a row per type that folds out into its
// buttons, its schedule and its runs, and the tab «Fehlschläge» (JOB-8). "Pausieren" in a row of
// «Läuft gerade» pauses the job of the run: one call with the type, and every row of that type
// shows "wird angehalten" afterwards (F-10). "Jetzt ausführen" queues runs; an event type offers
// no run button; a paused type offers "Fortsetzen" and is named in the banner.

const run = (overrides: Partial<JobRun>): JobRun => ({
  id: 1,
  type: 'scan.library',
  state: 'running',
  priority: 5,
  attempts: 1,
  maxAttempts: 5,
  updatedAt: '2026-10-30T12:00:00Z',
  startedAt: '2026-10-30T11:58:00Z',
  durationSeconds: 120,
  skipped: false,
  estimate: 'none',
  ...overrides,
});

const type = (overrides: Partial<JobTypeInfo>): JobTypeInfo => ({
  type: 'scan.library',
  group: 'library',
  trigger: 'schedule',
  pool: 'general',
  paused: false,
  stopping: false,
  runnable: true,
  counts: { queued: 0, planned: 0, running: 0, failed: 0 },
  running: [],
  known: true,
  ...overrides,
});

/** A long error: the row cuts it, the fold-out shows it whole (S-5). */
const PRUNE_ERROR =
  'prune: database is locked (SQLITE_BUSY) while deleting runs older than 30 days; retry later';

const scanRuns: JobRun[] = [
  run({
    id: 41,
    subject: 'Filme',
    progress: { done: 1240, total: 3980, note: 'Filme · Movies/2024' },
    estimatedSecondsLeft: 720,
    estimate: 'measured',
  }),
  run({ id: 42, subject: 'Serien', estimate: 'median' }),
];

const overview = (stopping = false): JobOverview => ({
  pools: [
    { pool: 'general', workers: 4, backlogSeconds: 11_400 },
    { pool: 'download', workers: 1 },
    { pool: 'notify', workers: 1 },
  ],
  paused: stopping ? 1 : 0,
  types: [
    type({
      paused: stopping,
      stopping,
      counts: { queued: 3, planned: 0, running: 2, failed: 0 },
      running: scanRuns,
      schedule: {
        everySeconds: 21_600,
        nextRunAt: '2026-10-30T16:00:00Z',
        lastFiredAt: '2026-10-30T10:00:00Z',
        enabled: true,
        timeZone: 'Europe/Zurich',
        limits: { minEverySeconds: 300, maxEverySeconds: 604_800 },
      },
      lastRun: run({
        id: 40,
        state: 'done',
        finishedAt: '2026-10-30T10:03:00Z',
        durationSeconds: 180,
      }),
      medianDurationSeconds: 190,
      queuedEstimateSeconds: 9600,
    }),
    type({
      type: 'metadata.match',
      group: 'metadata',
      trigger: 'event',
      runnable: false,
      counts: { queued: 236, planned: 2, running: 0, failed: 1 },
    }),
    type({ type: 'metadata.apply', group: 'metadata', trigger: 'manual', runnable: false }),
    type({
      type: 'jobs.prune',
      group: 'system',
      runnable: true,
      counts: { queued: 0, planned: 0, running: 0, failed: 1 },
      lastRun: run({
        id: 60,
        type: 'jobs.prune',
        state: 'failed',
        finishedAt: '2026-10-30T11:00:00Z',
        durationSeconds: 2,
        lastError: PRUNE_ERROR,
      }),
    }),
  ],
});

const failedJob = (overrides: Partial<JobSummary>): JobSummary => ({
  id: 70,
  type: 'metadata.match',
  state: 'failed',
  attempts: 5,
  maxAttempts: 5,
  lastError: 'tmdb: HTTP 503',
  updatedAt: '2026-10-30T11:00:00Z',
  ...overrides,
});

const routes = (extra: Routes = {}): Routes => ({
  ...signedIn,
  'GET /admin/job-types': () => json(overview()),
  'GET /admin/dashboard': () =>
    json(
      dashboardFixture({
        jobs: {
          queued: 239,
          running: 2,
          failed: 2,
          recentFailed: [failedJob({}), failedJob({ id: 71, type: 'probe.keyframes' })],
          active: [],
          failedByType: [],
        },
      }),
    ),
  ...extra,
});

/** The panel of the page's selected tab (the outer one; a fold-out holds tabs of its own). */
const groupPanel = () => screen.getAllByRole('tabpanel')[0]!;

/** Folds out the row of a job type and returns its fold-out. */
async function openRow(user: ReturnType<typeof userEvent.setup>, name: string) {
  const toggle = within(groupPanel()).getByRole('button', { name });
  if (toggle.getAttribute('aria-expanded') !== 'true') await user.click(toggle);
  const detail = document.getElementById(toggle.getAttribute('aria-controls') ?? '');
  if (!detail) throw new Error(`no fold-out for ${name}`);
  return detail;
}

describe('admin jobs page (concept 09, concept 12)', () => {
  afterEach(() => {
    vi.useRealTimers();
    vi.unstubAllGlobals();
  });

  it('shows counters, pools, running runs and one row per type in its group', async () => {
    mockApi(routes());
    renderApp('/admin/jobs');
    expect(await screen.findByRole('heading', { level: 1, name: 'Hintergrundjobs' })).toBeVisible();
    const strip = await screen.findByRole('group', { name: 'Zusammenfassung' });
    const terms = within(strip).getAllByRole('term');
    expect(terms.map((term) => [term.textContent, term.nextElementSibling?.textContent])).toEqual([
      ['Laufend', '2'],
      ['Wartend', '239'],
      ['Geplant', '2'],
      ['Pausiert', '0'],
      ['Fehlgeschlagen', '2'],
    ]);
    const pools = screen.getByRole('list', { name: 'Arbeiter' });
    expect(within(pools).getByText(/Allgemein: 4 Arbeiter · Stau ca. 3 h 10 min/)).toBeVisible();
    expect(within(pools).getByText(/Downloads: 1 Arbeiter · frei/)).toBeVisible();

    // «Läuft gerade» folds away like any group; it is open at first.
    expect(screen.getByRole('button', { name: 'Läuft gerade 2 Läufe' })).toHaveAttribute(
      'aria-expanded',
      'true',
    );
    const running = screen.getByRole('table', { name: 'Läuft gerade' });
    const rows = within(running).getAllByRole('row').slice(1);
    expect(rows).toHaveLength(2);
    expect(within(rows[0]!).getByText('Filme · Movies/2024')).toBeVisible();
    expect(within(rows[0]!).getByText('1240 von 3980')).toBeVisible();
    expect(within(rows[0]!).getByText('noch ca. 12 min')).toBeVisible();
    expect(
      within(rows[0]!).getByRole('progressbar', { name: 'Bibliotheksscan: 31 %' }),
    ).toBeVisible();
    expect(within(rows[1]!).getByText('länger als üblich')).toBeVisible();

    // The tabs: four groups with the number of their types, and the failures with the total.
    const tabs = screen.getByRole('tablist', { name: 'Jobs nach Gruppe' });
    expect(
      within(tabs)
        .getAllByRole('tab')
        .map((tab) => tab.textContent),
    ).toEqual([
      'Bibliothek 11 Jobart',
      'Metadaten 22 Jobarten',
      'Zustellung 00 Jobarten',
      'System 11 Jobart',
      'Fehlschläge 22 fehlgeschlagen',
    ]);
    expect(within(tabs).getByRole('tab', { selected: true })).toHaveAccessibleName(
      'Bibliothek 1 Jobart',
    );

    const user = userEvent.setup();
    const scanRow = within(groupPanel()).getByRole('row', { name: /^Bibliotheksscan/ });
    expect(within(scanRow).getByText('läuft')).toBeVisible();
    expect(within(scanRow).getByText(/3 · dran in ca. 2 h 40 min · üblich 3 min/)).toBeVisible();
    expect(within(scanRow).getByText(/^in .* · alle 6 h$/)).toBeVisible();
    const scan = await openRow(user, 'Bibliotheksscan');
    expect(
      within(scan).getByRole('button', { name: 'Bibliotheksscan jetzt ausführen' }),
    ).toBeVisible();
    expect(
      within(scan).getByRole('region', { name: 'Zeitplan von Bibliotheksscan' }),
    ).toBeVisible();
    expect(within(scan).getByText('scan.library')).toBeVisible();
    await user.click(
      within(scan).getByRole('button', { name: 'Weitere Aktionen für Bibliotheksscan' }),
    );
    const menu = await screen.findByRole('menu');
    expect(
      within(menu)
        .getAllByRole('menuitem')
        .map((item) => item.textContent),
    ).toEqual(['Wartende abbrechen (3)', 'Alle Läufe abbrechen']);
    await user.keyboard('{Escape}');

    // An event type: no run button, no schedule, "bei Bedarf"; metadata.apply has its name.
    await user.click(within(tabs).getByRole('tab', { name: /^Metadaten/ }));
    const matchRow = within(groupPanel()).getByRole('row', { name: /^Titel zuordnen/ });
    expect(within(matchRow).getByText('bei Bedarf')).toBeVisible();
    expect(within(matchRow).getByText(/236 · geplant 2/)).toBeVisible();
    expect(within(matchRow).getByText('wartet')).toBeVisible();
    const match = await openRow(user, 'Titel zuordnen');
    expect(within(match).queryByRole('button', { name: /jetzt ausführen/ })).toBeNull();
    expect(within(match).queryByRole('region', { name: /Zeitplan/ })).toBeNull();
    expect(within(groupPanel()).getByRole('button', { name: 'Metadaten anwenden' })).toBeVisible();
  });

  it('"Pausieren" in a running row pauses the job once; both rows show "wird angehalten"', async () => {
    let paused = false;
    const { calls } = mockApi(
      routes({
        'GET /admin/job-types': () => json(overview(paused)),
        'POST /admin/job-types/scan.library/pause': () => {
          paused = true;
          return json(overview(true).types[0]);
        },
      }),
    );
    renderApp('/admin/jobs');
    const running = await screen.findByRole('table', { name: 'Läuft gerade' });
    const rows = within(running).getAllByRole('row').slice(1);
    const user = userEvent.setup();
    await user.click(
      within(rows[0]!).getByRole('button', {
        name: 'Bibliotheksscan pausieren (hält diesen und alle laufenden Läufe des Jobs an)',
      }),
    );
    await waitFor(() =>
      expect(calls.filter((call) => call.method === 'POST')).toEqual([
        expect.objectContaining({ path: '/admin/job-types/scan.library/pause' }),
      ]),
    );
    expect(
      await screen.findByText(
        'Bibliotheksscan pausiert; laufende Läufe halten an und gehen in die Warteschlange zurück.',
      ),
    ).toBeVisible();
    await waitFor(() => {
      const after = within(screen.getByRole('table', { name: 'Läuft gerade' }))
        .getAllByRole('row')
        .slice(1);
      expect(after.map((row) => within(row).queryByText('wird angehalten') !== null)).toEqual([
        true,
        true,
      ]);
    });
    // The banner names the paused job with "Fortsetzen"; the row offers it instead of "Pausieren".
    expect(screen.getByText('1 Job pausiert')).toBeVisible();
    const scanRow = within(groupPanel()).getByRole('row', { name: /^Bibliotheksscan/ });
    expect(within(scanRow).getByText('wird angehalten')).toBeVisible();
    const scan = await openRow(user, 'Bibliotheksscan');
    expect(within(scan).getByRole('button', { name: 'Bibliotheksscan fortsetzen' })).toBeVisible();
    expect(
      within(scan).queryByRole('button', { name: 'Bibliotheksscan jetzt ausführen' }),
    ).toBeNull();
  });

  it('"Jetzt ausführen" queues runs and reports how many; a refusal shows the problem', async () => {
    const { calls } = mockApi(
      routes({
        'POST /admin/job-types/scan.library/run': () =>
          json(
            [
              run({ id: 50, state: 'queued', priority: 7 }),
              run({ id: 51, state: 'queued', priority: 7 }),
            ],
            202,
          ),
        'POST /admin/job-types/jobs.prune/run': () => problem(409, 'job_type_paused'),
      }),
    );
    renderApp('/admin/jobs');
    await screen.findByRole('tablist', { name: 'Jobs nach Gruppe' });
    const user = userEvent.setup();
    const scan = await openRow(user, 'Bibliotheksscan');
    await user.click(within(scan).getByRole('button', { name: 'Bibliotheksscan jetzt ausführen' }));
    expect(await screen.findByText('Bibliotheksscan: 2 Läufe eingereiht.')).toBeVisible();
    expect(calls.filter((call) => call.method === 'POST').map((call) => call.path)).toEqual([
      '/admin/job-types/scan.library/run',
    ]);
    await user.click(screen.getByRole('tab', { name: /^System/ }));
    const prune = await openRow(user, 'Verlauf aufräumen');
    // The row cuts the last run's error to one line; the fold-out shows all of it (S-5).
    const pruneRow = within(groupPanel()).getByRole('row', { name: /^Verlauf aufräumen/ });
    const endsWithError = (text: string) => text.endsWith(`Fehler: ${PRUNE_ERROR}`);
    expect(within(pruneRow).getByTitle(endsWithError)).toBeVisible();
    expect(within(prune).getByText(endsWithError)).toBeVisible();
    await user.click(
      within(prune).getByRole('button', { name: 'Verlauf aufräumen jetzt ausführen' }),
    );
    expect(await screen.findByText('Der Job ist pausiert; setze ihn zuerst fort.')).toBeVisible();
  });

  it('"Wartende abbrechen" in the menu asks first; Escape keeps them, the confirmation cancels with scope queued', async () => {
    const { calls } = mockApi(
      routes({
        'DELETE /admin/job-types/metadata.match/runs': () => json({ cancelled: 238 }),
      }),
    );
    renderApp('/admin/jobs?tab=metadata');
    await screen.findByRole('tablist', { name: 'Jobs nach Gruppe' });
    const user = userEvent.setup();
    const match = await openRow(user, 'Titel zuordnen');
    const menuButton = within(match).getByRole('button', {
      name: 'Weitere Aktionen für Titel zuordnen',
    });
    const pick = async () => {
      await user.click(menuButton);
      await user.click(await screen.findByRole('menuitem', { name: 'Wartende abbrechen (238)' }));
      return screen.findByRole('alertdialog', {
        name: 'Wartende Läufe von Titel zuordnen abbrechen?',
      });
    };
    const dialog = await pick();
    expect(
      within(dialog).getByText('238 wartende Läufe werden abgebrochen und nicht nachgeholt.'),
    ).toBeVisible();
    await user.keyboard('{Escape}');
    await waitFor(() => expect(screen.queryByRole('alertdialog')).toBeNull());
    await waitFor(() => expect(menuButton).toHaveFocus());
    expect(calls.filter((call) => call.method === 'DELETE')).toEqual([]);
    await user.click(within(await pick()).getByRole('button', { name: 'Abbrechen' }));
    expect(await screen.findByText('Titel zuordnen: 238 Läufe abgebrochen.')).toBeVisible();
    expect(calls.filter((call) => call.method === 'DELETE')).toEqual([
      expect.objectContaining({ path: '/admin/job-types/metadata.match/runs' }),
    ]);
  });

  it('the tab «Fehlschläge» lists the newest failures of every type with «Erneut» and «Abbrechen»', async () => {
    const { calls } = mockApi(
      routes({
        'POST /admin/jobs/70/retry': () => json(failedJob({ state: 'queued' })),
        'DELETE /admin/jobs/71': () => new Response(null, { status: 204 }),
      }),
    );
    const { router } = renderApp('/admin/jobs?tab=failed');
    const tab = await screen.findByRole('tab', { selected: true });
    expect(tab).toHaveAccessibleName('Fehlschläge 2 fehlgeschlagen');
    const table = await screen.findByRole('table', { name: 'Letzte Fehlschläge (alle Arten)' });
    const rows = within(table).getAllByRole('row').slice(1);
    expect(rows.map((row) => row.querySelector('th')?.textContent)).toEqual([
      'Titel zuordnen',
      'Keyframe-Index',
    ]);
    expect(within(rows[0]!).getByText('5 von 5')).toBeVisible();
    // The whole error, wrapped: readable without a pointer (S-5).
    expect(within(rows[0]!).getByText('tmdb: HTTP 503')).toBeVisible();
    expect(screen.getByText(/^2 von 2 · /)).toBeVisible();

    // «Erneut» queues the run again; its row goes, so the focus moves to the page's h1 (ALL-2).
    const user = userEvent.setup();
    await user.click(
      within(rows[0]!).getByRole('button', { name: 'Lauf 70 von Titel zuordnen erneut einreihen' }),
    );
    expect(await screen.findByText('Lauf 70 wartet wieder.')).toBeVisible();
    await waitFor(() =>
      expect(screen.getByRole('heading', { level: 1, name: 'Hintergrundjobs' })).toHaveFocus(),
    );

    // «Abbrechen» asks first.
    await user.click(
      within(rows[1]!).getByRole('button', { name: 'Lauf 71 von Keyframe-Index abbrechen' }),
    );
    const dialog = await screen.findByRole('alertdialog', {
      name: 'Lauf von Keyframe-Index abbrechen?',
    });
    expect(within(dialog).getByText('Slouchcat versucht diesen Lauf nicht mehr.')).toBeVisible();
    await user.click(within(dialog).getByRole('button', { name: 'Abbrechen' }));
    expect(await screen.findByText('Lauf 71 abgebrochen.')).toBeVisible();
    expect(
      calls.filter((call) => call.method !== 'GET').map((call) => `${call.method} ${call.path}`),
    ).toEqual(['POST /admin/jobs/70/retry', 'DELETE /admin/jobs/71']);

    // The tab lives in the address: another tab drops `?tab=failed`.
    await user.click(screen.getByRole('tab', { name: /^Bibliothek/ }));
    await waitFor(() => expect(router.state.location.search).toEqual({}));
    await user.click(screen.getByRole('tab', { name: /^System/ }));
    await waitFor(() => expect(router.state.location.search).toEqual({ tab: 'system' }));
  });

  it('keeps an unsaved schedule across tabs and folding (S-17), asks before leaving, and saves it', async () => {
    let saved = false;
    const { calls } = mockApi(
      routes({
        'GET /admin/job-types': () => {
          const data = overview();
          if (saved) data.types[0]!.schedule!.everySeconds = 7200;
          return json(data);
        },
        'PATCH /admin/job-types/scan.library/schedule': () => {
          saved = true;
          return json(overview().types[0]);
        },
      }),
    );
    renderApp('/admin/jobs');
    await screen.findByRole('tablist', { name: 'Jobs nach Gruppe' });
    const user = userEvent.setup();
    let scan = await openRow(user, 'Bibliotheksscan');
    const schedule = () =>
      within(groupPanel()).getByRole('region', { name: 'Zeitplan von Bibliotheksscan' });
    expect(within(schedule()).queryByRole('button', { name: 'Verwerfen' })).toBeNull();
    await user.selectOptions(within(schedule()).getByRole('combobox', { name: 'Takt' }), '7200');
    expect(within(schedule()).getByRole('button', { name: 'Verwerfen' })).toBeVisible();

    // Another tab and back, then folding the row away and out again: the draft stays.
    await user.click(screen.getByRole('tab', { name: /^System/ }));
    await user.click(screen.getByRole('tab', { name: /^Bibliothek/ }));
    scan = await openRow(user, 'Bibliotheksscan');
    await user.click(within(groupPanel()).getByRole('button', { name: 'Bibliotheksscan' }));
    scan = await openRow(user, 'Bibliotheksscan');
    expect(within(scan).getByRole('combobox', { name: 'Takt' })).toHaveValue('7200');
    // The select is a spatial item and had norigin's focus when the tab switch unmounted it, so
    // norigin restores its group 300 ms later and focuses the group's first item: the select of
    // the row folded out again. On a slow runner that landed after the click on «Speichern» below
    // and took its focus (toHaveFocus, run 41705097285). Back on the select, as a reader returning
    // to the field, norigin's focus is current again and the pending restore is cancelled.
    await user.click(within(scan).getByRole('combobox', { name: 'Takt' }));

    // Leaving the page with the draft asks; staying keeps it.
    await user.click(
      within(screen.getByRole('navigation', { name: 'Verwaltung' })).getByRole('link', {
        name: /^Integrationen/,
      }),
    );
    const leave = await screen.findByRole('alertdialog', {
      name: 'Seite mit ungespeicherten Änderungen verlassen?',
    });
    await user.click(within(leave).getByRole('button', { name: 'Auf der Seite bleiben' }));
    await waitFor(() => expect(screen.queryByRole('alertdialog')).toBeNull());
    expect(screen.getByRole('heading', { level: 1, name: 'Hintergrundjobs' })).toBeVisible();

    await user.click(within(schedule()).getByRole('button', { name: 'Speichern' }));
    expect(await screen.findByText('Zeitplan von Bibliotheksscan gespeichert.')).toBeVisible();
    expect(calls.filter((call) => call.method === 'PATCH')).toEqual([
      expect.objectContaining({
        path: '/admin/job-types/scan.library/schedule',
        body: { enabled: true, everySeconds: 7200 },
      }),
    ]);
    // The draft is spent: «Verwerfen» goes, the focus stays on «Speichern».
    await waitFor(() =>
      expect(within(schedule()).queryByRole('button', { name: 'Verwerfen' })).toBeNull(),
    );
    expect(within(schedule()).getByRole('button', { name: 'Speichern' })).toHaveFocus();
    expect(within(schedule()).getByRole('combobox', { name: 'Takt' })).toHaveValue('7200');
  });

  it('«Speichern» without a change sends nothing and keeps the focus', async () => {
    const { calls } = mockApi(routes());
    renderApp('/admin/jobs');
    await screen.findByRole('tablist', { name: 'Jobs nach Gruppe' });
    const user = userEvent.setup();
    await openRow(user, 'Bibliotheksscan');
    const schedule = within(groupPanel()).getByRole('region', {
      name: 'Zeitplan von Bibliotheksscan',
    });
    const save = within(schedule).getByRole('button', { name: 'Speichern' });
    expect(save).toBeEnabled();
    expect(within(schedule).queryByRole('button', { name: 'Verwerfen' })).toBeNull();
    await user.click(save);
    expect(save).toHaveFocus();
    expect(calls.filter((call) => call.method === 'PATCH')).toEqual([]);
  });

  it('keeps what is chosen while a schedule save waits for the server', async () => {
    let release = () => undefined as void;
    const answered = new Promise<void>((resolve) => {
      release = resolve;
    });
    let saved = false;
    const { calls } = mockApi(
      routes({
        'GET /admin/job-types': () => {
          const data = overview();
          if (saved) data.types[0]!.schedule!.everySeconds = 7200;
          return json(data);
        },
        'PATCH /admin/job-types/scan.library/schedule': async () => {
          await answered;
          saved = true;
          return json(overview().types[0]);
        },
      }),
    );
    renderApp('/admin/jobs');
    await screen.findByRole('tablist', { name: 'Jobs nach Gruppe' });
    const user = userEvent.setup();
    await openRow(user, 'Bibliotheksscan');
    const schedule = () =>
      within(groupPanel()).getByRole('region', { name: 'Zeitplan von Bibliotheksscan' });
    await user.selectOptions(within(schedule()).getByRole('combobox', { name: 'Takt' }), '7200');
    await user.click(within(schedule()).getByRole('button', { name: 'Speichern' }));
    await waitFor(() => expect(calls.filter((call) => call.method === 'PATCH')).toHaveLength(1));

    // Chosen while the save runs: it stays a draft once the answer is there.
    await user.selectOptions(within(schedule()).getByRole('combobox', { name: 'Takt' }), '43200');
    release();
    expect(await screen.findByText('Zeitplan von Bibliotheksscan gespeichert.')).toBeVisible();
    await waitFor(() =>
      expect(within(schedule()).getByRole('combobox', { name: 'Takt' })).toHaveValue('43200'),
    );
    expect(within(schedule()).getByRole('button', { name: 'Verwerfen' })).toBeVisible();
    expect(screen.getByText('Aktualisierung pausiert')).toBeVisible();
  });

  it('pauses the live indicator for a draft; a window switched off again is no draft; a 422 clears on change', async () => {
    const { calls } = mockApi(
      routes({
        'PATCH /admin/job-types/scan.library/schedule': () =>
          problem(422, 'validation_failed', {
            errors: [{ field: 'everySeconds', code: 'too_small' }],
          }),
      }),
    );
    renderApp('/admin/jobs');
    expect(await screen.findByText('Live, alle 5 Sekunden')).toBeVisible();
    const user = userEvent.setup();
    await openRow(user, 'Bibliotheksscan');
    const schedule = () =>
      within(groupPanel()).getByRole('region', { name: 'Zeitplan von Bibliotheksscan' });

    // A window with another start, switched off again: what the server holds, no draft.
    await user.click(within(schedule()).getByRole('switch', { name: 'Nur in einem Zeitfenster' }));
    const start = within(schedule()).getByLabelText('Von');
    await user.clear(start);
    await user.type(start, '01:30');
    expect(screen.getByText('Aktualisierung pausiert')).toBeVisible();
    expect(screen.queryByText('Live, alle 5 Sekunden')).toBeNull();
    await user.click(within(schedule()).getByRole('switch', { name: 'Nur in einem Zeitfenster' }));
    await waitFor(() =>
      expect(within(schedule()).queryByRole('button', { name: 'Verwerfen' })).toBeNull(),
    );
    expect(screen.getByText('Live, alle 5 Sekunden')).toBeVisible();

    // A refused save names the problem until the field changes.
    await user.selectOptions(within(schedule()).getByRole('combobox', { name: 'Takt' }), '7200');
    await user.click(within(schedule()).getByRole('button', { name: 'Speichern' }));
    expect(await within(schedule()).findByRole('alert')).toBeVisible();
    expect(calls.filter((call) => call.method === 'PATCH')).toHaveLength(1);
    await user.selectOptions(within(schedule()).getByRole('combobox', { name: 'Takt' }), '43200');
    await waitFor(() => expect(within(schedule()).queryByRole('alert')).toBeNull());
  });

  it('a confirmation stays when its row drops out meanwhile; closing it resumes polling and focuses the h1', async () => {
    let failures = [failedJob({})];
    mockApi(
      routes({
        'GET /admin/dashboard': () =>
          json(
            dashboardFixture({
              jobs: {
                queued: 0,
                running: 0,
                failed: failures.length,
                recentFailed: failures,
                active: [],
                failedByType: [],
              },
            }),
          ),
      }),
    );
    const { queryClient } = renderApp('/admin/jobs?tab=failed');
    const table = await screen.findByRole('table', { name: 'Letzte Fehlschläge (alle Arten)' });
    const user = userEvent.setup();
    await user.click(
      within(table).getByRole('button', { name: 'Lauf 70 von Titel zuordnen abbrechen' }),
    );
    const dialog = await screen.findByRole('alertdialog', {
      name: 'Lauf von Titel zuordnen abbrechen?',
    });
    expect(screen.getByText('Aktualisierung pausiert')).toBeVisible();
    // Another poll (the sidebar's) brings a list without the row.
    failures = [];
    await queryClient.invalidateQueries({ queryKey: ['admin', 'dashboard'] });
    expect(await screen.findByText('Keine Fehlschläge.')).toBeInTheDocument();
    expect(dialog).toBeVisible();
    await user.keyboard('{Escape}');
    await waitFor(() => expect(screen.queryByRole('alertdialog')).toBeNull());
    expect(screen.getByText('Live, alle 5 Sekunden')).toBeVisible();
    await waitFor(() =>
      expect(screen.getByRole('heading', { level: 1, name: 'Hintergrundjobs' })).toHaveFocus(),
    );
  });

  it('an unknown tab in the address shows «Bibliothek» and drops the parameter', async () => {
    mockApi(routes());
    const { router } = renderApp('/admin/jobs?tab=nonsense');
    const tabs = await screen.findByRole('tablist', { name: 'Jobs nach Gruppe' });
    expect(within(tabs).getByRole('tab', { selected: true })).toHaveAccessibleName(
      'Bibliothek 1 Jobart',
    );
    await waitFor(() => expect(router.state.location.search).toEqual({}));
    expect(
      within(groupPanel()).getByRole('heading', { level: 2, name: 'Bibliothek' }),
    ).toBeInTheDocument();
  });

  it('a group without job types says so', async () => {
    mockApi(routes());
    renderApp('/admin/jobs?tab=delivery');
    await screen.findByRole('tablist', { name: 'Jobs nach Gruppe' });
    expect(await screen.findByText('Keine Jobs in dieser Gruppe.')).toBeVisible();
  });

  it('shows the runs of a type in its row, one tab per state, with «Erneut» for a failed run', async () => {
    const { calls } = mockApi(
      routes({
        'GET /admin/jobs': () =>
          json({
            runs: [run({ id: 80, state: 'failed', subject: 'Alien', lastError: 'tmdb: 404' })],
            total: 1,
          }),
        'POST /admin/jobs/80/retry': () => json(failedJob({ id: 80, state: 'queued' })),
      }),
    );
    renderApp('/admin/jobs?tab=metadata');
    await screen.findByRole('tablist', { name: 'Jobs nach Gruppe' });
    const user = userEvent.setup();
    const match = await openRow(user, 'Titel zuordnen');
    const runs = within(match).getByRole('region', { name: 'Läufe von Titel zuordnen' });
    const states = within(runs).getByRole('tablist', { name: 'Läufe nach Zustand' });
    expect(
      within(states)
        .getAllByRole('tab')
        .map((tab) => tab.textContent),
    ).toEqual([
      'Laufend 0',
      'Wartend 236',
      'Geplant 2',
      'Fehlgeschlagen 1',
      'Erledigt',
      'Abgebrochen',
    ]);
    expect(within(states).getByRole('tab', { selected: true })).toHaveTextContent('Wartend');
    await user.click(within(states).getByRole('tab', { name: /^Fehlgeschlagen/ }));
    const table = await within(runs).findByRole('table', { name: 'Läufe von Titel zuordnen' });
    expect(within(table).getByText('tmdb: 404')).toBeVisible();
    await user.click(within(table).getByRole('button', { name: 'Lauf 80 erneut einreihen' }));
    expect(await screen.findByText('Lauf 80 wartet wieder.')).toBeVisible();
    expect(calls.filter((call) => call.method === 'POST').map((call) => call.path)).toEqual([
      '/admin/jobs/80/retry',
    ]);
  });

  it('the running time of a run ticks once a second (JOB-4a)', async () => {
    vi.useFakeTimers({ shouldAdvanceTime: true });
    vi.setSystemTime(Date.parse('2026-10-30T12:00:10Z'));
    const started = run({ id: 41, startedAt: '2026-10-30T12:00:00Z', durationSeconds: 0 });
    mockApi(
      routes({
        'GET /admin/job-types': () =>
          json({
            ...overview(),
            types: [
              type({
                counts: { queued: 0, planned: 0, running: 1, failed: 0 },
                running: [started],
              }),
            ],
          }),
      }),
    );
    renderApp('/admin/jobs');
    const running = await screen.findByRole('table', { name: 'Läuft gerade' });
    const since = () => {
      const text = within(running).getAllByRole('row')[1]!.textContent ?? '';
      const seconds = /seit (\d+) s/.exec(text);
      if (!seconds) throw new Error(`no running time in «${text}»`);
      return Number(seconds[1]);
    };
    // The clock also moves with real time (shouldAdvanceTime), so the steps are lower bounds.
    const first = since();
    expect(first).toBeGreaterThanOrEqual(10);
    await act(() => vi.advanceTimersByTimeAsync(1000));
    expect(since()).toBeGreaterThanOrEqual(first + 1);
    await act(() => vi.advanceTimersByTimeAsync(5000));
    expect(since()).toBeGreaterThanOrEqual(first + 6);
  });

  it('a type this page does not know has no actions, no schedule and no runs (JOB-6)', async () => {
    const { calls } = mockApi(
      routes({
        'GET /admin/job-types': () =>
          json({
            ...overview(),
            types: [
              ...overview().types,
              type({ type: 'legacy.sweep', group: 'system', known: false, runnable: true }),
            ],
          }),
      }),
    );
    renderApp('/admin/jobs?tab=system');
    await screen.findByRole('tablist', { name: 'Jobs nach Gruppe' });
    const user = userEvent.setup();
    const legacy = await openRow(user, 'legacy.sweep');
    expect(
      within(legacy).getByText('Dieser Server kennt den Job nicht mehr (älterer Stand).'),
    ).toBeVisible();
    expect(within(legacy).queryAllByRole('button')).toEqual([]);
    expect(within(legacy).queryByRole('region')).toBeNull();
    expect(calls.some((call) => call.path === '/admin/jobs')).toBe(false);
  });

  it('asks for at most 200 runs and names the total when there are more (JOB-7)', async () => {
    const { fetchMock } = mockApi(
      routes({
        'GET /admin/jobs': () =>
          json({
            runs: Array.from({ length: 200 }, (_, index) =>
              run({ id: 1000 + index, state: 'queued', subject: `Titel ${index}` }),
            ),
            total: 236,
          }),
      }),
    );
    renderApp('/admin/jobs?tab=metadata');
    await screen.findByRole('tablist', { name: 'Jobs nach Gruppe' });
    const user = userEvent.setup();
    const match = await openRow(user, 'Titel zuordnen');
    const runs = within(match).getByRole('region', { name: 'Läufe von Titel zuordnen' });
    const table = await within(runs).findByRole('table', { name: 'Läufe von Titel zuordnen' });
    expect(within(table).getAllByRole('row')).toHaveLength(201);
    // Once visible below the tabs, once for the screen reader.
    expect(within(runs).getAllByText('236 insgesamt, die neuesten 200 gezeigt')).toHaveLength(2);
    const asked = fetchMock.mock.calls
      .map(([request]) => new URL(request.url))
      .filter((url) => url.pathname.endsWith('/admin/jobs'));
    expect(asked.map((url) => url.searchParams.get('limit'))).toEqual(['200']);
  });

  it('refuses anyone but an administrator in the primary profile', async () => {
    const { calls } = mockApi(
      routes({
        'GET /auth/session': () =>
          json({ ...session, account: { ...session.account, role: 'user' } }),
      }),
    );
    renderApp('/admin/jobs');
    expect(await screen.findByText('Nur für Administratoren')).toBeVisible();
    expect(calls.some((call) => call.path === '/admin/job-types')).toBe(false);
  });
});
