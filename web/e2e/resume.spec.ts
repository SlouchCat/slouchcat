import { readFileSync } from 'node:fs';
import { expect, test, type Page } from '@playwright/test';
import { H264_TITLE, itemPath } from './catalog';
import { STORAGE_STATE } from './env';

/** Matrix W-5: on another device the title resumes within ±10 s of the reported position. */
const RESUME_TOLERANCE_S = 10;
/** Played before pausing: well below 90 % (completion) of the 10 s test clip. */
const PLAY_TO_S = 3;

const de = JSON.parse(readFileSync(new URL('../src/locales/de.json', import.meta.url), 'utf8')) as {
  item: { resume: string };
};

/** Resume label as the detail page renders it (`formatClock`: `hh:mm:ss`, whole seconds). */
function resumeLabel(positionMs: number): string {
  const total = Math.floor(positionMs / 1000);
  const pad = (value: number) => String(value).padStart(2, '0');
  const clock = `${pad(Math.floor(total / 3600))}:${pad(Math.floor((total % 3600) / 60))}:${pad(total % 60)}`;
  return de.item.resume.replace('{{time}}', clock);
}

const progressUrl = /\/api\/v1\/items\/[^/]+\/progress$/;
const videoTime = (page: Page) =>
  page.locator('video').evaluate((video: HTMLVideoElement) => video.currentTime);

interface Progress {
  positionMs: number;
  completed: boolean;
}

test('progress is saved per profile and resumes on another device (W-5)', async ({
  page,
  browser,
  baseURL,
  locale,
}) => {
  const detail = await itemPath(page, H264_TITLE);
  const itemId = detail.split('/').pop() ?? '';

  // Device A: play from the start for a few seconds, then pause; pausing reports the position.
  await page.goto(detail);
  await expect(page.getByRole('heading', { level: 1, name: H264_TITLE })).toBeVisible();
  await page.keyboard.press('ArrowDown');
  await page.keyboard.press('Enter');
  const player = page.getByRole('main', { name: 'Videoplayer' });
  await expect(player).toHaveAttribute('data-start-ms', /\d+/);
  // Earlier specs may have left a resume position; go back to 0 so the clip never nears its end.
  await page.keyboard.press('ArrowLeft');
  await expect.poll(() => videoTime(page)).toBeLessThan(PLAY_TO_S);
  await expect.poll(() => videoTime(page), { timeout: 15_000 }).toBeGreaterThanOrEqual(PLAY_TO_S);

  const paused = page.waitForResponse(
    (response) =>
      response.request().method() === 'POST' &&
      progressUrl.test(response.url()) &&
      (response.request().postDataJSON() as { state: string }).state === 'paused',
  );
  await page.keyboard.press('Space');
  const response = await paused;
  expect(response.status()).toBe(200);
  const reportedMs = (response.request().postDataJSON() as { positionMs: number }).positionMs;
  const merged = (await response.json()) as Progress;
  expect(merged.positionMs).toBe(reportedMs);
  expect(merged.completed).toBe(false);
  expect(reportedMs).toBeGreaterThanOrEqual(PLAY_TO_S * 1000);

  await page.locator('body').focus();
  await page.keyboard.press('Escape');
  await expect(page).toHaveURL(new RegExp(`${detail}$`));
  await expect(page.getByRole('link', { name: resumeLabel(reportedMs) })).toBeVisible();

  // Device B: a second browser context with a session of the same profile.
  const deviceB = await browser.newContext({ baseURL, locale, storageState: STORAGE_STATE });
  try {
    // Record the position of the first `playing` event before the app can seek further.
    await deviceB.addInitScript(() => {
      document.addEventListener(
        'playing',
        (event) => {
          const store = window as Window & { firstPlayingS?: number };
          store.firstPlayingS ??= (event.target as HTMLVideoElement).currentTime;
        },
        true,
      );
    });
    const pageB = await deviceB.newPage();

    const item = (await (await pageB.request.get(`/api/v1/items/${itemId}`)).json()) as {
      progress?: Progress;
    };
    expect(item.progress?.positionMs).toBe(reportedMs);
    expect(item.progress?.completed).toBe(false);

    await pageB.goto(detail);
    const resume = pageB.getByRole('link', { name: resumeLabel(reportedMs) });
    await expect(resume).toBeVisible();
    // The position is stored on the server, not in the page: it survives a reload.
    await pageB.reload();
    await expect(resume).toBeVisible();

    await pageB.keyboard.press('ArrowDown');
    await expect(resume).toBeFocused();
    await pageB.keyboard.press('Enter');
    await expect(pageB).toHaveURL(/\/play\//);
    await expect(pageB.getByRole('main', { name: 'Videoplayer' })).toHaveAttribute(
      'data-start-ms',
      /\d+/,
    );
    const startS = await pageB.evaluate(
      () => (window as Window & { firstPlayingS?: number }).firstPlayingS,
    );
    expect(startS).toBeDefined();
    const reportedS = reportedMs / 1000;
    expect(Math.abs((startS ?? 0) - reportedS)).toBeLessThanOrEqual(RESUME_TOLERANCE_S);
    // The clip is only 10 s long: also make sure B resumed instead of starting over.
    expect(startS ?? 0).toBeGreaterThan(reportedS - 1);

    await pageB.keyboard.press('Space');
    await expect
      .poll(() => pageB.locator('video').evaluate((v: HTMLVideoElement) => v.paused))
      .toBe(true);
  } finally {
    await deviceB.close();
  }
});
