import type { components } from '@slouchcat/api-client';
import { screen, waitFor, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { adminShell, renderAdminPage } from '@/test/admin';
import { json, mockApi, renderApp, signedIn, type Routes } from '@/test/api';

// Webradio in the administration (concept 16, 11, variant A): the tabs in `?tab=`, the warning
// without RAM buffer, the relays with «Stoppen», the editor in `?item=`. The server answers; the
// page shows and sends.

type RadioConfig = components['schemas']['RadioConfig'];
type AdminRadioStation = components['schemas']['AdminRadioStation'];

const config = (overrides: Partial<RadioConfig> = {}): RadioConfig => ({
  bufferMinutes: 30,
  budgetMb: 512,
  maxRelays: 10,
  graceSeconds: 120,
  hideBroken: true,
  countries: null,
  bufferMode: 'tmpfs',
  maxRelaysByBudget: 11,
  ...overrides,
});

const station: AdminRadioStation = {
  id: 7,
  name: 'Radio Swiss Pop',
  country: 'CH',
  region: 'CH-ZH',
  languages: ['de'],
  genres: ['pop'],
  kinds: ['music'],
  moods: [],
  hls: false,
  showsTitles: true,
  recommended: true,
  state: 'ok',
  source: 'catalog',
  hidden: false,
  failures: 0,
  tags: ['pop', 'swiss'],
  facets: [{ facet: 'genre', value: 'pop', manual: false }],
  locks: ['name'],
  logoState: 'catalog',
  aliasCount: 2,
  streams: [
    {
      id: 70,
      url: 'https://stream.example.test/pop.aac',
      codec: 'AAC',
      bitrateKbps: 128,
      hls: false,
      reachable: true,
      preferred: false,
      rank: 0,
    },
  ],
  createdAt: Date.UTC(2026, 10, 1),
  updatedAt: Date.UTC(2026, 10, 6),
};

function radioRoutes(overrides: Routes = {}): Routes {
  return {
    ...signedIn,
    ...adminShell,
    'GET /admin/radio/config': () => json(config()),
    'GET /admin/radio/catalog': () =>
      json({
        source: 'radio-browser',
        lastSyncAt: Date.UTC(2026, 10, 7, 3, 30),
        syncFailures: 0,
        entriesTotal: 51240,
        entriesReachable: 38912,
        stationsTotal: 40102,
        lastAdded: 214,
        lastRemoved: 88,
      }),
    'GET /admin/radio/relays': () =>
      json({
        relays: [
          {
            stationId: 7,
            name: 'Radio Swiss Pop',
            codec: 'AAC',
            bitrateKbps: 128,
            transcoded: false,
            startedAt: Date.UTC(2026, 10, 7, 8),
            bufferSeconds: 600,
            bufferBytes: 9_600_000,
            state: 'live',
            listeners: [
              {
                listenId: 'L1',
                profileName: 'Lena',
                accountEmail: 'gast@example.test',
                playing: true,
                since: Date.UTC(2026, 10, 7, 8),
              },
            ],
          },
        ],
        load: {
          relays: 1,
          maxRelays: 10,
          bufferBytes: 9_600_000,
          budgetBytes: 512_000_000,
          uploadKbps: 128,
          bufferMode: 'tmpfs',
        },
      }),
    'GET /admin/radio/stations': () => json({ items: [station], total: 1, offset: 0 }),
    'GET /admin/radio/stations/7': () => json(station),
    'GET /radio/facets': () =>
      json({
        country: [{ value: 'CH', count: 1 }],
        region: [],
        language: [],
        genre: [{ value: 'pop', count: 1 }],
        kind: [],
        mood: [],
        quality: [],
        titles: 1,
      }),
    ...overrides,
  };
}

describe('radio administration', () => {
  afterEach(() => vi.unstubAllGlobals());

  it('opens on «Übersicht» with the counters, the catalog and the relays', async () => {
    mockApi(radioRoutes());
    await renderAdminPage('/admin/radio');
    expect(await screen.findByRole('heading', { level: 1, name: 'Radio' })).toBeVisible();
    expect(screen.getByRole('tab', { name: /Übersicht/, selected: true })).toBeVisible();
    const strip = screen.getByRole('group', { name: 'Zusammenfassung' });
    expect(await within(strip).findByText('1 · 1')).toBeVisible();
    expect(
      await screen.findByText(/51.240 Einträge|51’240 Einträge|51'240 Einträge/),
    ).toBeVisible();
    const relays = await screen.findByTestId('radio-relays');
    expect(within(relays).getByRole('button', { name: 'Hören von Lena stoppen' })).toBeVisible();
    // With the RAM disk there is no warning.
    expect(screen.queryByText('Kein RAM-Puffer')).not.toBeInTheDocument();
  });

  it('warns when the relays have no RAM buffer', async () => {
    mockApi(
      radioRoutes({ 'GET /admin/radio/config': () => json(config({ bufferMode: 'memory' })) }),
    );
    await renderAdminPage('/admin/radio');
    expect(await screen.findByText('Kein RAM-Puffer')).toBeVisible();
  });

  it('takes the tab from ?tab= and writes it back', async () => {
    mockApi(radioRoutes());
    const { router } = await renderAdminPage('/admin/radio?tab=einstellungen');
    expect(await screen.findByRole('tab', { name: /Einstellungen/, selected: true })).toBeVisible();
    expect(await screen.findByRole('group', { name: 'Zeitversatz' })).toBeVisible();
    expect(
      screen.getByText(/Das Budget erlaubt beim jetzigen Zeitversatz 11 Sender/),
    ).toBeVisible();

    await userEvent.setup().click(screen.getByRole('tab', { name: /Sender/ }));
    await waitFor(() => expect(router.state.location.search).toEqual({ tab: 'sender' }));
    const table = await screen.findByRole('table', { name: 'Sender' });
    expect(within(table).getByRole('rowheader', { name: /Radio Swiss Pop/ })).toBeVisible();
  });

  it('stops a listener after the confirmation', async () => {
    const { calls } = mockApi(
      radioRoutes({
        'DELETE /admin/radio/listeners/L1': () => new Response(null, { status: 204 }),
      }),
    );
    await renderAdminPage('/admin/radio');
    const user = userEvent.setup();
    await user.click(await screen.findByRole('button', { name: 'Hören von Lena stoppen' }));
    const dialog = await screen.findByRole('alertdialog');
    await user.click(within(dialog).getByRole('button', { name: 'Stoppen' }));
    await waitFor(() =>
      expect(
        calls.some((call) => call.method === 'DELETE' && call.path === '/admin/radio/listeners/L1'),
      ).toBe(true),
    );
  });

  it('opens the editor from ?item= and saves a field with its lock', async () => {
    const { calls } = mockApi(
      radioRoutes({
        'PATCH /admin/radio/stations/7': (call) => json({ ...station, ...(call.body as object) }),
      }),
    );
    // The open editor hides the page (and its sidebar) from assistive technology.
    renderApp('/admin/radio?tab=sender&item=7');
    const dialog = await screen.findByRole('dialog');
    const user = userEvent.setup();
    const region = await within(dialog).findByRole('textbox', { name: 'Region' });
    await user.clear(region);
    await user.type(region, 'ch-be');
    await user.click(within(dialog).getByRole('button', { name: 'Speichern' }));
    await waitFor(() => {
      const patch = calls.find((call) => call.method === 'PATCH');
      expect(patch?.body).toEqual({ region: 'CH-BE', locks: ['name', 'region'] });
    });
  });
});
