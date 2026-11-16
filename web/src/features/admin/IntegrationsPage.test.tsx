import type { AdminSettings, AdminSettingsUpdate } from '@slouchcat/api-client';
import { screen, waitFor, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { json, mockApi, problem, renderApp, session, signedIn, type Routes } from '@/test/api';
import { dashboardFixture } from '@/test/admin';
import { fill } from '@/test/fill';

// Integrations (A-2; concept 12, variant A; admin redesign): a tile per service — provider keys,
// webhooks, e-mail server, Telegram bot, Web Push, and the server's update check and public
// address — with its state from the dashboard, and a view per service (`?item=`). Secrets are
// write-only; values from deploy/.env are read-only here. Unsaved input survives a change of view
// and makes leaving the page ask first (S-13, S-17).

const settings = (overrides: Partial<AdminSettings> = {}): AdminSettings => ({
  publicUrlSource: 'none',
  updateCheckEnabled: true,
  transcodeHw: 'auto',
  transcodeHwSource: 'none',
  transcodeHwRestart: false,
  keys: {
    tmdb: { source: 'stored', hint: 'a1b2' },
    tvdb: { source: 'none' },
    tvdbPin: { source: 'none' },
    fanart: { source: 'none' },
    fanartClient: { source: 'none' },
    youtube: { source: 'none' },
    trakt: { source: 'env' },
    googleBooks: { source: 'none' },
  },
  email: { source: 'none' },
  telegram: { source: 'none' },
  push: { configured: true, publicKey: 'BEl62iUYgUivxIkv69yViEuiBIa' },
  ...overrides,
});

const routes = (extra: Routes = {}): Routes => ({
  ...signedIn,
  'GET /admin/settings': () => json(settings()),
  // M9: the page also carries the webhook integrations (I-8); its own tests live next door.
  'GET /admin/integrations': () => json([]),
  ...extra,
});

const section = (name: string) => screen.getByRole('region', { name });

/** The field itself: its setting row is a group with the same name (Int-A, hidden field label). */
const INPUT = { selector: 'input' };

/** A setting row of the «Server» block: a group named by its label. */
const serverRow = async (name: string) =>
  within(await screen.findByRole('region', { name: 'Server' })).getByRole('group', { name });

const BLOCKS = [
  'Schlüssel der Anbieter',
  'Webhooks von Radarr und Sonarr',
  'E-Mail-Server',
  'Telegram-Bot',
  'Web-Push',
  'Server',
];

const puts = (calls: { method: string; path: string; body: unknown }[]) =>
  calls
    .filter((call) => call.method === 'PUT' && call.path === '/admin/settings')
    .map((call) => call.body as AdminSettingsUpdate);

/** The card of a provider in the band of the keys (concept 20, P-12). */
const card = (name: string) =>
  within(section('Schlüssel der Anbieter')).getByRole('region', { name });

/** Opens the card of a provider with its button and returns the card. */
async function openCard(user: ReturnType<typeof userEvent.setup>, name: string) {
  const region = within(
    await screen.findByRole('region', { name: 'Schlüssel der Anbieter' }),
  ).getByRole('region', { name });
  const toggle = within(region).getByRole('button', { name: new RegExp(`: ${name}$`) });
  if (toggle.getAttribute('aria-expanded') !== 'true') await user.click(toggle);
  return region;
}

describe('integrations (A-2)', () => {
  afterEach(() => vi.unstubAllGlobals());

  it('shows six provider cards, folded, with their purpose and state (P-12)', async () => {
    mockApi(routes());
    renderApp('/admin/integrations?item=keys');
    expect(await screen.findByRole('heading', { level: 1, name: 'Integrationen' })).toBeVisible();
    const keys = await screen.findByRole('region', { name: 'Schlüssel der Anbieter' });
    const cards = within(keys).getAllByRole('region');
    expect(
      cards.map((region) => within(region).getByRole('heading', { level: 3 }).textContent),
    ).toEqual(['TMDB', 'TVDB', 'Fanart.tv', 'YouTube', 'Trakt', 'Google Books']);
    expect(within(card('TMDB')).getByText('Gespeichert')).toBeVisible();
    expect(within(card('TMDB')).getByText(/Pflicht für Video-Bibliotheken/)).toBeVisible();
    expect(within(card('Trakt')).getByText('In deploy/.env gesetzt')).toBeVisible();
    expect(within(keys).getAllByText('Nicht gesetzt')).toHaveLength(4);
    // Folded: no field; the button says what opening does.
    expect(within(keys).queryByRole('textbox')).toBeNull();
    expect(within(card('TMDB')).getByRole('button', { name: 'Bearbeiten: TMDB' })).toHaveAttribute(
      'aria-expanded',
      'false',
    );
    expect(
      within(card('YouTube')).getByRole('button', { name: 'Einrichten: YouTube' }),
    ).toBeVisible();
    expect(within(card('Trakt')).getByRole('button', { name: 'Ansehen: Trakt' })).toBeVisible();
  });

  it('an open card holds the fields of its provider, one help sentence with one link and one «Speichern»', async () => {
    mockApi(routes());
    renderApp('/admin/integrations?item=keys');
    const user = userEvent.setup();
    const tvdb = await openCard(user, 'TVDB');
    const toggle = within(tvdb).getByRole('button', { name: 'Zuklappen: TVDB' });
    expect(toggle).toHaveAttribute('aria-expanded', 'true');
    expect(document.getElementById(toggle.getAttribute('aria-controls') ?? '')).toBeInTheDocument();
    const key = within(tvdb).getByLabelText('Schlüssel', INPUT);
    expect(key).toHaveAttribute('type', 'password');
    expect(key).toHaveAttribute('placeholder', 'Nicht gesetzt');
    expect(within(tvdb).getByLabelText('Abo-PIN (nur bei Abo-Schlüssel)', INPUT)).toBeVisible();
    const links = within(tvdb).getAllByRole('link');
    expect(links).toHaveLength(1);
    expect(links[0]).toHaveAccessibleName('Schlüssel bei TVDB holen');
    expect(links[0]).toHaveAttribute('href', 'https://thetvdb.com/dashboard/account/apikey');
    expect(links[0]).toHaveAttribute('target', '_blank');
    expect(within(tvdb).getAllByRole('button', { name: /speichern$/ })).toHaveLength(1);
    // Nothing set: nothing to check or delete.
    expect(within(tvdb).queryByRole('button', { name: 'TVDB prüfen' })).toBeNull();
    expect(within(tvdb).queryByRole('button', { name: 'TVDB löschen' })).toBeNull();

    // A stored key: the placeholder names its end, the description says empty keeps it.
    const tmdb = await openCard(user, 'TMDB');
    const token = within(tmdb).getByLabelText('Zugriffstoken (v4)', INPUT);
    expect(token).toHaveValue('');
    expect(token).toHaveAttribute('placeholder', 'Gesetzt, endet auf a1b2; leer lassen behält ihn');
    expect(token).toHaveAccessibleDescription(/Leer lassen, um den gespeicherten Wert zu behalten/);
    expect(within(tmdb).getByRole('button', { name: 'TMDB löschen' })).toBeVisible();
    expect(within(tmdb).getByRole('button', { name: 'TMDB prüfen' })).toBeVisible();

    await user.click(toggle);
    expect(within(tvdb).queryByRole('textbox')).toBeNull();
  });

  it('a value from deploy/.env locks the fields, says so once and can still be checked', async () => {
    const { calls } = mockApi(
      routes({ 'POST /admin/settings/keys/trakt/check': () => json({ ok: true }) }),
    );
    renderApp('/admin/integrations?item=keys');
    const user = userEvent.setup();
    const trakt = await openCard(user, 'Trakt');
    const field = within(trakt).getByLabelText('Client-ID', INPUT);
    expect(field).toBeDisabled();
    expect(field).toHaveAttribute('placeholder', 'In deploy/.env gesetzt');
    expect(within(trakt).getAllByText(/lässt sich nur dort ändern/)).toHaveLength(1);
    expect(within(trakt).queryByRole('button', { name: 'Trakt speichern' })).toBeNull();
    expect(within(trakt).queryByRole('button', { name: 'Trakt löschen' })).toBeNull();
    await user.click(within(trakt).getByRole('button', { name: 'Trakt prüfen' }));
    expect(
      await within(trakt).findByText('Trakt: Der Anbieter hat geantwortet, der Schlüssel gilt.'),
    ).toBeVisible();
    expect(calls.filter((call) => call.method === 'POST').map((call) => call.path)).toEqual([
      '/admin/settings/keys/trakt/check',
    ]);
  });

  it('«Prüfen» names the provider’s answer when the key does not hold', async () => {
    mockApi(
      routes({
        'POST /admin/settings/keys/tmdb/check': () =>
          json({ ok: false, problem: 'invalid', message: 'tmdb /configuration: status 401' }),
      }),
    );
    renderApp('/admin/integrations?item=keys');
    const user = userEvent.setup();
    const tmdb = await openCard(user, 'TMDB');
    await user.click(within(tmdb).getByRole('button', { name: 'TMDB prüfen' }));
    expect(
      await within(tmdb).findByText(
        'TMDB: Der Anbieter lehnt den Schlüssel ab (tmdb /configuration: status 401).',
      ),
    ).toBeVisible();
  });

  it('saves the fields of a card in one request; empty keeps, blanks alone are refused', async () => {
    let current = settings();
    const { calls } = mockApi(
      routes({
        'GET /admin/settings': () => json(current),
        'PUT /admin/settings': () => {
          current = settings({
            keys: {
              ...current.keys,
              tvdb: { source: 'stored', hint: '9xyz' },
              tvdbPin: { source: 'stored' },
            },
          });
          return json(current);
        },
      }),
    );
    renderApp('/admin/integrations?item=keys');
    const user = userEvent.setup();
    const tvdb = await openCard(user, 'TVDB');
    const save = within(tvdb).getByRole('button', { name: 'TVDB speichern' });
    const key = within(tvdb).getByLabelText('Schlüssel', INPUT);
    const pin = within(tvdb).getByLabelText('Abo-PIN (nur bei Abo-Schlüssel)', INPUT);
    // Empty keeps what is stored (decision 18): nothing is sent, the focus stays on «Speichern».
    await user.click(save);
    expect(save).toHaveFocus();
    expect(puts(calls)).toHaveLength(0);
    // Blanks alone are no key: the field says so.
    await fill(user, key, '   ');
    await user.click(save);
    await waitFor(() => expect(key).toHaveAccessibleDescription(/Pflichtfeld/));
    expect(puts(calls)).toHaveLength(0);

    await fill(user, key, 'tvdb-key-1239xyz');
    await fill(user, pin, '4321');
    // Typed input marks the card.
    expect(within(tvdb).getByText('ungespeichert')).toBeVisible();
    await user.click(save);
    expect(await within(tvdb).findByText('TVDB gespeichert.')).toBeVisible();
    expect(puts(calls)).toEqual([{ keys: { tvdb: 'tvdb-key-1239xyz', tvdbPin: '4321' } }]);
    expect(key).toHaveValue('');
    expect(pin).toHaveValue('');
    expect(within(tvdb).getByText('Gespeichert')).toBeVisible();
    expect(within(tvdb).queryByText('ungespeichert')).toBeNull();
  });

  it('a key the provider rejects is stored all the same, and the card says so (concept 21 P-2)', async () => {
    let current = settings();
    const { calls } = mockApi(
      routes({
        'GET /admin/settings': () => json(current),
        'PUT /admin/settings': () => {
          current = settings({
            keys: { ...current.keys, tvdb: { source: 'stored', hint: '0bad', rejected: true } },
          });
          return json(current);
        },
      }),
    );
    renderApp('/admin/integrations?item=keys');
    const user = userEvent.setup();
    const tvdb = await openCard(user, 'TVDB');
    expect(within(tvdb).queryByText('abgewiesen')).toBeNull();
    await fill(user, within(tvdb).getByLabelText('Schlüssel', INPUT), 'tvdb-key-0bad');
    await user.click(within(tvdb).getByRole('button', { name: 'TVDB speichern' }));
    expect(
      await within(tvdb).findByText('TVDB gespeichert, aber der Anbieter weist den Schlüssel ab.'),
    ).toBeVisible();
    expect(within(tvdb).queryByText('TVDB gespeichert.')).toBeNull();
    expect(within(tvdb).getByText('abgewiesen')).toBeVisible();
    expect(within(tvdb).getByLabelText('Schlüssel', INPUT)).not.toHaveAttribute(
      'aria-invalid',
      'true',
    );
    expect(puts(calls)).toEqual([{ keys: { tvdb: 'tvdb-key-0bad' } }]);
    // The other cards show no rejection.
    expect(within(card('TMDB')).queryByText('abgewiesen')).toBeNull();
  });

  it('an invalid TMDB key is reported at its field (servers before contract 1.32.0)', async () => {
    mockApi(routes({ 'PUT /admin/settings': () => problem(422, 'tmdb_key_invalid') }));
    renderApp('/admin/integrations?item=keys');
    const user = userEvent.setup();
    const tmdb = await openCard(user, 'TMDB');
    const field = within(tmdb).getByLabelText('Zugriffstoken (v4)', INPUT);
    await fill(user, field, 'bad');
    await user.click(within(tmdb).getByRole('button', { name: 'TMDB speichern' }));
    await waitFor(() => expect(field).toHaveAttribute('aria-invalid', 'true'));
    expect(field).toHaveAccessibleDescription(/TMDB hat den Schlüssel abgelehnt/);
  });

  it('deletes the stored keys of a card after the confirmation', async () => {
    const { calls } = mockApi(
      routes({
        'GET /admin/settings': () =>
          json(
            settings({
              keys: {
                ...settings().keys,
                fanart: { source: 'stored', hint: 'a91e' },
                fanartClient: { source: 'stored' },
              },
            }),
          ),
        'PUT /admin/settings': () => json(settings()),
      }),
    );
    renderApp('/admin/integrations?item=keys');
    const user = userEvent.setup();
    const fanart = await openCard(user, 'Fanart.tv');
    await user.click(within(fanart).getByRole('button', { name: 'Fanart.tv löschen' }));
    const dialog = await screen.findByRole('alertdialog', { name: 'Fanart.tv löschen?' });
    await user.click(within(dialog).getByRole('button', { name: 'Löschen' }));
    expect(await within(fanart).findByText('Fanart.tv gelöscht.')).toBeVisible();
    expect(puts(calls)).toEqual([{ clear: ['fanart', 'fanartClient'] }]);
    expect(within(fanart).queryByRole('button', { name: 'Fanart.tv löschen' })).toBeNull();
    // «Löschen …» is gone with the keys: the card's «Speichern» takes the focus (ALL-2).
    await waitFor(() =>
      expect(within(fanart).getByRole('button', { name: 'Fanart.tv speichern' })).toHaveFocus(),
    );
  });

  it('a value the environment sets cannot be overridden (env_wins)', async () => {
    mockApi(
      routes({
        'PUT /admin/settings': () =>
          problem(422, 'env_wins', { errors: [{ field: 'keys.tvdb', code: 'env_wins' }] }),
      }),
    );
    renderApp('/admin/integrations?item=keys');
    const user = userEvent.setup();
    const tvdb = await openCard(user, 'TVDB');
    const field = within(tvdb).getByLabelText('Schlüssel', INPUT);
    await fill(user, field, 'tvdb-key');
    await user.click(within(tvdb).getByRole('button', { name: 'TVDB speichern' }));
    await waitFor(() => expect(field).toHaveAttribute('aria-invalid', 'true'));
    expect(field).toHaveAccessibleDescription(/in deploy\/.env gesetzt/);
    expect(await screen.findByRole('alert')).toHaveTextContent(/lässt sich nur dort ändern/);
  });

  it('names every hidden-label field like the visible label of its row (WCAG 2.5.3)', async () => {
    mockApi(routes());
    const { router } = renderApp('/admin/integrations?item=keys');
    const keys = await screen.findByRole('region', { name: 'Schlüssel der Anbieter' });
    expect(within(keys).queryAllByRole('group')).toHaveLength(0);
    await router.navigate({ to: '/admin/integrations', search: { item: 'email' } });
    const rows = [
      ...within(await screen.findByRole('region', { name: 'E-Mail-Server' })).getAllByRole('group'),
    ];
    await router.navigate({ to: '/admin/integrations', search: { item: 'telegram' } });
    rows.push(
      ...within(await screen.findByRole('region', { name: 'Telegram-Bot' })).getAllByRole('group'),
    );
    let fields = 0;
    for (const row of rows) {
      const label = row.getAttribute('aria-labelledby');
      const visible = label ? document.getElementById(label)?.textContent : null;
      for (const input of row.querySelectorAll('input:not([type="radio"])')) {
        fields += 1;
        // «Server und Port» names the host; the port is named by its word in the label.
        const name = input.getAttribute('name') === 'email.port' ? 'Port' : (visible ?? '');
        expect(visible).toContain(name);
        expect(within(row).getByLabelText(name, INPUT)).toBe(input);
      }
    }
    // Five mail fields and the bot token; the provider cards start folded and label their
    // fields visibly (P-12).
    expect(fields).toBe(6);
  });

  it('stores the e-mail server, then sends a test mail to the main account', async () => {
    let current = settings();
    let testReply: Response = new Response(null, { status: 204 });
    const { calls } = mockApi(
      routes({
        'GET /admin/settings': () => json(current),
        'PUT /admin/settings': () => {
          current = settings({
            email: {
              source: 'stored',
              host: 'smtp.example.ch',
              port: 465,
              user: 'media',
              from: 'Slouchcat <media@example.ch>',
              tls: 'tls',
            },
          });
          return json(current);
        },
        'POST /admin/settings/email/test': () => testReply.clone(),
      }),
    );
    renderApp('/admin/integrations?item=email');
    const user = userEvent.setup();
    const email = await screen.findByRole('region', { name: 'E-Mail-Server' });
    expect(within(email).queryByRole('button', { name: 'Testmail senden' })).toBeNull();
    await fill(user, within(email).getByLabelText('Server und Port', INPUT), 'smtp.example.ch');
    const port = within(email).getByLabelText('Port', INPUT);
    expect(port).toHaveValue('587');
    await fill(user, port, '465');
    await fill(user, within(email).getByLabelText('Benutzer', INPUT), 'media');
    await fill(user, within(email).getByLabelText('Passwort', INPUT), 'smtp-secret');
    await fill(
      user,
      within(email).getByLabelText(/Absender/, INPUT),
      'Slouchcat <media@example.ch>',
    );
    await user.click(within(email).getByRole('radio', { name: 'TLS' }));
    await user.click(within(email).getByRole('button', { name: 'E-Mail-Server speichern' }));

    expect(await within(email).findByText('E-Mail-Server gespeichert.')).toBeVisible();
    expect(puts(calls)).toEqual([
      {
        email: {
          host: 'smtp.example.ch',
          port: 465,
          user: 'media',
          password: 'smtp-secret',
          from: 'Slouchcat <media@example.ch>',
          tls: 'tls',
        },
      },
    ]);
    // The password is write-only: the field is empty again and says how to keep it.
    expect(within(email).getByLabelText('Passwort', INPUT)).toHaveValue('');
    expect(within(email).getByLabelText('Passwort', INPUT)).toHaveAccessibleDescription(
      /gespeicherte Passwort zu behalten/,
    );

    await user.click(within(email).getByRole('button', { name: 'Testmail senden' }));
    expect(
      await within(email).findByText(`Testmail an ${session.account.email} gesendet.`),
    ).toBeVisible();

    testReply = problem(502, 'email_send_failed', { detail: '535 authentication failed' });
    await user.click(within(email).getByRole('button', { name: 'Testmail senden' }));
    expect(await within(email).findByRole('alert')).toHaveTextContent(
      'Die Testmail kam nicht an: 535 authentication failed',
    );

    testReply = problem(422, 'email_not_configured');
    await user.click(within(email).getByRole('button', { name: 'Testmail senden' }));
    expect(await within(email).findByRole('alert')).toHaveTextContent(
      'Es ist kein E-Mail-Server eingerichtet.',
    );
  });

  it('an e-mail server from deploy/.env is read-only but can send a test mail', async () => {
    mockApi(
      routes({
        'GET /admin/settings': () =>
          json(
            settings({
              email: { source: 'env', host: 'mail.local', port: 25, from: 'a@b.ch', tls: 'none' },
            }),
          ),
      }),
    );
    renderApp('/admin/integrations?item=email');
    const email = await screen.findByRole('region', { name: 'E-Mail-Server' });
    expect(within(email).getByLabelText('Server und Port', INPUT)).toBeDisabled();
    expect(within(email).getByLabelText('Server und Port', INPUT)).toHaveValue('mail.local');
    expect(within(email).getByText(/Ändern lässt sich der Wert nur dort/)).toBeVisible();
    expect(within(email).queryByRole('button', { name: 'E-Mail-Server speichern' })).toBeNull();
    expect(within(email).getByRole('button', { name: 'Testmail senden' })).toBeEnabled();
  });

  it('shows the e-mail server as setting rows; unchanged it sends nothing, «Verwerfen» only while changed', async () => {
    const stored = settings({
      email: {
        source: 'stored',
        host: 'smtp.example.ch',
        port: 587,
        user: 'media',
        from: 'media@example.ch',
        tls: 'starttls',
      },
    });
    const { calls } = mockApi(routes({ 'GET /admin/settings': () => json(stored) }));
    renderApp('/admin/integrations?item=email');
    const user = userEvent.setup();
    const email = await screen.findByRole('region', { name: 'E-Mail-Server' });
    // One row per setting (Int-A): a group named by its label; host and port share a row.
    for (const name of ['Server und Port', 'Benutzer', 'Passwort', 'Absender', 'Verschlüsselung']) {
      expect(within(email).getByRole('group', { name })).toBeVisible();
    }
    const hostPort = within(email).getByRole('group', { name: 'Server und Port' });
    expect(within(hostPort).getByLabelText('Server und Port', INPUT)).toHaveValue(
      'smtp.example.ch',
    );
    expect(within(hostPort).getByLabelText('Port', INPUT)).toHaveValue('587');
    expect(within(email).getByText(`Die Testmail geht an ${session.account.email}.`)).toBeVisible();

    // Unchanged: «Speichern» sends nothing and keeps the focus; there is nothing to discard.
    const save = within(email).getByRole('button', { name: 'E-Mail-Server speichern' });
    expect(within(email).queryByRole('button', { name: 'Verwerfen' })).toBeNull();
    await user.click(save);
    expect(save).toHaveFocus();
    expect(puts(calls)).toHaveLength(0);

    // Changed: «Verwerfen» brings back what is stored, empties the password and hands the focus
    // to «Speichern»; the band no longer says «ungespeichert».
    await fill(user, within(email).getByLabelText('Server und Port', INPUT), 'smtp.draft');
    await fill(user, within(email).getByLabelText('Passwort', INPUT), 'secret');
    expect(within(email).getByText('ungespeichert')).toBeVisible();
    await user.click(within(email).getByRole('button', { name: 'Verwerfen' }));
    expect(within(email).getByLabelText('Server und Port', INPUT)).toHaveValue('smtp.example.ch');
    expect(within(email).getByLabelText('Passwort', INPUT)).toHaveValue('');
    expect(within(email).queryByRole('button', { name: 'Verwerfen' })).toBeNull();
    expect(within(email).queryByText('ungespeichert')).toBeNull();
    await waitFor(() =>
      expect(within(email).getByRole('button', { name: 'E-Mail-Server speichern' })).toHaveFocus(),
    );
    expect(puts(calls)).toHaveLength(0);
  });

  it('«Verwerfen» takes the messages about the discarded input with it', async () => {
    const { calls } = mockApi(
      routes({
        'PUT /admin/settings': () =>
          problem(422, 'validation_failed', {
            errors: [{ field: 'email.host', code: 'invalid_value' }],
          }),
      }),
    );
    renderApp('/admin/integrations?item=email');
    const user = userEvent.setup();
    const email = await screen.findByRole('region', { name: 'E-Mail-Server' });
    const save = () => within(email).getByRole('button', { name: 'E-Mail-Server speichern' });

    // The field's own check: an invalid port is marked and nothing is sent.
    await fill(user, within(email).getByLabelText('Server und Port', INPUT), 'smtp.example.ch');
    await fill(user, within(email).getByLabelText('Absender', INPUT), 'a@b.ch');
    await fill(user, within(email).getByLabelText('Port', INPUT), 'abc');
    await user.click(save());
    await waitFor(() =>
      expect(within(email).getByLabelText('Port', INPUT)).toHaveAttribute('aria-invalid', 'true'),
    );
    expect(within(email).getByText('Ungültiger Wert.')).toBeVisible();
    expect(puts(calls)).toHaveLength(0);
    await user.click(within(email).getByRole('button', { name: 'Verwerfen' }));
    expect(within(email).getByLabelText('Port', INPUT)).toHaveValue('587');
    expect(within(email).getByLabelText('Port', INPUT)).not.toHaveAttribute('aria-invalid');
    expect(within(email).queryByText('Ungültiger Wert.')).toBeNull();
    await waitFor(() => expect(save()).toHaveFocus());

    // The server's answer: the field and the notice under the block go as well.
    await fill(user, within(email).getByLabelText('Server und Port', INPUT), 'smtp.example.ch');
    await fill(user, within(email).getByLabelText('Absender', INPUT), 'a@b.ch');
    await user.click(save());
    await waitFor(() =>
      expect(within(email).getByLabelText('Server und Port', INPUT)).toHaveAttribute(
        'aria-invalid',
        'true',
      ),
    );
    expect(within(email).getByRole('alert')).toBeVisible();
    expect(puts(calls)).toHaveLength(1);
    await user.click(within(email).getByRole('button', { name: 'Verwerfen' }));
    expect(within(email).getByLabelText('Server und Port', INPUT)).toHaveValue('');
    for (const name of ['Server und Port', 'Port', 'Absender']) {
      expect(within(email).getByLabelText(name, INPUT)).not.toHaveAttribute('aria-invalid');
    }
    expect(within(email).queryByRole('alert')).toBeNull();
    await waitFor(() => expect(save()).toHaveFocus());
  });

  it('keeps e-mail input typed while its save waits; deletes the server after the confirmation', async () => {
    let release = () => undefined as void;
    const answered = new Promise<void>((resolve) => {
      release = resolve;
    });
    const stored = settings({
      email: { source: 'stored', host: 'smtp.example.ch', port: 587, from: 'a@b.ch', tls: 'tls' },
    });
    const { calls } = mockApi(
      routes({
        'PUT /admin/settings': async (call) => {
          if ((call.body as AdminSettingsUpdate).clear) return json(settings());
          await answered;
          return json(stored);
        },
      }),
    );
    renderApp('/admin/integrations?item=email');
    const user = userEvent.setup();
    const email = await screen.findByRole('region', { name: 'E-Mail-Server' });
    const host = within(email).getByLabelText('Server und Port', INPUT);
    await fill(user, host, 'smtp.example.ch');
    await fill(user, within(email).getByLabelText('Absender', INPUT), 'a@b.ch');
    await user.click(within(email).getByRole('radio', { name: 'TLS' }));
    await user.click(within(email).getByRole('button', { name: 'E-Mail-Server speichern' }));
    await waitFor(() => expect(puts(calls)).toHaveLength(1));
    // Typed while the save runs: it stays a draft once the answer is there.
    await user.click(host);
    await user.paste('.ch');
    release();
    expect(await within(email).findByText('E-Mail-Server gespeichert.')).toBeVisible();
    expect(host).toHaveValue('smtp.example.ch.ch');
    expect(within(email).getByText('ungespeichert')).toBeVisible();
    expect(puts(calls)[0]).toEqual({
      email: { host: 'smtp.example.ch', port: 587, from: 'a@b.ch', tls: 'tls' },
    });

    // «Löschen …» asks first; deleting takes the draft with it.
    await user.click(within(email).getByRole('button', { name: 'E-Mail-Server löschen' }));
    const dialog = await screen.findByRole('alertdialog', { name: 'E-Mail-Server löschen?' });
    await user.click(within(dialog).getByRole('button', { name: 'Löschen' }));
    expect(await within(email).findByText('E-Mail-Server gelöscht.')).toBeVisible();
    expect(puts(calls)[1]).toEqual({ clear: ['email'] });
    expect(within(email).getByLabelText('Server und Port', INPUT)).toHaveValue('');
    expect(within(email).queryByRole('button', { name: 'E-Mail-Server löschen' })).toBeNull();
    expect(within(email).queryByRole('button', { name: 'Testmail senden' })).toBeNull();
    // «Löschen …» is gone with the server: «Speichern» takes the focus (ALL-2).
    await waitFor(() =>
      expect(within(email).getByRole('button', { name: 'E-Mail-Server speichern' })).toHaveFocus(),
    );
  });

  it('shows the Telegram bot and its token as setting rows', async () => {
    mockApi(
      routes({
        'GET /admin/settings': () =>
          json(settings({ telegram: { source: 'stored', botUsername: 'slouchcat_bot' } })),
      }),
    );
    renderApp('/admin/integrations?item=telegram');
    const telegram = await screen.findByRole('region', { name: 'Telegram-Bot' });
    const bot = within(telegram).getByRole('group', { name: 'Bot' });
    expect(within(bot).getByText('Bot: @slouchcat_bot')).toBeVisible();
    const row = within(telegram).getByRole('group', { name: 'Bot-Token' });
    const token = within(row).getByLabelText('Bot-Token', INPUT);
    // Write-only: empty, its placeholder says a token is set, the description that empty keeps it.
    expect(token).toHaveValue('');
    expect(token).toHaveAttribute('placeholder', 'Token gesetzt');
    expect(token).toHaveAccessibleDescription(/Leer lassen, um den gespeicherten Wert zu behalten/);
    expect(within(row).getByRole('button', { name: 'Telegram-Bot speichern' })).toBeVisible();
    expect(within(row).getByRole('button', { name: 'Telegram-Bot löschen' })).toBeVisible();
    expect(row).toHaveTextContent('Für Benachrichtigungen per Telegram.');
    expect(row).toHaveTextContent('Ein neues Token ersetzt das gespeicherte.');
  });

  it('a Telegram token from deploy/.env still says what it is for', async () => {
    mockApi(
      routes({ 'GET /admin/settings': () => json(settings({ telegram: { source: 'env' } })) }),
    );
    renderApp('/admin/integrations?item=telegram');
    const telegram = await screen.findByRole('region', { name: 'Telegram-Bot' });
    expect(within(telegram).queryByRole('group', { name: 'Bot' })).toBeNull();
    const row = within(telegram).getByRole('group', { name: 'Bot-Token' });
    expect(row).toHaveTextContent('Für Benachrichtigungen per Telegram.');
    expect(row).not.toHaveTextContent('Ein neues Token ersetzt das gespeicherte.');
    expect(within(row).getByLabelText('Bot-Token', INPUT)).toBeDisabled();
  });

  it('deletes the Telegram bot token; the focus moves on to «Speichern»', async () => {
    const { calls } = mockApi(
      routes({
        'GET /admin/settings': () =>
          json(settings({ telegram: { source: 'stored', botUsername: 'slouchcat_bot' } })),
        'PUT /admin/settings': () => json(settings()),
      }),
    );
    renderApp('/admin/integrations?item=telegram');
    const user = userEvent.setup();
    const telegram = await screen.findByRole('region', { name: 'Telegram-Bot' });
    await user.click(within(telegram).getByRole('button', { name: 'Telegram-Bot löschen' }));
    const dialog = await screen.findByRole('alertdialog', { name: 'Telegram-Bot löschen?' });
    await user.click(within(dialog).getByRole('button', { name: 'Löschen' }));
    expect(await within(telegram).findByText('Telegram-Bot gelöscht.')).toBeVisible();
    expect(puts(calls)).toEqual([{ clear: ['telegram'] }]);
    expect(within(telegram).queryByRole('button', { name: 'Telegram-Bot löschen' })).toBeNull();
    await waitFor(() =>
      expect(
        within(telegram).getByRole('button', { name: 'Telegram-Bot speichern' }),
      ).toHaveFocus(),
    );
  });

  it('a Telegram bot that is not set up says so', async () => {
    mockApi(routes());
    renderApp('/admin/integrations?item=telegram');
    const telegram = await screen.findByRole('region', { name: 'Telegram-Bot' });
    expect(
      within(within(telegram).getByRole('group', { name: 'Bot' })).getByText(
        'Kein Bot eingerichtet.',
      ),
    ).toBeVisible();
    expect(within(telegram).queryByRole('button', { name: 'Telegram-Bot löschen' })).toBeNull();
  });

  it('stores the Telegram bot token; a rejected token shows at its field', async () => {
    let reject = true;
    const { calls } = mockApi(
      routes({
        'PUT /admin/settings': () =>
          reject
            ? problem(422, 'telegram_token_invalid')
            : json(settings({ telegram: { source: 'stored', botUsername: 'slouchcat_bot' } })),
      }),
    );
    renderApp('/admin/integrations?item=telegram');
    const user = userEvent.setup();
    const telegram = await screen.findByRole('region', { name: 'Telegram-Bot' });
    const token = within(telegram).getByLabelText('Bot-Token', INPUT);
    // Empty keeps what is stored (decision 18): nothing is sent, the focus stays on «Speichern».
    const save = within(telegram).getByRole('button', { name: 'Telegram-Bot speichern' });
    await user.click(save);
    expect(save).toHaveFocus();
    expect(puts(calls)).toHaveLength(0);
    await fill(user, token, '123:abc');
    await user.click(within(telegram).getByRole('button', { name: 'Telegram-Bot speichern' }));
    await waitFor(() => expect(token).toHaveAttribute('aria-invalid', 'true'));
    expect(token).toHaveAccessibleDescription(/Telegram hat das Token abgelehnt/);

    // The field stays marked until the token changes (react-aria clears server errors on input).
    reject = false;
    await user.clear(token);
    await fill(user, token, '123:good');
    await user.click(within(telegram).getByRole('button', { name: 'Telegram-Bot speichern' }));
    expect(await within(telegram).findByText('Telegram-Bot gespeichert.')).toBeVisible();
    expect(within(telegram).getByText('Bot: @slouchcat_bot')).toBeVisible();
    expect(puts(calls)).toEqual([
      { telegram: { botToken: '123:abc' } },
      { telegram: { botToken: '123:good' } },
    ]);
  });

  it('keeps keys, the SMTP password and the bot token out of the mutation cache', async () => {
    mockApi(
      routes({
        'PUT /admin/settings': (call) =>
          (call.body as AdminSettingsUpdate).telegram?.botToken === '123:refused-token'
            ? problem(422, 'telegram_token_invalid')
            : json(settings()),
      }),
    );
    const { queryClient, router } = renderApp('/admin/integrations?item=keys');
    const user = userEvent.setup();
    const secrets = [
      'yt-secret-key-0001',
      'smtp-secret-0001',
      '123:refused-token',
      '123:good-token',
    ];
    const held = () =>
      queryClient
        .getMutationCache()
        .getAll()
        .map((mutation) => JSON.stringify(mutation.state.variables ?? null))
        .filter((variables) => secrets.some((secret) => variables.includes(secret)));

    const youtube = await openCard(user, 'YouTube');
    await fill(user, within(youtube).getByLabelText('Schlüssel', INPUT), secrets[0]!);
    await user.click(within(youtube).getByRole('button', { name: 'YouTube speichern' }));
    expect(await within(youtube).findByText('YouTube gespeichert.')).toBeVisible();

    await router.navigate({ to: '/admin/integrations', search: { item: 'email' } });
    const email = await screen.findByRole('region', { name: 'E-Mail-Server' });
    await fill(user, within(email).getByLabelText('Server und Port', INPUT), 'smtp.example.ch');
    await fill(user, within(email).getByLabelText('Passwort', INPUT), secrets[1]!);
    await fill(user, within(email).getByLabelText(/Absender/, INPUT), 'media@example.ch');
    await user.click(within(email).getByRole('button', { name: 'E-Mail-Server speichern' }));
    expect(await within(email).findByText('E-Mail-Server gespeichert.')).toBeVisible();

    // A refused token does not stay either. `gcTime: 0` drops a settled mutation on a zero
    // timeout, which can come after the render that marks the field, so the check waits for it.
    await router.navigate({ to: '/admin/integrations', search: { item: 'telegram' } });
    const telegram = await screen.findByRole('region', { name: 'Telegram-Bot' });
    const token = within(telegram).getByLabelText('Bot-Token', INPUT);
    await fill(user, token, secrets[2]!);
    await user.click(within(telegram).getByRole('button', { name: 'Telegram-Bot speichern' }));
    await waitFor(() => expect(token).toHaveAttribute('aria-invalid', 'true'));
    await waitFor(() => expect(held()).toEqual([]));
    await fill(user, token, secrets[3]!);
    await user.click(within(telegram).getByRole('button', { name: 'Telegram-Bot speichern' }));
    expect(await within(telegram).findByText('Telegram-Bot gespeichert.')).toBeVisible();
    await waitFor(() => expect(held()).toEqual([]));
  });

  it('creates new Web Push keys after the confirmation', async () => {
    const { calls } = mockApi(
      routes({
        'POST /admin/settings/push/rotate': () =>
          json(settings({ push: { configured: true, publicKey: 'BNeuerSchluessel' } })),
      }),
    );
    renderApp('/admin/integrations?item=push');
    const user = userEvent.setup();
    const push = await screen.findByRole('region', { name: 'Web-Push' });
    expect(within(push).getByText('BEl62iUYgUivxIkv69yViEuiBIa')).toBeVisible();
    await user.click(within(push).getByRole('button', { name: 'Schlüssel neu erzeugen' }));
    const dialog = await screen.findByRole('alertdialog', {
      name: 'Neue Web-Push-Schlüssel erzeugen?',
    });
    expect(dialog).toHaveTextContent(/Alle Geräte verlieren ihr Abo/);
    await user.click(within(dialog).getByRole('button', { name: 'Neu erzeugen' }));
    expect(await within(push).findByText(/Neue Schlüssel erzeugt/)).toBeVisible();
    expect(within(push).getByText('BNeuerSchluessel')).toBeVisible();
    expect(calls.some((call) => call.path === '/admin/settings/push/rotate')).toBe(true);
  });

  it('switches the update check off and on (A-5)', async () => {
    let enabled = true;
    const { calls } = mockApi(
      routes({
        'GET /admin/settings': () => json(settings({ updateCheckEnabled: enabled })),
        'PUT /admin/settings': (call) => {
          enabled = (call.body as AdminSettingsUpdate).updateCheckEnabled ?? enabled;
          return json(settings({ updateCheckEnabled: enabled }));
        },
      }),
    );
    renderApp('/admin/integrations?item=server');
    const user = userEvent.setup();
    const update = await serverRow('Update-Prüfung');
    expect(update).toHaveAccessibleDescription(/Die Prüfung selbst installiert nichts\.$/);
    const toggle = within(update).getByRole('checkbox', {
      name: 'Täglich nach neuen Versionen suchen',
    });
    expect(toggle).toBeChecked();
    await user.click(toggle);
    expect(await within(update).findByText('Update-Prüfung ausgeschaltet.')).toBeVisible();
    expect(toggle).not.toBeChecked();
    await user.click(toggle);
    expect(await screen.findByText('Update-Prüfung eingeschaltet.')).toBeVisible();
    expect(toggle).toBeChecked();
    expect(puts(calls)).toEqual([{ updateCheckEnabled: false }, { updateCheckEnabled: true }]);
  });

  it('checks, stores and deletes the public address', async () => {
    let current = settings();
    const { calls } = mockApi(
      routes({
        'GET /admin/settings': () => json(current),
        'PUT /admin/settings': (call) => {
          const body = call.body as AdminSettingsUpdate;
          current = body.clear
            ? settings()
            : settings({ publicUrl: body.publicUrl, publicUrlSource: 'stored' });
          return json(current);
        },
      }),
    );
    renderApp('/admin/integrations?item=server');
    const user = userEvent.setup();
    const address = await serverRow('Öffentliche Adresse');
    const field = within(address).getByLabelText('Adresse');
    await fill(user, field, 'media.example.ch');
    await user.click(within(address).getByRole('button', { name: 'Speichern' }));
    await waitFor(() =>
      expect(field).toHaveAccessibleDescription(/Gib eine Adresse mit http:\/\/ oder https:\/\//),
    );
    expect(puts(calls)).toHaveLength(0);

    await fill(user, field, 'https://media.example.ch');
    await user.click(within(address).getByRole('button', { name: 'Speichern' }));
    expect(await within(address).findByText('Öffentliche Adresse gespeichert.')).toBeVisible();
    expect(within(address).getByText('Gespeichert')).toBeVisible();

    await user.click(within(address).getByRole('button', { name: 'Öffentliche Adresse löschen' }));
    const dialog = await screen.findByRole('alertdialog', { name: 'Öffentliche Adresse löschen?' });
    await user.click(within(dialog).getByRole('button', { name: 'Löschen' }));
    expect(await within(address).findByText('Öffentliche Adresse gelöscht.')).toBeVisible();
    expect(field).toHaveValue('');
    expect(puts(calls)).toEqual([
      { publicUrl: 'https://media.example.ch' },
      { clear: ['publicUrl'] },
    ]);
  });

  it('«Speichern» of an unchanged public address sends nothing and keeps the focus', async () => {
    const { calls } = mockApi(
      routes({
        'GET /admin/settings': () =>
          json(settings({ publicUrl: 'https://media.example.ch', publicUrlSource: 'stored' })),
      }),
    );
    renderApp('/admin/integrations?item=server');
    const user = userEvent.setup();
    const address = await serverRow('Öffentliche Adresse');
    expect(within(address).getByLabelText('Adresse')).toHaveValue('https://media.example.ch');
    const save = within(address).getByRole('button', { name: 'Speichern' });
    await user.click(save);
    expect(save).toHaveFocus();
    expect(puts(calls)).toHaveLength(0);
  });

  it('keeps a public address typed while its save waits for the server', async () => {
    let release = () => undefined as void;
    const answered = new Promise<void>((resolve) => {
      release = resolve;
    });
    const { calls } = mockApi(
      routes({
        'PUT /admin/settings': async (call) => {
          await answered;
          const body = call.body as AdminSettingsUpdate;
          return json(settings({ publicUrl: body.publicUrl, publicUrlSource: 'stored' }));
        },
      }),
    );
    renderApp('/admin/integrations?item=server');
    const user = userEvent.setup();
    const address = await serverRow('Öffentliche Adresse');
    const field = within(address).getByLabelText('Adresse');
    await fill(user, field, 'https://media.example.ch');
    await user.click(within(address).getByRole('button', { name: 'Speichern' }));
    await waitFor(() => expect(puts(calls)).toHaveLength(1));
    // Typed while the save runs: it stays once the answer is there.
    await user.click(field);
    await user.paste('/neu');
    release();
    expect(await within(address).findByText('Öffentliche Adresse gespeichert.')).toBeVisible();
    expect(field).toHaveValue('https://media.example.ch/neu');
    expect(puts(calls)).toEqual([{ publicUrl: 'https://media.example.ch' }]);
  });

  it('keeps a key typed while its save waits for the server', async () => {
    let release = () => undefined as void;
    const answered = new Promise<void>((resolve) => {
      release = resolve;
    });
    const { calls } = mockApi(
      routes({
        'PUT /admin/settings': async () => {
          await answered;
          return json(
            settings({ keys: { ...settings().keys, youtube: { source: 'stored', hint: '0001' } } }),
          );
        },
      }),
    );
    renderApp('/admin/integrations?item=keys');
    const user = userEvent.setup();
    const youtube = await openCard(user, 'YouTube');
    const field = within(youtube).getByLabelText('Schlüssel', INPUT);
    await user.click(field);
    await user.paste('yt-key-0001');
    await user.click(within(youtube).getByRole('button', { name: 'YouTube speichern' }));
    await waitFor(() => expect(puts(calls)).toHaveLength(1));
    // Typed while the save runs: it stays once the answer is there.
    await user.click(field);
    await user.paste('-2');
    release();
    expect(await screen.findByText('YouTube gespeichert.')).toBeVisible();
    expect(field).toHaveValue('yt-key-0001-2');
    expect(puts(calls)).toEqual([{ keys: { youtube: 'yt-key-0001' } }]);
  });

  it('shows the intro open below the title and a tile per service with its state', async () => {
    mockApi(
      routes({
        'GET /admin/dashboard': () =>
          json(
            dashboardFixture({
              integrations: [
                { key: 'tmdb', state: 'configured' },
                { key: 'tvdb', state: 'error', detail: 'TVDB lehnt den Schlüssel ab.' },
                { key: 'youtube', state: 'missing' },
                { key: 'trakt', state: 'configured' },
                { key: 'email', state: 'missing' },
                { key: 'push', state: 'configured' },
                { key: 'telegram', state: 'missing' },
                { key: 'updateCheck', state: 'disabled' },
              ],
              webhooks: { total: 2, eventsLast24h: 1, silent: ['Sonarr Keller'] },
            }),
          ),
      }),
    );
    renderApp('/admin/integrations');
    expect(await screen.findByRole('heading', { level: 1, name: 'Integrationen' })).toBeVisible();
    // Build plan 2.6: the intro stands open, the help button is gone.
    expect(screen.queryByRole('button', { name: 'Hilfe zu dieser Seite' })).toBeNull();
    expect(screen.getByText(/Geheime Werte speichert Slouchcat verschlüsselt/)).toBeVisible();

    // Six tiles, each named by its link into the service's view; no view shows yet.
    const list = await screen.findByRole('list', { name: 'Dienste' });
    const links = within(list).getAllByRole('link');
    expect(links.map((link) => [link.textContent, link.getAttribute('href')])).toEqual(
      BLOCKS.map((name, index) => [
        name,
        `/admin/integrations?item=${['keys', 'webhooks', 'email', 'telegram', 'push', 'server'][index]}`,
      ]),
    );
    expect(screen.queryByRole('region')).toBeNull();
    const tile = (name: string) => within(list).getByRole('link', { name }).closest('li')!;

    // What is set up, as the bands say it, and the whole state from the dashboard (build plan
    // 7.2): every key with its state, the reason of an error, the webhooks of the last 24 hours.
    expect(await screen.findByText('3 von 8 eingerichtet')).toBeVisible();
    const keys = tile('Schlüssel der Anbieter');
    expect(within(keys).getByText('2 von 6 Anbietern eingerichtet')).toBeVisible();
    expect(within(keys).getByText('TMDB: Eingerichtet')).toBeVisible();
    expect(within(keys).getByText('TVDB: Fehler')).toBeVisible();
    expect(within(keys).getByText('TVDB lehnt den Schlüssel ab.')).toBeVisible();
    expect(within(keys).getByText('YouTube: Nicht eingerichtet')).toBeVisible();
    expect(within(keys).getByText('Trakt: Eingerichtet')).toBeVisible();
    expect(within(tile('E-Mail-Server')).getByText('E-Mail: Nicht eingerichtet')).toBeVisible();
    expect(within(tile('E-Mail-Server')).getByText('Nicht gesetzt')).toBeVisible();
    expect(within(tile('Telegram-Bot')).getByText('Telegram: Nicht eingerichtet')).toBeVisible();
    expect(within(tile('Web-Push')).getByText('Web-Push: Eingerichtet')).toBeVisible();
    expect(within(tile('Web-Push')).getByText('Schlüsselpaar vorhanden.')).toBeVisible();
    expect(within(tile('Server')).getByText('Update-Prüfung: Ausgeschaltet')).toBeVisible();
    const hooks = tile('Webhooks von Radarr und Sonarr');
    expect(
      within(hooks).getByText(
        '1 Meldung im Protokoll der letzten 24 Stunden, von 2 Integrationen.',
      ),
    ).toBeVisible();
    expect(within(hooks).getByText('Seit sieben Tagen ohne Meldung: Sonarr Keller')).toBeVisible();
    expect(await within(hooks).findByText('0 Integrationen')).toBeVisible();
  });

  it('a tile opens its service; «Alle Integrationen» leads back to it, the focus follows', async () => {
    mockApi(
      routes({
        'GET /admin/dashboard': () =>
          json(
            dashboardFixture({
              integrations: [
                { key: 'tvdb', state: 'error', detail: 'TVDB lehnt den Schlüssel ab.' },
              ],
            }),
          ),
      }),
    );
    const { router } = renderApp('/admin/integrations');
    const user = userEvent.setup();
    const list = await screen.findByRole('list', { name: 'Dienste' });
    await user.click(within(list).getByRole('link', { name: 'Schlüssel der Anbieter' }));
    await waitFor(() => expect(router.state.location.search).toEqual({ item: 'keys' }));
    const keys = await screen.findByRole('region', { name: 'Schlüssel der Anbieter' });
    expect(within(keys).getByRole('heading', { level: 2 })).toHaveFocus();
    // The view carries the state of its integrations, the reason of the error too.
    expect(await within(keys).findByText('TVDB lehnt den Schlüssel ab.')).toBeVisible();
    expect(screen.queryByRole('list', { name: 'Dienste' })).toBeNull();
    // One view at a time.
    expect(screen.queryByRole('region', { name: 'E-Mail-Server' })).toBeNull();

    await user.click(screen.getByRole('link', { name: 'Alle Integrationen' }));
    const back = await screen.findByRole('list', { name: 'Dienste' });
    await waitFor(() =>
      expect(within(back).getByRole('link', { name: 'Schlüssel der Anbieter' })).toHaveFocus(),
    );
    expect(screen.queryByRole('region', { name: 'Schlüssel der Anbieter' })).toBeNull();
  });

  it('an unknown service in the address shows the tiles', async () => {
    mockApi(routes());
    const { router } = renderApp('/admin/integrations?item=nonsense');
    expect(await screen.findByRole('list', { name: 'Dienste' })).toBeVisible();
    await waitFor(() => expect(router.state.location.search).toEqual({}));
  });

  it('the page works without the dashboard and shows no states', async () => {
    mockApi(routes({ 'GET /admin/dashboard': () => problem(503, 'unavailable') }));
    renderApp('/admin/integrations');
    const list = await screen.findByRole('list', { name: 'Dienste' });
    expect(within(list).getAllByRole('link')).toHaveLength(6);
    expect(await within(list).findByText('2 von 6 Anbietern eingerichtet')).toBeVisible();
    expect(screen.queryByText(/: Eingerichtet$/)).toBeNull();
    expect(screen.queryByText(/von \d+ eingerichtet$/)).toBeNull();
  });

  it('a change of view keeps what was typed; the tile says it is unsaved (S-13)', async () => {
    const { calls } = mockApi(routes({ 'PUT /admin/settings': () => json(settings()) }));
    const { router } = renderApp('/admin/integrations?item=keys');
    const user = userEvent.setup();
    const show = async (item: string, name: string) => {
      await router.navigate({ to: '/admin/integrations', search: { item } });
      return screen.findByRole('region', { name });
    };
    await fill(
      user,
      within(await openCard(user, 'YouTube')).getByLabelText('Schlüssel', INPUT),
      'yt-draft',
    );
    const keys = section('Schlüssel der Anbieter');
    expect(within(keys).getAllByText('ungespeichert')[0]).toBeVisible();
    await fill(
      user,
      within(await show('telegram', 'Telegram-Bot')).getByLabelText('Bot-Token', INPUT),
      '1:draft',
    );
    await fill(
      user,
      within(await show('email', 'E-Mail-Server')).getByLabelText('Server und Port', INPUT),
      'smtp.draft',
    );

    // The tiles name the services with unsaved input.
    await router.navigate({ to: '/admin/integrations', search: {} });
    const list = await screen.findByRole('list', { name: 'Dienste' });
    const tile = (name: string) => within(list).getByRole('link', { name }).closest('li')!;
    for (const name of ['Schlüssel der Anbieter', 'E-Mail-Server', 'Telegram-Bot']) {
      expect(within(tile(name)).getByText('ungespeichert')).toBeVisible();
    }
    expect(within(tile('Web-Push')).queryByText('ungespeichert')).toBeNull();

    // A folded card keeps it too.
    await show('keys', 'Schlüssel der Anbieter');
    await user.click(within(card('YouTube')).getByRole('button', { name: 'Zuklappen: YouTube' }));
    expect(within(card('YouTube')).getByText('ungespeichert')).toBeVisible();
    const youtube = await openCard(user, 'YouTube');
    expect(within(youtube).getByLabelText('Schlüssel', INPUT)).toHaveValue('yt-draft');
    expect(
      within(await show('telegram', 'Telegram-Bot')).getByLabelText('Bot-Token', INPUT),
    ).toHaveValue('1:draft');
    expect(
      within(await show('email', 'E-Mail-Server')).getByLabelText('Server und Port', INPUT),
    ).toHaveValue('smtp.draft');

    // The kept value is what gets saved; afterwards nothing is unsaved any more.
    await show('keys', 'Schlüssel der Anbieter');
    await user.click(within(youtube).getByRole('button', { name: 'YouTube speichern' }));
    expect(await within(youtube).findByText('YouTube gespeichert.')).toBeVisible();
    expect(puts(calls)).toEqual([{ keys: { youtube: 'yt-draft' } }]);
    expect(within(keys).queryAllByText('ungespeichert')).toHaveLength(0);
  });

  it('leaving the page with unsaved input asks first; staying keeps it', async () => {
    mockApi(routes({ 'GET /admin/dashboard': () => problem(503, 'unavailable') }));
    const { router } = renderApp('/admin/integrations?item=server');
    const user = userEvent.setup();
    await screen.findByLabelText('Adresse');

    // Nothing typed: leaving does not ask.
    await router.navigate({ to: '/admin/jobs' });
    await waitFor(() => expect(router.state.location.pathname).toBe('/admin/jobs'));
    expect(screen.queryByRole('alertdialog')).toBeNull();
    await router.navigate({ to: '/admin/integrations', search: { item: 'server' } });
    await fill(user, await screen.findByLabelText('Adresse'), 'https://media.example.ch');

    void router.navigate({ to: '/admin' });
    const question = await screen.findByRole('alertdialog', {
      name: 'Seite mit ungespeicherten Änderungen verlassen?',
    });
    expect(within(question).getByRole('button', { name: 'Auf der Seite bleiben' })).toHaveFocus();
    await user.click(within(question).getByRole('button', { name: 'Auf der Seite bleiben' }));
    await waitFor(() => expect(question).not.toBeInTheDocument());
    expect(router.state.location.pathname).toBe('/admin/integrations');
    expect(screen.getByLabelText('Adresse')).toHaveValue('https://media.example.ch');

    void router.navigate({ to: '/admin' });
    await user.click(await screen.findByRole('button', { name: 'Verwerfen und verlassen' }));
    await waitFor(() => expect(router.state.location.pathname).toBe('/admin'));
  });

  it('the bands count what is set up', async () => {
    mockApi(routes());
    const { router } = renderApp('/admin/integrations?item=keys');
    // TMDB is stored and Trakt comes from deploy/.env; the other four providers are not set up.
    expect(await screen.findByText('2 von 6 Anbietern eingerichtet')).toBeVisible();
    expect(section('Schlüssel der Anbieter')).toContainElement(
      screen.getByText('2 von 6 Anbietern eingerichtet'),
    );
    await router.navigate({ to: '/admin/integrations', search: { item: 'webhooks' } });
    expect(
      await within(
        await screen.findByRole('region', { name: 'Webhooks von Radarr und Sonarr' }),
      ).findByText('0 Integrationen'),
    ).toBeVisible();
  });

  it('links to the notification channels, which link back here', async () => {
    mockApi(routes());
    renderApp('/admin/integrations?item=email');
    const email = await screen.findByRole('region', { name: 'E-Mail-Server' });
    expect(
      within(email).getByRole('link', { name: 'Zu den Benachrichtigungskanälen' }),
    ).toHaveAttribute('href', '/admin/notifications');
  });

  it('a guest gets the notice and no request', async () => {
    const guest = {
      ...session,
      account: { ...session.account, role: 'user' as const, isOwner: false },
    };
    const { calls } = mockApi({ 'GET /auth/session': () => json(guest) });
    renderApp('/admin/integrations');
    expect(await screen.findByText('Nur für Administratoren')).toBeVisible();
    expect(calls.some((call) => call.path.startsWith('/admin/'))).toBe(false);
  });
});
