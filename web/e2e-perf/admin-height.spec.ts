import { expect, test, type Locator, type Page } from '@playwright/test';

// Concept 12, section 3: the height from the top of the page to the first work row (table, list or
// form) on three administration pages, at a window width of 1440 px. Run with perf.config.ts; the
// spec prints one line per page and checks nothing beyond the page having loaded. Measured before
// the rebuild (K12-2a) and again with the compact density (K12-8); goal: at most half.

const VIEWPORT = { width: 1440, height: 900 };

interface Target {
  name: string;
  path: string;
  /** The first work row: its top edge is the measured height. */
  workRow: (page: Page) => Locator;
}

/**
 * The first work row carries `data-perf="first-work-row"` in the page (K12-8): the rebuilt pages
 * (concept 12) have no stable structure for a locator of their own, and the marker keeps the meaning
 * of the baseline in docs/perf/verwaltungsseiten.md.
 */
const firstWorkRow = (page: Page) => page.locator('main [data-perf="first-work-row"]');

const targets: Target[] = [
  {
    // The list "Läuft gerade" (the table of the running runs, or its empty line) is the page's
    // first list, in its fold-out group below the counters; the job types follow below it.
    name: 'Hintergrundjobs',
    path: '/admin/jobs',
    workRow: firstWorkRow,
  },
  {
    // The first form: the row of the TMDB key in "Schlüssel der Anbieter".
    name: 'Integrationen',
    path: '/admin/integrations',
    workRow: firstWorkRow,
  },
  {
    // The table of the accounts, from its header row (tab "Konten").
    name: 'Konten',
    path: '/admin/accounts',
    workRow: firstWorkRow,
  },
];

test.use({ viewport: VIEWPORT });

for (const target of targets) {
  test(`height to the first work row: ${target.name}`, async ({ page }) => {
    await page.goto(target.path);
    await expect(page.getByRole('heading', { level: 1, name: target.name })).toBeVisible();
    const row = target.workRow(page);
    await expect(row).toBeVisible();
    // Fonts and late data (counters, notices) settle before the measurement.
    await page.evaluate(() => document.fonts.ready);
    await page.waitForLoadState('networkidle');
    const top = await row.evaluate((element) =>
      Math.round(element.getBoundingClientRect().top + window.scrollY),
    );
    console.log(
      `${target.name} (${target.path}) at ${VIEWPORT.width} px: ${top} px to the first work row`,
    );
  });
}
