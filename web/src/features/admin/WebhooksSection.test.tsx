import type { AdminSettings, Integration, IntegrationEvent } from '@slouchcat/api-client';
import { screen, waitFor, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { json, mockApi, problem, renderApp, signedIn, type Routes } from '@/test/api';

// M9 webhooks (I-8, decision 5.6): one address per integration, shown exactly once with a copy
// button, afterwards only its last four characters; path mapping, event log, renew and delete.

const settings: AdminSettings = {
  publicUrlSource: 'none',
  updateCheckEnabled: true,
  keys: {
    tmdb: { source: 'none' },
    tvdb: { source: 'none' },
    tvdbPin: { source: 'none' },
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

const webhooks = () => screen.getByRole('region', { name: 'Webhooks von Radarr und Sonarr' });

describe('webhook integrations (I-8)', () => {
  afterEach(() => vi.unstubAllGlobals());

  it('lists an integration with its last four characters, last contact and event log', async () => {
    mockApi(routes());
    renderApp('/admin/integrations');

    const section = await screen.findByRole('region', { name: 'Webhooks von Radarr und Sonarr' });
    expect(await within(section).findByRole('heading', { level: 3, name: 'Radarr' })).toBeVisible();
    expect(within(section).getByText('Adresse endet auf …wxyz')).toBeVisible();
    expect(within(section).getByText(/Letzte Meldung:/)).toBeVisible();

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

  it('shows the full address once after creating an integration, with the warning and a copy button', async () => {
    const url = 'https://slouchcat.example.ch/api/v1/webhooks/TOKEN-shown-once';
    const user = userEvent.setup();
    const writeText = vi.spyOn(navigator.clipboard, 'writeText');
    mockApi(
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

    const section = await screen.findByRole('region', { name: 'Webhooks von Radarr und Sonarr' });
    await within(section).findByRole('heading', { level: 3, name: 'Radarr' });
    await user.type(within(section).getByLabelText('Name'), 'Sonarr');
    await user.click(within(section).getByRole('radio', { name: 'Sonarr' }));
    await user.click(within(section).getByRole('button', { name: 'Integration anlegen' }));

    const panel = await screen.findByRole('region', { name: 'Adresse für «Sonarr»' });
    expect(
      within(panel).getByText('Jetzt kopieren — danach ist die Adresse nicht mehr sichtbar.'),
    ).toBeVisible();
    expect(within(panel).getByLabelText('Vollständige Adresse')).toHaveValue(url);

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

    const section = await screen.findByRole('region', { name: 'Webhooks von Radarr und Sonarr' });
    await within(section).findByRole('heading', { level: 3, name: 'Radarr' });
    await user.click(within(section).getByRole('button', { name: 'Token von Radarr erneuern' }));
    const dialog = await screen.findByRole('alertdialog');
    expect(within(dialog).getByText(/hört sofort auf zu arbeiten/)).toBeVisible();
    await user.click(within(dialog).getByRole('button', { name: 'Erneuern' }));

    const panel = await screen.findByRole('region', { name: 'Adresse für «Radarr»' });
    expect(within(panel).getByLabelText('Vollständige Adresse')).toHaveValue(url);
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

    const section = await screen.findByRole('region', { name: 'Webhooks von Radarr und Sonarr' });
    await within(section).findByRole('heading', { level: 3, name: 'Radarr' });
    const from = within(section).getByLabelText('Pfad im Dienst');
    await user.clear(from);
    await user.type(from, 'relativ');
    await user.click(within(section).getByRole('button', { name: 'Speichern' }));

    await waitFor(() => expect(calls).toHaveLength(1));
    expect(calls[0]).toEqual({ pathMap: [{ from: 'relativ', to: '/media/Filme' }] });
    await waitFor(() => expect(from).toHaveAccessibleDescription(/ungültig/i));
  });

  it('deletes an integration after a confirmation', async () => {
    const user = userEvent.setup();
    let deleted = 0;
    mockApi(
      routes({
        'DELETE /admin/integrations/1': () => {
          deleted += 1;
          return new Response(null, { status: 204 });
        },
      }),
    );
    renderApp('/admin/integrations');

    const section = await screen.findByRole('region', { name: 'Webhooks von Radarr und Sonarr' });
    await within(section).findByRole('heading', { level: 3, name: 'Radarr' });
    await user.click(within(section).getByRole('button', { name: 'Radarr löschen' }));
    const dialog = await screen.findByRole('alertdialog');
    await user.click(within(dialog).getByRole('button', { name: 'Löschen' }));
    await waitFor(() => expect(deleted).toBe(1));
  });

  it('says so when no integration exists yet', async () => {
    mockApi(routes({ 'GET /admin/integrations': () => json([]) }));
    renderApp('/admin/integrations');
    expect(
      await within(
        await screen.findByRole('region', { name: 'Webhooks von Radarr und Sonarr' }),
      ).findByText('Noch keine Integration eingerichtet.'),
    ).toBeVisible();
    expect(webhooks()).toBeVisible();
  });
});
