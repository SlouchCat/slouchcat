import type { ChannelSetting, ChannelSettings } from '@slouchcat/api-client';
import type { QueryClient } from '@tanstack/react-query';
import { act, screen, waitFor, within } from '@testing-library/react';
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
const linkedTelegram: ChannelSetting = {
  ...telegram,
  enabled: true,
  telegram: { linked: true, chatId: '4242', botUsername: 'slouchcat_bot' },
};
const openCode: ChannelSetting = {
  ...telegram,
  telegram: {
    linked: false,
    botUsername: 'slouchcat_bot',
    linkCode: 'AB2C3D',
    linkExpiresAt: '2026-10-21T10:15:00Z',
  },
};

const settings = (...channels: ChannelSetting[]): ChannelSettings => ({ channels });
const noContent = () => new Response(null, { status: 204 });

// The server lists push before telegram; the page shows e-mail, Telegram, Web-Push.
const page: Routes = {
  ...signedIn,
  'GET /me/channels': () => json(settings(email, push, telegram)),
};

/** A channel's block: a region named by its band. */
const block = (name: string) => screen.findByRole('region', { name });

const guestSession = () =>
  json({
    ...session,
    account: { ...session.account, role: 'user', isOwner: false, email: 'gast@example.test' },
  });

/** The account's second profile: it sees the channels but cannot change them. */
const secondProfileSession = () =>
  json({ ...session, profile: { ...session.profile!, id: 2, name: 'Lena', isPrimary: false } });

/** What the mutation cache holds (variables and data), as text. */
const mutationCache = (queryClient: QueryClient) =>
  queryClient
    .getMutationCache()
    .getAll()
    .map((mutation) => JSON.stringify([mutation.state.variables, mutation.state.data]))
    .join('\n');

const channelGets = (calls: { method: string; path: string }[]) =>
  calls.filter((call) => call.method === 'GET' && call.path === '/me/channels').length;

