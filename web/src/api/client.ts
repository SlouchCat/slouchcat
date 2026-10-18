import { createApiClient, DEFAULT_BASE_URL, isProblem, type Problem } from '@slouchcat/api-client';

/** Shared same-origin API client; the dev server proxies /api to the Go server. */
export const api = createApiClient();

/**
 * Resolves a URL the API returned (image and stream URLs are relative to the API base) into a
 * same-origin path. URLs that already carry the base or an origin stay untouched.
 */
export function apiUrl(url: string): string {
  if (/^[a-z][a-z0-9+.-]*:/i.test(url) || url.startsWith(`${DEFAULT_BASE_URL}/`)) return url;
  return `${DEFAULT_BASE_URL}${url.startsWith('/') ? '' : '/'}${url}`;
}

/** A failed API call: HTTP status plus the problem details when the server sent them. */
export class ApiError extends Error {
  readonly status: number;
  readonly problem: Problem | null;

  constructor(status: number, problem: Problem | null) {
    super(problem?.code ?? `http_${status}`);
    this.name = 'ApiError';
    this.status = status;
    this.problem = problem;
  }

  /** Stable problem code, or `unknown` when the server sent no problem body. */
  get code(): string {
    return this.problem?.code ?? 'unknown';
  }
}

interface ClientResult<T> {
  data?: T;
  error?: unknown;
  response: Response;
}

/** Awaits an openapi-fetch call and returns its data, or throws an {@link ApiError}. */
export async function request<T>(call: Promise<ClientResult<T>>): Promise<T> {
  const { data, error, response } = await call;
  if (!response.ok) throw new ApiError(response.status, isProblem(error) ? error : null);
  return data as T;
}

export const isApiError = (error: unknown, status?: number): error is ApiError =>
  error instanceof ApiError && (status === undefined || error.status === status);
