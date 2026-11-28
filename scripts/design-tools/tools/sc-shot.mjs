#!/usr/bin/env node
// Opens a page at several viewport sizes, takes a screenshot of each and reports what a visual
// check would otherwise miss: console errors, failed or 4xx/5xx requests, images that did not
// load and horizontal overflow.
//
//   sc-shot <url> --out <dir> [--viewports 390x844,768x1024,1440x900,1920x1080] [--name page]
//           [--full-page] [--color-scheme dark|light] [--locale de-CH] [--scale 1]
//           [--wait-for <selector>] [--wait <ms>] [--no-fail]
//
// Exit code 1 when a viewport has findings (unless --no-fail), 2 on usage errors.
import { mkdirSync, writeFileSync } from 'node:fs';
import { join } from 'node:path';
import { parseArgs } from 'node:util';
import { contextOptions, launch, parseViewport } from './browser.mjs';

const { values, positionals } = parseArgs({
  allowPositionals: true,
  options: {
    out: { type: 'string' },
    viewports: { type: 'string', default: '390x844,768x1024,1440x900,1920x1080' },
    name: { type: 'string', default: 'page' },
    'full-page': { type: 'boolean', default: false },
    'color-scheme': { type: 'string', default: 'dark' },
    locale: { type: 'string', default: 'de-CH' },
    scale: { type: 'string', default: '1' },
    'wait-for': { type: 'string' },
    wait: { type: 'string', default: '0' },
    'no-fail': { type: 'boolean', default: false },
  },
});

const url = positionals[0];
if (!url || !values.out) {
  console.error('usage: sc-shot <url> --out <dir> [--viewports 390x844,1440x900] [--full-page]');
  process.exit(2);
}
mkdirSync(values.out, { recursive: true });

const browser = await launch();
const report = { url, viewports: [] };
try {
  for (const text of values.viewports.split(',')) {
    const viewport = parseViewport(text);
    const context = await browser.newContext(
      contextOptions(viewport, {
        colorScheme: values['color-scheme'],
        locale: values.locale,
        scale: Number(values.scale),
      }),
    );
    const page = await context.newPage();
    const consoleErrors = [];
    const failedRequests = [];
    page.on('console', (message) => {
      if (message.type() === 'error') consoleErrors.push(message.text());
    });
    page.on('pageerror', (error) => consoleErrors.push(String(error)));
    page.on('requestfailed', (request) =>
      failedRequests.push({ url: request.url(), error: request.failure()?.errorText ?? 'failed' }),
    );
    page.on('response', (response) => {
      if (response.status() >= 400) {
        failedRequests.push({ url: response.url(), status: response.status() });
      }
    });

    await page.goto(url, { waitUntil: 'networkidle' });
    if (values['wait-for']) await page.waitForSelector(values['wait-for']);
    if (Number(values.wait) > 0) await page.waitForTimeout(Number(values.wait));

    const layout = await page.evaluate(() => ({
      brokenImages: [...document.images]
        .filter((image) => image.complete && image.naturalWidth === 0)
        .map((image) => image.currentSrc || image.src),
      scrollWidth: document.documentElement.scrollWidth,
      clientWidth: document.documentElement.clientWidth,
    }));
    const file = join(values.out, `${values.name}-${viewport.width}x${viewport.height}.png`);
    await page.screenshot({ path: file, fullPage: values['full-page'] });
    await context.close();

    report.viewports.push({
      viewport: text.trim(),
      screenshot: file,
      consoleErrors,
      failedRequests,
      brokenImages: layout.brokenImages,
      horizontalOverflow: Math.max(0, layout.scrollWidth - layout.clientWidth),
    });
  }
} finally {
  await browser.close();
}

writeFileSync(join(values.out, `${values.name}-report.json`), JSON.stringify(report, null, 2));
let findings = 0;
for (const entry of report.viewports) {
  const count =
    entry.consoleErrors.length +
    entry.failedRequests.length +
    entry.brokenImages.length +
    (entry.horizontalOverflow > 0 ? 1 : 0);
  findings += count;
  console.log(
    `${entry.viewport}: ${entry.consoleErrors.length} console errors, ` +
      `${entry.failedRequests.length} failed requests, ${entry.brokenImages.length} broken images, ` +
      `overflow ${entry.horizontalOverflow}px -> ${entry.screenshot}`,
  );
}
process.exit(findings > 0 && !values['no-fail'] ? 1 : 0);
