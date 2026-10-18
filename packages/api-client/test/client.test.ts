import { afterEach, describe, expect, it, vi } from 'vitest';
import { createApiClient, isProblem, type Health, type Problem } from '../src/index';

function jsonResponse(body: unknown, status: number, contentType = 'application/json'): Response {
  return new Response(JSON.stringify(body), { status, headers: { 'Content-Type': contentType } });
}

describe('createApiClient', () => {
  afterEach(() => {
    vi.unstubAllGlobals();
  });

  it('calls getHealth on /healthz below the base URL and returns typed data', async () => {
    const health: Health = { status: 'ok', version: '0.1.0+abc1234', uptimeSeconds: 42 };
    const fetchMock = vi.fn(async (_request: Request) => jsonResponse(health, 200));
    vi.stubGlobal('fetch', fetchMock);

    const client = createApiClient('http://slouchcat.test/api/v1');
    const { data, error } = await client.GET('/healthz');

    expect(error).toBeUndefined();
    expect(data).toEqual(health);
    expect(fetchMock).toHaveBeenCalledOnce();
    const request = fetchMock.mock.calls[0]?.[0];
    expect(request?.method).toBe('GET');
    expect(request?.url).toBe('http://slouchcat.test/api/v1/healthz');
    // Reads carry no custom header, so they can reuse a preloaded response.
    expect(request?.headers.get('X-Requested-With')).toBeNull();
  });

  it('sends the CSRF header with state-changing requests', async () => {
    const fetchMock = vi.fn(async (_request: Request) => new Response(null, { status: 204 }));
    vi.stubGlobal('fetch', fetchMock);

    const client = createApiClient('http://slouchcat.test/api/v1');
    await client.POST('/auth/logout');

    const request = fetchMock.mock.calls[0]?.[0];
    expect(request?.method).toBe('POST');
    expect(request?.headers.get('X-Requested-With')).toBe('Slouchcat');
  });

  it('returns a problem document as error when the server is not ready', async () => {
    const problem: Problem = {
      type: 'about:blank',
      title: 'Service Unavailable',
      status: 503,
      code: 'not_ready',
    };
    vi.stubGlobal('fetch', async () => jsonResponse(problem, 503, 'application/problem+json'));

    const { data, error } = await createApiClient('http://slouchcat.test/api/v1').GET('/healthz');

    expect(data).toBeUndefined();
    expect(isProblem(error)).toBe(true);
    expect(error).toEqual(problem);
  });
});

describe('isProblem', () => {
  it('rejects values without the required members', () => {
    expect(isProblem(undefined)).toBe(false);
    expect(isProblem('not_ready')).toBe(false);
    expect(isProblem({ title: 'x', status: 500 })).toBe(false);
  });
});
