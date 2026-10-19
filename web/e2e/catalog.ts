import { expect, type Page } from '@playwright/test';

/** Title of the generated H.264/AAC MP4 (testdata/media/Movies), parsed from its file name. */
export const H264_TITLE = 'Slouchcat Test H264';
/** 90-s HEVC 10-bit 720p MKV: transcoded in Chromium and Firefox (M1b HLS specs). */
export const HEVC_LONG_TITLE = 'Slouchcat Test HEVC Long';
/** 90-s H.264/AAC 720p MKV with irregular keyframes: remuxed (M1b HLS specs). */
export const H264_MKV_LONG_TITLE = 'Slouchcat Test H264 MKV Long';

// M3 test media (testdata/generate-media.sh). The e2e server has no TMDB key: every title keeps
// the name parsed from its file, stays unmatched and unrated, and has no cast and no trailer.
/** Series library: Season 01 with S01E01, S01E02, S01E03-E04 (one file) and Specials S00E01. */
export const SHOW_TITLE = 'Slouchcat Test Show';
/** Film with Trailers/, Featurettes/, Deleted Scenes/, Behind The Scenes/ and a -trailer file. */
export const EXTRAS_TITLE = 'Slouchcat Test Extras';
/** The five extras of EXTRAS_TITLE (titles from the file names). */
export const EXTRA_TITLES = [
  'Slouchcat Test Extras Trailer',
  'Making Of',
  'Scene 1',
  'On Set',
  'Slouchcat Test Extras (2024)',
] as const;
/** A text file with a video extension: ffprobe fails, the file is unplayable (B-9). */
export const BROKEN_TITLE = 'Slouchcat Test Broken';
/** A byte copy of the H264 film in a folder of its own: a duplicate by file hash (B-9). */
export const H264_COPY_TITLE = 'Slouchcat Test H264 Copy';
/** A film without year and without rating (B-9 "ohne Freigabe"). */
export const UNRATED_TITLE = 'Slouchcat Test Unrated';

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

/** The item id at the end of a detail path (`/items/<id>`). */
export const idOf = (path: string): string => decodeURIComponent(path.split('/').pop() ?? '');
