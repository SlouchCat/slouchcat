import type { AdminUserPick } from '@slouchcat/api-client';
import { screen, waitFor, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { ids, profiles } from '@/test/fixtures';
import { json, mockApi, problem, signedIn } from '@/test/api';
import { adminShell, renderAdminPage } from '@/test/admin';

// /admin/picks (concept 18 part B, 14.7, ADR 0028): the retention of the user picks and the titles
// recommended now, newest first, anonymous (count and day only), each with «Entfernen».

const heatPick: AdminUserPick = {
  itemId: ids.heat,
  kind: 'movie',
  title: 'Heat',
  year: 1995,
  count: 2,
  lastPickedOn: '2026-11-08',
  expiresInDays: 59,
  posterUrl: `/api/v1/admin/items/${ids.heat}/images/poster/w200?v=heat`,
};
const alienPick: AdminUserPick = {
  itemId: ids.alien,
  kind: 'movie',
  title: 'Alien',
  year: 1979,
  count: 1,
  lastPickedOn: '2026-09-10',
  expiresInDays: 1,
};

const routes = {
  ...signedIn,
  ...adminShell,
  'GET /profiles': () => json(profiles),
  'GET /admin/picks/settings': () => json({ retentionDays: 60 }),
};

describe('admin user picks page (concept 18 part B)', () => {
  afterEach(() => {
    vi.restoreAllMocks();
    vi.unstubAllGlobals();
  });

  it('lists the picks newest first with day, days left and count, never a profile', async () => {
    mockApi({ ...routes, 'GET /admin/picks': () => json([heatPick, alienPick]) });
    await renderAdminPage('/admin/picks');

    expect(
      await screen.findByRole('heading', { level: 1, name: 'User-Empfehlungen' }),
    ).toBeVisible();
    const table = await screen.findByRole('table', { name: 'Aktuelle Empfehlungen' });
    const rows = within(table).getAllByRole('row').slice(1);
    expect(rows.map((row) => within(row).getByRole('rowheader').textContent)).toEqual([
      'HeatFilm · 1995',
      'AlienFilm · 1979',
    ]);
    expect(within(rows[0]!).getByText('08.11.2026')).toBeVisible();
    expect(within(rows[0]!).getByText('in 59 Tagen')).toBeVisible();
    expect(within(rows[0]!).getByText('2 Empfehlungen')).toBeVisible();
    expect(within(rows[1]!).getByText('in 1 Tag')).toBeVisible();
    expect(within(rows[1]!).getByText('1 Empfehlung')).toBeVisible();
    expect(screen.getByText('2 Titel')).toBeVisible();
    expect(
      screen.getByText(
        'Die Verwaltung sieht die Anzahl der Empfehlungen, aber nie, welches Profil empfohlen hat.',
      ),
    ).toBeVisible();
    // The administration adds nothing: the profiles recommend on the title page.
    expect(screen.queryByRole('button', { name: 'Hinzufügen' })).not.toBeInTheDocument();
    expect(screen.getByRole('spinbutton', { name: 'Empfehlungen bleiben' })).toHaveValue(60);
  });

  it('removes every pick of a title after asking', async () => {
    let list = [heatPick, alienPick];
    const { calls } = mockApi({
      ...routes,
      'GET /admin/picks': () => json(list),
      [`DELETE /admin/picks/${ids.heat}`]: () => {
        list = [alienPick];
        return new Response(null, { status: 204 });
      },
    });
    await renderAdminPage('/admin/picks');
    const user = userEvent.setup();
    await user.click(await screen.findByRole('button', { name: 'Heat entfernen' }));
    const dialog = await screen.findByRole('alertdialog', {
      name: 'Heat aus den User-Empfehlungen entfernen?',
    });
    await user.click(within(dialog).getByRole('button', { name: 'Entfernen' }));
    await waitFor(() =>
      expect(screen.queryByRole('button', { name: 'Heat entfernen' })).not.toBeInTheDocument(),
    );
    expect(calls.filter((call) => call.method === 'DELETE').map((call) => call.path)).toEqual([
      `/admin/picks/${ids.heat}`,
    ]);
    expect(await screen.findByText('Heat ist nicht mehr empfohlen.')).toBeInTheDocument();
  });

  it('saves the retention and shows the server refusal at the field', async () => {
    const bodies: unknown[] = [];
    mockApi({
      ...routes,
      'GET /admin/picks': () => json([]),
      'PUT /admin/picks/settings': (call) => {
        const body = call.body as { retentionDays: number };
        bodies.push(body);
        return body.retentionDays < 7
          ? problem(422, 'validation_failed', {
              errors: [{ field: 'retentionDays', code: 'invalid_value' }],
            })
          : json(body);
      },
    });
    await renderAdminPage('/admin/picks');
    expect(
      await screen.findByText(
        'Zurzeit empfiehlt kein Profil einen Titel. Ohne Empfehlung zeigt die Startseite keine Reihe «User-Empfehlungen».',
      ),
    ).toBeVisible();
    const user = userEvent.setup();
    const field = await screen.findByRole('spinbutton', { name: 'Empfehlungen bleiben' });
    await user.clear(field);
    await user.type(field, '6');
    await user.click(screen.getByRole('button', { name: 'Speichern' }));
    await waitFor(() => expect(field).toHaveAttribute('aria-invalid', 'true'));
    await user.clear(field);
    await user.type(field, '30{Enter}');
    expect(await screen.findByText('Empfehlungen bleiben jetzt 30 Tage.')).toBeInTheDocument();
    expect(bodies).toEqual([{ retentionDays: 6 }, { retentionDays: 30 }]);
  });
});
