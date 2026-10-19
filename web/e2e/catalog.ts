import { expect, type Page } from '@playwright/test';

/** Title of the generated H.264/AAC MP4 (testdata/media/Movies), parsed from its file name. */
export const H264_TITLE = 'Slouchcat Test H264';
/** 90-s HEVC 10-bit 720p MKV: transcoded in Chromium and Firefox (M1b HLS specs). */
export const HEVC_LONG_TITLE = 'Slouchcat Test HEVC Long';
/** 90-s H.264/AAC 720p MKV with irregular keyframes: remuxed (M1b HLS specs). */
export const H264_MKV_LONG_TITLE = 'Slouchcat Test H264 MKV Long';

/** Header every state-changing API request needs (CSRF rule of the contract). */
export const CSRF = { 'X-Requested-With': 'Slouchcat' } as const;

/**
 * Opens the start page and returns the detail path of the first card with this title. A string
 * matches the card title exactly ("Slouchcat Test H264" is a prefix of another title).
 */
export async function itemPath(page: Page, title: string | RegExp): Promise<string> {
  await page.goto('/');
  const row = page.getByRole('region', { name: 'Neu hinzugefügt' });
  const card = (
    typeof title === 'string'
      ? row.getByRole('link').filter({ has: page.getByText(title, { exact: true }) })
      : row.getByRole('link', { name: title })
  ).first();
  await expect(card).toBeVisible();
  const href = await card.getAttribute('href');
  if (!href) throw new Error(`card ${String(title)} has no href`);
  return href;
}
