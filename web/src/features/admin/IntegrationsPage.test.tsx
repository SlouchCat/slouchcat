import type { AdminSettings, AdminSettingsUpdate } from '@slouchcat/api-client';
import { screen, waitFor, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { json, mockApi, problem, renderApp, session, signedIn, type Routes } from '@/test/api';
import { fill } from '@/test/fill';

// Integrations (A-2; concept 12, variant A): six blocks that fold away by their band — provider
// keys, webhooks, e-mail server, Telegram bot, Web Push, and the server's update check and public
// address. Secrets are write-only; values from deploy/.env are read-only here. Unsaved input
// survives a folded block and makes leaving the page ask first (S-13, S-17).

const settings = (overrides: Partial<AdminSettings> = {}): AdminSettings => ({
  publicUrlSource: 'none',
  updateCheckEnabled: true,
  keys: {
    tmdb: { source: 'stored', hint: 'a1b2' },
    tvdb: { source: 'none' },
    tvdbPin: { source: 'none' },
    fanart: { source: 'none' },
    fanartClient: { source: 'none' },
    youtube: { source: 'none' },
    trakt: { source: 'env' },
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

describe('integrations (A-2)', () => {
  afterEach(() => vi.unstubAllGlobals());

  it('shows where each key comes from; a key from deploy/.env is read-only', async () => {
    mockApi(routes());
    renderApp('/admin/integrations');
    expect(await screen.findByRole('heading', { level: 1, name: 'Integrationen' })).toBeVisible();
    const keys = await screen.findByRole('region', { name: 'Schlüssel der Anbieter' });
    expect(within(keys).getByText('Gespeichert, endet auf a1b2')).toBeVisible();
    expect(within(keys).getAllByText('Nicht gesetzt')).toHaveLength(5);
    expect(within(keys).getByText('In deploy/.env gesetzt')).toBeVisible();

    // One row per key (S-7): the row is a group named like the key, the field inside it.
    const traktRow = within(keys).getByRole('group', { name: 'Trakt-Client-ID' });
    expect(traktRow).toHaveAccessibleDescription(/Trending/);
    const trakt = within(traktRow).getByLabelText('Trakt-Client-ID', INPUT);
    expect(trakt).toBeDisabled();
    expect(trakt).toHaveAccessibleDescription(/Ändern lässt sich der Wert nur dort/);
    expect(
      within(keys).queryByRole('button', { name: 'Trakt-Client-ID speichern' }),
    ).not.toBeInTheDocument();
    expect(within(keys).getByRole('link', { name: 'Stand der Trakt-Listen' })).toHaveAttribute(
      'href',
      '/admin/spotlight',
    );
    // Only a stored key can be deleted; the value itself is never shown.
    expect(within(keys).getByRole('button', { name: 'TMDB-Token löschen' })).toBeVisible();
    expect(within(keys).queryByRole('button', { name: 'YouTube-Schlüssel löschen' })).toBeNull();
    expect(within(keys).getByLabelText('TMDB-Token', INPUT)).toHaveValue('');
    expect(within(keys).getByLabelText('TMDB-Token', INPUT)).toHaveAttribute('type', 'password');
    // A stored key says in its placeholder and its description that empty keeps it.
    expect(within(keys).getByLabelText('TMDB-Token', INPUT)).toHaveAttribute(
      'placeholder',
      'Leer lassen zum Behalten',
    );
    expect(within(keys).getByLabelText('TMDB-Token', INPUT)).toHaveAccessibleDescription(
      /Leer lassen, um den gespeicherten Wert zu behalten/,
    );
  });

  it('links every key to the page that issues it at its provider, in a new tab (K12-9)', async () => {
    mockApi(routes());
    renderApp('/admin/integrations');
    const keys = await screen.findByRole('region', { name: 'Schlüssel der Anbieter' });
    const links: [string, string, string][] = [
      ['TMDB-Token', 'Token bei themoviedb.org holen', 'https://www.themoviedb.org/settings/api'],
      [
        'TVDB-Schlüssel',
        'Schlüssel bei thetvdb.com holen',
        'https://thetvdb.com/dashboard/account/apikey',
      ],
      [
        'TVDB-PIN',
        'PIN bei thetvdb.com nachsehen',
        'https://thetvdb.com/dashboard/account/subscription',
      ],
      [
        'YouTube-Schlüssel',
        'Schlüssel in der Google Cloud Console holen',
        'https://console.cloud.google.com/apis/credentials',
      ],
      [
        'Fanart.tv-Schlüssel',
        'Projektschlüssel bei fanart.tv holen',
        'https://fanart.tv/get-an-api-key/',
      ],
      [
        'Persönlicher Fanart.tv-Schlüssel',
        'Persönlichen Schlüssel bei fanart.tv holen',
        'https://fanart.tv/get-an-api-key/',
      ],
      ['Trakt-Client-ID', 'Client-ID bei trakt.tv holen', 'https://developer.trakt.tv/apps'],
    ];
    for (const [row, name, href] of links) {
      const group = within(keys).getByRole('group', { name: row });
      const link = within(group).getByRole('link', { name });
      expect(link).toHaveAttribute('href', href);
      expect(link).toHaveAttribute('target', '_blank');
      expect(link).toHaveAttribute('rel', 'noreferrer noopener');
      // The link sits in the row's help: the field keeps the row's visible label as its name.
      expect(within(group).getByLabelText(row, INPUT)).toBeInTheDocument();
    }
    expect(within(keys).getAllByRole('link', { name: /holen|nachsehen/ })).toHaveLength(7);
  });

  it('names every hidden-label field like the visible label of its row (WCAG 2.5.3)', async () => {
    mockApi(routes());
    renderApp('/admin/integrations');
    const keys = await screen.findByRole('region', { name: 'Schlüssel der Anbieter' });
    const rows = [
      ...within(keys).getAllByRole('group'),
      ...within(section('E-Mail-Server')).getAllByRole('group'),
      ...within(section('Telegram-Bot')).getAllByRole('group'),
    ];
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
    // Seven keys, five mail fields, the bot token.
    expect(fields).toBe(13);
  });

  it('saves a key write-only and shows its hint; an empty or blank key is not sent', async () => {
    let current = settings();
    const { calls } = mockApi(
      routes({
        'GET /admin/settings': () => json(current),
        'PUT /admin/settings': () => {
          current = settings({
            keys: { ...current.keys, youtube: { source: 'stored', hint: '9xyz' } },
          });
          return json(current);
        },
      }),
    );
    renderApp('/admin/integrations');
    const user = userEvent.setup();
    const save = await screen.findByRole('button', { name: 'YouTube-Schlüssel speichern' });
    const field = screen.getByLabelText('YouTube-Schlüssel', INPUT);
    // Empty keeps what is stored (decision 18): nothing is sent, the focus stays on «Speichern».
    await user.click(save);
    expect(save).toHaveFocus();
    expect(field).not.toHaveAccessibleDescription(/Pflichtfeld/);
    expect(puts(calls)).toHaveLength(0);
    // Blanks alone are no key: the field says so.
    await fill(user, field, '   ');
    await user.click(save);
    await waitFor(() => expect(field).toHaveAccessibleDescription(/Pflichtfeld/));
    expect(puts(calls)).toHaveLength(0);

    await fill(user, field, 'yt-key-1239xyz');
    await user.click(save);
    expect(await screen.findByText('YouTube-Schlüssel gespeichert.')).toBeVisible();
    expect(puts(calls)).toEqual([{ keys: { youtube: 'yt-key-1239xyz' } }]);
    expect(field).toHaveValue('');
    expect(section('Schlüssel der Anbieter')).toHaveTextContent('Gespeichert, endet auf 9xyz');
  });

  it('an invalid TMDB key is reported at its field', async () => {
    mockApi(routes({ 'PUT /admin/settings': () => problem(422, 'tmdb_key_invalid') }));
    renderApp('/admin/integrations');
    const user = userEvent.setup();
    const field = await screen.findByLabelText('TMDB-Token', INPUT);
    await fill(user, field, 'bad');
    await user.click(screen.getByRole('button', { name: 'TMDB-Token speichern' }));
    await waitFor(() => expect(field).toHaveAttribute('aria-invalid', 'true'));
    expect(field).toHaveAccessibleDescription(/TMDB hat den Schlüssel abgelehnt/);
  });

  it('deletes a stored key after the confirmation', async () => {
    const { calls } = mockApi(
      routes({
        'PUT /admin/settings': () =>
          json(settings({ keys: { ...settings().keys, tmdb: { source: 'none' } } })),
      }),
    );
    renderApp('/admin/integrations');
    const user = userEvent.setup();
    await user.click(await screen.findByRole('button', { name: 'TMDB-Token löschen' }));
    const dialog = await screen.findByRole('alertdialog', { name: 'TMDB-Token löschen?' });
    await user.click(within(dialog).getByRole('button', { name: 'Löschen' }));
    expect(await screen.findByText('TMDB-Token gelöscht.')).toBeVisible();
    expect(puts(calls)).toEqual([{ clear: ['tmdb'] }]);
    expect(screen.queryByRole('button', { name: 'TMDB-Token löschen' })).not.toBeInTheDocument();
    // «Löschen …» is gone with the key: the row's «Speichern» takes the focus (ALL-2).
    await waitFor(() =>
      expect(screen.getByRole('button', { name: 'TMDB-Token speichern' })).toHaveFocus(),
    );
  });

  it('a value the environment sets cannot be overridden (env_wins)', async () => {
    mockApi(
      routes({
        'PUT /admin/settings': () =>
          problem(422, 'env_wins', { errors: [{ field: 'keys.tvdb', code: 'env_wins' }] }),
      }),
    );
    renderApp('/admin/integrations');
    const user = userEvent.setup();
    const field = await screen.findByLabelText('TVDB-Schlüssel', INPUT);
    await fill(user, field, 'tvdb-key');
    await user.click(screen.getByRole('button', { name: 'TVDB-Schlüssel speichern' }));
    await waitFor(() => expect(field).toHaveAttribute('aria-invalid', 'true'));
    expect(field).toHaveAccessibleDescription(/in deploy\/.env gesetzt/);
    expect(await screen.findByRole('alert')).toHaveTextContent(/lässt sich nur dort ändern/);
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
    renderApp('/admin/integrations');
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
    renderApp('/admin/integrations');
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
    renderApp('/admin/integrations');
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
    renderApp('/admin/integrations');
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
    renderApp('/admin/integrations');
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
    renderApp('/admin/integrations');
    const telegram = await screen.findByRole('region', { name: 'Telegram-Bot' });
    const bot = within(telegram).getByRole('group', { name: 'Bot' });
    expect(within(bot).getByText('Bot: @slouchcat_bot')).toBeVisible();
    const row = within(telegram).getByRole('group', { name: 'Bot-Token' });
    const token = within(row).getByLabelText('Bot-Token', INPUT);
    // Write-only: empty, with a short placeholder and the whole sentence as its description.
    expect(token).toHaveValue('');
    expect(token).toHaveAttribute('placeholder', 'Leer lassen zum Behalten');
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
    renderApp('/admin/integrations');
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
    renderApp('/admin/integrations');
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
    renderApp('/admin/integrations');
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
    renderApp('/admin/integrations');
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
    const { queryClient } = renderApp('/admin/integrations');
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

    const keys = await screen.findByRole('region', { name: 'Schlüssel der Anbieter' });
    await fill(user, within(keys).getByLabelText('YouTube-Schlüssel', INPUT), secrets[0]!);
    await user.click(within(keys).getByRole('button', { name: 'YouTube-Schlüssel speichern' }));
    expect(await within(keys).findByText('YouTube-Schlüssel gespeichert.')).toBeVisible();

    const email = section('E-Mail-Server');
    await fill(user, within(email).getByLabelText('Server und Port', INPUT), 'smtp.example.ch');
    await fill(user, within(email).getByLabelText('Passwort', INPUT), secrets[1]!);
    await fill(user, within(email).getByLabelText(/Absender/, INPUT), 'media@example.ch');
    await user.click(within(email).getByRole('button', { name: 'E-Mail-Server speichern' }));
    expect(await within(email).findByText('E-Mail-Server gespeichert.')).toBeVisible();

    // A refused token does not stay either. `gcTime: 0` drops a settled mutation on a zero
    // timeout, which can come after the render that marks the field, so the check waits for it.
    const telegram = section('Telegram-Bot');
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
    renderApp('/admin/integrations');
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
    renderApp('/admin/integrations');
    const user = userEvent.setup();
    const update = await serverRow('Update-Prüfung');
    expect(update).toHaveAccessibleDescription(/Installiert wird nichts/);
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
    renderApp('/admin/integrations');
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
    renderApp('/admin/integrations');
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
    renderApp('/admin/integrations');
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
    renderApp('/admin/integrations');
    const user = userEvent.setup();
    const field = await screen.findByLabelText('YouTube-Schlüssel', INPUT);
    await user.click(field);
    await user.paste('yt-key-0001');
    await user.click(screen.getByRole('button', { name: 'YouTube-Schlüssel speichern' }));
    await waitFor(() => expect(puts(calls)).toHaveLength(1));
    // Typed while the save runs: it stays once the answer is there.
    await user.click(field);
    await user.paste('-2');
    release();
    expect(await screen.findByText('YouTube-Schlüssel gespeichert.')).toBeVisible();
    expect(field).toHaveValue('yt-key-0001-2');
    expect(puts(calls)).toEqual([{ keys: { youtube: 'yt-key-0001' } }]);
  });

  it('keeps the intro behind the help button; six blocks start open and fold by keyboard', async () => {
    mockApi(routes());
    renderApp('/admin/integrations');
    const user = userEvent.setup();
    expect(await screen.findByRole('heading', { level: 1, name: 'Integrationen' })).toBeVisible();
    const help = screen.getByRole('button', { name: 'Hilfe zu dieser Seite' });
    expect(screen.getByText(/Geheime Werte speichert Slouchcat verschlüsselt/)).not.toBeVisible();
    await user.click(help);
    expect(screen.getByText(/Geheime Werte speichert Slouchcat verschlüsselt/)).toBeVisible();

    await screen.findByRole('region', { name: 'Schlüssel der Anbieter' });
    const bands = BLOCKS.map((name) =>
      within(section(name)).getByRole('button', { name, expanded: true }),
    );
    expect(bands).toHaveLength(6);
    for (const band of bands) {
      expect(within(band.closest('h2')!).getByRole('button')).toBe(band);
    }

    // Enter folds the e-mail block away, Space opens it again.
    const mail = bands[2]!;
    mail.focus();
    await user.keyboard('{Enter}');
    expect(mail).toHaveAttribute('aria-expanded', 'false');
    expect(
      within(section('E-Mail-Server')).queryByRole('textbox', { name: 'Server und Port' }),
    ).toBeNull();
    expect(mail).toHaveFocus();
    await user.keyboard(' ');
    expect(mail).toHaveAttribute('aria-expanded', 'true');
    expect(
      within(section('E-Mail-Server')).getByRole('textbox', { name: 'Server und Port' }),
    ).toBeVisible();
  });

  it('a folded block keeps what was typed in it (S-13)', async () => {
    const { calls } = mockApi(routes({ 'PUT /admin/settings': () => json(settings()) }));
    renderApp('/admin/integrations');
    const user = userEvent.setup();
    const keys = await screen.findByRole('region', { name: 'Schlüssel der Anbieter' });
    await fill(user, within(keys).getByLabelText('YouTube-Schlüssel', INPUT), 'yt-draft');
    await fill(user, within(section('Telegram-Bot')).getByLabelText('Bot-Token', INPUT), '1:draft');
    await fill(
      user,
      within(section('E-Mail-Server')).getByLabelText('Server und Port', INPUT),
      'smtp.draft',
    );
    const band = within(keys).getByRole('button', { name: 'Schlüssel der Anbieter' });
    // The band says the block holds unsaved input, folded or not.
    expect(within(keys).getByText('ungespeichert')).toBeVisible();

    for (const name of ['Schlüssel der Anbieter', 'E-Mail-Server', 'Telegram-Bot']) {
      await user.click(within(section(name)).getByRole('button', { name }));
    }
    expect(band).toHaveAttribute('aria-expanded', 'false');
    expect(within(keys).getByText('ungespeichert')).toBeVisible();
    for (const name of ['Schlüssel der Anbieter', 'E-Mail-Server', 'Telegram-Bot']) {
      await user.click(within(section(name)).getByRole('button', { name }));
    }
    expect(within(keys).getByLabelText('YouTube-Schlüssel', INPUT)).toHaveValue('yt-draft');
    expect(within(section('Telegram-Bot')).getByLabelText('Bot-Token', INPUT)).toHaveValue(
      '1:draft',
    );
    expect(within(section('E-Mail-Server')).getByLabelText('Server und Port', INPUT)).toHaveValue(
      'smtp.draft',
    );

    // The kept value is what gets saved; afterwards nothing is unsaved any more.
    await user.click(within(keys).getByRole('button', { name: 'YouTube-Schlüssel speichern' }));
    expect(await within(keys).findByText('YouTube-Schlüssel gespeichert.')).toBeVisible();
    expect(puts(calls)).toEqual([{ keys: { youtube: 'yt-draft' } }]);
    expect(within(keys).queryByText('ungespeichert')).toBeNull();
  });

  it('leaving the page with unsaved input asks first; staying keeps it', async () => {
    mockApi(routes({ 'GET /admin/dashboard': () => problem(503, 'unavailable') }));
    const { router } = renderApp('/admin/integrations');
    const user = userEvent.setup();
    await screen.findByLabelText('Adresse');

    // Nothing typed: leaving does not ask.
    await router.navigate({ to: '/admin/jobs' });
    await waitFor(() => expect(router.state.location.pathname).toBe('/admin/jobs'));
    expect(screen.queryByRole('alertdialog')).toBeNull();
    await router.navigate({ to: '/admin/integrations' });
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
    renderApp('/admin/integrations');
    // TMDB is stored and Trakt comes from deploy/.env; the other five are not set.
    expect(await screen.findByText('2 von 7 gesetzt')).toBeVisible();
    expect(section('Schlüssel der Anbieter')).toContainElement(screen.getByText('2 von 7 gesetzt'));
    expect(
      await within(section('Webhooks von Radarr und Sonarr')).findByText('0 Integrationen'),
    ).toBeVisible();
  });

  it('links to the notification channels, which link back here', async () => {
    mockApi(routes());
    renderApp('/admin/integrations');
    const email = await screen.findByRole('region', { name: 'E-Mail-Server' });
    expect(
      within(email).getByRole('link', { name: 'Zu den Benachrichtigungskanälen' }),
    ).toHaveAttribute('href', '/settings/notifications');
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
