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

/**
 * Generated audiobooks (`task testdata:media`, M6): one M4B with 12 chapters and one folder of 10
 * MP3 files, scanned as the library "Hörbücher" (kind audiobooks, default rating 0).
 */
export const AUDIOBOOKS_DIR = fileURLToPath(
  new URL('../../testdata/media/Audiobooks', import.meta.url),
);

/**
 * Port of the local podcast feed (feed-server.mjs, started by start-server.mjs): the server
 * subscribes to it instead of the internet (H-2, no network in CI). Two below the app's port
 * (8095 next to 8097), so parallel runs on other ports do not collide.
 */
export const FEED_PORT = Number(process.env.SLOUCHCAT_E2E_FEED_PORT ?? PORT - 2);
export const FEED_BASE = `http://127.0.0.1:${FEED_PORT}`;
/** The feed the global setup subscribes to in the library "Podcasts". */
export const FEED_URL = `${FEED_BASE}/feed.xml`;

/** Ids of the test audiobooks by title, written by global-setup.ts (e2e/audio.ts audioItemId). */
export const AUDIO_IDS = `${ARTEFACTS}audio-ids.json`;

/** Fixed test account; exists only in the throw-away config directory of the run. */
export const OWNER = {
  email: 'e2e-owner@slouchcat.test',
  password: 'e2e-password-0123',
} as const;

/** Name of the main account's primary profile: the setup wizard takes it from the e-mail address. */
export const OWNER_PROFILE = OWNER.email.split('@')[0];

// Mocks of the notification channels and the update check (M8, start-server.mjs), below the app's
// port like the feed and Trakt: SMTP four, Web Push five, Telegram six, the release manifest seven,
// the mailbox of the SMTP mock (HTTP) eight below (8093 … 8089 next to 8097).
export const SMTP_PORT = Number(process.env.SLOUCHCAT_E2E_SMTP_PORT ?? PORT - 4);
export const SMTP_HTTP_BASE = `http://127.0.0.1:${Number(process.env.SLOUCHCAT_E2E_SMTP_HTTP_PORT ?? PORT - 8)}`;
export const PUSH_BASE = `http://127.0.0.1:${Number(process.env.SLOUCHCAT_E2E_PUSH_PORT ?? PORT - 5)}`;
export const TELEGRAM_BASE = `http://127.0.0.1:${Number(process.env.SLOUCHCAT_E2E_TELEGRAM_PORT ?? PORT - 6)}`;
export const UPDATE_BASE = `http://127.0.0.1:${Number(process.env.SLOUCHCAT_E2E_UPDATE_PORT ?? PORT - 7)}`;

/** Copy of the server's log, written by start-server.mjs and scanned by global-teardown.ts (A-9). */
export const SERVER_LOG = `${ARTEFACTS}server.log`;

/** Passphrase of the backups the specs create (A-4); a fake secret the log scan looks for (A-9). */
export const BACKUP_PASSPHRASE = 'e2e-backup-passphrase-7c41d9';
