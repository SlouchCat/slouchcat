import type { AdminSettings, Integration, IntegrationEvent } from '@slouchcat/api-client';
import { screen, waitFor, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { json, mockApi, problem, renderApp, signedIn, type Routes } from '@/test/api';

// M9 webhooks (I-8, decision 5.6): one address per integration, shown exactly once with a copy
// button, afterwards only its last four characters; a table whose rows fold out into the path
// mapping and the event log (concept 12, Int-A); renew and delete. Unsaved rows of a mapping live
// in the page, so folding the row or the block away keeps them (S-13, S-17).

const settings: AdminSettings = {
  publicUrlSource: 'none',
  updateCheckEnabled: true,
  keys: {
    tmdb: { source: 'none' },
    tvdb: { source: 'none' },
    tvdbPin: { source: 'none' },
    fanart: { source: 'none' },
    fanartClient: { source: 'none' },
    youtube: { source: 'none' },
    trakt: { source: 'none' },
  },
  email: { source: 'none' },
  telegram: { source: 'none' },
  push: { configured: false },
};

const radarr: Integration = {
  id: 1,
  name: 'Radarr',
  kind: 'radarr',
  tokenTail: 'wxyz',
  pathMap: [{ from: '/movies', to: '/media/Filme' }],
  lastSeenAt: '2026-10-22T10:00:00Z',
  createdAt: '2026-10-01T08:00:00Z',
};

const events: IntegrationEvent[] = [
  {
    id: 2,
    receivedAt: '2026-10-22T10:00:00Z',
    event: 'Download',
    reportedPath: '/movies/Dune (2021)',
    resolvedPath: '/media/Filme/Dune (2021)',
    result: 'scan_queued',
  },
  { id: 1, receivedAt: '2026-10-22T09:00:00Z', event: 'Test', result: 'acknowledged' },
];

const routes = (extra: Routes = {}): Routes => ({
  ...signedIn,
  'GET /admin/settings': () => json(settings),
  'GET /admin/integrations': () => json([radarr]),
  'GET /admin/integrations/1/events': () => json(events),
  ...extra,
});

const BLOCK = 'Webhooks von Radarr und Sonarr';

const webhooks = () => screen.getByRole('region', { name: BLOCK });

/** The row of an integration in the table; its first cell is the button that folds it out. */
async function integrationRow(name: string) {
  const section = await screen.findByRole('region', { name: BLOCK });
  const table = within(section).getByRole('table', { name: 'Webhook-Integrationen' });
  const toggle = await within(table).findByRole('button', { name });
  return { section, table, toggle, row: toggle.closest('tr')! };
}

/** Folds the row of an integration out and returns its section. */
async function openIntegration(name: string) {
  const user = userEvent.setup();
  const { section, toggle } = await integrationRow(name);
  await user.click(toggle);
  expect(toggle).toHaveAttribute('aria-expanded', 'true');
  return section;
}

describe('webhook integrations (I-8)', () => {
  afterEach(() => vi.unstubAllGlobals());

  it('lists an integration with its last four characters and last contact; the row folds out into mapping and event log', async () => {
    const { calls } = mockApi(routes());
    renderApp('/admin/integrations');

    const { table, toggle, row } = await integrationRow('Radarr');
    const headers = within(table)
      .getAllByRole('columnheader')
      .map((header) => header.textContent);
    expect(headers).toEqual(['Name', 'Dienst', 'Adresse', 'Letzte Meldung', 'Aktionen']);
    expect(within(row).getByRole('rowheader')).toContainElement(toggle);
    expect(within(row).getByText('…wxyz')).toBeVisible();
    expect(within(row).getByText('Radarr', { selector: 'span' })).toBeVisible();
    expect(within(row).getByText(/2026/)).toBeVisible();
    expect(within(row).queryByText('Hat noch nie etwas gemeldet.')).toBeNull();
    // Folded: neither the mapping nor the log, and the log is not even asked for.
    expect(toggle).toHaveAttribute('aria-expanded', 'false');
    expect(screen.queryByLabelText('Pfad im Dienst')).toBeNull();
    expect(calls.some((call) => call.path === '/admin/integrations/1/events')).toBe(false);

    const section = await openIntegration('Radarr');
    // The mapping is editable, pre-filled with the stored pair.
    expect(within(section).getByLabelText('Pfad im Dienst')).toHaveValue('/movies');
    expect(within(section).getByLabelText('Pfad auf diesem Server')).toHaveValue('/media/Filme');

    // The log shows both paths and the outcome of every report, newest first.
    const log = await within(section).findByRole('table', { name: 'Meldungen von Radarr' });
    const [, downloadRow, testRow] = within(log).getAllByRole('row');
    expect(within(downloadRow!).getByText('Download')).toBeVisible();
    expect(within(downloadRow!).getByText('/media/Filme/Dune (2021)')).toBeVisible();
    expect(within(downloadRow!).getByText('Ordner wird gelesen')).toBeVisible();
    expect(within(testRow!).getByText('Test quittiert')).toBeVisible();
  });

  it('an integration that never reported says so', async () => {
    mockApi(
      routes({ 'GET /admin/integrations': () => json([{ ...radarr, lastSeenAt: undefined }]) }),
    );
    renderApp('/admin/integrations');
    const { row } = await integrationRow('Radarr');
    expect(within(row).getByText('Hat noch nie etwas gemeldet.')).toBeVisible();
  });

  it('shows the full address once after creating an integration, with the warning and a copy button', async () => {
    const url = 'https://slouchcat.example.ch/api/v1/webhooks/TOKEN-shown-once';
    const user = userEvent.setup();
    const writeText = vi.spyOn(navigator.clipboard, 'writeText');
    const { calls } = mockApi(
      routes({
        'POST /admin/integrations': () =>
          json(
            {
              integration: { ...radarr, id: 2, name: 'Sonarr', kind: 'sonarr' },
              url,
              token: 'TOKEN-shown-once',
            },
            201,
          ),
      }),
    );
    renderApp('/admin/integrations');

    const { section } = await integrationRow('Radarr');
    const form = within(section).getByRole('group', { name: 'Neue Integration' });
    await user.type(within(form).getByLabelText('Name'), 'Sonarr');
    await user.click(within(form).getByRole('radio', { name: 'Sonarr' }));
    await user.click(within(form).getByRole('button', { name: 'Integration anlegen' }));
    expect(calls.find((call) => call.method === 'POST')?.body).toEqual({
      name: 'Sonarr',
      kind: 'sonarr',
    });

    const panel = await screen.findByRole('region', { name: 'Adresse für «Sonarr»' });
    expect(
      within(panel).getByText('Jetzt kopieren — danach ist die Adresse nicht mehr sichtbar.'),
    ).toBeVisible();
    expect(within(panel).getByLabelText('Vollständige Adresse')).toHaveValue(url);
    expect(within(panel).getByLabelText('Vollständige Adresse')).toHaveFocus();

    await user.click(within(panel).getByRole('button', { name: 'Adresse kopieren' }));
    expect(writeText).toHaveBeenCalledWith(url);
    expect(await within(panel).findByText('Adresse kopiert.')).toBeVisible();

    // Hiding it is final: the address is nowhere on the page any more.
    await user.click(within(panel).getByRole('button', { name: 'Ausblenden' }));
    await waitFor(() => expect(screen.queryByDisplayValue(url)).not.toBeInTheDocument());
  });

  it('renews a token after a confirmation and shows the new address once', async () => {
    const url = 'https://slouchcat.example.ch/api/v1/webhooks/NEW-token';
    const user = userEvent.setup();
    mockApi(
      routes({
        'POST /admin/integrations/1/token': () =>
          json({ integration: { ...radarr, tokenTail: 'oken' }, url, token: 'NEW-token' }),
      }),
    );
    renderApp('/admin/integrations');

    const { row } = await integrationRow('Radarr');
    await user.click(within(row).getByRole('button', { name: 'Token von Radarr erneuern' }));
    const dialog = await screen.findByRole('alertdialog', {
      name: 'Token von «Radarr» erneuern?',
    });
    expect(within(dialog).getByText(/hört sofort auf zu arbeiten/)).toBeVisible();
    await user.click(within(dialog).getByRole('button', { name: 'Erneuern' }));

    const panel = await screen.findByRole('region', { name: 'Adresse für «Radarr»' });
    expect(within(panel).getByLabelText('Vollständige Adresse')).toHaveValue(url);
  });

  it('a failed renewal is reported in the block', async () => {
    const user = userEvent.setup();
    mockApi(routes({ 'POST /admin/integrations/1/token': () => problem(500, 'internal') }));
    renderApp('/admin/integrations');
    const { row } = await integrationRow('Radarr');
    await user.click(within(row).getByRole('button', { name: 'Token von Radarr erneuern' }));
    await user.click(
      within(await screen.findByRole('alertdialog')).getByRole('button', { name: 'Erneuern' }),
    );
    expect(await within(webhooks()).findByRole('alert')).toBeVisible();
  });

  it('saves the path mapping and reports a rejected pair on its field', async () => {
    const user = userEvent.setup();
    const calls: unknown[] = [];
    mockApi(
      routes({
        'PUT /admin/integrations/1/path-map': (call) => {
          calls.push(call.body);
          return problem(422, 'validation_failed', {
            errors: [{ field: 'pathMap.0.from', code: 'invalid' }],
          });
        },
      }),
    );
    renderApp('/admin/integrations');

    const section = await openIntegration('Radarr');
    const from = within(section).getByLabelText('Pfad im Dienst');
    await user.clear(from);
    await user.type(from, 'relativ');
    await user.click(
      within(section).getByRole('button', { name: 'Pfad-Umsetzung von Radarr speichern' }),
    );

    await waitFor(() => expect(calls).toHaveLength(1));
    expect(calls[0]).toEqual({ pathMap: [{ from: 'relativ', to: '/media/Filme' }] });
    await waitFor(() => expect(from).toHaveAccessibleDescription(/ungültig/i));
  });

  it('adds and removes rows and saves them together', async () => {
    const user = userEvent.setup();
    let pathMap = radarr.pathMap;
    const calls: unknown[] = [];
    mockApi(
      routes({
        'GET /admin/integrations': () => json([{ ...radarr, pathMap }]),
        'PUT /admin/integrations/1/path-map': (call) => {
          calls.push(call.body);
          pathMap = (call.body as { pathMap: Integration['pathMap'] }).pathMap;
          return json({ ...radarr, pathMap });
        },
      }),
    );
    renderApp('/admin/integrations');

    const section = await openIntegration('Radarr');
    await user.click(within(section).getByRole('button', { name: 'Zeile hinzufügen' }));
    const froms = within(section).getAllByLabelText('Pfad im Dienst');
    const tos = within(section).getAllByLabelText('Pfad auf diesem Server');
    expect(froms).toHaveLength(2);
    await user.type(froms[1]!, ' /tv ');
    await user.type(tos[1]!, '/media/Serien');
    await user.click(within(section).getByRole('button', { name: 'Zeile für /movies entfernen' }));
    await user.click(
      within(section).getByRole('button', { name: 'Pfad-Umsetzung von Radarr speichern' }),
    );
    expect(await within(section).findByText('Pfad-Umsetzung gespeichert.')).toBeVisible();
    expect(calls).toEqual([{ pathMap: [{ from: '/tv', to: '/media/Serien' }] }]);
    expect(within(section).getByLabelText('Pfad im Dienst')).toHaveValue('/tv');
    expect(within(section).queryByText('ungespeichert')).toBeNull();
  });

  it('keeps unsaved rows when the row or the block is folded away (S-13)', async () => {
    const user = userEvent.setup();
    const { calls } = mockApi(routes());
    renderApp('/admin/integrations');

    const section = await openIntegration('Radarr');
    const { toggle } = await integrationRow('Radarr');
    await user.clear(within(section).getByLabelText('Pfad auf diesem Server'));
    await user.type(within(section).getByLabelText('Pfad auf diesem Server'), '/media/Kino');

    // Folding the row away unmounts the form; the draft stays in the page and the row says so.
    await user.click(toggle);
    expect(toggle).toHaveAttribute('aria-expanded', 'false');
    expect(screen.queryByLabelText('Pfad auf diesem Server')).toBeNull();
    expect(within(toggle).getByText('ungespeichert')).toBeVisible();
    const band = within(section).getByRole('button', { name: BLOCK });
    expect(band).toHaveAttribute('aria-expanded', 'true');
    await user.click(band);
    expect(band).toHaveAttribute('aria-expanded', 'false');
    await user.click(band);
    await user.click(toggle);
    expect(within(section).getByLabelText('Pfad auf diesem Server')).toHaveValue('/media/Kino');
    expect(calls.some((call) => call.method === 'PUT')).toBe(false);

    // Typing the stored value back is no change any more.
    await user.clear(within(section).getByLabelText('Pfad auf diesem Server'));
    await user.type(within(section).getByLabelText('Pfad auf diesem Server'), '/media/Filme');
    await waitFor(() => expect(within(section).queryByText('ungespeichert')).toBeNull());
  });

  it('follows a mapping another admin saved, and keeps unsaved rows with a warning', async () => {
    const user = userEvent.setup();
    let pathMap = radarr.pathMap;
    mockApi(routes({ 'GET /admin/integrations': () => json([{ ...radarr, pathMap }]) }));
    const { queryClient } = renderApp('/admin/integrations');

    const section = await openIntegration('Radarr');
    const to = within(section).getByLabelText('Pfad auf diesem Server');

    // Nothing typed here: the form takes the other version over without saying anything.
    pathMap = [{ from: '/movies', to: '/media/Kino' }];
    await queryClient.invalidateQueries({ queryKey: ['admin', 'integrations'] });
    await waitFor(() => expect(to).toHaveValue('/media/Kino'));
    expect(within(section).queryByText(/Ein anderer Zugang/)).not.toBeInTheDocument();

    // With a row typed here the form keeps it and offers the other version instead.
    await user.clear(to);
    await user.type(to, '/media/Eigen');
    pathMap = [{ from: '/movies', to: '/media/Fremd' }];
    await queryClient.invalidateQueries({ queryKey: ['admin', 'integrations'] });

    expect(await within(section).findByText(/Ein anderer Zugang/)).toBeVisible();
    expect(to).toHaveValue('/media/Eigen');

    await user.click(within(section).getByRole('button', { name: 'Andere Fassung übernehmen' }));
    expect(to).toHaveValue('/media/Fremd');
    expect(within(section).queryByText(/Ein anderer Zugang/)).not.toBeInTheDocument();
  });

  it('deletes an integration after a confirmation and moves the focus to the page heading', async () => {
    const user = userEvent.setup();
    let deleted = 0;
    mockApi(
      routes({
        'GET /admin/integrations': () => json(deleted ? [] : [radarr]),
        'DELETE /admin/integrations/1': () => {
          deleted += 1;
          return new Response(null, { status: 204 });
        },
      }),
    );
    renderApp('/admin/integrations');

    const { row } = await integrationRow('Radarr');
    await user.click(within(row).getByRole('button', { name: 'Radarr löschen' }));
    const dialog = await screen.findByRole('alertdialog', { name: '«Radarr» löschen?' });
    await user.click(within(dialog).getByRole('button', { name: 'Löschen' }));
    await waitFor(() => expect(deleted).toBe(1));
    expect(
      await within(webhooks()).findByText('Noch keine Integration eingerichtet.'),
    ).toBeVisible();
    await waitFor(() =>
      expect(screen.getByRole('heading', { level: 1, name: 'Integrationen' })).toHaveFocus(),
    );
  });

  it('says so when no integration exists yet', async () => {
    mockApi(routes({ 'GET /admin/integrations': () => json([]) }));
    renderApp('/admin/integrations');
    expect(
      await within(await screen.findByRole('region', { name: BLOCK })).findByText(
        'Noch keine Integration eingerichtet.',
      ),
    ).toBeVisible();
    expect(within(webhooks()).queryByRole('table')).toBeNull();
  });
});
