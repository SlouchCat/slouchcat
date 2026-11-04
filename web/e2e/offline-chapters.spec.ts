import { CSRF } from './catalog';
import { expect, test } from './fixtures';
import { expectFocusOn, expectNoFocusTrap, expectVisibleFocus } from './keyboard';
import {
  createDownload,
  downloadRow,
  offlinePlayer,
  prepareOffline,
  videoTime,
  waitForComplete,
} from './offline';
import { arrowTo, CHAPTERS_TITLE, itemIdByTitle, resetProgress } from './playback';

// Concept 13, 11.10: a downloaded film offers its chapters offline, from the manifest (contract
// 1.10.0: the chapter marks of the file, as playback offers them online). «Hoch» copies the
// H.264 video of the source, «Niedrig» encodes it: both keep the source timeline, so a chapter
// jump lands at the chapter's start in either.

const TITLES = ['Kapitel 1: Anfang', 'Kapitel 2: Mitte', 'Kapitel 3: Ende'];

for (const quality of ['high', 'low'] as const) {
  test(`offline chapters (${quality}): ↓ to the tab «Kapitel», Enter, the arrows pick, Enter jumps`, async ({
    page,
    context,
    owner,
  }) => {
    test.setTimeout(240_000);
    const itemId = await itemIdByTitle(owner, CHAPTERS_TITLE);
    await resetProgress(owner, itemId, 10_000);

    await page.goto('/downloads');
    const [created] = await createDownload(page, itemId, { quality });
    const [download] = await waitForComplete(page, 1, (entry) => entry.id === created.id);
    const chapters = download.manifest!.chapters ?? [];
    expect(chapters.map((chapter) => chapter.title)).toEqual(TITLES);
    // The same marks as online.
    const started = await owner.post(`/api/v1/items/${itemId}/play`, {
      headers: CSRF,
      data: { capabilities: { video: ['h264'], audio: ['aac'] } },
    });
    expect(started.status(), await started.text()).toBe(200);
    const online = (await started.json()) as { chapters: unknown[]; playbackSessionId: string };
    expect(chapters).toEqual(online.chapters);
    // The session served only the comparison: it ends here, not by timeout.
    const stop = await owner.delete(`/api/v1/playback/sessions/${online.playbackSessionId}`, {
      headers: CSRF,
    });
    expect(stop.status()).toBe(204);

    // Flight mode, then play the download from the device.
    await page.goto('/downloads');
    await prepareOffline(page);
    const row = downloadRow(page, new RegExp(CHAPTERS_TITLE));
    await expect(row.getByText('Auf diesem Gerät')).toBeVisible();
    await context.setOffline(true);
    await expect(page.getByText(/Keine Verbindung zum Server/)).toBeVisible();
    await row.getByRole('link', { name: `${CHAPTERS_TITLE} abspielen` }).focus();
    await page.keyboard.press('Enter');
    const player = offlinePlayer(page);
    await expect(player).toHaveAttribute('data-start-ms', /\d+/, { timeout: 15_000 });
    const pause = player.getByRole('button', { name: 'Pause' });
    await expectFocusOn(page, pause);
    // Paused, so the position stays where a jump put it.
    await page.keyboard.press(' ');
    await expect
      .poll(() => page.locator('video').evaluate((video: HTMLVideoElement) => video.paused))
      .toBe(true);

    // ↓ from play/pause to the tab strip, on «Kapitel»; Enter opens it.
    await page.keyboard.press('ArrowDown');
    const strip = player.getByRole('toolbar', { name: 'Mehr zum Titel' });
    const tab = strip.getByRole('button', { name: 'Kapitel' });
    await expectFocusOn(page, tab);
    await expectVisibleFocus(page);
    await page.keyboard.press('Enter');
    await expect(tab).toHaveAttribute('aria-expanded', 'true');
    const cards = player.getByRole('list', { name: 'Kapitel' });
    await expect(cards.getByRole('button')).toHaveCount(3);
    await expect(cards.getByRole('button', { name: /^Kapitel 1: Anfang/ })).toHaveAttribute(
      'aria-current',
      'true',
    );

    // ↓ into the cards, → walks them (the position stays), Enter jumps to the chapter's start.
    await page.keyboard.press('ArrowDown');
    await expectFocusOn(page, cards.getByRole('button', { name: /^Kapitel 1: Anfang/ }));
    const before = await videoTime(page);
    const third = cards.getByRole('button', { name: /^Kapitel 3: Ende/ });
    await arrowTo(page, third, 'ArrowRight');
    await expectVisibleFocus(page);
    expect(await videoTime(page)).toBe(before);
    await page.keyboard.press('Enter');
    const thirdStart = chapters[2].startMs / 1000;
    await expect.poll(() => videoTime(page)).toBeGreaterThanOrEqual(thirdStart);
    expect(await videoTime(page)).toBeLessThanOrEqual(thirdStart + 1);
    await expect(third).toHaveAttribute('aria-current', 'true');

    // ← to the second chapter, Enter.
    await page.keyboard.press('ArrowLeft');
    const second = cards.getByRole('button', { name: /^Kapitel 2: Mitte/ });
    await expectFocusOn(page, second);
    await expectVisibleFocus(page);
    await page.keyboard.press('Enter');
    const secondStart = chapters[1].startMs / 1000;
    await expect.poll(() => videoTime(page)).toBeLessThanOrEqual(secondStart + 1);
    expect(await videoTime(page)).toBeGreaterThanOrEqual(secondStart);
    await expect(second).toHaveAttribute('aria-current', 'true');

    // ↑ back to the tab; Escape closes the tab, the focus stays on it; no focus trap.
    await page.keyboard.press('ArrowUp');
    await expectFocusOn(page, tab);
    await page.keyboard.press('Escape');
    await expect(cards).toHaveCount(0);
    await expectFocusOn(page, tab);
    await expectNoFocusTrap(page);
    // Back: Escape leaves the player for the downloads.
    await page.keyboard.press('Escape');
    await expect(page).toHaveURL(/\/downloads$/);
  });
}
