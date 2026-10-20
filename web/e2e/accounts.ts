import { expect, type APIRequestContext, type PlaywrightWorkerArgs } from '@playwright/test';
import { mkdirSync, mkdtempSync, writeFileSync } from 'node:fs';
import { tmpdir } from 'node:os';
import { join } from 'node:path';
import { CSRF } from './catalog';
import { OWNER, STORAGE_STATE } from './env';

// Helpers for the profile and access specs. They log in with a session of their own (the cookie
// lands in the page's context): a profile switch changes the session on the server, and the
// shared storage-state session of the other specs must stay on the primary profile.

type PlaywrightWorker = PlaywrightWorkerArgs['playwright'];

export interface ProfileData {
  id: number;
  name: string;
  isPrimary: boolean;
  hasPin: boolean;
  maxRating: number;
  language: string;
}

export interface SessionData {
  account: { id: number; role: string; email: string };
  profile?: ProfileData;
}

/** POST/PUT/DELETE that waits out the attempt limiter (429) instead of failing the test. */
async function send(
  api: APIRequestContext,
  method: 'post' | 'put' | 'patch' | 'delete',
  path: string,
  data?: unknown,
) {
  for (let attempt = 0; ; attempt += 1) {
    const response = await api[method](`/api/v1${path}`, { headers: CSRF, data });
    if (response.status() !== 429 || attempt >= 4) return response;
    const wait = Number(response.headers()['retry-after'] ?? '6');
    await new Promise((resolve) => setTimeout(resolve, Math.min(wait, 20) * 1000));
  }
}

async function json<T>(response: Awaited<ReturnType<typeof send>>, ok: number[]): Promise<T> {
  expect(ok, `${response.url()} answered ${response.status()}: ${await response.text()}`).toContain(
    response.status(),
  );
  return (response.status() === 204 ? undefined : await response.json()) as T;
}

/** Logs the main account in with a fresh session; the cookie is stored in `api`'s context. */
export async function loginOwner(api: APIRequestContext): Promise<SessionData> {
  const response = await send(api, 'post', '/auth/login', {
    email: OWNER.email,
    password: OWNER.password,
  });
  return json<SessionData>(response, [200]);
}

export async function listMyProfiles(api: APIRequestContext): Promise<ProfileData[]> {
  return json<ProfileData[]>(await api.get('/api/v1/auth/profiles'), [200]);
}

/**
 * Creates a profile. `api` must have the primary profile active (the `owner` fixture's shared
 * session always has).
 */
export async function createProfile(
  api: APIRequestContext,
  body: Record<string, unknown>,
): Promise<ProfileData> {
  return json<ProfileData>(await send(api, 'post', '/profiles', body), [201]);
}

/**
 * Leaves the account with its primary profile only, without a PIN, as the setup wizard made it.
 * `api` must have the primary profile active.
 */
export async function resetProfiles(api: APIRequestContext): Promise<void> {
  const profiles = await json<ProfileData[]>(await api.get('/api/v1/profiles'), [200]);
  for (const profile of profiles) {
    if (profile.isPrimary) {
      await json(await send(api, 'delete', `/profiles/${profile.id}/pin`), [204]);
    } else {
      await json(await send(api, 'delete', `/profiles/${profile.id}`), [204]);
    }
  }
}

export interface InvitationData {
  id: number;
  state: string;
  url: string;
  token: string;
}

export async function createInvitation(
  api: APIRequestContext,
  body: Record<string, unknown> = {
    note: 'E2E',
    maxRating: 18,
    allLibraries: true,
    libraryIds: [],
  },
): Promise<InvitationData> {
  return json<InvitationData>(await send(api, 'post', '/admin/invitations', body), [201]);
}

export async function revokeInvitation(api: APIRequestContext, id: number): Promise<void> {
  await json(await send(api, 'delete', `/admin/invitations/${id}`), [204]);
}

/** Deletes every guest account (the ones the specs created). */
/**
 * Ends every running stream of the server (`DELETE /admin/streams/{id}`), so a transcode a spec
 * left behind (a page closed before its stop reached the server) does not hold the admission
 * budget of the next spec: on the CI runner the software budget is 2, one HDR transcode with CPU
 * tonemapping costs 6 and an idle session keeps its budget until it turns cold.
 */
export async function stopAllStreams(api: APIRequestContext): Promise<void> {
  const response = await api.get('/api/v1/admin/streams');
  if (!response.ok()) return;
  const streams = (await response.json()) as { id: string }[];
  for (const stream of streams) {
    await api.delete(`/api/v1/admin/streams/${stream.id}`, { headers: CSRF });
  }
}

export async function removeGuests(api: APIRequestContext): Promise<void> {
  const guests = await json<{ id: number }[]>(await api.get('/api/v1/admin/guests'), [200]);
  for (const guest of guests)
    await json(await send(api, 'delete', `/admin/guests/${guest.id}`), [204]);
}

/** Accepts an invitation through the API; the guest's session cookie lands in `api`'s context. */
export async function acceptInvitation(
  api: APIRequestContext,
  token: string,
  body: Record<string, unknown>,
): Promise<SessionData> {
  return json<SessionData>(await send(api, 'post', `/invitations/${token}/accept`, body), [201]);
}

/** API client with the shared storage-state session of the main account (primary profile). */
export function ownerApi(
  playwright: PlaywrightWorker,
  baseURL: string,
  extraHTTPHeaders: Record<string, string> = {},
): Promise<APIRequestContext> {
  return playwright.request.newContext({ baseURL, storageState: STORAGE_STATE, extraHTTPHeaders });
}

/** API client without a session, like a visitor with an invitation link. */
export function anonymousApi(
  playwright: PlaywrightWorker,
  baseURL: string,
  extraHTTPHeaders: Record<string, string> = {},
): Promise<APIRequestContext> {
  return playwright.request.newContext({ baseURL, extraHTTPHeaders });
}

/**
 * Id of a second library without any title (a folder with one text file), created once per run:
 * the way to tell a library-restricted profile or guest from an unrestricted one. There is no API
 * to delete a library, so it stays for the rest of the run.
 */
export async function ensureEmptyLibrary(api: APIRequestContext): Promise<number> {
  const name = 'E2E leer';
  const libraries = await json<{ id: number; name: string }[]>(
    await api.get('/api/v1/admin/libraries'),
    [200],
  );
  const existing = libraries.find((library) => library.name === name);
  if (existing) return existing.id;
  const root = mkdtempSync(join(tmpdir(), 'slouchcat-e2e-empty-'));
  mkdirSync(root, { recursive: true });
  writeFileSync(join(root, 'readme.txt'), 'no media here');
  const created = await json<{ id: number }>(
    await send(api, 'post', '/admin/libraries', { name, kind: 'movies', path: root }),
    [201, 202],
  );
  return created.id;
}
