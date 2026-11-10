// Runs the Playwright suite as several shards side by side on one machine (`task ci:e2e:parallel`),
// each a whole run of its own like a CI shard (ADR 0013): its own server on its own port, its own
// artefact folder and its own copy of the test media, cut by measured duration (shard-plan.mjs).
// At the end the blob reports are merged into one HTML report (playwright-report/), as in CI.
//
// Usage (from web/):
//   node e2e/parallel.mjs [--shards 3] [--base-port 8097] [-- <playwright arguments>]
//
// Ports: shard i uses base + 20·(i-1) for the app and nine of the ten below it for its mocks
// (env.ts; one below stays free), so 8097, 8117, 8137 by default. Every port is checked before anything
// starts: a busy one stops the run instead of colliding with a server that runs there.
//
// Media: the specs write into the media folder (a sidecar subtitle, copied episodes, library
// folders under .e2e). Every shard gets a copy of testdata/media made of hard links (instant, no
// space; the specs only add and remove files, they never write into one), in
// node_modules/.playwright-parallel/shard-<i>/testdata/media.
import { execFileSync, spawn } from 'node:child_process';
import {
  createWriteStream,
  existsSync,
  mkdirSync,
  readFileSync,
  rmSync,
  writeFileSync,
} from 'node:fs';
import { createServer } from 'node:net';
import { join } from 'node:path';
import { fileURLToPath } from 'node:url';
import { groupsOf, listTests, plan, readDurations, testList } from './shard-plan.mjs';

const WEB = fileURLToPath(new URL('..', import.meta.url));
const MEDIA = fileURLToPath(new URL('../../testdata/media', import.meta.url));
const ROOT = join(WEB, 'node_modules', '.playwright-parallel');
/** The parts of testdata/media the suite reads (env.ts, feed-server.mjs reads Podcasts in place). */
const MEDIA_PARTS = ['Movies', 'TV', 'Audiobooks'];

const argv = process.argv.slice(2);
const split = argv.indexOf('--');
const own = split >= 0 ? argv.slice(0, split) : argv;
const extra = split >= 0 ? argv.slice(split + 1) : [];
const value = (name, fallback) => {
  const index = own.indexOf(name);
  return index >= 0 ? own[index + 1] : fallback;
};
const count = Number(value('--shards', '3'));
const base = Number(value('--base-port', process.env.SLOUCHCAT_E2E_PORT ?? '8097'));
if (!(count >= 1 && Number.isInteger(count)) || !(base > 1024)) {
  console.error('usage: node e2e/parallel.mjs [--shards n] [--base-port p] [-- playwright args]');
  process.exit(2);
}

const free = (port, host) =>
  new Promise((resolve) => {
    const server = createServer();
    server.once('error', () => resolve(false));
    server.listen({ port, host, exclusive: true }, () => server.close(() => resolve(true)));
  });

// The offsets a run really uses below its app port (env.ts, start-server.mjs): feed 2, Trakt 3,
// SMTP 4, push 5, Telegram 6, update manifest 7, SMTP mailbox 8, radio catalog and test station 9,
// book sources 10. One below is free on purpose: 8096 next to the default 8097 is the development
// server, which must not stop `task ci`.
const USED_OFFSETS = [0, 2, 3, 4, 5, 6, 7, 8, 9, 10];

const ports = Array.from({ length: count }, (_, index) => base + 20 * index);
const busy = [];
for (const port of ports) {
  for (const offset of USED_OFFSETS) {
    const candidate = port - offset;
    if (!(await free(candidate, '127.0.0.1')) || !(await free(candidate, '::')))
      busy.push(candidate);
  }
}
if (busy.length > 0) {
  console.error(`ports in use: ${busy.join(', ')}; pick another --base-port (SLOUCHCAT_E2E_PORT)`);
  process.exit(1);
}
for (const part of MEDIA_PARTS) {
  if (!existsSync(join(MEDIA, part))) {
    console.error(`${join(MEDIA, part)} is missing: run \`task testdata:media\` first`);
    process.exit(1);
  }
}

// One plan for all shards, as in CI. Extra arguments that narrow the run (a spec, --project) go
// into the listing too, so the plan only holds what will run.
const shards = plan(groupsOf(listTests(extra), readDurations()), count);
rmSync(ROOT, { recursive: true, force: true });

const started = Date.now();
const runs = shards.map((shard, index) => {
  const dir = join(ROOT, `shard-${shard.index}`);
  const media = join(dir, 'testdata', 'media');
  mkdirSync(media, { recursive: true });
  for (const part of MEDIA_PARTS) execFileSync('cp', ['-al', join(MEDIA, part), join(media, part)]);
  const list = join(dir, 'test-list.txt');
  writeFileSync(list, testList(shard));
  const log = join(dir, 'run.log');
  const out = createWriteStream(log);
  console.log(
    `shard ${shard.index}/${count}: port ${ports[index]}, ${shard.tests} tests, ` +
      `about ${Math.round(shard.seconds)} s by durations.json; log ${log}`,
  );
  const child = spawn('pnpm', ['exec', 'playwright', 'test', '--test-list', list, ...extra], {
    cwd: WEB,
    env: {
      ...process.env,
      SLOUCHCAT_E2E_PORT: String(ports[index]),
      SLOUCHCAT_E2E_ARTEFACTS: dir,
      SLOUCHCAT_E2E_MEDIA: media,
      SLOUCHCAT_E2E_SHARD: `${shard.index}/${count}`,
      SLOUCHCAT_E2E_BLOB: '1',
    },
    stdio: ['ignore', 'pipe', 'pipe'],
  });
  child.stdout.pipe(out, { end: false });
  child.stderr.pipe(out, { end: false });
  return new Promise((resolve) => {
    child.on('exit', (code) => {
      out.end();
      const seconds = Math.round((Date.now() - started) / 1000);
      const summary = readFileSync(log, 'utf8')
        .split('\n')
        .filter((line) => /^\s+\d+ (passed|failed|flaky|skipped|did not run)/.test(line))
        .map((line) => line.trim())
        .join(', ');
      console.log(
        `shard ${shard.index}/${count} done after ${seconds} s: ${summary || `exit ${code}`}`,
      );
      resolve({ shard: shard.index, code: code ?? 1, dir });
    });
  });
});

const results = await Promise.all(runs);
const blobs = join(ROOT, 'blobs');
mkdirSync(blobs, { recursive: true });
for (const result of results) {
  const zip = join(result.dir, 'blob', `report-${result.shard}.zip`);
  if (existsSync(zip)) execFileSync('cp', [zip, blobs]);
}
try {
  execFileSync('pnpm', ['exec', 'playwright', 'merge-reports', '--reporter', 'html', blobs], {
    cwd: WEB,
    env: { ...process.env, PLAYWRIGHT_HTML_OPEN: 'never' },
    stdio: ['ignore', 'ignore', 'inherit'],
  });
  console.log('merged report: web/playwright-report/index.html (pnpm exec playwright show-report)');
} catch {
  console.error('merging the blob reports failed; the logs of the shards are above');
}
const failed = results.filter((result) => result.code !== 0);
console.log(
  `${count} shards in ${Math.round((Date.now() - started) / 1000)} s` +
    (failed.length
      ? `; failed: ${failed.map((result) => result.shard).join(', ')}`
      : '; all green'),
);
process.exit(failed.length ? 1 : 0);
