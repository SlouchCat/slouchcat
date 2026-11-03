// Splits the Playwright suite into shards of about equal duration (ADR 0013, addendum of
// 3 November 2026) and writes the test list of one shard for `playwright test --test-list`.
//
// Playwright 1.64 shards by count, not by time: `--shard=i/n` cuts the ordered list of test
// groups into n runs of equal test numbers (PWTEST_SHARD_WEIGHTS only changes the share of each
// shard, still counted in tests). One spec of one long test (offline-watch-limit, 67 s) weighs as
// much as one spec of 21 short ones, and with Chromium and Firefox in separate halves of the list
// the longest shard ran 507 s next to a shortest of 353 s (run 41707865418). This script plans by
// measured time instead:
//
//  1. `playwright test --list --reporter=json` names every test that would run, with the config's
//     projects and their grep (Firefox runs a subset), without starting a server.
//  2. A group is one spec file in one project -- what Playwright keeps together with
//     fullyParallel off -- and weighs the sum of its tests' durations from durations.json (seconds
//     per test, per project; tags such as @keyboard are left out of the key). A test without a
//     measured duration (a new one) counts with the project's median, so it is planned, never lost.
//  3. Longest group first onto the shard with the least time so far (LPT), deterministic for
//     equal inputs: every shard computes the same plan.
//
// Usage (from web/):
//   node e2e/shard-plan.mjs --shard 2/7 --out <file>   test list of shard 2 of 7
//   node e2e/shard-plan.mjs --shards 7                 the plan: tests and seconds per shard
//   node e2e/shard-plan.mjs --check --shards 7         self-test, see check() below
//
// Refresh durations.json after a CI run with e2e/durations.mjs (task web:e2e:durations).
import { execFileSync } from 'node:child_process';
import { mkdtempSync, readFileSync, rmSync, writeFileSync } from 'node:fs';
import { tmpdir } from 'node:os';
import { join } from 'node:path';
import { fileURLToPath, pathToFileURL } from 'node:url';

const WEB = fileURLToPath(new URL('..', import.meta.url));
const DURATIONS = fileURLToPath(new URL('./durations.json', import.meta.url));

/** Seconds a shard pays per spec file on top of its tests (a fresh page and context per file). */
const PER_GROUP_SECONDS = 0.5;

/** A test's key in durations.json: describe titles and title, tags stripped. */
export function testKey(titlePath) {
  return titlePath
    .map((title) => title.replace(/(^|\s)@[\w-]+/g, '').trim())
    .filter(Boolean)
    .join(' › ');
}

/**
 * Every test of a Playwright JSON report (of a run or of `--list`): project, spec file (relative
 * to testDir), key and the time its results took, retries included.
 */
export function testsOf(report) {
  const tests = [];
  const walk = (suite, file, titles) => {
    const here = suite.file ?? file;
    // The file suite is titled with the file name; describe blocks add their titles.
    const path = suite.title && suite.title !== here ? [...titles, suite.title] : titles;
    for (const spec of suite.specs ?? []) {
      for (const test of spec.tests ?? []) {
        tests.push({
          project: test.projectName,
          file: spec.file ?? here,
          key: testKey([...path, spec.title]),
          seconds: (test.results ?? []).reduce((sum, result) => sum + result.duration, 0) / 1000,
        });
      }
    }
    for (const child of suite.suites ?? []) walk(child, here, path);
  };
  for (const suite of report.suites ?? []) walk(suite, suite.file, []);
  return tests;
}

/** The tests the config would run now, from `playwright test --list --reporter=json`. */
export function listTests(extraArgs = []) {
  const dir = mkdtempSync(join(tmpdir(), 'slouchcat-shard-plan-'));
  const out = join(dir, 'list.json');
  try {
    execFileSync(
      'pnpm',
      ['exec', 'playwright', 'test', '--list', '--reporter=json', ...extraArgs],
      {
        cwd: WEB,
        env: { ...process.env, PLAYWRIGHT_JSON_OUTPUT_NAME: out },
        stdio: ['ignore', 'ignore', 'inherit'],
      },
    );
    return testsOf(JSON.parse(readFileSync(out, 'utf8')));
  } finally {
    rmSync(dir, { recursive: true, force: true });
  }
}

const median = (values) => {
  if (values.length === 0) return 10;
  const sorted = values.toSorted((a, b) => a - b);
  return sorted[Math.floor(sorted.length / 2)];
};

/** Groups (project + file) with their estimated seconds, longest first. */
export function groupsOf(tests, durations) {
  const medians = Object.fromEntries(
    Object.entries(durations.projects ?? {}).map(([project, files]) => [
      project,
      median(Object.values(files).flatMap((byTest) => Object.values(byTest))),
    ]),
  );
  const groups = new Map();
  for (const test of tests) {
    const id = `${test.project}|${test.file}`;
    const known = durations.projects?.[test.project]?.[test.file]?.[test.key];
    const seconds = known ?? medians[test.project] ?? 10;
    const group = groups.get(id) ?? {
      project: test.project,
      file: test.file,
      tests: 0,
      unknown: 0,
      seconds: PER_GROUP_SECONDS,
    };
    group.tests += 1;
    group.unknown += known === undefined ? 1 : 0;
    group.seconds += seconds;
    groups.set(id, group);
  }
  return [...groups.values()].sort(
    (a, b) =>
      b.seconds - a.seconds || a.project.localeCompare(b.project) || a.file.localeCompare(b.file),
  );
}

