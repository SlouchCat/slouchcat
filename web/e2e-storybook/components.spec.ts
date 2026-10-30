import { readdirSync, readFileSync } from 'node:fs';
import { expect, test, type Page } from '@playwright/test';

// One screenshot per story of web/src/ui (A-2), compared with the committed baseline in
// __screenshots__/ (maxDiffPixelRatio 0.001, storybook.config.ts). A story tagged `press:<keys>`
// (keys joined by '+') is shot after those key presses, for focus states: a real key press is the
// only way to get :focus-visible exactly as a viewer sees it.

interface IndexEntry {
  id: string;
  type: 'story' | 'docs';
  importPath: string;
  tags?: string[];
}

const index = JSON.parse(
  readFileSync(new URL('../storybook-static/index.json', import.meta.url), 'utf8'),
) as { entries: Record<string, IndexEntry> };

const stories = Object.values(index.entries).filter((entry) => entry.type === 'story');

const keysOf = (entry: IndexEntry) =>
  (entry.tags ?? [])
    .filter((tag) => tag.startsWith('press:'))
    .flatMap((tag) => tag.slice('press:'.length).split('+'));

/** Waits until the story rendered, its images decoded and the fonts are ready. */
async function storyReady(page: Page) {
  // A story that is nothing but an open modal (SideNavDrawer) mounts into react-aria's portal on
  // <body>, so #storybook-root stays empty; the open dialog is then the sign that it rendered.
  await page.waitForFunction(() => {
    const root = document.querySelector('#storybook-root');
    return (
      (root !== null && root.childElementCount > 0) ||
      document.querySelector('[role="dialog"]') !== null
    );
  });
  await page.waitForLoadState('networkidle');
  await page.evaluate(async () => {
    await document.fonts.ready;
    await Promise.all(
      [...document.images]
        .filter((image) => !image.complete)
        .map((image) => image.decode().catch(() => undefined)),
    );
  });
}

// A-2: every component of web/src/ui has a story file next to it.
test('every component in src/ui has stories', () => {
  const components = readdirSync(new URL('../src/ui/', import.meta.url)).filter(
    (file) => file.endsWith('.tsx') && !/\.(stories|test)\.tsx$/.test(file),
  );
  const storyFiles = new Set(stories.map((entry) => entry.importPath.split('/').pop()));
  const missing = components.filter(
    (file) => !storyFiles.has(file.replace(/\.tsx$/, '.stories.tsx')),
  );
  expect(missing).toEqual([]);
});

for (const story of stories) {
  test(story.id, async ({ page }) => {
    await page.goto(`/iframe.html?id=${story.id}&viewMode=story`);
    await storyReady(page);
    for (const key of keysOf(story)) await page.keyboard.press(key);
    await expect(page).toHaveScreenshot(`${story.id}.png`);
  });
}
