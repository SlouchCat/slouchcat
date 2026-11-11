import { mkdirSync, rmSync } from 'node:fs';
import { join } from 'node:path';
import type { APIRequestContext } from '@playwright/test';
import {
  episodeTitle,
  feedKey,
  PODCAST_TITLE,
  podcastLibraryId,
  podcastRow,
  subscribeFeed,
  unsubscribe,
} from './audio';
import { CSRF } from './catalog';
import { MEDIA_DIR } from './env';
import { expect, test } from './fixtures';
import {
  expectFocusOn,
  expectNoFocusTrap,
  expectVisibleFocus,
  resetFocus,
  tabTo,
} from './keyboard';

// The page «Podcast» and the section «Downloads» (concept 19 part A, 12.2 and 12.3; P-12) by
// keyboard (rule 8): from the row of the list to the page of a podcast, the views of its episode
// list by arrow keys (kept in ?tab=), «Laden» and «Abbrechen» on one episode with the focus kept
// on its button, then back to the list, where the section «Downloads» shows the waiting download
// and «Abbrechen» there hands the focus to «Fortsetzen»; visible focus, no focus trap, browser
// back. The job type podcast.download is paused for the test, so the downloads wait and nothing
// is written; the library gets a writable podcast folder, which «Laden» needs.

const JOB = '/api/v1/admin/job-types/podcast.download';

/** Removes a folder of a library; a scan that still runs refuses it for a moment (409). */
async function removeRoot(
  owner: APIRequestContext,
  libraryId: number,
  rootId: number,
  path: string,
) {
  await expect
    .poll(
      async () =>
        (
          await owner.post(`/api/v1/admin/libraries/${libraryId}/roots/${rootId}/removal`, {
            headers: CSRF,
            data: { confirmPath: path },
          })
        ).status(),
      { timeout: 30_000 },
    )
    .toBe(200);
}

