import { fileURLToPath } from 'node:url';

/** Port of the server the suite starts (start-server.mjs); 8096 stays free for development. */
export const PORT = Number(process.env.SLOUCHCAT_E2E_PORT ?? 8097);
/**
 * Origin the specs open. Set SLOUCHCAT_E2E_BASE_URL to run against a server you started yourself
 * (for example `vite preview` proxying to a Go server); Playwright then starts no server.
 */
export const EXTERNAL_BASE_URL = process.env.SLOUCHCAT_E2E_BASE_URL;
export const BASE_URL = EXTERNAL_BASE_URL ?? `http://localhost:${PORT}`;

/**
 * Run artefacts (session state, traces, report) live under node_modules/.playwright, which git,
 * ESLint and Prettier already ignore.
 */
export const ARTEFACTS = fileURLToPath(new URL('../node_modules/.playwright/', import.meta.url));

/** Storage state with the session cookie of the main account, written by global-setup.ts. */
export const STORAGE_STATE = `${ARTEFACTS}owner.json`;

/** Generated test media (`task testdata:media`), scanned as the first library. */
export const MOVIES_DIR = fileURLToPath(new URL('../../testdata/media/Movies', import.meta.url));

/** Generated series (`task testdata:media`), scanned as the series library "Serien" (M3). */
export const TV_DIR = fileURLToPath(new URL('../../testdata/media/TV', import.meta.url));

/** Fixed test account; exists only in the throw-away config directory of the run. */
export const OWNER = {
  email: 'e2e-owner@slouchcat.test',
  password: 'e2e-password-0123',
} as const;

/** Name of the main account's primary profile: the setup wizard takes it from the e-mail address. */
export const OWNER_PROFILE = OWNER.email.split('@')[0];
