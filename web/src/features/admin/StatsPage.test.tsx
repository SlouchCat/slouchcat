import type { Stats, StatTitle } from '@slouchcat/api-client';
import { fireEvent, screen, waitFor, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { ids } from '@/test/fixtures';
import { json, mockApi, problem, renderApp, session, signedIn } from '@/test/api';

// A-8 (concept 12, mockup Sta-B): «Wiedergabe» as the status strip, the most watched titles with
// places 1 to 3 as tiles and the rest in a table, watch time per profile and per device type, and
// «Wann geschaut wird» over 7, 30, 90 or 365 days. The server counts; the page shows its numbers
// and only adds up the quarter hours of the week.

const more = { dune: '01JB8Z6Q4M7N9P2R3S5T7V9W4A', bear: '01JB8Z6Q4M7N9P2R3S5T7V9W5B' };

const topTitles: StatTitle[] = [
  {
    itemId: ids.heat,
    title: 'Heat',
    kind: 'movie',
    year: 1995,
    plays: 3,
    watchedMs: 18_000_000,
    posterUrl: '/api/v1/admin/items/heat/images/poster/w200?v=1',
    backdropUrl: '/api/v1/admin/items/heat/images/backdrop/w780?v=1',
  },
  // An episode carries the series' images (contract 1.8.0).
  {
    itemId: ids.alien,
    title: 'Pilot',
    kind: 'episode',
    plays: 2,
    watchedMs: 5_400_000,
    backdropUrl: '/api/v1/admin/items/series/images/backdrop/w780?v=2',
  },
  // No backdrop stored: the tile shows the fallback with the title.
  {
    itemId: ids.arrival,
    title: 'Arrival',
    kind: 'movie',
    year: 2016,
    plays: 1,
    watchedMs: 600_000,
  },
  {
    itemId: more.dune,
    title: 'Dune',
    kind: 'movie',
    year: 2021,
    plays: 1,
    watchedMs: 2_700_000,
    posterUrl: '/api/v1/admin/items/dune/images/poster/w200?v=3',
  },
  { itemId: more.bear, title: 'The Bear', kind: 'series', plays: 1, watchedMs: 1_800_000 },
];

/** Quarter hour `quarter` (0 to 3) of `hour` on `day` (0 = Monday), in the contract's order. */
const at = (day: number, hour: number, quarter: number) => day * 96 + hour * 4 + quarter;

/**
 * The quarter hours of the fixture: Monday 00:00 and 00:45 (10 min each), Wednesday 08:30
 * (45 min), Friday 20:15 and 20:45 (30 min each) and Sunday 20:00 (20 min).
 */
const quarters = (): number[] => {
  const values = Array.from({ length: 672 }, () => 0);
  values[at(0, 0, 0)] = 600_000;
  values[at(0, 0, 3)] = 600_000;
  values[at(2, 8, 2)] = 2_700_000;
  values[at(4, 20, 1)] = 1_800_000;
  values[at(4, 20, 3)] = 1_800_000;
  values[at(6, 20, 0)] = 1_200_000;
  return values;
};

const windows: Record<Stats['days'], number> = { 7: 7, 30: 28, 90: 84, 365: 84 };

const stats = (days: Stats['days']): Stats => ({
  days,
  topTitles,
  watchTime: [
    {
      profileId: 1,
      profileName: 'Marco',
      accountEmail: 'owner@example.test',
      watchedMs: 18_000_000,
      plays: 3,
    },
    {
      profileId: 9,
      profileName: 'Lena',
      accountEmail: 'gast@example.test',
      watchedMs: 2_700_000,
      plays: 1,
    },
  ],
  playback: { direct: 2, remux: 1, transcode: 1, transcodeShare: 0.25, hardwareShare: 1 },
  devices: [
    { type: 'desktop', count: 2, watchedMs: 18_000_000 },
    { type: 'unknown', count: 1, watchedMs: 2_700_000 },
  ],
  watchTimeByQuarterHour: quarters(),
  timeWindowDays: windows[days],
  timeZone: 'Europe/Zurich',
});

const empty: Stats = {
  days: 30,
  topTitles: [],
  watchTime: [],
  playback: { direct: 0, remux: 0, transcode: 0, transcodeShare: 0, hardwareShare: 0 },
  devices: [],
  watchTimeByQuarterHour: Array.from({ length: 672 }, () => 0),
  timeWindowDays: 28,
  timeZone: 'Europe/Zurich',
};

/** The `days` of every statistics request, in order. */
function askedDays(fetchMock: ReturnType<typeof mockApi>['fetchMock']): (string | null)[] {
  return fetchMock.mock.calls
    .map(([request]) => new URL(request.url))
    .filter((url) => url.pathname === '/api/v1/admin/stats')
    .map((url) => url.searchParams.get('days'));
}

/** The rows of a table as their cell texts, header row included. */
const rowsOf = (table: HTMLElement) =>
  within(table)
    .getAllByRole('row')
    .map((row) => [...row.querySelectorAll('th, td')].map((cell) => cell.textContent));

const routes = {
  ...signedIn,
  'GET /admin/stats': () => json(stats(30)),
};

describe('admin statistics (A-8)', () => {
  afterEach(() => vi.unstubAllGlobals());

  it('shows the playback strip, the blocks and their tables for the last 30 days', async () => {
    const { fetchMock } = mockApi(routes);
    renderApp('/admin/stats');
    expect(await screen.findByRole('heading', { level: 1, name: 'Statistiken' })).toBeVisible();
    expect(screen.getByRole('radio', { name: '30 Tage' })).toBeChecked();
    expect(askedDays(fetchMock)).toEqual(['30']);
    // The intro sits behind the help button (S-2).
    const intro = screen.getByText(
      'Was in letzter Zeit geschaut wurde, über alle Konten und Profile.',
    );
    expect(intro).not.toBeVisible();
    await userEvent.setup().click(screen.getByRole('button', { name: 'Hilfe zu dieser Seite' }));
    expect(intro).toBeVisible();

    // «Wiedergabe» (STA-2): counts with their shares and the GPU share of the transcodes.
    const playback = await screen.findByRole('group', { name: 'Wiedergabe' });
    const tiles = within(playback)
      .getAllByRole('term')
      .map((term) => [term.textContent, term.parentElement?.textContent]);
    expect(tiles.map(([label]) => label)).toEqual([
      'Direct Play',
      'Remux',
      'Transcode',
      'Transcode auf dem Grafikchip',
    ]);
    expect(tiles[0]?.[1]).toMatch(/2 Wiedergaben50\s%\saller Wiedergaben$/);
    expect(tiles[1]?.[1]).toMatch(/1 Wiedergabe25\s%\saller Wiedergaben$/);
    expect(tiles[2]?.[1]).toMatch(/1 Wiedergabe25\s%\saller Wiedergaben$/);
    expect(tiles[3]?.[1]).toMatch(/100\s%Anteil der Transcodes$/);

    // «Meistgesehen»: places 1 to 3 as tiles, each title a link to its administration page.
    const top = screen.getByRole('region', { name: 'Meistgesehen' });
    expect(top).toHaveTextContent('5 Titel');
    const podium = within(top).getByRole('list', { name: 'Platz 1 bis 3' });
    const places = within(podium).getAllByRole('listitem');
    expect(places.map((place) => within(place).getByRole('link').textContent)).toEqual([
      'Heat (1995)',
      'Pilot',
      'Arrival (2016)',
    ]);
    expect(within(podium).getByRole('link', { name: 'Heat (1995)' })).toHaveAttribute(
      'href',
      `/admin/items/${ids.heat}`,
    );
    expect(places[0]).toHaveTextContent(/^1Heat \(1995\)Film · 3 Wiedergaben · 5 h$/);
    expect(places[1]).toHaveTextContent('Folge · 2 Wiedergaben · 1 h 30 min');

    // From place 4 a table with the poster before the title.
    const rest = within(top).getByRole('table', { name: 'Meistgesehen ab Platz 4' });
    expect(rowsOf(rest)).toEqual([
      ['Platz', 'Titel', 'Art', 'Wiedergaben', 'Schauzeit'],
      ['4.', 'Dune (2021)', 'Film', '1', '45 min'],
      ['5.', 'The Bear', 'Serie', '1', '30 min'],
    ]);
    expect(within(rest).getByRole('link', { name: 'Dune (2021)' })).toHaveAttribute(
      'href',
      `/admin/items/${more.dune}`,
    );

    // Watch time per profile and the devices: each a chart with its numbers in a table.
    const profiles = screen.getByRole('region', { name: 'Schauzeit je Profil' });
    expect(profiles).toHaveTextContent('2 Profile · 5 h 45 min');
    expect(within(profiles).getByRole('figure')).toHaveTextContent(
      'Schauzeit je Profil als Diagramm',
    );
    const table = within(profiles).getByRole('table', { name: 'Schauzeit je Profil' });
    const lena = within(table).getByRole('rowheader', { name: /^Lena/ }).closest('tr')!;
    expect(lena).toHaveTextContent('gast@example.test');
    expect(lena).toHaveTextContent('145 min');

    const devices = screen.getByRole('region', { name: 'Geräte' });
    expect(devices).toHaveTextContent('3 Geräte · 5 h 45 min');
    expect(within(devices).getByRole('figure')).toHaveTextContent(
      'Schauzeit je Gerätetyp als Diagramm',
    );
    expect(rowsOf(within(devices).getByRole('table', { name: 'Geräte' }))).toEqual([
      ['Gerätetyp', 'Geräte', 'Schauzeit'],
      ['Computer', '2', '5 h'],
      ['Unbekannt', '1', '45 min'],
    ]);
  });

  it('shows the backdrops as decoration and falls back to the title surface', async () => {
    mockApi(routes);
    renderApp('/admin/stats');
    const podium = await screen.findByRole('list', { name: 'Platz 1 bis 3' });
    const [heat, pilot, arrival] = within(podium).getAllByRole('listitem');
    // Places 1 and 2 show their backdrop, the episode its series'; images are decorative.
    expect(heat!.querySelector('img')).toHaveAttribute('src', topTitles[0]!.backdropUrl);
    expect(heat!.querySelector('img')).toHaveAttribute('alt', '');
    expect(pilot!.querySelector('img')).toHaveAttribute('src', topTitles[1]!.backdropUrl);
    // Place 3 has no backdrop: the fallback surface carries the title, hidden from screen readers.
    expect(arrival!.querySelector('img')).toBeNull();
    expect(within(arrival!).getAllByText('Arrival', { exact: false })).toHaveLength(2);
    // An image that fails to load gives way to the fallback as well.
    fireEvent.error(heat!.querySelector('img')!);
    expect(heat!.querySelector('img')).toBeNull();

    // The table from place 4: Dune's poster, The Bear without one shows the fallback.
    const rest = screen.getByRole('table', { name: 'Meistgesehen ab Platz 4' });
    const dune = within(rest).getByRole('rowheader', { name: 'Dune (2021)' });
    const poster = dune.querySelector('img')!;
    expect(poster).toHaveAttribute('src', topTitles[3]!.posterUrl);
    expect(poster).toHaveAttribute('alt', '');
    fireEvent.error(poster);
    expect(dune.querySelector('img')).toBeNull();
    expect(
      within(rest).getByRole('rowheader', { name: 'The Bear' }).querySelector('img'),
    ).toBeNull();
    // No image anywhere on the page has a text of its own: the titles are written next to them.
    expect(document.querySelectorAll('img[alt]:not([alt=""])')).toHaveLength(0);
  });

  it('folds the quarter hours into hours, days of the week and the grid with its peak', async () => {
    mockApi(routes);
    renderApp('/admin/stats');
    const when = await screen.findByRole('region', { name: 'Wann geschaut wird' });
    expect(within(when).getByText('Aus den letzten 4 Wochen')).toBeVisible();
    expect(
      within(when).getByText('Stunden in der Zeitzone des Servers (Europe/Zurich).'),
    ).toBeVisible();

    // «Nach Tageszeit»: 24 hours, each the sum of its quarter hours over the seven days.
    const hours = within(when).getByRole('table', { name: 'Nach Tageszeit' });
    const hourRows = rowsOf(hours);
    expect(hourRows[0]).toEqual(['Stunde', 'Schauzeit']);
    expect(hourRows).toHaveLength(25);
    expect(hourRows[1]).toEqual(['0–1 Uhr', '20 min']);
    expect(hourRows[2]).toEqual(['1–2 Uhr', '0 s']);
    expect(hourRows[9]).toEqual(['8–9 Uhr', '45 min']);
    expect(hourRows[21]).toEqual(['20–21 Uhr', '1 h 20 min']);
    expect(hourRows[24]).toEqual(['23–24 Uhr', '0 s']);
    expect(within(when).getByText('Am meisten 20–21 Uhr')).toBeVisible();

    // «Nach Wochentag»: Monday to Sunday, each the sum of its day.
    const days = within(when).getByRole('table', { name: 'Nach Wochentag' });
    expect(rowsOf(days)).toEqual([
      ['Wochentag', 'Schauzeit'],
      ['Montag', '20 min'],
      ['Dienstag', '0 s'],
      ['Mittwoch', '45 min'],
      ['Donnerstag', '0 s'],
      ['Freitag', '1 h'],
      ['Samstag', '0 s'],
      ['Sonntag', '20 min'],
    ]);
    expect(within(when).getByText('Am meisten am Freitag')).toBeVisible();

    // «Woche und Stunde» is always shown: a plain heading, no toggle, the peak in its line.
    expect(within(when).getByRole('heading', { name: 'Woche und Stunde', level: 3 })).toBeVisible();
    expect(within(when).queryByRole('button', { name: 'Woche und Stunde' })).toBeNull();
    expect(within(when).getByText('Spitze: Freitag, 20–21 Uhr')).toBeVisible();
    const grid = within(when).getByRole('table', { name: 'Schauzeit nach Wochentag und Stunde' });
    const friday = within(grid).getByRole('rowheader', { name: 'Freitag' }).closest('tr')!;
    const cells = within(friday).getAllByRole('cell');
    expect(cells).toHaveLength(24);
    expect(cells[20]).toHaveTextContent('1 h');
    // An hour without watch time reads «–», spoken «keine».
    expect(cells[21]).toHaveTextContent(/^–keine$/);
    const monday = within(grid).getByRole('rowheader', { name: 'Montag' }).closest('tr')!;
    expect(within(monday).getAllByRole('cell')[0]).toHaveTextContent('20 min');
  });

  it.each([
    [7, 'Aus der letzten Woche'],
    [30, 'Aus den letzten 4 Wochen'],
    [90, 'Aus den letzten 12 Wochen'],
    [365, 'Aus den letzten 12 Wochen'],
  ] as const)(
    'says over which weeks «Wann geschaut wird» counts for %i days',
    async (days, note) => {
      mockApi({ ...signedIn, 'GET /admin/stats': () => json(stats(days)) });
      renderApp(`/admin/stats?days=${days}`);
      const when = await screen.findByRole('region', { name: 'Wann geschaut wird' });
      expect(within(when).getByText(note)).toBeVisible();
    },
  );

  it('switches the period with the arrow keys, keeps it in the address and says so', async () => {
    const { fetchMock } = mockApi(routes);
    const { router } = renderApp('/admin/stats');
    const user = userEvent.setup();
    await screen.findByRole('table', { name: 'Meistgesehen ab Platz 4' });
    // The status region is there from the start and says nothing before a switch; the content
    // dims only while another period loads.
    const status = screen.getByRole('status');
    expect(status).toBeEmptyDOMElement();
    const content = screen.getByRole('group', { name: 'Wiedergabe' }).parentElement!;
    expect(content).toHaveAttribute('aria-busy', 'false');
    expect(content).not.toHaveAttribute('data-switching');
    const thirty = screen.getByRole('radio', { name: '30 Tage' });
    thirty.focus();
    await user.keyboard('{ArrowLeft}');
    await waitFor(() => expect(screen.getByRole('radio', { name: '7 Tage' })).toBeChecked());
    await waitFor(() => expect(router.state.location.search).toEqual({ days: 7 }));
    await waitFor(() => expect(askedDays(fetchMock)).toEqual(['30', '7']));
    await waitFor(() => expect(status).toHaveTextContent('Zahlen für 7 Tage geladen.'));
    expect(content).not.toHaveAttribute('data-switching');
    await user.keyboard('{ArrowRight}{ArrowRight}{ArrowRight}');
    await waitFor(() => expect(router.state.location.search).toEqual({ days: 365 }));
    expect(screen.getByRole('radio', { name: '365 Tage' })).toHaveFocus();
    await waitFor(() => expect(status).toHaveTextContent('Zahlen für 365 Tage geladen.'));
  });

  it('takes the period from the address and ignores one it does not offer', async () => {
    const first = mockApi(routes);
    const view = renderApp('/admin/stats?days=90');
    expect(await screen.findByRole('radio', { name: '90 Tage' })).toBeChecked();
    await waitFor(() => expect(askedDays(first.fetchMock)).toEqual(['90']));
    view.unmount();
    vi.unstubAllGlobals();

    const second = mockApi(routes);
    renderApp('/admin/stats?days=12');
    expect(await screen.findByRole('radio', { name: '30 Tage' })).toBeChecked();
    await waitFor(() => expect(askedDays(second.fetchMock)).toEqual(['30']));
  });

  it('names a full list «Top 20» in the head of «Meistgesehen»', async () => {
    const twenty = Array.from({ length: 20 }, (_, index) => ({
      itemId: `01JB8Z6Q4M7N9P2R3S5T7V9X${String(index).padStart(2, '0')}`,
      title: `Titel ${index + 1}`,
      kind: 'movie' as const,
      plays: 20 - index,
      watchedMs: (20 - index) * 600_000,
    }));
    mockApi({ ...signedIn, 'GET /admin/stats': () => json({ ...stats(30), topTitles: twenty }) });
    renderApp('/admin/stats');
    const top = await screen.findByRole('region', { name: 'Meistgesehen' });
    expect(top).toHaveTextContent(/^Meistgesehen\s*Top 20/);
    expect(within(top).getAllByRole('listitem')).toHaveLength(3);
    expect(
      within(within(top).getByRole('table', { name: 'Meistgesehen ab Platz 4' })).getAllByRole(
        'row',
      ),
    ).toHaveLength(18);
  });

  it('says when nothing was watched', async () => {
    mockApi({ ...signedIn, 'GET /admin/stats': () => json(empty) });
    renderApp('/admin/stats');
    expect(await screen.findByText('In diesem Zeitraum wurde nichts geschaut.')).toBeVisible();
    expect(screen.getByRole('region', { name: 'Meistgesehen' })).toHaveTextContent('0 Titel');
    expect(screen.queryByRole('list', { name: 'Platz 1 bis 3' })).toBeNull();
    expect(screen.getByText('Kein Profil hat in diesem Zeitraum geschaut.')).toBeVisible();
    expect(screen.getByText('Kein Gerät hat in diesem Zeitraum geschaut.')).toBeVisible();
    const playback = screen.getByRole('group', { name: 'Wiedergabe' });
    expect(playback).toHaveTextContent(/^Direct Play0 Wiedergaben0\s%\saller Wiedergaben/);
    const when = screen.getByRole('region', { name: 'Wann geschaut wird' });
    expect(within(when).getByText('Aus den letzten 4 Wochen')).toBeVisible();
    expect(within(when).getByText('In diesen Wochen wurde nichts geschaut.')).toBeVisible();
    expect(within(when).queryByRole('button', { name: 'Woche und Stunde' })).toBeNull();
    expect(screen.queryByRole('figure')).toBeNull();
  });

  it('shows the error with a retry', async () => {
    let fail = true;
    mockApi({
      ...signedIn,
      'GET /admin/stats': () => (fail ? problem(500, 'internal') : json(stats(30))),
    });
    renderApp('/admin/stats');
    const user = userEvent.setup();
    const retry = await screen.findByRole('button', { name: 'Erneut versuchen' });
    fail = false;
    await user.click(retry);
    expect(await screen.findByRole('list', { name: 'Platz 1 bis 3' })).toBeVisible();
  });

  it('is for the main account only', async () => {
    const { calls } = mockApi({
      ...signedIn,
      'GET /auth/session': () =>
        json({ ...session, profile: { ...session.profile, isPrimary: false } }),
    });
    renderApp('/admin/stats');
    expect(await screen.findByText('Nur für Administratoren')).toBeVisible();
    expect(calls.some((call) => call.path.startsWith('/admin/'))).toBe(false);
  });
});