describe('notification channels page', () => {
  afterEach(() => {
    vi.useRealTimers();
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
    const mail = await block('E-Mail');
    expect(within(mail).getByRole('switch', { name: 'Aktiv' })).not.toBeChecked();
    expect(within(mail).getByText('Gilt für das ganze Konto mit allen Profilen.')).toBeVisible();
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
    expect(within(mail).getByText(/«Fehler» gibt es nur für Administratoren/)).toBeVisible();
    expect(within(mail).getByText('Aus')).toBeVisible();
    // E-mail, Telegram, Web-Push, each a block that folds away by its band (mockup Ben-A).
    const list = screen.getByRole('list', { name: 'Kanäle' });
    expect(
      within(list)
        .getAllByRole('heading', { level: 2 })
        .map((heading) => heading.textContent),
    ).toEqual(['E-Mail', 'Telegram', 'Web-Push']);
    expect(within(mail).getByRole('button', { name: 'E-Mail' })).toHaveAttribute(
      'aria-expanded',
      'true',
    );
  });

  it('keeps the intro behind the help button and today’s density (S-1, S-2)', async () => {
    mockApi(page);
    renderApp('/settings/notifications');
    await block('E-Mail');
    const intro = screen.getByText(/zusätzlich zum Eingang der App/);
    expect(intro).not.toBeVisible();
    await userEvent.setup().click(screen.getByRole('button', { name: 'Hilfe zu dieser Seite' }));
    expect(intro).toBeVisible();
    // Every account has this page: no administration shell, so nothing gets compact.
    expect(document.documentElement.dataset.shell).toBeUndefined();
  });

  it('hides "Fehler" from a guest account', async () => {
    mockApi({ ...page, 'GET /auth/session': guestSession });
    renderApp('/settings/notifications');
    const mail = await block('E-Mail');
    expect(within(mail).getByRole('checkbox', { name: 'Einladungen' })).toBeVisible();
    expect(within(mail).queryByRole('checkbox', { name: 'Fehler' })).not.toBeInTheDocument();
    expect(within(mail).getByText('Wofür eine Nachricht kommt.')).toBeVisible();
  });

  it('shows another profile than the primary one the channels without controls', async () => {
    const { calls } = mockApi({
      ...page,
      'GET /auth/session': secondProfileSession,
      'GET /me/channels': () =>
        json(
          settings(
            { ...email, enabled: true, events: ['new_content', 'announcements'] },
            push,
            linkedTelegram,
          ),
        ),
    });
    renderApp('/settings/notifications');
    expect(await screen.findByText('Nur im Hauptprofil')).toBeVisible();
    expect(screen.getByText(/ändern lassen sie sich im Hauptprofil/)).toBeVisible();
    expect(screen.getByRole('link', { name: 'Profil wechseln' })).toHaveAttribute(
      'href',
      '/profiles',
    );

    const mail = await block('E-Mail');
    expect(within(mail).getByText('Aktiv')).toBeVisible();
    const facts = (region: HTMLElement) =>
      within(region)
        .getAllByRole('term')
        .map((term) => [term.textContent, term.nextElementSibling?.textContent]);
    expect(facts(mail)).toEqual([
      ['Ereignisse', 'Neue Inhalte · Ankündigungen'],
      ['E-Mail-Adresse', 'owner@example.test'],
    ]);
    expect(facts(await block('Web-Push'))).toEqual([
      ['Ereignisse', 'Neue Inhalte'],
      ['Angemeldete Geräte', '1'],
    ]);
    expect(facts(await block('Telegram'))).toEqual([
      ['Ereignisse', 'Neue Inhalte'],
      ['Verknüpfung', 'Verknüpft'],
    ]);
    // No switch, no checkbox, no field: nothing to change. The only buttons fold the blocks.
    const list = screen.getByRole('list', { name: 'Kanäle' });
    for (const role of ['switch', 'checkbox', 'textbox'] as const) {
      expect(within(list).queryAllByRole(role)).toHaveLength(0);
    }
    expect(
      within(list)
        .getAllByRole('button')
        .map((button) => button.textContent),
    ).toEqual(['E-Mail', 'Telegram', 'Web-Push']);
    for (const button of within(list).getAllByRole('button')) {
      expect(button).toHaveAttribute('aria-expanded', 'true');
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
    const mail = await block('E-Mail');
    const user = userEvent.setup();
    // Without a change «Speichern» sends nothing (decision 18): change the switch first.
    await user.click(within(mail).getByRole('switch', { name: 'Aktiv' }));
    await user.click(within(mail).getByRole('button', { name: 'Speichern' }));
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
    const mail = await block('E-Mail');
    const user = userEvent.setup();
    await user.click(within(mail).getByRole('switch', { name: 'Aktiv' }));
    await user.click(within(mail).getByRole('checkbox', { name: 'Neue Inhalte' }));
    const address = within(mail).getByLabelText('E-Mail-Adresse');
    await user.clear(address);
    await user.type(address, 'familie@example.test');
    expect(within(mail).getByText('Ungespeicherte Änderungen')).toBeVisible();
    await user.click(within(mail).getByRole('button', { name: 'Speichern' }));

    expect(await within(mail).findByText('Gespeichert.')).toBeVisible();
    expect(calls.find((call) => call.method === 'PUT')?.body).toEqual({
      enabled: true,
      events: ['new_content', 'announcements'],
      email: { address: 'familie@example.test' },
    });
    // The band says «Aktiv» now, and nothing is unsaved any more.
    expect(within(mail).queryByText('Aus')).not.toBeInTheDocument();
    expect(within(mail).getByRole('switch', { name: 'Aktiv' })).toBeChecked();
    expect(within(mail).queryByText('Ungespeicherte Änderungen')).not.toBeInTheDocument();
  });

  it('announces the same message again on a second save', async () => {
    mockApi({
      ...page,
      'PUT /me/channels/email': (call) =>
        json({ ...email, ...(call.body as object), available: true }),
    });
    renderApp('/settings/notifications');
    const mail = await block('E-Mail');
    const user = userEvent.setup();
    // Each save needs a change (decision 18): an event on, then off again.
    await user.click(within(mail).getByRole('checkbox', { name: 'Neue Inhalte' }));
    await user.click(within(mail).getByRole('button', { name: 'Speichern' }));
    const first = await within(mail).findByText('Gespeichert.');
    await user.click(within(mail).getByRole('checkbox', { name: 'Neue Inhalte' }));
    await user.click(within(mail).getByRole('button', { name: 'Speichern' }));
    // A new element in the region that is always there: screen readers read it again.
    await waitFor(() => expect(first).not.toBeInTheDocument());
    const second = await within(mail).findByText('Gespeichert.');
    expect(second.closest('[role="status"]')).toBeInTheDocument();
  });

  it('«Speichern» without a change sends nothing and keeps the focus', async () => {
    const { calls } = mockApi(page);
    renderApp('/settings/notifications');
    const mail = await block('E-Mail');
    const save = within(mail).getByRole('button', { name: 'Speichern' });
    await userEvent.setup().click(save);
    expect(save).toHaveFocus();
    expect(calls.some((call) => call.method === 'PUT')).toBe(false);
  });

  it('keeps a change made while the save waits for the server', async () => {
    let release = () => undefined as void;
    const answered = new Promise<void>((resolve) => {
      release = resolve;
    });
    const { calls } = mockApi({
      ...page,
      'PUT /me/channels/email': async (call) => {
        await answered;
        return json({ ...email, ...(call.body as object), available: true });
      },
    });
    renderApp('/settings/notifications');
    const mail = await block('E-Mail');
    const user = userEvent.setup();
    await user.click(within(mail).getByRole('checkbox', { name: 'Neue Inhalte' }));
    await user.click(within(mail).getByRole('button', { name: 'Speichern' }));
    await waitFor(() => expect(calls.some((call) => call.method === 'PUT')).toBe(true));
    // Changed while the save runs: it stays a draft once the answer is there.
    const neu = within(mail).getByRole('checkbox', { name: 'Neue Inhalte' });
    const before = (neu as HTMLInputElement).checked;
    await user.click(within(mail).getByRole('switch', { name: 'Aktiv' }));
    release();
    expect(await within(mail).findByText('Gespeichert.')).toBeVisible();
    expect(within(mail).getByText('Ungespeicherte Änderungen')).toBeVisible();
    expect(neu).toHaveProperty('checked', before);
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
    const mail = await block('E-Mail');
    const user = userEvent.setup();
    const address = within(mail).getByLabelText('E-Mail-Adresse');
    await user.clear(address);
    await user.type(address, 'familie@example');
    await user.click(within(mail).getByRole('button', { name: 'Speichern' }));
    expect(await within(mail).findByText('Bitte prüfe die markierten Felder.')).toBeVisible();
    expect(within(mail).getByLabelText('E-Mail-Adresse')).toHaveAttribute('aria-invalid', 'true');
  });

  it('sends a test message and says what went wrong', async () => {
    let answer: () => Response = noContent;
    const { calls } = mockApi({ ...page, 'POST /me/channels/email/test': () => answer() });
    renderApp('/settings/notifications');
    const mail = await block('E-Mail');
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
    const bot = await block('Telegram');
    expect(within(bot).getByText('Nicht eingerichtet')).toBeVisible();
    expect(within(bot).getByText(/richtet diesen Kanal in der Verwaltung/)).toBeVisible();
    expect(within(bot).getByRole('link', { name: 'Integrationen öffnen' })).toHaveAttribute(
      'href',
      '/admin/integrations',
    );
    expect(within(bot).getByRole('switch', { name: 'Aktiv' })).toBeDisabled();
    expect(within(bot).getByRole('button', { name: 'Speichern' })).toBeDisabled();
    expect(within(bot).getByRole('button', { name: 'Testnachricht' })).toBeDisabled();
    expect(within(bot).queryByRole('button', { name: 'Verknüpfen' })).not.toBeInTheDocument();
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
    const bot = await block('Telegram');
    expect(within(bot).getByText(/richtet diesen Kanal in der Verwaltung/)).toBeVisible();
    expect(within(bot).queryByRole('link')).not.toBeInTheDocument();
  });

  it('links Telegram: the code with the instruction, then "Verknüpft" once the bot got it', async () => {
    let state = telegram;
    const { calls } = mockApi({
      ...page,
      'GET /me/channels': () => json(settings(email, push, state)),
      'POST /me/channels/telegram/link': () => {
        state = openCode;
        return json(state);
      },
    });
    const { queryClient } = renderApp('/settings/notifications');
    const bot = await block('Telegram');
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
    expect(within(bot).getByText(/Der Code gilt bis/)).toBeVisible();
    expect(within(bot).getByText(/Abfrage alle 3 Sekunden/)).toBeVisible();
    // The header says the page polls while the code is open.
    expect(screen.getByText('Live, alle 3 Sekunden')).toBeVisible();
    // The code stays out of the mutation cache (transient mutation).
    expect(mutationCache(queryClient)).not.toContain('AB2C3D');

    // The bot receives "/start AB2C3D"; the page asks again while the code is open.
    state = linkedTelegram;
    expect(
      await within(bot).findByText('Verknüpft mit Chat 4242', {}, { timeout: 5000 }),
    ).toBeVisible();
    expect(within(bot).queryByText(/Sende dem Bot/)).not.toBeInTheDocument();
    expect(channelGets(calls)).toBeGreaterThan(1);
    // The status line announces the link; the poll and its live indicator end.
    await waitFor(() =>
      expect(
        within(bot)
          .getAllByRole('status')
          .map((status) => status.textContent),
      ).toContain('Verknüpft mit Chat 4242'),
    );
    expect(screen.queryByText('Live, alle 3 Sekunden')).not.toBeInTheDocument();
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
    const bot = await block('Telegram');
    await userEvent.setup().click(within(bot).getByRole('button', { name: 'Verknüpfen' }));
    expect(
      await within(bot).findByText('Sende dem Slouchcat-Bot die Nachricht /start AB2C3D'),
    ).toBeVisible();
    expect(within(bot).getByText(/muss zuerst das Bot-Token unter Integrationen/)).toBeVisible();
    expect(within(bot).queryByText(/@/)).not.toBeInTheDocument();
    expect(within(bot).queryByRole('link', { name: /In Telegram öffnen/ })).toBeNull();
  });

  it('a new Telegram action clears the error of the last one', async () => {
    let state: ChannelSetting = openCode;
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
    const bot = await block('Telegram');
    const user = userEvent.setup();
    await user.click(within(bot).getByRole('button', { name: 'Code erneuern' }));
    expect(await within(bot).findByRole('alert')).toBeVisible();
    refuse = false;
    // An open code goes after a question too.
    await user.click(within(bot).getByRole('button', { name: 'Verknüpfung lösen' }));
    const question = await screen.findByRole('alertdialog', { name: 'Code verwerfen?' });
    expect(question).toHaveTextContent('Der offene Code gilt sofort nicht mehr.');
    await user.click(within(question).getByRole('button', { name: 'Code verwerfen' }));
    expect(await within(bot).findByText('Die Verknüpfung ist gelöst.')).toBeVisible();
    expect(within(bot).queryByRole('alert')).not.toBeInTheDocument();
  });

  it('unlinks a linked Telegram chat only after the question (11.5 point 5)', async () => {
    let state: ChannelSetting = linkedTelegram;
    const { calls } = mockApi({
      ...page,
      'GET /me/channels': () => json(settings(email, push, state)),
      'DELETE /me/channels/telegram/link': () => {
        state = { ...telegram, enabled: false };
        return noContent();
      },
    });
    renderApp('/settings/notifications');
    const bot = await block('Telegram');
    const user = userEvent.setup();
    const unlink = within(bot).getByRole('button', { name: 'Verknüpfung lösen' });

    // One click no longer unlinks: it asks; «Abbrechen» keeps the link.
    await user.click(unlink);
    let question = await screen.findByRole('alertdialog', { name: 'Verknüpfung lösen?' });
    expect(question).toHaveTextContent(/gehen nicht mehr an den verknüpften Chat/);
    await user.click(within(question).getByRole('button', { name: 'Abbrechen' }));
    await waitFor(() => expect(question).not.toBeInTheDocument());
    // The focus goes back to the button that asked, and the h1 does not take it from there.
    await waitFor(() => expect(unlink).toHaveFocus());
    await new Promise((resolve) => setTimeout(resolve, 50));
    expect(unlink).toHaveFocus();
    expect(calls.some((call) => call.method === 'DELETE')).toBe(false);

    await user.click(unlink);
    question = await screen.findByRole('alertdialog', { name: 'Verknüpfung lösen?' });
    await user.click(within(question).getByRole('button', { name: 'Verknüpfung lösen' }));
    expect(await within(bot).findByText('Die Verknüpfung ist gelöst.')).toBeVisible();
    expect(await within(bot).findByText('Nicht verknüpft')).toBeVisible();
    expect(calls.some((call) => call.method === 'DELETE')).toBe(true);
    // The button went with the link: the h1 takes the focus.
    await waitFor(() =>
      expect(
        screen.getByRole('heading', { level: 1, name: 'Benachrichtigungskanäle' }),
      ).toHaveFocus(),
    );
  });

  it('a folded block keeps what was typed in it and shows its summary', async () => {
    mockApi({ ...page, 'GET /me/channels': () => json(settings(email, push, linkedTelegram)) });
    renderApp('/settings/notifications');
    const mail = await block('E-Mail');
    const user = userEvent.setup();
    const address = within(mail).getByLabelText('E-Mail-Adresse');
    await user.clear(address);
    await user.type(address, 'neu@example.test');
    await user.click(within(mail).getByRole('checkbox', { name: 'Fehler' }));

    const band = within(mail).getByRole('button', { name: 'E-Mail' });
    await user.click(band);
    expect(band).toHaveAttribute('aria-expanded', 'false');
    expect(address).not.toBeVisible();
    // Folded, the band says what the block holds and that it holds unsaved input.
    expect(within(mail).getByText('Ungespeicherte Änderungen')).toBeVisible();
    // The summary is the address as typed: what «Speichern» would store.
    expect(within(mail).getByText('neu@example.test')).toBeVisible();
    const bot = await block('Telegram');
    await user.click(within(bot).getByRole('button', { name: 'Telegram' }));
    expect(within(bot).getByText('Verknüpft')).toBeVisible();
    const web = await block('Web-Push');
    await user.click(within(web).getByRole('button', { name: 'Web-Push' }));
    expect(within(web).getByText('1 Gerät')).toBeVisible();

    await user.click(band);
    expect(within(mail).getByLabelText('E-Mail-Adresse')).toHaveValue('neu@example.test');
    expect(within(mail).getByRole('checkbox', { name: 'Fehler' })).toBeChecked();
    // Changing everything back is no change.
    await user.click(within(mail).getByRole('checkbox', { name: 'Fehler' }));
    await user.clear(within(mail).getByLabelText('E-Mail-Adresse'));
    await user.type(within(mail).getByLabelText('E-Mail-Adresse'), 'owner@example.test');
    expect(within(mail).queryByText('Ungespeicherte Änderungen')).not.toBeInTheDocument();
  });

  it('leaving the page with unsaved input asks first; staying keeps it', async () => {
    mockApi(page);
    const { router } = renderApp('/settings/notifications');
    const user = userEvent.setup();
    const mail = await block('E-Mail');
    await user.click(within(mail).getByRole('checkbox', { name: 'Neue Inhalte' }));

    void router.navigate({ to: '/notifications' });
    const question = await screen.findByRole('alertdialog', {
      name: 'Seite mit ungespeicherten Änderungen verlassen?',
    });
    expect(within(question).getByRole('button', { name: 'Auf der Seite bleiben' })).toHaveFocus();
    await user.click(within(question).getByRole('button', { name: 'Auf der Seite bleiben' }));
    await waitFor(() => expect(question).not.toBeInTheDocument());
    expect(router.state.location.pathname).toBe('/settings/notifications');
    expect(within(mail).getByRole('checkbox', { name: 'Neue Inhalte' })).toBeChecked();

    void router.navigate({ to: '/notifications' });
    await user.click(await screen.findByRole('button', { name: 'Verwerfen und verlassen' }));
    await waitFor(() => expect(router.state.location.pathname).toBe('/notifications'));
  });

  it('holds the poll of an open code while a channel holds unsaved input', async () => {
    vi.useFakeTimers({ shouldAdvanceTime: true });
    const { calls } = mockApi({
      ...page,
      'GET /me/channels': () => json(settings(email, push, openCode)),
    });
    renderApp('/settings/notifications');
    const user = userEvent.setup({ advanceTimers: (ms) => vi.advanceTimersByTime(ms) });
    const mail = await block('E-Mail');
    const bot = await block('Telegram');
    expect(within(bot).getByText(/Abfrage alle 3 Sekunden/)).toBeVisible();
    expect(screen.getByText('Live, alle 3 Sekunden')).toBeVisible();
    await user.click(within(mail).getByRole('checkbox', { name: 'Neue Inhalte' }));
    expect(within(bot).getByText(/Die Abfrage ruht/)).toBeVisible();
    expect(screen.queryByText('Live, alle 3 Sekunden')).not.toBeInTheDocument();
    const before = channelGets(calls);
    await act(() => vi.advanceTimersByTimeAsync(10_000));
    expect(channelGets(calls)).toBe(before);

    // Unchecked again, nothing is unsaved: the poll goes on.
    await user.click(within(mail).getByRole('checkbox', { name: 'Neue Inhalte' }));
    await act(() => vi.advanceTimersByTimeAsync(3500));
    await waitFor(() => expect(channelGets(calls)).toBeGreaterThan(before));
  });

  it('another profile than the primary one neither polls nor shows the live indicator', async () => {
    vi.useFakeTimers({ shouldAdvanceTime: true });
    const { calls } = mockApi({
      ...page,
      'GET /auth/session': secondProfileSession,
      'GET /me/channels': () => json(settings(email, push, openCode)),
    });
    renderApp('/settings/notifications');
    await block('Telegram');
    expect(screen.queryByText('Live, alle 3 Sekunden')).not.toBeInTheDocument();
    const before = channelGets(calls);
    await act(() => vi.advanceTimersByTimeAsync(10_000));
    expect(channelGets(calls)).toBe(before);
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
      const { queryClient } = renderApp('/settings/notifications');
      const web = await block('Web-Push');
      expect(within(web).getByText('Firefox 140 · Linux')).toBeVisible();
      expect(within(web).getByText(/nur in der installierten App/)).toBeVisible();
      const user = userEvent.setup();
      const activate = within(web).getByRole('button', { name: 'Auf diesem Gerät aktivieren' });
      await user.click(activate);
      // Busy until the list is fresh again: a second press registers nothing.
      await user.click(activate);

      expect(
        await within(web).findByText('Dieses Gerät erhält jetzt Push-Nachrichten.'),
      ).toBeVisible();
      expect(calls.filter((call) => call.path === '/me/push-subscriptions')).toHaveLength(1);
      const options = subscribe.mock.calls[0]?.[0];
      expect(options?.userVisibleOnly).toBe(true);
      expect((options?.applicationServerKey as Uint8Array).length).toBe(65);
      const body = calls.find((call) => call.path === '/me/push-subscriptions')?.body;
      expect(body).toMatchObject({
        endpoint: 'https://push.example.test/send/abc',
        keys: { p256dh: 'BPUBLIC', auth: 'AUTHSECRET' },
      });
      // Nothing of the subscription stays in the mutation cache (transient mutation).
      const cache = mutationCache(queryClient);
      expect(cache).not.toContain('AUTHSECRET');
      expect(cache).not.toContain('push.example.test');
    });

    it('says so when the permission is refused, and registers nothing', async () => {
      const { subscribe } = pushBrowser('denied');
      const { calls } = mockApi(page);
      renderApp('/settings/notifications');
      const web = await block('Web-Push');
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
      const web = await block('Web-Push');
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
      const web = await block('Web-Push');
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
      // The removed device's button had the focus: the h1 takes it.
      await waitFor(() =>
        expect(
          screen.getByRole('heading', { level: 1, name: 'Benachrichtigungskanäle' }),
        ).toHaveFocus(),
      );
    });
  });
});
