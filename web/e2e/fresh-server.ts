import { spawn, type ChildProcess } from 'node:child_process';
import { mkdtempSync, rmSync } from 'node:fs';
import { tmpdir } from 'node:os';
import { join } from 'node:path';
import { fileURLToPath } from 'node:url';
import { MEDIA_DIR, PORT } from './env';

/**
 * A server of its own for a spec that needs a server without an account (the guided wizard,
 * concept 22, 3.6): the same binary as the run's (start-server.mjs), with fresh temporary config
 * and cache directories, on a port 300 above the run's (8397 next to 8097), so it never meets
 * the shared server or a run on another port. It stops and removes its directories at the end.
 */
export interface FreshServer {
  url: string;
  stop: () => Promise<void>;
}

const binary = fileURLToPath(new URL('../../server/bin/slouchcat-e2e', import.meta.url));

/** The port of the fresh server: SLOUCHCAT_E2E_FRESH_PORT, else 300 above the run's. */
export const FRESH_PORT = Number(process.env.SLOUCHCAT_E2E_FRESH_PORT ?? PORT + 300);

async function healthy(url: string, deadline: number): Promise<void> {
  for (;;) {
    try {
      const res = await fetch(`${url}/api/v1/healthz`);
      if (res.ok) return;
    } catch {
      // Not listening yet.
    }
    if (Date.now() > deadline) throw new Error(`fresh server at ${url} did not come up`);
    await new Promise((resolve) => setTimeout(resolve, 200));
  }
}

export async function startFreshServer(port = FRESH_PORT): Promise<FreshServer> {
  const root = mkdtempSync(join(tmpdir(), 'slouchcat-e2e-fresh-'));
  const url = `http://localhost:${port}`;
  const server: ChildProcess = spawn(
    binary,
    [
      'serve',
      '--listen',
      `:${port}`,
      '--config-dir',
      join(root, 'config'),
      '--cache-dir',
      join(root, 'cache'),
      '--dev',
      '--log-format',
      'text',
    ],
    {
      stdio: ['ignore', 'ignore', 'inherit'],
      env: {
        ...process.env,
        SLOUCHCAT_TMDB_API_KEY: '',
        SLOUCHCAT_BROWSE_ROOTS: MEDIA_DIR,
        SLOUCHCAT_RADIO_DIR: join(root, 'radio'),
        SLOUCHCAT_HW: '',
      },
    },
  );
  const exited = new Promise<void>((resolve) => server.once('exit', () => resolve()));
  await healthy(url, Date.now() + 30_000);
  return {
    url,
    stop: async () => {
      server.kill('SIGTERM');
      await exited;
      rmSync(root, { recursive: true, force: true });
    },
  };
}
