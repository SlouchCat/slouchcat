import { screen, waitFor, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { dashboardFixture } from '@/test/admin';
import { json, mockApi, problem, renderApp, signedIn, type Routes } from '@/test/api';
import { fill } from '@/test/fill';
import type { ArrStatus, SeerrSettings, SeerrStatus, SeerrTestResult } from '../queries';

// Integrationen › Seerr (concept 25, M-22): the tile with the state from the dashboard, the band
// «Verbindung» with test and save (saving runs no test, the view tests the saved values right
// after), a value of deploy/.env that locks its field, the check list, the decision about another
// instance and the call log. The key is write-only.

const settings = (overrides: Partial<SeerrSettings> = {}): SeerrSettings => ({
  enabled: false,
  urlSource: 'none',
  apiKey: { source: 'none' },
  ...overrides,
});

const status = (overrides: Partial<SeerrStatus> = {}): SeerrStatus => ({
  state: 'not_configured',
  instanceChanged: false,
  tested: false,
  counts: { open: 0, queued: 0, unmapped: 0 },
  log: [],
  ...overrides,
});

const passed: SeerrTestResult = {
  ok: true,
  version: '3.5.1',
  checks: [
    { key: 'reachable', level: 'ok' },
    { key: 'version', level: 'ok', detail: '3.5.1' },
    { key: 'key_valid', level: 'ok' },
    { key: 'radarr', level: 'ok', detail: '1' },
    { key: 'tvdb_seasons', level: 'warn', detail: 'tmdb' },
  ],
};

const routes = (extra: Routes = {}): Routes => ({
  ...signedIn,
  'GET /admin/settings': () =>
    json({
      publicUrlSource: 'none',
      updateCheckEnabled: true,
      transcodeHw: 'auto',
      transcodeHwSource: 'none',
      transcodeHwRestart: false,
      keys: {
        tmdb: { source: 'none' },
        tvdb: { source: 'none' },
        tvdbPin: { source: 'none' },
        fanart: { source: 'none' },
        fanartClient: { source: 'none' },
        youtube: { source: 'none' },
        trakt: { source: 'none' },
        googleBooks: { source: 'none' },
      },
      email: { source: 'none' },
      telegram: { source: 'none' },
      push: { configured: false },
    }),
  'GET /admin/integrations': () => json([]),
  'GET /admin/seerr': () => json(settings()),
  'GET /admin/seerr/status': () => json(status()),
  'GET /admin/wishes/mappings': () =>
    json({ accounts: [], unlinkedUsers: [], opened: 2, total: 3 }),
  ...extra,
});

/** The field itself: its setting row is a group with the same name (hidden field label). */
const INPUT = { selector: 'input' };

const view = () => screen.findByRole('region', { name: 'Seerr' });

/** The page's text without the code to copy: Seerr's JSON template has «{{…}}» of its own. */
const textOutsideCode = () => {
  const copy = document.body.cloneNode(true) as HTMLElement;
  copy.querySelectorAll('pre').forEach((code) => code.remove());
  return copy.textContent;
};

afterEach(() => {
  vi.unstubAllGlobals();
});

describe('Integrationen › Seerr (concept 25)', () => {
  it('the tile names Seerr after the webhooks, with its state from the dashboard', async () => {
    mockApi(
      routes({
        'GET /admin/dashboard': () =>
          json(dashboardFixture({ integrations: [{ key: 'seerr', state: 'configured' }] })),
        'GET /admin/seerr': () =>
          json(
            settings({
              enabled: true,
              url: 'http://seerr.lan:5055',
              urlSource: 'stored',
              apiKey: { source: 'stored', hint: 'b8c9' },
            }),
          ),
        'GET /admin/seerr/status': () => json(status({ state: 'connected', version: '3.5.1' })),
      }),
    );
    renderApp('/admin/integrations');
    const list = await screen.findByRole('list', { name: 'Dienste' });
    const links = within(list).getAllByRole('link');
    expect(links.map((link) => link.textContent).slice(1, 3)).toEqual([
      'Webhooks von Radarr und Sonarr',
      'Seerr',
    ]);
    const tile = within(list).getByRole('link', { name: 'Seerr' }).closest('li')!;
    expect(await within(tile).findByText('Seerr: Eingerichtet')).toBeVisible();
    // The version is what only getSeerrStatus knows: it stands in the view, not on the tile.
    expect(within(tile).queryByText('Version 3.5.1')).toBeNull();
    expect(await within(tile).findByText('Seerr-Benutzer: 2 von 3 Konten')).toBeVisible();
  });

  it('tests the typed values, saves them, then tests the saved ones', async () => {
    let current = settings();
    const { calls } = mockApi(
      routes({
        'GET /admin/seerr': () => json(current),
        'PUT /admin/seerr': () => {
          current = settings({
            enabled: true,
            url: 'http://seerr.lan:5055',
            urlSource: 'stored',
            apiKey: { source: 'stored', hint: 'b8c9' },
          });
          return json(current);
        },
        'POST /admin/seerr/test': () => json(passed),
      }),
    );
    renderApp('/admin/integrations?item=seerr');
    const user = userEvent.setup();
    const seerr = await view();
    // Not set up: the band «Verbindung» is open (it needs something), «Protokoll» is closed.
    const connection = within(seerr).getByRole('button', { name: 'Verbindung' });
    expect(connection).toHaveAttribute('aria-expanded', 'true');
    expect(within(seerr).getByRole('button', { name: 'Protokoll' })).toHaveAttribute(
      'aria-expanded',
      'false',
    );
    await user.click(within(seerr).getByRole('switch', { name: 'Seerr verwenden' }));
    await fill(user, within(seerr).getByLabelText('Adresse', INPUT), 'http://seerr.lan:5055');
    await fill(user, within(seerr).getByLabelText('API-Schlüssel', INPUT), 'seerr-key-b8c9');
    expect(within(seerr).getAllByText('ungespeichert').length).toBeGreaterThan(0);

    await user.click(within(seerr).getByRole('button', { name: 'Verbindung testen' }));
    const checks = await within(seerr).findByRole('list', {
      name: 'Ergebnis des Verbindungstests',
    });
    expect(within(seerr).getByText('Verbunden · 1 Hinweis')).toBeVisible();
    expect(within(checks).getByText('Seerr 3.5.1')).toBeVisible();
    expect(within(checks).getByText('Film-Wünsche gehen an Radarr (1 Server).')).toBeVisible();
    expect(within(checks).getByText(/Seerr liest Staffeln aus TMDB/)).toBeVisible();

    await user.click(within(seerr).getByRole('button', { name: 'Verbindung zu Seerr speichern' }));
    expect(await within(seerr).findByText('Verbindung zu Seerr gespeichert.')).toBeVisible();
    await waitFor(() =>
      expect(calls.filter((call) => call.path === '/admin/seerr/test')).toHaveLength(2),
    );
    const sent = calls.filter((call) => call.method !== 'GET').map((call) => call.body);
    expect(sent).toEqual([
      { url: 'http://seerr.lan:5055', apiKey: 'seerr-key-b8c9' },
      { enabled: true, url: 'http://seerr.lan:5055', apiKey: 'seerr-key-b8c9' },
      {},
    ]);
    // The key is write-only: the field is empty again and its placeholder names the stored one.
    const key = within(seerr).getByLabelText('API-Schlüssel', INPUT);
    expect(key).toHaveValue('');
    expect(key).toHaveAttribute('placeholder', 'Gespeichert, endet auf b8c9');
  });

  it('a new host needs the key again: the server says so at the key field', async () => {
    mockApi(
      routes({
        'GET /admin/seerr': () =>
          json(
            settings({
              url: 'http://seerr.lan:5055',
              urlSource: 'stored',
              apiKey: { source: 'stored', hint: 'b8c9' },
            }),
          ),
        'GET /admin/seerr/status': () => json(status({ state: 'connected', version: '3.5.1' })),
        'PUT /admin/seerr': () => problem(422, 'seerr_key_required'),
      }),
    );
    renderApp('/admin/integrations?item=seerr');
    const user = userEvent.setup();
    const seerr = await view();
    await user.click(within(seerr).getByRole('button', { name: 'Verbindung' }));
    await fill(user, within(seerr).getByLabelText('Adresse', INPUT), 'http://other.lan:5055');
    await user.click(within(seerr).getByRole('button', { name: 'Verbindung zu Seerr speichern' }));
    const key = within(seerr).getByLabelText('API-Schlüssel', INPUT);
    await waitFor(() => expect(key).toHaveAttribute('aria-invalid', 'true'));
    expect(key).toHaveAccessibleDescription(/braucht den API-Schlüssel neu/);
  });

  it('values from deploy/.env lock their fields and cannot be deleted here', async () => {
    mockApi(
      routes({
        'GET /admin/seerr': () =>
          json(
            settings({
              enabled: true,
              url: 'http://seerr.lan:5055',
              urlSource: 'env',
              apiKey: { source: 'env' },
            }),
          ),
        'GET /admin/seerr/status': () => json(status({ state: 'untested' })),
      }),
    );
    renderApp('/admin/integrations?item=seerr');
    const seerr = await view();
    expect(within(seerr).getByLabelText('Adresse', INPUT)).toBeDisabled();
    expect(within(seerr).getByLabelText('API-Schlüssel', INPUT)).toBeDisabled();
    expect(within(seerr).getAllByText(/In deploy\/.env gesetzt/).length).toBeGreaterThan(0);
    expect(within(seerr).queryByRole('button', { name: /löschen$/ })).toBeNull();
    expect(within(seerr).getByRole('switch', { name: 'Seerr verwenden' })).toBeEnabled();
    expect(
      await within(seerr).findByText(/damit sich «Wünsche aktiv» einschalten lässt/),
    ).toBeVisible();
  });

  it('another instance waits: «Zuordnung bestätigen» takes it over', async () => {
    let current = status({ state: 'instance_changed', instanceChanged: true });
    const { calls } = mockApi(
      routes({
        'GET /admin/seerr': () =>
          json(
            settings({
              enabled: true,
              url: 'http://seerr.lan:5055',
              urlSource: 'stored',
              apiKey: { source: 'stored', hint: 'b8c9' },
            }),
          ),
        'GET /admin/seerr/status': () => json(current),
        'POST /admin/seerr/instance': () => {
          current = status({ state: 'connected', version: '3.5.1' });
          return json(current);
        },
      }),
    );
    renderApp('/admin/integrations?item=seerr');
    const user = userEvent.setup();
    const seerr = await view();
    const notice = await within(seerr).findByRole('alert');
    expect(notice).toHaveTextContent('Andere Seerr-Instanz erkannt');
    await user.click(within(notice).getByRole('button', { name: 'Zuordnung bestätigen' }));
    expect(await within(seerr).findByText(/Neue Instanz übernommen/)).toBeVisible();
    expect(calls.find((call) => call.path === '/admin/seerr/instance')?.body).toEqual({
      action: 'confirm',
    });
    await waitFor(() =>
      expect(within(seerr).queryByText('Andere Seerr-Instanz erkannt')).toBeNull(),
    );
  });

  it('the call log lists the last calls with their result', async () => {
    mockApi(
      routes({
        'GET /admin/seerr/status': () =>
          json(
            status({
              state: 'connected',
              log: [
                {
                  at: '2026-11-17T10:00:00Z',
                  method: 'GET',
                  path: '/status',
                  status: 200,
                  durationMs: 12,
                },
                {
                  at: '2026-11-17T09:59:00Z',
                  method: 'GET',
                  path: '/auth/me',
                  durationMs: 10003,
                  error: 'timeout',
                },
              ],
            }),
          ),
      }),
    );
    renderApp('/admin/integrations?item=seerr');
    const user = userEvent.setup();
    const seerr = await view();
    const log = within(seerr).getByRole('button', { name: 'Protokoll' });
    await waitFor(() => expect(log).toHaveAccessibleDescription('2 Einträge · 1 Fehler'));
    await user.click(log);
    const table = within(seerr).getByRole('table', { name: 'Die letzten Aufrufe an Seerr' });
    expect(within(table).getByText('GET /status')).toBeVisible();
    expect(within(table).getByText('200')).toBeVisible();
    expect(within(table).getByText('Keine Antwort')).toBeVisible();
    expect(within(table).getByText('timeout')).toBeVisible();
    // Through the locale's number format (n3).
    expect(
      within(table).getByText(`${new Intl.NumberFormat('de-CH').format(10003)} ms`),
    ).toBeVisible();
  });

  it('two calls alike in one second both stand in the call log', async () => {
    const call = {
      at: '2026-11-17T10:00:00Z',
      method: 'GET',
      path: '/request',
      status: 200,
      durationMs: 5,
    };
    mockApi(
      routes({
        'GET /admin/seerr/status': () => json(status({ state: 'connected', log: [call, call] })),
      }),
    );
    renderApp('/admin/integrations?item=seerr');
    const user = userEvent.setup();
    const seerr = await view();
    await user.click(within(seerr).getByRole('button', { name: 'Protokoll' }));
    const table = within(seerr).getByRole('table', { name: 'Die letzten Aufrufe an Seerr' });
    expect(within(table).getAllByText('GET /request')).toHaveLength(2);
  });

  it('«Testen» of one service keeps its own spinner and error', async () => {
    const service = {
      source: 'seerr' as const,
      address: 'http://arr.lan',
      confirmed: false,
      counts: { set: 0, pending: 0, failed: 0 },
      progress: { enabled: true, tracked: 0 },
    };
    mockApi(
      routes({
        'GET /admin/seerr': () =>
          json(settings({ enabled: true, urlSource: 'stored', url: 'http://seerr.lan' })),
        'GET /admin/seerr/status': () => json(status({ state: 'connected', tested: true })),
        'GET /admin/seerr/arr': () => json({ radarr: service, sonarr: service }),
        'GET /admin/wishes/settings': () => json({ profileTags: true, showProgress: true }),
        'POST /admin/seerr/arr/test': (sent) =>
          (sent.body as { service: string }).service === 'radarr'
            ? new Promise<Response>(() => undefined)
            : problem(503, 'seerr_unreachable'),
      }),
    );
    renderApp('/admin/integrations?item=seerr');
    const user = userEvent.setup();
    const seerr = await view();
    const radarr = await within(seerr).findByRole('region', { name: 'Radarr' });
    const sonarr = within(seerr).getByRole('region', { name: 'Sonarr' });
    await user.click(within(radarr).getByRole('button', { name: 'Radarr testen' }));
    await user.click(within(sonarr).getByRole('button', { name: 'Sonarr testen' }));
    await waitFor(() => expect(within(sonarr).getByRole('alert')).toBeVisible());
    expect(within(radarr).queryByRole('alert')).toBeNull();
    expect(within(radarr).getByRole('button', { name: 'Radarr testen' })).toHaveAttribute(
      'data-pending',
      'true',
    );
  });

  it('Radarr and Sonarr: where the address comes from, «Testen», an own address; never a key', async () => {
    let arr: ArrStatus = {
      radarr: {
        source: 'seerr',
        address: 'http://radarr.lan:7878',
        confirmed: false,
        tagRequests: false,
        counts: { set: 3, pending: 1, failed: 1 },
        progress: { enabled: true, tracked: 2, lastReadAt: '2026-11-17T10:00:00Z' },
      },
      sonarr: {
        source: 'none',
        confirmed: false,
        counts: { set: 0, pending: 0, failed: 0 },
        progress: { enabled: true, tracked: 0, error: 'unreachable' },
      },
    };
    const { calls } = mockApi(
      routes({
        'GET /admin/seerr': () =>
          json(settings({ enabled: true, urlSource: 'stored', url: 'http://seerr.lan' })),
        'GET /admin/seerr/status': () => json(status({ state: 'connected', tested: true })),
        'GET /admin/seerr/arr': () => json(arr),
        'GET /admin/seerr/webhook': () =>
          json({
            jsonPayload: '{"notification_type":"{{notification_type}}"}',
            types: 6,
            typeNames: ['MEDIA_PENDING', 'MEDIA_APPROVED'],
          }),
        'GET /admin/wishes/settings': () => json({ profileTags: true, showProgress: true }),
        'POST /admin/seerr/arr/test': () =>
          json({
            reachable: true,
            keyValid: true,
            tagRequests: false,
            host: 'http://radarr.lan:7878',
            source: 'seerr',
          }),
        'PUT /admin/seerr/arr': () => {
          arr = {
            ...arr,
            sonarr: {
              ...arr.sonarr,
              source: 'own',
              address: 'http://sonarr:8989',
              confirmed: true,
            },
          };
          return json(arr);
        },
      }),
    );
    renderApp('/admin/integrations?item=seerr');
    const user = userEvent.setup();
    const seerr = await view();
    // Radarr waits for «Testen»: the band is open with it (design rule 1.3).
    const band = within(seerr).getByRole('button', {
      name: 'Radarr und Sonarr (Profil-Tags und Fortschritt)',
    });
    await waitFor(() => expect(band).toHaveAttribute('aria-expanded', 'true'));
    const radarr = within(seerr).getByRole('region', { name: 'Radarr' });
    expect(within(radarr).getByText('Aus Seerr: http://radarr.lan:7878')).toBeVisible();
    expect(within(radarr).getByText('Radarr wartet auf «Testen»')).toBeVisible();
    expect(
      within(radarr).getByText(
        '1 gesetzt · 1 offen · 1 fehlgeschlagen'.replace('1 gesetzt', '3 gesetzt'),
      ),
    ).toBeVisible();
    const sonarr = within(seerr).getByRole('region', { name: 'Sonarr' });
    expect(
      within(sonarr).getByText('Keine: Seerr hat keinen Standardserver für Sonarr.'),
    ).toBeVisible();
    expect(
      within(sonarr).getByText('Fortschritt nicht lesbar: Sonarr ist nicht erreichbar.'),
    ).toBeVisible();

    await user.click(within(radarr).getByRole('button', { name: 'Radarr testen' }));
    expect(
      await within(radarr).findByText(
        'Radarr antwortet unter http://radarr.lan:7878, der Schlüssel aus Seerr gilt. Die Adresse ist bestätigt.',
      ),
    ).toBeVisible();

    await fill(
      user,
      within(seerr).getByLabelText('Eigene Adresse für Sonarr', INPUT),
      'http://sonarr:8989',
    );
    await user.click(
      within(seerr).getByRole('button', {
        name: 'Eigene Adressen von Radarr und Sonarr speichern',
      }),
    );
    expect(await within(seerr).findByText('Eigene Adressen gespeichert.')).toBeVisible();
    const put = calls.find((call) => call.method === 'PUT' && call.path === '/admin/seerr/arr');
    expect(put?.body).toEqual({ sonarrUrl: 'http://sonarr:8989' });
    expect(within(sonarr).getByText('Eigene Adresse: http://sonarr:8989')).toBeVisible();
    expect(textOutsideCode()).not.toContain('{{');
  });

  it('«Sofortmeldung» makes the address of a webhook integration of the kind seerr', async () => {
    const { calls } = mockApi(
      routes({
        'GET /admin/seerr': () =>
          json(settings({ enabled: true, urlSource: 'stored', url: 'http://seerr.lan' })),
        'GET /admin/seerr/status': () => json(status({ state: 'connected', tested: true })),
        'GET /admin/seerr/arr': () =>
          json({
            radarr: {
              source: 'none',
              confirmed: false,
              counts: { set: 0, pending: 0, failed: 0 },
              progress: { enabled: true, tracked: 0 },
            },
            sonarr: {
              source: 'none',
              confirmed: false,
              counts: { set: 0, pending: 0, failed: 0 },
              progress: { enabled: true, tracked: 0 },
            },
          }),
        'GET /admin/seerr/webhook': () =>
          json({
            jsonPayload: '{"notification_type":"x"}',
            types: 6,
            typeNames: ['MEDIA_PENDING', 'MEDIA_APPROVED'],
          }),
        'GET /admin/wishes/settings': () => json({ profileTags: true, showProgress: true }),
        'POST /admin/integrations': () =>
          json(
            {
              integration: {
                id: 4,
                name: 'Seerr',
                kind: 'seerr',
                tokenTail: 'ab12',
                pathMap: [],
                createdAt: '2026-11-17T10:00:00Z',
              },
              url: 'http://slouchcat.lan/api/v1/webhooks/secret-token-ab12',
            },
            201,
          ),
      }),
    );
    renderApp('/admin/integrations?item=seerr');
    const user = userEvent.setup();
    const seerr = await view();
    const band = within(seerr).getByRole('button', { name: 'Sofortmeldung' });
    expect(band).toHaveAttribute('aria-expanded', 'false');
    await user.click(band);
    expect(await within(seerr).findByText('MEDIA_PENDING', { exact: false })).toBeVisible();
    await user.click(within(seerr).getByRole('button', { name: 'Adresse erzeugen' }));
    expect(await within(seerr).findByDisplayValue(/secret-token-ab12$/)).toBeVisible();
    const post = calls.find(
      (call) => call.method === 'POST' && call.path === '/admin/integrations',
    );
    expect(post?.body).toEqual({ name: 'Seerr', kind: 'seerr' });
    // Pressed once: no second address while the list does not show the first one yet (m5).
    expect(within(seerr).queryByRole('button', { name: 'Adresse erzeugen' })).toBeNull();
    expect(within(seerr).getByRole('button', { name: 'Vorlage kopieren' })).toBeVisible();
    expect(within(seerr).getByRole('button', { name: 'Arten kopieren' })).toBeVisible();
  });
});
