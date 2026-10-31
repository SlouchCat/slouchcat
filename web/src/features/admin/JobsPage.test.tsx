import type { JobOverview, JobRun, JobTypeInfo } from '@slouchcat/api-client';
import { screen, waitFor, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { json, mockApi, problem, renderApp, session, signedIn, type Routes } from '@/test/api';

// The page "Hintergrundjobs" (concept 09): counters, the running runs with progress and
// remaining time, one card per type with its lines and buttons. "Pausieren" in a row of
// "Läuft gerade" pauses the job of the run: one call with the type, and every row of that type
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
    type({ type: 'jobs.prune', group: 'system', runnable: true }),
  ],
});

const routes = (extra: Routes = {}): Routes => ({
  ...signedIn,
  'GET /admin/job-types': () => json(overview()),
  ...extra,
});

describe('admin jobs page (concept 09)', () => {
  afterEach(() => {
    vi.useRealTimers();
    vi.unstubAllGlobals();
  });

  it('shows counters, pools, running runs and one card per type', async () => {
    mockApi(routes());
    renderApp('/admin/jobs');
    expect(await screen.findByRole('heading', { level: 1, name: 'Hintergrundjobs' })).toBeVisible();
    const terms = await screen.findAllByRole('term');
    expect(
      terms.map((term) => [term.textContent, term.nextElementSibling?.textContent]).slice(0, 5),
    ).toEqual([
      ['Laufend', '2'],
      ['Wartend', '239'],
      ['Geplant', '2'],
      ['Pausiert', '0'],
      ['Fehlgeschlagen', '1'],
    ]);
    const pools = screen.getByRole('list', { name: 'Arbeiter' });
    expect(within(pools).getByText(/Allgemein: 4 Arbeiter · Stau ca. 3 h 10 min/)).toBeVisible();
    expect(within(pools).getByText(/Downloads: 1 Arbeiter · frei/)).toBeVisible();

    const running = screen.getByRole('region', { name: 'Läuft gerade' });
    const rows = within(running).getAllByRole('listitem');
    expect(rows).toHaveLength(2);
    expect(within(rows[0]!).getByText('Filme · Movies/2024')).toBeVisible();
    expect(within(rows[0]!).getByText('1240 von 3980')).toBeVisible();
    expect(within(rows[0]!).getByText('noch ca. 12 min')).toBeVisible();
    expect(
      within(rows[0]!).getByRole('progressbar', { name: 'Bibliotheksscan: 31 %' }),
    ).toBeVisible();
    expect(within(rows[1]!).getByText('länger als üblich')).toBeVisible();

    const scan = screen.getByRole('article', { name: /Bibliotheksscan/ });
    expect(within(scan).getByText('läuft')).toBeVisible();
    expect(within(scan).getByText(/3 · dran in ca. 2 h 40 min · üblich 3 min/)).toBeVisible();
    expect(within(scan).getByText(/^in .* · alle 6 h$/)).toBeVisible();
    expect(
      within(scan).getByRole('button', { name: 'Bibliotheksscan jetzt ausführen' }),
    ).toBeVisible();
    expect(
      within(scan).getByRole('button', { name: 'Zeitplan von Bibliotheksscan' }),
    ).toBeVisible();
    expect(
      within(scan).getByRole('button', { name: 'Wartende Läufe von Bibliotheksscan abbrechen' }),
    ).toHaveTextContent('Wartende abbrechen (3)');

    // An event type: no run button, no schedule, "bei Bedarf".
    const match = screen.getByRole('article', { name: /Titel zuordnen/ });
    expect(within(match).queryByRole('button', { name: /jetzt ausführen/ })).toBeNull();
    expect(within(match).queryByRole('button', { name: /Zeitplan/ })).toBeNull();
    expect(within(match).getByText('bei Bedarf')).toBeVisible();
    expect(within(match).getByText(/236 · geplant 2/)).toBeVisible();
    expect(within(match).getByText('wartet')).toBeVisible();
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
    const running = await screen.findByRole('region', { name: 'Läuft gerade' });
    const rows = within(running).getAllByRole('listitem');
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
      const after = within(screen.getByRole('region', { name: 'Läuft gerade' })).getAllByRole(
        'listitem',
      );
      expect(after.map((row) => within(row).queryByText('wird angehalten') !== null)).toEqual([
        true,
        true,
      ]);
    });
    // The banner names the paused job with "Fortsetzen"; the card offers it instead of "Pausieren".
    expect(screen.getByText('1 Job pausiert')).toBeVisible();
    const scan = screen.getByRole('article', { name: /Bibliotheksscan/ });
    expect(within(scan).getByRole('button', { name: 'Bibliotheksscan fortsetzen' })).toBeVisible();
    expect(
      within(scan).queryByRole('button', { name: 'Bibliotheksscan jetzt ausführen' }),
    ).toBeNull();
    expect(within(scan).getByText('pausiert')).toBeVisible();
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
    const scan = await screen.findByRole('article', { name: /Bibliotheksscan/ });
    const user = userEvent.setup();
    await user.click(within(scan).getByRole('button', { name: 'Bibliotheksscan jetzt ausführen' }));
    expect(await screen.findByText('Bibliotheksscan: 2 Läufe eingereiht.')).toBeVisible();
    expect(calls.filter((call) => call.method === 'POST').map((call) => call.path)).toEqual([
      '/admin/job-types/scan.library/run',
    ]);
    const prune = screen.getByRole('article', { name: /Verlauf aufräumen/ });
    await user.click(
      within(prune).getByRole('button', { name: 'Verlauf aufräumen jetzt ausführen' }),
    );
    expect(await screen.findByText('Der Job ist pausiert; setze ihn zuerst fort.')).toBeVisible();
  });

  it('"Wartende abbrechen" asks first; Escape keeps them, the confirmation cancels with scope queued', async () => {
    const { calls } = mockApi(
      routes({
        'DELETE /admin/job-types/metadata.match/runs': () => json({ cancelled: 238 }),
      }),
    );
    renderApp('/admin/jobs');
    const match = await screen.findByRole('article', { name: /Titel zuordnen/ });
    const user = userEvent.setup();
    const button = within(match).getByRole('button', {
      name: 'Wartende Läufe von Titel zuordnen abbrechen',
    });
    await user.click(button);
    const dialog = await screen.findByRole('alertdialog', {
      name: 'Wartende Läufe von Titel zuordnen abbrechen?',
    });
    expect(
      within(dialog).getByText('238 wartende Läufe werden abgebrochen und nicht nachgeholt.'),
    ).toBeVisible();
    await user.keyboard('{Escape}');
    await waitFor(() => expect(screen.queryByRole('alertdialog')).toBeNull());
    await waitFor(() => expect(button).toHaveFocus());
    expect(calls.filter((call) => call.method === 'DELETE')).toEqual([]);
    await user.click(button);
    await user.click(
      within(await screen.findByRole('alertdialog')).getByRole('button', { name: 'Abbrechen' }),
    );
    expect(await screen.findByText('Titel zuordnen: 238 Läufe abgebrochen.')).toBeVisible();
    expect(calls.filter((call) => call.method === 'DELETE').map((call) => call.path)).toEqual([
      '/admin/job-types/metadata.match/runs',
    ]);
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
