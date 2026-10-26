import type { AdminSettings, AdminSettingsUpdate } from '@slouchcat/api-client';
import { screen, waitFor, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { json, mockApi, problem, renderApp, session, signedIn, type Routes } from '@/test/api';

// Integrations (A-2): provider keys, e-mail server, Telegram bot, Web Push, update check and
// public address. Secrets are write-only; values from deploy/.env are read-only here.

const settings = (overrides: Partial<AdminSettings> = {}): AdminSettings => ({
  publicUrlSource: 'none',
  updateCheckEnabled: true,
  keys: {
    tmdb: { source: 'stored', hint: 'a1b2' },
    tvdb: { source: 'none' },
    tvdbPin: { source: 'none' },
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
    expect(within(keys).getAllByText('Nicht gesetzt')).toHaveLength(3);
    expect(within(keys).getByText('In deploy/.env gesetzt')).toBeVisible();

    const trakt = within(keys).getByLabelText('Trakt-Client-ID');
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
    expect(within(keys).getByLabelText('TMDB-Lesezugriffs-Token')).toHaveValue('');
    expect(within(keys).getByLabelText('TMDB-Lesezugriffs-Token')).toHaveAttribute(
      'type',
      'password',
    );
  });

  it('saves a key write-only and shows its hint; an empty key is not sent', async () => {
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
    const field = screen.getByLabelText('YouTube-API-Schlüssel');
    await user.click(save);
    await waitFor(() => expect(field).toHaveAccessibleDescription(/Pflichtfeld/));
    expect(puts(calls)).toHaveLength(0);

    await user.type(field, 'yt-key-1239xyz');
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
    const field = await screen.findByLabelText('TMDB-Lesezugriffs-Token');
    await user.type(field, 'bad');
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
    const field = await screen.findByLabelText('TVDB-API-Schlüssel');
    await user.type(field, 'tvdb-key');
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
    await user.type(within(email).getByLabelText('Server'), 'smtp.example.ch');
    const port = within(email).getByLabelText('Port');
    expect(port).toHaveValue('587');
    await user.clear(port);
    await user.type(port, '465');
    await user.type(within(email).getByLabelText('Benutzer'), 'media');
    await user.type(within(email).getByLabelText('Passwort'), 'smtp-secret');
    await user.type(within(email).getByLabelText(/Absender/), 'Slouchcat <media@example.ch>');
    await user.click(within(email).getByRole('radio', { name: 'TLS' }));
    await user.click(within(email).getByRole('button', { name: 'Speichern' }));

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
    expect(within(email).getByLabelText('Passwort')).toHaveValue('');
    expect(within(email).getByLabelText('Passwort')).toHaveAccessibleDescription(
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
    expect(within(email).getByLabelText('Server')).toBeDisabled();
    expect(within(email).getByLabelText('Server')).toHaveValue('mail.local');
    expect(within(email).getByText(/Ändern lässt sich der Wert nur dort/)).toBeVisible();
    expect(within(email).queryByRole('button', { name: 'Speichern' })).toBeNull();
    expect(within(email).getByRole('button', { name: 'Testmail senden' })).toBeEnabled();
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
    const token = within(telegram).getByLabelText('Bot-Token');
    await user.type(token, '123:abc');
    await user.click(within(telegram).getByRole('button', { name: 'Speichern' }));
    await waitFor(() => expect(token).toHaveAttribute('aria-invalid', 'true'));
    expect(token).toHaveAccessibleDescription(/Telegram hat das Token abgelehnt/);

    // The field stays marked until the token changes (react-aria clears server errors on input).
    reject = false;
    await user.clear(token);
    await user.type(token, '123:good');
    await user.click(within(telegram).getByRole('button', { name: 'Speichern' }));
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
    await user.type(within(keys).getByLabelText('YouTube-API-Schlüssel'), secrets[0]!);
    await user.click(within(keys).getByRole('button', { name: 'YouTube-Schlüssel speichern' }));
    expect(await within(keys).findByText('YouTube-Schlüssel gespeichert.')).toBeVisible();

    const email = section('E-Mail-Server');
    await user.type(within(email).getByLabelText('Server'), 'smtp.example.ch');
    await user.type(within(email).getByLabelText('Passwort'), secrets[1]!);
    await user.type(within(email).getByLabelText(/Absender/), 'media@example.ch');
    await user.click(within(email).getByRole('button', { name: 'Speichern' }));
    expect(await within(email).findByText('E-Mail-Server gespeichert.')).toBeVisible();

    // A refused token does not stay either.
    const telegram = section('Telegram-Bot');
    const token = within(telegram).getByLabelText('Bot-Token');
    await user.type(token, secrets[2]!);
    await user.click(within(telegram).getByRole('button', { name: 'Speichern' }));
    await waitFor(() => expect(token).toHaveAttribute('aria-invalid', 'true'));
    expect(held()).toEqual([]);
    await user.clear(token);
    await user.type(token, secrets[3]!);
    await user.click(within(telegram).getByRole('button', { name: 'Speichern' }));
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
    const toggle = await screen.findByRole('checkbox', {
      name: 'Täglich nach neuen Versionen suchen',
    });
    expect(toggle).toBeChecked();
    await user.click(toggle);
    expect(await screen.findByText('Update-Prüfung ausgeschaltet.')).toBeVisible();
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
    const address = await screen.findByRole('region', { name: 'Öffentliche Adresse' });
    const field = within(address).getByLabelText('Adresse');
    await user.type(field, 'media.example.ch');
    await user.click(within(address).getByRole('button', { name: 'Speichern' }));
    await waitFor(() =>
      expect(field).toHaveAccessibleDescription(/Gib eine Adresse mit http:\/\/ oder https:\/\//),
    );
    expect(puts(calls)).toHaveLength(0);

    await user.clear(field);
    await user.type(field, 'https://media.example.ch');
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

  it('a guest gets the notice and no request', async () => {
    const guest = { ...session, account: { ...session.account, role: 'guest' as const } };
    const { calls } = mockApi({ 'GET /auth/session': () => json(guest) });
    renderApp('/admin/integrations');
    expect(await screen.findByText('Nur für das Hauptkonto')).toBeVisible();
    expect(calls.some((call) => call.path.startsWith('/admin/'))).toBe(false);
  });
});
