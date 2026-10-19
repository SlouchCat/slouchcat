import type { Page } from '@playwright/test';
import { CSRF, EXTRAS_TITLE, H264_TITLE, idOf, itemPath } from './catalog';
import { expect, test } from './fixtures';
import { expectFocusOn, expectVisibleFocus, resetFocus } from './keyboard';

// Trailers (B-8, W-10). The e2e server finds none (no TMDB or YouTube key), so the test sets a
// manual trailer through the administration API, as the "manueller Ersatz" of B-8 does. The
// YouTube player itself is not loaded: requests to youtube-nocookie.com get an empty page.

const YOUTUBE_ID = 'dQw4w9WgXcQ';
let detail = '';
let extras = '';

const trailerButton = (page: Page) => page.getByRole('button', { name: 'Trailer', exact: true });

test.beforeEach(async ({ page, owner }) => {
  detail ||= await itemPath(page, H264_TITLE);
  extras ||= await itemPath(page, EXTRAS_TITLE);
  const created = await owner.post(`/api/v1/admin/items/${idOf(detail)}/trailers`, {
    headers: CSRF,
    data: { lang: 'de', youtubeId: YOUTUBE_ID },
  });
  expect(created.status(), await created.text()).toBe(201);
  await page.route('**://www.youtube-nocookie.com/**', (route) =>
    route.fulfill({
      status: 200,
      contentType: 'text/html',
      body: '<!doctype html><title>yt</title>',
    }),
  );
  await page.goto('/');
  await page.goto(detail);
  await expect(page.getByRole('heading', { level: 1, name: H264_TITLE })).toBeVisible();
  await resetFocus(page);
});

test.afterEach(async ({ owner }) => {
  // Leave the title without trailers for the other specs.
  const item = (await (await owner.get(`/api/v1/admin/items/${idOf(detail)}`)).json()) as {
    trailers: { id: number }[];
  };
  for (const trailer of item.trailers) {
    await owner.delete(`/api/v1/admin/items/${idOf(detail)}/trailers/${trailer.id}`, {
      headers: CSRF,
    });
  }
});

test('arrows reach "Trailer"; Enter opens the YouTube frame; Escape closes', async ({ page }) => {
  await page.keyboard.press('ArrowDown');
  await page.keyboard.press('ArrowRight');
  await expectFocusOn(page, page.getByRole('button', { name: 'Zurück' }));
  await page.keyboard.press('ArrowRight');
  await expectFocusOn(page, trailerButton(page));

  await page.keyboard.press('Enter');
  const dialog = page.getByRole('dialog', { name: `Trailer: ${H264_TITLE}` });
  await expect(dialog).toBeVisible();
  const frame = dialog.locator('iframe');
  await expect(frame).toHaveAttribute(
    'src',
    new RegExp(`^https://www\\.youtube-nocookie\\.com/embed/${YOUTUBE_ID}`),
  );
  await expect(frame).toHaveAttribute('title', `Trailer von ${H264_TITLE} (YouTube)`);

  // Focus stays inside the dialog.
  for (let presses = 0; presses < 4; presses += 1) {
    await page.keyboard.press('Tab');
    const inside = await page.evaluate(
      () => document.activeElement?.closest('[role="dialog"]') !== null,
    );
    expect(inside).toBe(true);
  }

  await page.keyboard.press('Escape');
  await expect(dialog).toHaveCount(0);
  await expect(page).toHaveURL(new RegExp(`${detail}$`));
  await expectFocusOn(page, trailerButton(page));
  await expectVisibleFocus(page);
});

test('offline the trailer button disappears (W-10) and comes back online', async ({
  page,
  context,
}) => {
  await expect(trailerButton(page)).toBeVisible();
  await context.setOffline(true);
  await expect(trailerButton(page)).toHaveCount(0);
  await context.setOffline(false);
  await expect(trailerButton(page)).toBeVisible();
});

test('a title without trailers has no trailer button', async ({ page }) => {
  await page.goto(extras);
  await expect(page.getByRole('heading', { level: 1, name: EXTRAS_TITLE })).toBeVisible();
  await expect(page.getByRole('button', { name: 'Zurück', exact: true })).toBeVisible();
  await expect(trailerButton(page)).toHaveCount(0);
});
