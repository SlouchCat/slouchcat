import createClient, { type Client } from 'openapi-fetch';
import type { components, paths } from './generated/schema';

export type { components, paths } from './generated/schema';

export type Health = components['schemas']['Health'];
/** RFC 9457 problem details as returned by every failing JSON endpoint. */
export type Problem = components['schemas']['Problem'];
export type ApiClient = Client<paths>;

/** Same-origin base path of the API (the `servers` entry of api/openapi.yaml). */
export const DEFAULT_BASE_URL = '/api/v1';

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
    fetch: (request) => globalThis.fetch(request),
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
