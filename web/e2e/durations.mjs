// Rewrites e2e/durations.json, the measured seconds per test that shard-plan.mjs splits the suite
// by, from the reports of CI runs.
//
// Usage (from web/; `task web:e2e:durations RUNS="<id> <id>"` downloads and calls it):
//   node e2e/durations.mjs <input>...
//
// An input is a Playwright JSON report, a blob report (report-*.zip) or a folder holding blob
// reports at any depth (what `gh run download <run> -p 'e2e-blob-*'` writes). The blob reports of
// one run are merged with `playwright merge-reports`, as the gate job does. Two runs or more are
// averaged per test, which evens out a slow runner. Retries count: a test that needed one took
// the time of both on its shard.
//
// Only what the inputs measured is written. A test they do not hold (new, renamed, or not run in
// those runs) gets the project's median in the plan and nothing is lost; the next refresh measures
// it.
import { execFileSync } from 'node:child_process';
import {
  copyFileSync,
  mkdirSync,
  mkdtempSync,
  readdirSync,
  readFileSync,
  rmSync,
  statSync,
  writeFileSync,
} from 'node:fs';
import { tmpdir } from 'node:os';
import { basename, join, resolve } from 'node:path';
import { fileURLToPath } from 'node:url';
import { testsOf } from './shard-plan.mjs';

const WEB = fileURLToPath(new URL('..', import.meta.url));
const DURATIONS = fileURLToPath(new URL('./durations.json', import.meta.url));

/** Every report-*.zip below `path`. */
function blobs(path) {
  if (statSync(path).isFile()) return path.endsWith('.zip') ? [path] : [];
  return readdirSync(path).flatMap((name) => blobs(join(path, name)));
}

/** The JSON report of one input. */
function reportOf(input) {
  const path = resolve(input);
  if (statSync(path).isFile() && path.endsWith('.json'))
    return JSON.parse(readFileSync(path, 'utf8'));
  const zips = blobs(path);
  if (zips.length === 0) throw new Error(`${input}: neither a JSON report nor blob reports`);
  const dir = mkdtempSync(join(tmpdir(), 'slouchcat-durations-'));
  try {
    // merge-reports reads one folder; the download puts every shard into a folder of its own.
    const flat = join(dir, 'blobs');
    mkdirSync(flat);
    for (const zip of zips) copyFileSync(zip, join(flat, basename(zip)));
    const out = join(dir, 'report.json');
    execFileSync('pnpm', ['exec', 'playwright', 'merge-reports', '--reporter', 'json', flat], {
      cwd: WEB,
      env: { ...process.env, PLAYWRIGHT_JSON_OUTPUT_NAME: out },
      stdio: ['ignore', 'ignore', 'inherit'],
    });
    return JSON.parse(readFileSync(out, 'utf8'));
  } finally {
    rmSync(dir, { recursive: true, force: true });
  }
}

const inputs = process.argv.slice(2);
if (inputs.length === 0) {
  console.error('usage: node e2e/durations.mjs <json report | blob zip | folder of blobs>...');
  process.exit(2);
}

/** project → file → test → measured seconds of every input. */
const measured = {};
for (const input of inputs) {
  for (const test of testsOf(reportOf(input))) {
    if (test.seconds <= 0) continue; // skipped, or not run
    const files = (measured[test.project] ??= {});
    const byTest = (files[test.file] ??= {});
    (byTest[test.key] ??= []).push(test.seconds);
  }
}

const sorted = (object) =>
  Object.fromEntries(Object.entries(object).sort(([a], [b]) => a.localeCompare(b)));
const projects = {};
let count = 0;
for (const [project, files] of Object.entries(measured)) {
  projects[project] = {};
  for (const [file, byTest] of Object.entries(files)) {
    projects[project][file] = sorted(
      Object.fromEntries(
        Object.entries(byTest).map(([key, values]) => {
          count += 1;
          const mean = values.reduce((sum, value) => sum + value, 0) / values.length;
          return [key, Math.round(mean * 10) / 10];
        }),
      ),
    );
  }
  projects[project] = sorted(projects[project]);
}

writeFileSync(
  DURATIONS,
  `${JSON.stringify(
    {
      about:
        'Seconds per e2e test and project, the mean of the inputs below; written by e2e/durations.mjs, read by e2e/shard-plan.mjs.',
      inputs: inputs.map((input) => basename(resolve(input))),
      projects: sorted(projects),
    },
    null,
    2,
  )}\n`,
);
console.log(`durations.json: ${count} tests from ${inputs.length} input(s)`);
