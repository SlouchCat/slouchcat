import { defineConfig, devices } from '@playwright/test';
import { ARTEFACTS, BASE_URL, EXTERNAL_BASE_URL, PORT, STORAGE_STATE } from './e2e/env';

// WebKit needs system libraries (libavif16 and friends, `sudo pnpm exec playwright install-deps`)
// that a dev machine may lack; its project runs only when SLOUCHCAT_E2E_WEBKIT=1.
const withWebkit = process.env.SLOUCHCAT_E2E_WEBKIT === '1';

// What Firefox runs (Marco's decision of 3 November 2026, ADR 0013 addendum). Chromium runs every
// spec; Firefox runs what differs between the engines: media in the browser and the flows a viewer
// cannot get around, whole, and of every other page exactly one keyboard test (rule 8), tagged
// `@keyboard` in its title. Firefox's focus model is where the two engines part most often (Tab
// does not wrap, the sequential start point stays behind a replaced button, :focus-visible after a
// pointer), and one Tab and arrow walk per page in Firefox keeps catching that; the other tests
// of those pages check what the server answers and the page draws, which the engine does not
// change. Before: Chromium 309 tests, Firefox 307. After: Chromium 309, Firefox 118 (31 whole
// files with 79 tests, 39 `@keyboard` tests in 38 files).
//
// Every spec, by group (73 files; whole = all tests of the file run in Firefox):
//
//  Playback, whole (Video/HLS/MSE, tracks, quality, chapters, versions, HDR, the player reacting
//  to the server): audio-transcode, admin-streams, chapters, hdr, hls, player, quality, resume,
//  sidecar, stream-token, tracks, versions, watch-limit.
//  Audio, whole (audiobooks, podcasts): audio-download, audio-offline, audiobook, podcast,
//  podcast-auto-download.
//  Offline, whole: auto-download, downloads, offline-measure, offline-pin, offline-playback,
//  offline-profiles, offline-revocation, offline-start, offline-watch-limit, season-download.
//  (offline-cold-start stays Chromium-only, see testIgnore below.)
//  Sign-in and setup, whole: invite, login, setup.
//  One `@keyboard` test each: admin-accounts, admin-announcements, admin-backup,
//  admin-collections, admin-devices, admin-folder-browser, admin-home, admin-home-rows,
//  admin-integrations, admin-item, admin-jobs (the cancel menu and the h1 focus, a known Firefox
//  focus bug), admin-libraries, admin-library-folders (the Tab budget to the folders dialog, a
//  known Firefox focus bug), admin-metadata, admin-picks, admin-podcasts, admin-spotlight,
//  admin-stats, admin-webhooks, collection, detail, episode, extras, favorites, home,
//  invitations, library, maintenance, notification-channels, notifications, person, picks-page,
//  profiles (two: the picker and the profile management are two pages), search, series, sidenav,
//  trailer, watchlist.
//  Chromium only, no keyboard test of a page of their own: admin-density and admin-phone (sizes
//  and widths of the admin pages), announcements (the notice at the next start).
//
// A new spec runs in Chromium only until it is listed here or one of its tests carries
// `@keyboard`; a new page needs its keyboard test anyway (rule 8), so tag that one.
const FIREFOX_WHOLE_SPECS = [
  // playback
  'audio-transcode',
  'admin-streams',
  'chapters',
  'hdr',
  'hls',
  'player',
  'quality',
  'resume',
  'sidecar',
  'stream-token',
  'tracks',
  'versions',
  'watch-limit',
  // audio
  'audio-download',
  'audio-offline',
  'audiobook',
  'podcast',
  'podcast-auto-download',
  // offline
  'auto-download',
  'downloads',
  'offline-measure',
  'offline-pin',
  'offline-playback',
  'offline-profiles',
  'offline-revocation',
  'offline-start',
  'offline-watch-limit',
  'season-download',
  // sign-in and setup
  'invite',
  'login',
  'setup',
];