/** Longest group first onto the lightest shard; ties go to the lower shard number. */
export function plan(groups, count) {
  const shards = Array.from({ length: count }, (_, index) => ({
    index: index + 1,
    seconds: 0,
    tests: 0,
    groups: [],
  }));
  for (const group of groups) {
    const lightest = shards.reduce((best, shard) => (shard.seconds < best.seconds ? shard : best));
    lightest.groups.push(group);
    lightest.seconds += group.seconds;
    lightest.tests += group.tests;
  }
  return shards;
}

/** `--test-list` lines of a shard: `[project] › file`, the whole file of that project. */
export const testList = (shard) =>
  `${shard.groups.map((group) => `[${group.project}] › ${group.file}`).join('\n')}\n`;

export function readDurations() {
  return JSON.parse(readFileSync(DURATIONS, 'utf8'));
}

/**
 * Self-test: the shards together hold every test exactly once. Checked twice -- on the plan
 * (every group in exactly one shard) and on what Playwright itself makes of the written test
 * lists (`--list --test-list` per shard), so a list line Playwright does not match, a project
 * grep the plan did not know of or a spec in no list fails here instead of silently not running.
 */
function check(count) {
  const tests = listTests();
  const shards = plan(groupsOf(tests, readDurations()), count);
  const id = (test) => `${test.project}|${test.file}|${test.key}`;
  const all = tests.map(id).sort();
  const problems = [];
  if (new Set(all).size !== all.length) problems.push('two tests share one key');

  const planned = shards.flatMap((shard) => shard.groups.map((g) => `${g.project}|${g.file}`));
  if (new Set(planned).size !== planned.length) problems.push('a group sits in two shards');
  const groups = new Set(tests.map((test) => `${test.project}|${test.file}`));
  for (const group of groups) if (!planned.includes(group)) problems.push(`unplanned: ${group}`);

  const dir = mkdtempSync(join(tmpdir(), 'slouchcat-shard-check-'));
  const seen = [];
  try {
    for (const shard of shards) {
      const file = join(dir, `shard-${shard.index}.txt`);
      writeFileSync(file, testList(shard));
      const listed = listTests(['--test-list', file]).map(id);
      if (listed.length !== shard.tests) {
        problems.push(
          `shard ${shard.index}: planned ${shard.tests} tests, Playwright lists ${listed.length}`,
        );
      }
      seen.push(...listed);
    }
  } finally {
    rmSync(dir, { recursive: true, force: true });
  }
  seen.sort();
  if (seen.length !== all.length || seen.some((key, index) => key !== all[index])) {
    const missing = all.filter((key) => !seen.includes(key));
    const extra = seen.filter((key, index) => seen.indexOf(key) !== index || !all.includes(key));
    problems.push(
      `union of the shards != all tests (missing ${missing.length}, extra ${extra.length})`,
    );
    for (const key of [...missing, ...extra].slice(0, 10)) problems.push(`  ${key}`);
  }
  report(shards);
  if (problems.length > 0) {
    console.error(`shard-plan check FAILED:\n${problems.join('\n')}`);
    process.exit(1);
  }
  console.log(`shard-plan check: ${all.length} tests, each in exactly one of ${count} shards`);
}

function report(shards) {
  for (const shard of shards) {
    const unknown = shard.groups.reduce((sum, group) => sum + group.unknown, 0);
    console.log(
      `shard ${shard.index}: ${shard.tests} tests in ${shard.groups.length} groups, ` +
        `${Math.round(shard.seconds)} s${unknown ? ` (${unknown} without a measured duration)` : ''}`,
    );
  }
}

function main(argv) {
  const value = (name) => {
    const index = argv.indexOf(name);
    return index >= 0 ? argv[index + 1] : undefined;
  };
  const shardArg = value('--shard');
  if (argv.includes('--check')) {
    check(Number(value('--shards') ?? 0) || fail('--check needs --shards <n>'));
    return;
  }
  if (shardArg) {
    const [current, total] = shardArg.split('/').map(Number);
    if (!(current >= 1 && total >= current)) fail(`--shard wants i/n, got ${shardArg}`);
    const out = value('--out') ?? fail('--shard needs --out <file>');
    const shards = plan(groupsOf(listTests(), readDurations()), total);
    const shard = shards[current - 1];
    writeFileSync(out, testList(shard));
    console.log(
      `shard ${current}/${total}: ${shard.tests} tests in ${shard.groups.length} spec files, ` +
        `about ${Math.round(shard.seconds)} s by durations.json`,
    );
    return;
  }
  const count = Number(value('--shards') ?? 0) || fail('usage: see the head of shard-plan.mjs');
  report(plan(groupsOf(listTests(), readDurations()), count));
}

function fail(message) {
  console.error(message);
  process.exit(2);
}

if (process.argv[1] && import.meta.url === pathToFileURL(process.argv[1]).href) {
  main(process.argv.slice(2));
}
