import type { APIRequestContext } from '@playwright/test';
import { CSRF } from './catalog';
import { RADARR_BASE, SONARR_BASE } from './env';
import { expect } from './fixtures';

// The Radarr and Sonarr mocks (arr-server.mjs, concept 25 stage 4) for the administration's specs:
// steer what they hold, test them through the server (which confirms their host, so the jobs call
// them), and run the two jobs that read them instead of waiting for their schedule.

type Service = 'radarr' | 'sonarr';

const base = (service: Service) => (service === 'radarr' ? RADARR_BASE : SONARR_BASE);

/** Steers a mock: `absent`, `queue`, `failEditor`, `files` (see arr-server.mjs). */
export async function arrControl(service: Service, body: object): Promise<void> {
  const response = await fetch(`${base(service)}/__control`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(body),
  });
  expect(response.ok).toBe(true);
}

/** Both mocks as at the start: no tags, no queue, every title there, no failure. */
export async function arrReset(): Promise<void> {
  for (const service of ['radarr', 'sonarr'] as const) {
    const response = await fetch(`${base(service)}/__reset`, { method: 'POST' });
    expect(response.ok).toBe(true);
  }
}

/** «Testen» through the server: reachable with the key from Seerr, the host confirmed. */
export async function testArr(api: APIRequestContext, service: Service): Promise<void> {
  const response = await api.post('/api/v1/admin/seerr/arr/test', {
    headers: CSRF,
    data: { service },
  });
  expect(response.ok(), await response.text()).toBe(true);
  const result = (await response.json()) as { reachable: boolean; keyValid: boolean };
  expect(result, service).toMatchObject({ reachable: true, keyValid: true });
}

/**
 * Runs a job type now (the job page's «Jetzt ausführen»): `wishes.tag` sets the profile tags,
 * `wishes.progress` reads the queues.
 */
export async function runJob(api: APIRequestContext, type: string): Promise<void> {
  const response = await api.post(`/api/v1/admin/job-types/${type}/run`, { headers: CSRF });
  expect(response.status(), await response.text()).toBe(202);
}