// A project's grep is matched against "<project> <file> <describe…> <title> <tags>" with the file
// relative to testDir, so the first pattern picks the whole files and the second the tagged tests.
const firefoxGrep = [
  new RegExp(` (${FIREFOX_WHOLE_SPECS.join('|')})\\.spec\\.ts `),
  /(^| )@keyboard( |$)/,
];

export default defineConfig({
  testDir: './e2e',
  fullyParallel: false,
  // One server, one database: specs that change data (progress) must not race each other.
  //
  // CI splits the suite with `--shard` instead (ADR 0013). A shard is a whole run of its own on a
  // runner of its own -- own server, own database, own global setup -- so this stays 1 inside a
  // shard and nothing races. Playwright keeps a spec file whole while sharding (with
  // fullyParallel off, a file is one group), so the order within a file is the order it always had.
  // What a shard does not have is the data of the spec files that landed in the other shards:
  // every spec must create what it needs. On one machine the shards must still run one after the
  // other, because they all want the same port and the same artefact folder.
  workers: 1,
  forbidOnly: Boolean(process.env.CI),
  retries: process.env.CI ? 1 : 0,
  outputDir: `${ARTEFACTS}results`,
  // In CI every shard writes a blob report -- the machine-readable form that keeps the traces and
  // the screenshots -- and the gate job merges all of them into the one HTML report that a red run
  // uploads (`playwright merge-reports`). A single HTML report per shard would be six reports with
  // a sixth of the run in each.
  reporter: process.env.CI ? [['list'], ['blob', { outputDir: `${ARTEFACTS}blob` }]] : 'list',
  timeout: 30_000,
  expect: { timeout: 10_000 },
  globalSetup: './e2e/global-setup.ts',
  // A-9: scans the server log of the run for secrets while the server still runs.
  globalTeardown: './e2e/global-teardown.ts',
  use: {
    baseURL: BASE_URL,
    storageState: STORAGE_STATE,
    locale: 'de-CH',
    trace: 'retain-on-failure',
    screenshot: 'only-on-failure',
  },
  webServer: EXTERNAL_BASE_URL
    ? undefined
    : {
        command: 'node e2e/start-server.mjs',
        url: `${BASE_URL}/api/v1/healthz`,
        reuseExistingServer: false,
        timeout: 60_000,
        stdout: 'pipe',
        stderr: 'pipe',
        env: { SLOUCHCAT_E2E_PORT: String(PORT) },
      },
  projects: [
    { name: 'chromium', use: { ...devices['Desktop Chrome'] } },
    {
      name: 'firefox',
      use: {
        ...devices['Desktop Firefox'],
        // Chromium runs muted in Playwright, Firefox does not: the sine tones of the test media
        // would beep through the speakers during every run. Volume 0 still needs an audio output:
        // without one Firefox fails the audio sink of an audio-only title (MediaError 3
        // "OnMediaSinkAudioError") and hls.js re-attaches the element over and over. A machine
        // without a sound card therefore needs a null sink (CI workflow, docs/dev/SETUP.md).
        launchOptions: { firefoxUserPrefs: { 'media.volume_scale': '0.0' } },
      },
      // Media, offline, sign-in and one keyboard test per page (FIREFOX_WHOLE_SPECS above).
      grep: firefoxGrep,
      // Flight mode cannot be emulated with a service worker in Playwright's Firefox: setOffline
      // does not reach the worker's own requests (the app still gets its session), and a route
      // that cuts the API makes Firefox bypass the worker for the page (NS_ERROR_OFFLINE). The
      // cold start in flight mode (C-1) therefore runs in Chromium; the offline specs that run
      // here prove the worker's control and its precache (prepareOffline, offline playback).
      testIgnore: ['**/offline-cold-start.spec.ts'],
    },
    ...(withWebkit ? [{ name: 'webkit', use: { ...devices['Desktop Safari'] } }] : []),
  ],
});