test('podcast page and downloads by keyboard: views, «Laden», «Abbrechen», back @keyboard', async ({
  page,
  owner,
}, testInfo) => {
  test.setTimeout(90_000);
  const key = feedKey(testInfo);
  const dir = join(MEDIA_DIR, '.e2e', `podcast-page-${key}`);
  mkdirSync(dir, { recursive: true });
  const libraryId = await podcastLibraryId(owner);
  const podcast = await subscribeFeed(owner, key);
  const added = await owner.post(`/api/v1/admin/libraries/${libraryId}/roots`, {
    headers: CSRF,
    data: { roots: [{ path: dir, content: 'podcast', providers: ['local'] }] },
  });
  expect(added.status(), await added.text()).toBe(201);
  const root = ((await added.json()) as { roots: { id: number; path: string }[] }).roots.find(
    (entry) => entry.path === dir,
  );
  expect((await owner.post(`${JOB}/pause`, { headers: CSRF })).status()).toBe(200);
  try {
    // Both pages must have rendered before going back (Firefox, see admin-libraries.spec).
    await page.goto('/admin/podcasts');
    const row = podcastRow(page, podcast.id);
    await expect(row).toBeVisible();
    // The row's link to the page of the podcast, by Tab and Enter.
    const open = row.getByRole('link', { name: `Folgen von ${PODCAST_TITLE} ansehen` });
    await resetFocus(page);
    await tabTo(page, open, 80);
    await expectVisibleFocus(page);
    await page.keyboard.press('Enter');
    await expect(page).toHaveURL(new RegExp(`/admin/podcasts/${podcast.id}$`));
    await expect(page.getByRole('heading', { level: 1, name: PODCAST_TITLE })).toBeVisible();
    const counters = page.getByRole('group', { name: 'Zahlen dieses Podcasts' });
    await expect(counters).toContainText('Folgen');
    const table = page.getByRole('table', { name: `Folgen von ${PODCAST_TITLE}` });
    // The row of episode 1, found by its row header (the date comes first in the row).
    const first = table
      .getByRole('row')
      .filter({ has: page.getByRole('rowheader', { name: new RegExp(`^${episodeTitle(1)}`) }) });
    await expect(first.getByText('Nur Link', { exact: true })).toBeVisible();

    // The views by ←/→ and Enter: «Gespeichert» is empty, «Ohne Datei» has both episodes.
    const views = page.getByRole('toolbar', { name: 'Folgen filtern' });
    await resetFocus(page);
    await tabTo(page, views.getByRole('button', { name: /^Alle/ }));
    await page.keyboard.press('ArrowRight');
    await expectFocusOn(page, views.getByRole('button', { name: /^Gespeichert/ }));
    await page.keyboard.press('Enter');
    await expect(page).toHaveURL(/\?tab=with_file$/);
    await expect(page.getByText('Keine Folge in dieser Ansicht.')).toBeVisible();
    await page.keyboard.press('ArrowRight');
    await page.keyboard.press('Enter');
    await expect(page).toHaveURL(/\?tab=without_file$/);
    await expect(table.getByRole('row')).toHaveCount(3); // header and two episodes
    await expectVisibleFocus(page);

    // «Laden» on episode 1: it waits (the type is paused); the same button now cancels.
    const load = table.getByRole('button', { name: `Laden: ${episodeTitle(1)}` });
    await tabTo(page, load);
    const queued = page.waitForResponse(
      (response) =>
        response.request().method() === 'POST' &&
        response.url().endsWith(`/admin/podcasts/${podcast.id}/download`),
    );
    await page.keyboard.press('Enter');
    expect((await queued).status()).toBe(202);
    await expect(page.getByText(`${episodeTitle(1)} wird geladen.`)).toBeVisible();
    await expect(first.getByText('Wartet', { exact: true })).toBeVisible();
    const cancel = table.getByRole('button', { name: `Abbrechen: ${episodeTitle(1)}` });
    await expectFocusOn(page, cancel);
    await expectVisibleFocus(page);
    const cancelled = page.waitForResponse(
      (response) =>
        response.request().method() === 'DELETE' &&
        response.url().includes(`/admin/podcasts/${podcast.id}/episodes/`),
    );
    await page.keyboard.press('Enter');
    expect((await cancelled).status()).toBe(204);
    await expect(first.getByText('Nur Link', { exact: true })).toBeVisible();
    await expectFocusOn(page, load);

    // Episode 2 waits for the section «Downloads» of the list. Its row is above (newest first):
    // Tab from the top, since Firefox leaves the document after the last control.
    const second = table.getByRole('button', { name: `Laden: ${episodeTitle(2)}` });
    await resetFocus(page);
    await tabTo(page, second, 80);
    await page.keyboard.press('Enter');
    await expect(
      table.getByRole('button', { name: `Abbrechen: ${episodeTitle(2)}` }),
    ).toBeFocused();
    await expectNoFocusTrap(page);

    // «Alle Podcasts» leads back to the list; the section «Downloads» holds episode 2.
    await resetFocus(page);
    await tabTo(page, page.getByRole('link', { name: 'Alle Podcasts' }), 80);
    await page.keyboard.press('Enter');
    await expect(page).toHaveURL(/\/admin\/podcasts$/);
    const downloads = page.getByRole('table', { name: 'Downloads' });
    const waiting = downloads.getByRole('row', { name: new RegExp(`^${episodeTitle(2)}`) });
    await expect(waiting).toContainText(PODCAST_TITLE);
    await expect(waiting).toContainText('angehalten');
    const resume = page.getByRole('button', { name: 'Fortsetzen' });
    await expect(resume).toBeVisible();
    await resetFocus(page);
    await tabTo(page, waiting.getByRole('button', { name: `Abbrechen: ${episodeTitle(2)}` }), 80);
    await expectVisibleFocus(page);
    await page.keyboard.press('Enter');
    await expect(
      page.getByText(`Der Download von ${episodeTitle(2)} ist abgebrochen.`),
    ).toBeVisible();
    await expect(downloads).toHaveCount(0);
    await expectFocusOn(page, resume);

    // Browser back returns to the page of the podcast, in the view it was left in.
    await page.goBack();
    await expect(page).toHaveURL(new RegExp(`/admin/podcasts/${podcast.id}\\?tab=without_file$`));
    await expect(page.getByRole('heading', { level: 1, name: PODCAST_TITLE })).toBeVisible();
  } finally {
    await owner.post(`${JOB}/resume`, { headers: CSRF });
    await owner.patch(`/api/v1/admin/podcasts/${podcast.id}`, {
      headers: CSRF,
      data: { mode: 'link' },
    });
    await unsubscribe(owner, podcast.id);
    if (root) await removeRoot(owner, libraryId, root.id, dir);
    rmSync(dir, { recursive: true, force: true });
  }
});

test('podcast page of an unknown podcast: the notice leads back to the list', async ({ page }) => {
  await page.goto('/admin/podcasts/01J0000000000000000000XXXX');
  await expect(
    page.getByRole('heading', { level: 1, name: 'Podcast nicht gefunden' }),
  ).toBeVisible();
  const back = page.getByRole('link', { name: 'Alle Podcasts' });
  await resetFocus(page);
  await tabTo(page, back, 60);
  await expectVisibleFocus(page);
  await page.keyboard.press('Enter');
  await expect(page).toHaveURL(/\/admin\/podcasts$/);
});
