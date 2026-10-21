import { expect, type APIRequestContext, type APIResponse } from '@playwright/test';
import { CSRF } from './catalog';
import { EXTERNAL_BASE_URL, PUSH_BASE, SMTP_HTTP_BASE, TELEGRAM_BASE, UPDATE_BASE } from './env';
import { CHAT_ID } from './telegram-server.mjs';

// Helpers of the M8 specs: the mocks of the notification channels and the update check
// (smtp-server.mjs, push-server.mjs, telegram-server.mjs, update-server.mjs, started by
// start-server.mjs) and the channel and announcement state of an account through the API.

/** A mail the SMTP mock received (envelope plus decoded subject and text parts). */
export interface Mail {
  id: number;
  from: string;
  to: string[];
  headerFrom: string;
  headerTo: string;
  subject: string;
  text: string;
  html: string | null;
  receivedAt: string;
}

/** A push message the push mock received; `payload` is the decrypted JSON. */
export interface Push {
  id: string;
  status: number;
  ttl: string | null;
  bodyLength: number;
  payload: { title?: string; body?: string; url?: string } | string | null;
  error: string | null;
  receivedAt: string;
}

/** A message the Telegram mock's bot sent. */
export interface BotMessage {
  chatId: string;
  text: string;
  parseMode: string | null;
  at: string;
}

/** A subscription as the browser's PushSubscription.toJSON() gives it. */
export interface PushSubscriptionJson {
  endpoint: string;
  keys: { p256dh: string; auth: string };
}

async function mock(base: string, path: string, init?: RequestInit): Promise<Response> {
  try {
    return await fetch(`${base}${path}`, init);
  } catch (error) {
    throw new Error(
      `The mock at ${base} does not answer. start-server.mjs starts it; against a server of your ` +
        `own (SLOUCHCAT_E2E_BASE_URL) start it yourself (node e2e/<name>-server.mjs).`,
      { cause: error },
    );
  }
}

async function control(base: string, path: string, body: unknown): Promise<void> {
  const response = await mock(base, path, { method: 'POST', body: JSON.stringify(body) });
  expect([200, 201, 204], `${base}${path} answered ${response.status}`).toContain(response.status);
}

async function list<T>(base: string, path: string): Promise<T> {
  const response = await mock(base, path);
  expect(response.status, `${base}${path}`).toBe(200);
  return (await response.json()) as T;
}

/** Every mail the SMTP mock received since the last reset. */
export const mails = () => list<Mail[]>(SMTP_HTTP_BASE, '/__mails');

/** Mails to `address`. */
export const mailsTo = async (address: string) =>
  (await mails()).filter((mail) => mail.to.includes(address));

/** The next `count` AUTH attempts fail with 535 although the credentials are right. */
export const smtpRejectAuth = (count: number) =>
  control(SMTP_HTTP_BASE, '/__control', { rejectAuth: count });

/** Push messages the push mock received since the last reset. */
export const pushes = () => list<Push[]>(PUSH_BASE, '/__pushes');

/**
 * A fresh subscription of the push mock (its own keys, so it can decrypt what it gets); with
 * `gone` its endpoint answers 410 like an expired one.
 */
export async function newPushSubscription(gone = false): Promise<PushSubscriptionJson> {
  const response = await mock(PUSH_BASE, '/__subscriptions', {
    method: 'POST',
    body: JSON.stringify({ gone }),
  });
  expect(response.status, 'push mock subscription').toBe(201);
  return (await response.json()) as PushSubscriptionJson;
}

/** The auth secrets and endpoints the push mock handed out (the log scan looks for them). */
export const pushSubscriptionSecrets = () =>
  list<{ id: string; endpoint: string; auth: string }[]>(PUSH_BASE, '/__subscriptions');

/** Messages the Telegram mock's bot sent since the last reset. */
export const botMessages = () => list<BotMessage[]>(TELEGRAM_BASE, '/__messages');

/** The user sends "/start CODE" to the bot from `chatId` (the link instruction of the page). */
export const telegramStart = (code: string, chatId = CHAT_ID) =>
  control(TELEGRAM_BASE, '/__control', { start: code, chatId });

/** The next `count` bot requests lose their connection without an answer (A-9 probe). */
export const telegramDrop = (count: number) =>
  control(TELEGRAM_BASE, '/__control', { drop: count });

/** Publishes a release in the manifest mock, or withdraws it (null: 404 again). */
export const setUpdateManifest = (version: string | null) =>
  control(UPDATE_BASE, '/__control', { version });

