import type { ChannelSetting, ChannelSettings } from '@slouchcat/api-client';
import { screen, waitFor, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { json, mockApi, problem, renderApp, session, signedIn, type Routes } from '@/test/api';

const VAPID_KEY =
  'BEl62iUYgUivxIkv69yViEuiBIa-Ib9-SkvMeAtA3LFgDzkrxZJjSgSnfckjBJuBkr3qBUYIHBQFLXYp5Nksh8U';

const email: ChannelSetting = {
  channel: 'email',
  enabled: false,
  events: ['announcements'],
  available: true,
  email: { address: 'owner@example.test' },
};
const push: ChannelSetting = {
  channel: 'push',
  enabled: false,
  events: ['new_content'],
  available: true,
  push: {
    subscriptions: [
      {
        id: 3,
        deviceName: 'Firefox 140 · Linux',
        createdAt: '2026-10-20T10:00:00Z',
        thisDevice: false,
      },
    ],
  },
};
const telegram: ChannelSetting = {
  channel: 'telegram',
  enabled: false,
  events: ['new_content'],
  available: true,
  telegram: { linked: false, botUsername: 'slouchcat_bot' },
};

const settings = (...channels: ChannelSetting[]): ChannelSettings => ({ channels });
const noContent = () => new Response(null, { status: 204 });

const page: Routes = {
  ...signedIn,
  'GET /me/channels': () => json(settings(email, push, telegram)),
};

const card = (name: string) => screen.findByRole('article', { name });

const guestSession = () =>
  json({
    ...session,
    account: { ...session.account, role: 'user', isOwner: false, email: 'gast@example.test' },
  });

/** The account's second profile: it sees the channels but cannot change them. */
const secondProfileSession = () =>
  json({ ...session, profile: { ...session.profile!, id: 2, name: 'Lena', isPrimary: false } });

describe('notification channels page', () => {
  afterEach(() => {
    vi.unstubAllGlobals();
    Reflect.deleteProperty(navigator, 'serviceWorker');
    Reflect.deleteProperty(window, 'isSecureContext');
  });

  it('shows the three channels with switch, events and the account address', async () => {
    mockApi(page);
    renderApp('/settings/notifications');
    expect(
      await screen.findByRole('heading', { level: 1, name: 'Benachrichtigungskanäle' }),
    ).toBeVisible();
    const mail = await card('E-Mail');
    expect(within(mail).getByRole('switch', { name: 'Aktiv' })).not.toBeChecked();
    expect(within(mail).getByLabelText('E-Mail-Adresse')).toHaveValue('owner@example.test');
    expect(within(mail).getByRole('checkbox', { name: 'Ankündigungen' })).toBeChecked();
    expect(within(mail).getByRole('checkbox', { name: 'Neue Inhalte' })).not.toBeChecked();
    // The main account may choose errors. M9 removed the acquisition event with its producer
    // I-1, so four types are left.
    expect(within(mail).getByRole('checkbox', { name: 'Fehler' })).toBeVisible();
    expect(within(mail).getAllByRole('checkbox')).toHaveLength(4);
    expect(
      within(mail).queryByRole('checkbox', { name: 'Beschaffungsstatus' }),
    ).not.toBeInTheDocument();
    expect(within(mail).getByText('Aus')).toBeVisible();
    expect(await card('Web-Push')).toBeVisible();
    expect(await card('Telegram')).toBeVisible();
  });

  it('hides "Fehler" from a guest account', async () => {
    mockApi({ ...page, 'GET /auth/session': guestSession });
    renderApp('/settings/notifications');
    const mail = await card('E-Mail');
    expect(within(mail).getByRole('checkbox', { name: 'Einladungen' })).toBeVisible();
    expect(within(mail).queryByRole('checkbox', { name: 'Fehler' })).not.toBeInTheDocument();
  });

  it('shows another profile than the primary one the channels without controls', async () => {
    const { calls } = mockApi({
      ...page,
      'GET /auth/session': secondProfileSession,
      'GET /me/channels': () =>
        json(
          settings({ ...email, enabled: true, events: ['new_content', 'announcements'] }, push, {
            ...telegram,
            telegram: { linked: true, chatId: '4242', botUsername: 'slouchcat_bot' },
          }),
        ),
    });
    renderApp('/settings/notifications');
    expect(await screen.findByText('Nur im Hauptprofil')).toBeVisible();
    expect(screen.getByText(/ändern lassen sie sich im Hauptprofil/)).toBeVisible();
    expect(screen.getByRole('link', { name: 'Profil wechseln' })).toHaveAttribute(
      'href',
      '/profiles',
    );

    const mail = await card('E-Mail');
    expect(within(mail).getByText('Aktiv')).toBeVisible();
    const facts = (article: HTMLElement) =>
      within(article)
        .getAllByRole('term')
        .map((term) => [term.textContent, term.nextElementSibling?.textContent]);
    expect(facts(mail)).toEqual([
      ['Ereignisse', 'Neue Inhalte · Ankündigungen'],
      ['E-Mail-Adresse', 'owner@example.test'],
    ]);
    expect(facts(await card('Web-Push'))).toEqual([
      ['Ereignisse', 'Neue Inhalte'],
      ['Angemeldete Geräte', '1'],
    ]);
    expect(facts(await card('Telegram'))).toEqual([
      ['Ereignisse', 'Neue Inhalte'],
      ['Verknüpfung', 'Verknüpft'],
    ]);
    // No switch, no checkbox, no field, no button: nothing to change.
    const list = screen.getByRole('list', { name: 'Kanäle' });
    for (const role of ['switch', 'checkbox', 'textbox', 'button'] as const) {
      expect(within(list).queryAllByRole(role)).toHaveLength(0);
    }
    expect(calls.filter((call) => call.method !== 'GET')).toHaveLength(0);
  });

  it('shows the refusal of a change from another profile than the primary one', async () => {
    // The page offers no controls then; the server refuses anyway (the session may be stale).
    mockApi({
      ...page,
      'PUT /me/channels/email': () => problem(403, 'primary_profile_required'),
    });
    renderApp('/settings/notifications');
    const mail = await card('E-Mail');
    await userEvent.setup().click(within(mail).getByRole('button', { name: 'Speichern' }));
    expect(await within(mail).findByRole('alert')).toHaveTextContent(
      'Das darf nur das Hauptprofil dieses Kontos.',
    );
  });

  it('switches e-mail on with chosen events and stores them', async () => {
    const { calls } = mockApi({
      ...page,
      'PUT /me/channels/email': (call) =>
        json({ ...email, ...(call.body as object), available: true }),
    });
    renderApp('/settings/notifications');
    const mail = await card('E-Mail');
    const user = userEvent.setup();
    await user.click(within(mail).getByRole('switch', { name: 'Aktiv' }));
    await user.click(within(mail).getByRole('checkbox', { name: 'Neue Inhalte' }));
    const address = within(mail).getByLabelText('E-Mail-Adresse');
    await user.clear(address);
    await user.type(address, 'familie@example.test');
    await user.click(within(mail).getByRole('button', { name: 'Speichern' }));

    expect(await within(mail).findByText('Gespeichert.')).toBeVisible();
    expect(calls.find((call) => call.method === 'PUT')?.body).toEqual({
      enabled: true,
      events: ['new_content', 'announcements'],
      email: { address: 'familie@example.test' },
    });
    expect(within(mail).getByText('Aktiv', { selector: 'span' })).toBeVisible();
  });

  it('shows an invalid address at the field', async () => {
    mockApi({
      ...page,
      'PUT /me/channels/email': () =>
        problem(422, 'validation_failed', {
          errors: [{ field: 'email.address', code: 'invalid_email' }],
        }),
    });
    renderApp('/settings/notifications');
    const mail = await card('E-Mail');
    const user = userEvent.setup();
    await user.click(within(mail).getByRole('button', { name: 'Speichern' }));
    expect(await within(mail).findByText('Bitte prüfe die markierten Felder.')).toBeVisible();
    expect(within(mail).getByLabelText('E-Mail-Adresse')).toHaveAttribute('aria-invalid', 'true');
  });

  it('sends a test message and says what went wrong', async () => {
    let answer: () => Response = noContent;
    const { calls } = mockApi({ ...page, 'POST /me/channels/email/test': () => answer() });
    renderApp('/settings/notifications');
    const mail = await card('E-Mail');
    const user = userEvent.setup();
    const test = within(mail).getByRole('button', { name: 'Testnachricht' });

    await user.click(test);
    expect(await within(mail).findByText('Testnachricht gesendet.')).toBeVisible();
    expect(calls.some((call) => call.path === '/me/channels/email/test')).toBe(true);

    answer = () => problem(422, 'channel_disabled');
    await user.click(test);
    expect(
      await within(mail).findByText('Der Kanal ist ausgeschaltet oder noch nicht verbunden.'),
    ).toBeVisible();

    answer = () => problem(502, 'send_failed');
    await user.click(test);
    expect(await within(mail).findByText(/Die Nachricht ging nicht weg/)).toBeVisible();
  });

  it('disables a channel the server lacks and points the main user to the integrations', async () => {
    mockApi({
      ...page,
      'GET /me/channels': () =>
        json(
          settings(email, push, {
            channel: 'telegram',
            enabled: false,
            events: [],
            available: false,
          }),
        ),
    });
    renderApp('/settings/notifications');
    const bot = await card('Telegram');
    expect(within(bot).getByText('Nicht eingerichtet')).toBeVisible();
    expect(within(bot).getByText(/richtet diesen Kanal in der Verwaltung/)).toBeVisible();
    expect(within(bot).getByRole('link', { name: 'Integrationen öffnen' })).toHaveAttribute(
      'href',
      '/admin/integrations',
    );
    expect(within(bot).getByRole('switch', { name: 'Aktiv' })).toBeDisabled();
    expect(within(bot).getByRole('button', { name: 'Speichern' })).toBeDisabled();
    expect(within(bot).getByRole('button', { name: 'Testnachricht' })).toBeDisabled();
  });

  it('gives a guest the hint without the link', async () => {
    mockApi({
      ...page,
      'GET /auth/session': guestSession,
      'GET /me/channels': () =>
        json(
          settings(email, push, {
            channel: 'telegram',
            enabled: false,
            events: [],
            available: false,
          }),
        ),
    });
    renderApp('/settings/notifications');
    const bot = await card('Telegram');
    expect(within(bot).getByText(/richtet diesen Kanal in der Verwaltung/)).toBeVisible();
    expect(within(bot).queryByRole('link')).not.toBeInTheDocument();
  });

  it('links Telegram: the code with the instruction, then "Verknüpft" once the bot got it', async () => {
    let state = telegram;
    const { calls } = mockApi({
      ...page,
      'GET /me/channels': () => json(settings(email, push, state)),
      'POST /me/channels/telegram/link': () => {
        state = {
          ...telegram,
          telegram: {
            linked: false,
            botUsername: 'slouchcat_bot',
            linkCode: 'AB2C3D',
            linkExpiresAt: '2026-10-21T10:15:00Z',
          },
        };
        return json(state);
      },
    });
    renderApp('/settings/notifications');
    const bot = await card('Telegram');
    expect(within(bot).getByText('Nicht verknüpft')).toBeVisible();
    await userEvent.setup().click(within(bot).getByRole('button', { name: 'Verknüpfen' }));

    expect(
      await within(bot).findByText('Sende dem Bot @slouchcat_bot die Nachricht /start AB2C3D'),
    ).toBeVisible();
    expect(within(bot).getByRole('link', { name: /In Telegram öffnen/ })).toHaveAttribute(
      'href',
      'https://t.me/slouchcat_bot?start=AB2C3D',
    );
    expect(within(bot).getByRole('button', { name: 'Code erneuern' })).toBeVisible();

    // The bot receives "/start AB2C3D"; the page asks again while the code is open.
    state = {
      ...telegram,
      telegram: { linked: true, chatId: '4242', botUsername: 'slouchcat_bot' },
    };
    expect(
      await within(bot).findByText('Verknüpft mit Chat 4242', {}, { timeout: 5000 }),
    ).toBeVisible();
    expect(within(bot).queryByText(/Sende dem Bot/)).not.toBeInTheDocument();
    expect(calls.filter((call) => call.path === '/me/channels').length).toBeGreaterThan(1);
  });

  it('without the bot name the instruction says what the main user has to do', async () => {
    mockApi({
      ...page,
      'POST /me/channels/telegram/link': () =>
        json({
          ...telegram,
          telegram: { linked: false, botUsername: '', linkCode: 'AB2C3D' },
        }),
    });
    renderApp('/settings/notifications');
    const bot = await card('Telegram');
    await userEvent.setup().click(within(bot).getByRole('button', { name: 'Verknüpfen' }));
    expect(
      await within(bot).findByText('Sende dem Slouchcat-Bot die Nachricht /start AB2C3D'),
    ).toBeVisible();
    expect(within(bot).getByText(/muss zuerst das Bot-Token unter Integrationen/)).toBeVisible();
    expect(within(bot).queryByText(/@/)).not.toBeInTheDocument();
    expect(within(bot).queryByRole('link', { name: /In Telegram öffnen/ })).toBeNull();
  });

  it('a new Telegram action clears the error of the last one', async () => {
    let state: ChannelSetting = {
      ...telegram,
      telegram: { linked: false, botUsername: 'slouchcat_bot', linkCode: 'AB2C3D' },
    };
    let refuse = true;
    mockApi({
      ...page,
      'GET /me/channels': () => json(settings(email, push, state)),
      'POST /me/channels/telegram/link': () =>
        refuse ? problem(429, 'too_many_attempts') : json(state),
      'DELETE /me/channels/telegram/link': () => {
        state = telegram;
        return noContent();
      },
    });
    renderApp('/settings/notifications');
    const bot = await card('Telegram');
    const user = userEvent.setup();
    await user.click(within(bot).getByRole('button', { name: 'Code erneuern' }));
    expect(await within(bot).findByRole('alert')).toBeVisible();
    refuse = false;
    await user.click(within(bot).getByRole('button', { name: 'Verknüpfung lösen' }));
    expect(await within(bot).findByText('Die Verknüpfung ist gelöst.')).toBeVisible();
    expect(within(bot).queryByRole('alert')).not.toBeInTheDocument();
  });

  it('unlinks a linked Telegram chat', async () => {
    let state: ChannelSetting = {
      ...telegram,
      enabled: true,
      telegram: { linked: true, chatId: '4242', botUsername: 'slouchcat_bot' },
    };
    const { calls } = mockApi({
      ...page,
      'GET /me/channels': () => json(settings(email, push, state)),
      'DELETE /me/channels/telegram/link': () => {
        state = { ...telegram, enabled: false };
        return noContent();
      },
    });
    renderApp('/settings/notifications');
    const bot = await card('Telegram');
    await userEvent.setup().click(within(bot).getByRole('button', { name: 'Verknüpfung lösen' }));
    expect(await within(bot).findByText('Die Verknüpfung ist gelöst.')).toBeVisible();
    expect(await within(bot).findByText('Nicht verknüpft')).toBeVisible();
    expect(calls.some((call) => call.method === 'DELETE')).toBe(true);
  });

  describe('Web Push', () => {
    /** A browser with a service worker and PushManager; `subscribe` resolves a subscription. */
    function pushBrowser(permission: NotificationPermission = 'granted') {
      const subscribe = vi.fn((options: PushSubscriptionOptionsInit) =>
        Promise.resolve({
          options,
          toJSON: () => ({
            endpoint: 'https://push.example.test/send/abc',
            keys: { p256dh: 'BPUBLIC', auth: 'AUTHSECRET' },
          }),
          unsubscribe: () => Promise.resolve(true),
        }),
      );
      const registration = {
        pushManager: { getSubscription: vi.fn(() => Promise.resolve(null)), subscribe },
      };
      Object.defineProperty(window, 'isSecureContext', { value: true, configurable: true });
      Object.defineProperty(navigator, 'serviceWorker', {
        value: {
          ready: Promise.resolve(registration),
          getRegistration: () => Promise.resolve(registration),
        },
        configurable: true,
      });
      vi.stubGlobal('PushManager', function PushManager() {});
      const requestPermission = vi.fn(() => Promise.resolve(permission));
      vi.stubGlobal('Notification', { requestPermission });
      return { subscribe, requestPermission };
    }

    it('subscribes this device with the server key and registers it', async () => {
      const { subscribe } = pushBrowser();
      const { calls } = mockApi({
        ...page,
        'GET /push/vapid-key': () => json({ publicKey: VAPID_KEY }),
        'POST /me/push-subscriptions': () =>
          json({ id: 9, createdAt: '2026-10-21T10:00:00Z', thisDevice: true }, 201),
      });
      renderApp('/settings/notifications');
      const web = await card('Web-Push');
      expect(within(web).getByText('Firefox 140 · Linux')).toBeVisible();
      expect(within(web).getByText(/nur in der installierten App/)).toBeVisible();
      await userEvent
        .setup()
        .click(within(web).getByRole('button', { name: 'Auf diesem Gerät aktivieren' }));

      expect(
        await within(web).findByText('Dieses Gerät erhält jetzt Push-Nachrichten.'),
      ).toBeVisible();
      const options = subscribe.mock.calls[0]?.[0];
      expect(options?.userVisibleOnly).toBe(true);
      expect((options?.applicationServerKey as Uint8Array).length).toBe(65);
      const body = calls.find((call) => call.path === '/me/push-subscriptions')?.body;
      expect(body).toMatchObject({
        endpoint: 'https://push.example.test/send/abc',
        keys: { p256dh: 'BPUBLIC', auth: 'AUTHSECRET' },
      });
    });

    it('says so when the permission is refused, and registers nothing', async () => {
      const { subscribe } = pushBrowser('denied');
      const { calls } = mockApi(page);
      renderApp('/settings/notifications');
      const web = await card('Web-Push');
      await userEvent
        .setup()
        .click(within(web).getByRole('button', { name: 'Auf diesem Gerät aktivieren' }));
      expect(
        await within(web).findByText(/Benachrichtigungen sind für Slouchcat blockiert/),
      ).toBeVisible();
      expect(subscribe).not.toHaveBeenCalled();
      expect(calls.some((call) => call.path === '/me/push-subscriptions')).toBe(false);
    });

    it('explains a browser without push and offers no subscription', async () => {
      mockApi(page);
      renderApp('/settings/notifications');
      const web = await card('Web-Push');
      expect(within(web).getByText(/kann hier keine Push-Nachrichten empfangen/)).toBeVisible();
      expect(
        within(web).queryByRole('button', { name: 'Auf diesem Gerät aktivieren' }),
      ).not.toBeInTheDocument();
    });

    it('marks this device and removes a subscription', async () => {
      pushBrowser();
      let subscriptions = [
        ...(push.push?.subscriptions ?? []),
        {
          id: 4,
          deviceName: 'Chrome 156 · Linux',
          createdAt: '2026-10-21T08:00:00Z',
          thisDevice: true,
        },
      ];
      const { calls } = mockApi({
        ...page,
        'GET /me/channels': () =>
          json(settings(email, { ...push, push: { subscriptions } }, telegram)),
        'DELETE /me/push-subscriptions/3': () => {
          subscriptions = subscriptions.filter((entry) => entry.id !== 3);
          return noContent();
        },
      });
      renderApp('/settings/notifications');
      const web = await card('Web-Push');
      const list = within(web).getByRole('list', { name: 'Angemeldete Geräte' });
      expect(within(list).getByText('Dieses Gerät')).toBeVisible();
      // This device has a subscription already.
      expect(
        within(web).queryByRole('button', { name: 'Auf diesem Gerät aktivieren' }),
      ).not.toBeInTheDocument();
      await userEvent
        .setup()
        .click(within(web).getByRole('button', { name: 'Firefox 140 · Linux entfernen' }));
      expect(await within(web).findByText('Das Gerät ist entfernt.')).toBeVisible();
      await waitFor(() =>
        expect(within(web).queryByText('Firefox 140 · Linux')).not.toBeInTheDocument(),
      );
      expect(
        calls.some((call) => call.method === 'DELETE' && call.path === '/me/push-subscriptions/3'),
      ).toBe(true);
    });
  });
});
