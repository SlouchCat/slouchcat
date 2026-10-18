import { expect, type Page } from '@playwright/test';

/** Title of the generated H.264/AAC MP4 (testdata/media/Movies), parsed from its file name. */
export const H264_TITLE = 'Slouchcat Test H264';

/** Opens the start page and returns the detail path of the first card whose name matches. */
export async function itemPath(page: Page, title: string | RegExp): Promise<string> {
  await page.goto('/');
  const card = page
    .getByRole('region', { name: 'Neu hinzugefügt' })
    .getByRole('link', { name: title })
    .first();
  await expect(card).toBeVisible();
  const href = await card.getAttribute('href');
  if (!href) throw new Error(`card ${String(title)} has no href`);
  return href;
}
