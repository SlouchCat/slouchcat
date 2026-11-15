import type { Page } from '@playwright/test';
import { H264_TITLE, idOf, itemPath } from './catalog';
import { expect, test } from './fixtures';

// The administration on a phone (concept 12, S-1; designer check after K12-8): no page may be
// wider than the screen. Tables and tab lists scroll inside their own box; a visually hidden
// caption or tab count must not escape that box and widen the page (it did: 449 to 1130 px).
// The page title wraps instead of running out of the screen.

const PHONE = { width: 390, height: 844 };

const PAGES = [
  '/admin',
  '/admin/libraries',
  '/admin/podcasts',
  '/admin/maintenance',
  '/admin/metadata',
  '/admin/metadata/audiobooks',
  '/admin/radio',
  '/admin/accounts',
  '/admin/accounts?item=me#meine-profile',
  '/admin/devices',
  '/admin/streams',
  '/admin/spotlight',
  '/admin/picks',
  '/admin/collections',
  '/admin/home-rows',
  '/admin/my-media',
  '/admin/announcements',
  '/admin/jobs',
  '/admin/integrations',
  '/admin/integrations?item=keys',
  '/admin/integrations?item=webhooks',
  '/admin/integrations?item=email',
  '/admin/stats',
  '/admin/backup',
  '/admin/server',
  '/admin/notifications',
  '/settings/notifications',
  '/settings/profiles',
];

/** What a page shows while its data is on the way: a busy region or a skeleton. */
const LOADING = '#content [aria-busy="true"], #content [class*="skeleton"]';

test.use({ viewport: PHONE, hasTouch: true });

/** The page of the first podcast of the global setup (`/admin/podcasts/:id`). */
async function podcastPath(page: Page): Promise<string> {
  const response = await page.request.get('/api/v1/admin/podcasts');
  const [podcast] = (await response.json()) as { id: string }[];
  expect(podcast, 'a podcast of the global setup').toBeDefined();
  return `/admin/podcasts/${podcast.id}`;
}

for (const path of [...PAGES, 'title', 'podcast']) {
  test(`${path} is not wider than the phone`, async ({ page }) => {
    const target =
      path === 'title'
        ? `/admin/items/${idOf(await itemPath(page, H264_TITLE))}`
        : path === 'podcast'
          ? await podcastPath(page)
          : path;
    await page.goto(target);
    // Loaded: the h1 shows and nothing in the content waits for an answer any more.
    await expect(page.getByRole('heading', { level: 1 })).toBeVisible();
    await expect(page.locator(LOADING)).toHaveCount(0);
    await expect
      .poll(() => page.evaluate(() => document.documentElement.scrollWidth))
      .toBeLessThanOrEqual(PHONE.width);
  });
}
