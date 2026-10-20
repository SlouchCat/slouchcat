import type { Page } from '@playwright/test';
import { createProfile } from './accounts';
import { CSRF } from './catalog';
import { expect, test } from './fixtures';
import { expectNoFocusTrap, expectVisibleFocus, resetFocus, tabTo } from './keyboard';
import {
  DOWNLOAD_SHOW_TITLE,
  downloadRow,
  freshDevice,
  removeDeviceDownloads,
  storedDownloads,
  syncNow,
  waitForComplete,
} from './offline';
import { episodeIds, itemIdByTitle } from './playback';

// O-3 and AN-13: the auto-download rule of a profile on this device. With "next episode" on,
// watching episode n of a series makes the server create episode n+1 at the next sync (auto),
// the device fetches it; a watched auto-download no longer counts towards the maximum.

/** Reports an episode as watched to the end (the server marks it completed). */
async function watched(page: Page, episodeId: string): Promise<void> {
  const response = await page.request.post(`/api/v1/items/${episodeId}/progress`, {
    headers: CSRF,
    data: {
      positionMs: 7900,
      durationMs: 8000,
      playedMs: 7900,
      seq: 1,
      state: 'ended',
      clientTime: new Date().toISOString(),
    },
  });
  expect(response.ok(), await response.text()).toBe(true);
}

test('episode n watched → the next sync downloads n+1 by itself (O-3)', async ({
  browser,
  baseURL,
  extraHTTPHeaders,
  owner,
}) => {
  test.setTimeout(360_000);
  const profile = await createProfile(owner, { name: 'E2E Auto', maxRating: 18 });
  const seriesId = await itemIdByTitle(owner, DOWNLOAD_SHOW_TITLE);
  const episodes = await episodeIds(owner, seriesId);
  const { context, page } = await freshDevice(browser, {
    baseURL,
    extraHTTPHeaders,
    profileId: profile.id,
  });
  try {
    // The rule by keyboard: Space toggles the checkboxes, Tab moves, Enter saves.
    await page.goto('/downloads');
    const rules = page.getByRole('region', { name: 'Automatische Downloads' });
    const enabled = rules.getByRole('checkbox', {
      name: 'Automatische Downloads auf diesem Gerät',
    });
    await expect(enabled).not.toBeChecked();
    await expect(
      rules.getByRole('checkbox', { name: 'Nächste Folge begonnener Serien' }),
    ).toBeChecked();
    await expect(rules.getByRole('checkbox', { name: 'Nur im WLAN' })).toBeChecked();
    await resetFocus(page);
    await tabTo(page, enabled, 40);
    await page.keyboard.press('Space');
    await expect(enabled).toBeChecked();
    const count = rules.getByRole('textbox', { name: 'Höchstens so viele ungesehene Folgen' });
    await tabTo(page, count, 5);
    await page.keyboard.press('ControlOrMeta+A');
    await page.keyboard.type('1');
    const quality = rules.getByRole('radiogroup', { name: 'Qualität' });
    await tabTo(page, quality.getByRole('radio', { checked: true }), 3);
    await page.keyboard.press('ArrowRight');
    await expect(quality.getByRole('radio', { name: 'Niedrig' })).toBeChecked();
    await expectVisibleFocus(page);
    await tabTo(page, rules.getByRole('button', { name: 'Regel speichern' }), 3);
    await page.keyboard.press('Enter');
    await expect(rules.getByText(/Gespeichert/)).toBeVisible();
    const stored = (await (await page.request.get('/api/v1/offline/rules')).json()) as {
      enabled: boolean;
      maxCount: number;
      quality: string;
    };
    expect(stored).toMatchObject({ enabled: true, maxCount: 1, quality: 'low' });
    await expectNoFocusTrap(page);

    // Episode 1 watched: the next sync creates episode 2 as an auto-download.
    await watched(page, episodes[0]);
    await syncNow(page);
    const [second] = await waitForComplete(
      page,
      1,
      (download) => download.auto && download.item?.id === episodes[1],
    );
    expect(second.quality).toBe('low');
    await page.reload();
    const row = downloadRow(page, /^S01E02 · /);
    await expect(row.getByText('Automatisch')).toBeVisible();
    await expect(row.getByText('Auf diesem Gerät')).toBeVisible();
    // Its play link is in the table's row navigation.
    await tabTo(page, row.getByRole('link', { name: /abspielen$/ }), 40);

    // At most one unwatched auto-download: nothing more until episode 2 is watched, then n+1.
    await syncNow(page);
    expect((await storedDownloads(page)).filter((download) => download.auto)).toHaveLength(1);
    await watched(page, episodes[1]);
    await waitForComplete(
      page,
      1,
      (download) => download.auto && download.item?.id === episodes[2],
    );
    // The watched one stays (O-6: no expiry).
    expect((await storedDownloads(page)).map((download) => download.item?.id)).toEqual(
      expect.arrayContaining([episodes[1], episodes[2]]),
    );
  } finally {
    await removeDeviceDownloads(context.request).catch(() => undefined);
    await context.close();
  }
});
