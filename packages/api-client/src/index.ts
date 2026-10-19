import createClient, { type Client } from 'openapi-fetch';
import type { components, paths } from './generated/schema';

export type { components, paths } from './generated/schema';

export type Health = components['schemas']['Health'];
export type SessionInfo = components['schemas']['SessionInfo'];
export type Account = components['schemas']['Account'];
export type Profile = components['schemas']['Profile'];
export type ProfileSummary = components['schemas']['ProfileSummary'];
export type ProfileRequest = components['schemas']['ProfileRequest'];
export type ProfileUpdate = components['schemas']['ProfileUpdate'];
export type Accent = components['schemas']['Accent'];
export type Language = components['schemas']['Language'];
export type Rating = components['schemas']['Rating'];
export type Invitation = components['schemas']['Invitation'];
export type InvitationCreated = components['schemas']['InvitationCreated'];
export type InvitationRequest = components['schemas']['InvitationRequest'];
export type InvitationPreview = components['schemas']['InvitationPreview'];
export type AcceptInvitationRequest = components['schemas']['AcceptInvitationRequest'];
export type Guest = components['schemas']['Guest'];
export type GuestUpdate = components['schemas']['GuestUpdate'];
export type SetupRequest = components['schemas']['SetupRequest'];
export type LoginRequest = components['schemas']['LoginRequest'];
export type HomePage = components['schemas']['HomePage'];
export type HomeRow = components['schemas']['HomeRow'];
export type ItemRef = components['schemas']['ItemRef'];
export type ItemDetail = components['schemas']['ItemDetail'];
export type MediaFileInfo = components['schemas']['MediaFileInfo'];
export type PlaybackInfo = components['schemas']['PlaybackInfo'];
export type ProgressEvent = components['schemas']['ProgressEvent'];
export type Progress = components['schemas']['Progress'];
export type Library = components['schemas']['Library'];
export type LibrarySummary = components['schemas']['LibrarySummary'];
export type LibraryRequest = components['schemas']['LibraryRequest'];
export type FieldError = components['schemas']['FieldError'];
/** RFC 9457 problem details as returned by every failing JSON endpoint. */
export type Problem = components['schemas']['Problem'];
export type ApiClient = Client<paths>;

/** Same-origin base path of the API (the `servers` entry of api/openapi.yaml). */
export const DEFAULT_BASE_URL = '/api/v1';

/**
 * Header every state-changing request must carry (contract description). Browsers never add a
 * custom header to cross-site form posts, so with the SameSite=Lax cookie this blocks CSRF.
 */
export const CSRF_HEADER = 'X-Requested-With';
export const CSRF_HEADER_VALUE = 'Slouchcat';

const SAFE_METHODS = new Set(['GET', 'HEAD', 'OPTIONS']);

/**
 * Creates a typed client for the Slouchcat API.
 *
 * A relative `baseUrl` is resolved against the current document location when there is one,
 * so the same client works in the browser and in DOM test environments. `fetch` is looked up
 * on every request, which lets tests replace `globalThis.fetch` after the client was created.
 */
export function createApiClient(baseUrl: string = DEFAULT_BASE_URL): ApiClient {
  const location = (globalThis as { location?: { href: string } }).location;
  return createClient<paths>({
    baseUrl: location ? new URL(baseUrl, location.href).href : baseUrl,
    credentials: 'same-origin',
    fetch: (request) => {
      // Only state-changing requests need the CSRF header. Reads go without it, so they match the
      // responses the server preloads for the start page (Link: rel=preload; as=fetch, P-10).
      if (!SAFE_METHODS.has(request.method)) request.headers.set(CSRF_HEADER, CSRF_HEADER_VALUE);
      return globalThis.fetch(request);
    },
  });
}

/** Narrows an unknown error body to {@link Problem}. */
export function isProblem(value: unknown): value is Problem {
  if (typeof value !== 'object' || value === null) return false;
  const candidate = value as Record<string, unknown>;
  return (
    typeof candidate.type === 'string' &&
    typeof candidate.title === 'string' &&
    typeof candidate.status === 'number' &&
    typeof candidate.code === 'string'
  );
}
