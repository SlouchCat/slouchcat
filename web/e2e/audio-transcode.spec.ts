import { expect, test } from './fixtures';
import {
  AC3_TITLE,
  DTS_TITLE,
  itemIdByTitle,
  player,
  resetProgress,
  startOf,
  videoState,
} from './playback';

// Plan 10.3 row 4: DTS and AC3 5.1 cannot be decoded by Chromium and Firefox; the server
// converts the audio (reason audio_codec) and the title plays.

for (const title of [DTS_TITLE, AC3_TITLE]) {
  test(`${title}: audio is converted (audio_codec) and the title plays`, async ({
    page,
    owner,
  }) => {
    test.setTimeout(60_000);
    const itemId = await itemIdByTitle(owner, title);
    await resetProgress(page.request, itemId, 10_000);
    const start = await startOf(page, () => page.goto(`/play/${itemId}`));
    expect(start.status).toBe(200);
    // The browser does not claim the codec (the check is the device's, the decision the server's).
    expect(start.request.capabilities.audio).not.toContain('dts');
    expect(start.body.mode).toBe('hls');
    expect(start.body.transcoding?.reason).toBe('audio_codec');

    await expect(player(page)).toHaveAttribute('data-start-ms', /\d+/, { timeout: 20_000 });
    await expect.poll(async () => (await videoState(page)).time).toBeGreaterThan(0.5);
    expect((await videoState(page)).paused).toBe(false);
    await page.keyboard.press('i');
    const info = page.getByRole('region', { name: 'Stream-Info' });
    await expect(
      info.getByText('Grund', { exact: true }).locator('xpath=following-sibling::*[1]'),
    ).toHaveText('Audiocodec');
  });
}
