#!/usr/bin/env node
// Runs axe-core against a rendered page and lists the accessibility violations.
//
//   sc-axe <url> [--viewport 1440x900] [--tags wcag2a,wcag2aa,wcag21a,wcag21aa,wcag22aa]
//          [--color-scheme dark|light] [--locale de-CH] [--wait-for <selector>] [--out report.json]
//          [--fail-on minor|moderate|serious|critical|none]
//
// Exit code 1 when a violation reaches the --fail-on impact (default serious), 2 on usage errors.
// An automatic run finds only part of the problems; keyboard and screen reader checks stay manual.
import { writeFileSync } from 'node:fs';
import { parseArgs } from 'node:util';
import { AxeBuilder } from '@axe-core/playwright';
import { contextOptions, launch, parseViewport } from './browser.mjs';

const { values, positionals } = parseArgs({
  allowPositionals: true,
  options: {
    viewport: { type: 'string', default: '1440x900' },
    tags: { type: 'string', default: 'wcag2a,wcag2aa,wcag21a,wcag21aa,wcag22aa' },
    'color-scheme': { type: 'string', default: 'dark' },
    locale: { type: 'string', default: 'de-CH' },
    'wait-for': { type: 'string' },
    out: { type: 'string' },
    'fail-on': { type: 'string', default: 'serious' },
  },
});

const url = positionals[0];
const impacts = ['minor', 'moderate', 'serious', 'critical'];
if (!url || (values['fail-on'] !== 'none' && !impacts.includes(values['fail-on']))) {
  console.error('usage: sc-axe <url> [--viewport 1440x900] [--fail-on serious] [--out report.json]');
  process.exit(2);
}

const browser = await launch();
let results;
try {
  const context = await browser.newContext(
    contextOptions(parseViewport(values.viewport), {
      colorScheme: values['color-scheme'],
      locale: values.locale,
      scale: 1,
    }),
  );
  const page = await context.newPage();
  await page.goto(url, { waitUntil: 'networkidle' });
  if (values['wait-for']) await page.waitForSelector(values['wait-for']);
  results = await new AxeBuilder({ page }).withTags(values.tags.split(',')).analyze();
} finally {
  await browser.close();
}

if (values.out) writeFileSync(values.out, JSON.stringify(results, null, 2));
for (const violation of results.violations) {
  console.log(
    `${(violation.impact ?? 'unknown').padEnd(8)} ${violation.id} (${violation.nodes.length}): ${violation.help}`,
  );
  for (const node of violation.nodes.slice(0, 5)) console.log(`         ${node.target.join(' ')}`);
}
console.log(
  `${results.violations.length} violations, ${results.passes.length} rules passed, ` +
    `${results.incomplete.length} need a manual look`,
);

const limit = impacts.indexOf(values['fail-on']);
const failed =
  limit >= 0 && results.violations.some((violation) => impacts.indexOf(violation.impact) >= limit);
process.exit(failed ? 1 : 0);
