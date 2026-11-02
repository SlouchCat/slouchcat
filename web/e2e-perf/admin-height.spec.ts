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

/** The content directly below a section heading of the jobs page ("Läuft gerade", the groups). */
const belowHeading = (page: Page, heading: string) =>
  page
    .locator('section')
    .filter({ has: page.getByRole('heading', { level: 2, name: heading, exact: true }) })
    .locator(':scope > :nth-child(2)');

const targets: Target[] = [
  {
    // The list "Läuft gerade" (a panel with the running runs, or its empty line) is the page's first
    // list; the job types follow below it.
    name: 'Hintergrundjobs',
    path: '/admin/jobs',
    workRow: (page) => belowHeading(page, 'Läuft gerade'),
  },
  {
    // The first form: the TMDB key in "Schlüssel der Anbieter".
    name: 'Integrationen',
    path: '/admin/integrations',
    workRow: (page) => page.locator('main form').first(),
  },
  {
    // The table of the accounts, from its header row.
    name: 'Konten',
    path: '/admin/accounts',
    workRow: (page) => page.getByRole('table', { name: 'Konten' }),
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