/**
 * Empties the SMTP, push and Telegram mocks and withdraws the release of the manifest mock
 * (fixtures.ts, before every test). Only for the suite's own server: against another one the
 * mocks may not run at all.
 */
export async function resetChannelMocks(): Promise<void> {
  if (EXTERNAL_BASE_URL) return;
  await Promise.all(
    [SMTP_HTTP_BASE, PUSH_BASE, TELEGRAM_BASE, UPDATE_BASE].map((base) =>
      control(base, '/__reset', {}),
    ),
  );
}

async function expectStatus(response: APIResponse, ok: number[]): Promise<void> {
  expect(ok, `${response.url()} answered ${response.status()}: ${await response.text()}`).toContain(
    response.status(),
  );
}

export type ChannelName = 'email' | 'push' | 'telegram';
export type EventName = 'new_content' | 'acquisition' | 'errors' | 'invitations' | 'announcements';

export interface ChannelData {
  channel: ChannelName;
  enabled: boolean;
  events: EventName[];
  available: boolean;
  email?: { address: string };
  telegram?: { linked: boolean; chatId?: string; linkCode?: string; botUsername?: string };
  push?: { subscriptions: { id: number; deviceName?: string; thisDevice: boolean }[] };
}

/** The channels of the request context's account. */
export async function channels(api: APIRequestContext): Promise<ChannelData[]> {
  const response = await api.get('/api/v1/me/channels');
  await expectStatus(response, [200]);
  return ((await response.json()) as { channels: ChannelData[] }).channels;
}

/** The channel `name` of the request context's account. */
export async function channel(api: APIRequestContext, name: ChannelName): Promise<ChannelData> {
  const found = (await channels(api)).find((entry) => entry.channel === name);
  if (!found) throw new Error(`channel ${name} missing in GET /me/channels`);
  return found;
}

/** Switches a channel and chooses its events (PUT /me/channels/{channel}). */
export async function setChannel(
  api: APIRequestContext,
  name: ChannelName,
  data: { enabled: boolean; events: EventName[]; email?: { address: string } },
): Promise<ChannelData> {
  const response = await api.put(`/api/v1/me/channels/${name}`, { headers: CSRF, data });
  await expectStatus(response, [200]);
  return (await response.json()) as ChannelData;
}

/** Registers a push subscription for the request context's account (as the page does). */
export async function addPushSubscription(
  api: APIRequestContext,
  subscription: PushSubscriptionJson,
  deviceName = 'E2E',
): Promise<{ id: number }> {
  const response = await api.post('/api/v1/me/push-subscriptions', {
    headers: CSRF,
    data: { ...subscription, deviceName },
  });
  await expectStatus(response, [201]);
  return (await response.json()) as { id: number };
}

/**
 * The account's channels as plan 5.2 leaves an owner: every channel off, no Telegram link, no push
 * subscription; the events stay as they are. For the specs that switch channels on (afterEach).
 */
export async function resetChannels(api: APIRequestContext): Promise<void> {
  for (const entry of await channels(api)) {
    for (const subscription of entry.push?.subscriptions ?? []) {
      const removed = await api.delete(`/api/v1/me/push-subscriptions/${subscription.id}`, {
        headers: CSRF,
      });
      await expectStatus(removed, [204, 404]);
    }
    if (entry.channel === 'telegram') {
      const unlinked = await api.delete('/api/v1/me/channels/telegram/link', { headers: CSRF });
      await expectStatus(unlinked, [204]);
    }
    if (entry.enabled)
      await setChannel(api, entry.channel, { enabled: false, events: entry.events });
  }
}

/**
 * Deletes every announcement (admin API). A published one opens a dialog on every page of every
 * profile that has not acknowledged it, so a spec that publishes one removes it afterwards.
 */
export async function deleteAnnouncements(api: APIRequestContext): Promise<void> {
  const response = await api.get('/api/v1/admin/announcements');
  await expectStatus(response, [200]);
  for (const { id } of (await response.json()) as { id: number }[]) {
    await expectStatus(
      await api.delete(`/api/v1/admin/announcements/${id}`, { headers: CSRF }),
      [204, 404],
    );
  }
}

/** Writes and publishes an announcement (admin API). */
export async function publishAnnouncement(
  api: APIRequestContext,
  title: string,
  body: string,
): Promise<{ id: number }> {
  const response = await api.post('/api/v1/admin/announcements', {
    headers: CSRF,
    data: { title, body, publish: true },
  });
  await expectStatus(response, [201]);
  return (await response.json()) as { id: number };
}
