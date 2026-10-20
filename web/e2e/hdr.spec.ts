import { expect, test } from './fixtures';
import { expectFocusOn } from './keyboard';
import { HDR_TITLE, itemIdByTitle, player, resetProgress, startOf, videoState } from './playback';

// Plan 10.3 row 3: the generated HEVC HDR10 4K clip plays in Chromium and Firefox as a transcode
// with tonemapping, at most 1080p (AN-17), and starts within 5 s (W-3).

const START_BUDGET_MS = 5000;

test('HDR10 4K: transcode with tonemapping, at most 1080p, starts within 5 s', async ({
  page,
  owner,
  browserName,
}) => {
  test.setTimeout(90_000);
  const itemId = await itemIdByTitle(owner, HDR_TITLE);
  await resetProgress(page.request, itemId, 10_000);
  const start = await startOf(page, () => page.goto(`/play/${itemId}`));
  expect(start.status).toBe(200);
  // The embedded browsers have no HDR display.
  expect(start.request.capabilities.hdr).toBe(false);
  const { body } = start;
  expect(body.mode).toBe('hls');
  expect(body.transcoding?.mode).toBe('transcode');
  expect(body.transcoding?.reason).toBe('hdr');
  expect(body.transcoding?.tonemap).toBe(true);
  expect(body.transcoding?.height).toBeDefined();
  expect(body.transcoding!.height).toBeLessThanOrEqual(1080);

  await expect(player(page)).toHaveAttribute('data-engine', 'hlsjs');
  await expect(player(page)).toHaveAttribute('data-start-ms', /\d+/, { timeout: 30_000 });
  const startMs = Number(await player(page).getAttribute('data-start-ms'));
  expect(startMs).toBeLessThan(START_BUDGET_MS);
  await expectFocusOn(page, page.getByRole('button', { name: 'Pause' }));
  await expect.poll(async () => (await videoState(page)).time).toBeGreaterThan(0.5);

  // Stream-Info names the tonemapping.
  await page.keyboard.press('i');
  const info = page.getByRole('region', { name: 'Stream-Info' });
  await expect(info.getByText('HDR-Video', { exact: true })).toBeVisible();
  await expect(
    info.getByText('Tonemapping', { exact: true }).locator('xpath=following-sibling::*[1]'),
  ).toHaveText('Ja');
  console.log(
    JSON.stringify({
      measurement: 'hdr-start',
      browser: browserName,
      backend: body.transcoding?.backend ?? null,
      hardware: body.transcoding?.hardware ?? null,
      startMs,
    }),
  );
});
