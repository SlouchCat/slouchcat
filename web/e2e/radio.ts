import type { APIRequestContext, Page } from '@playwright/test';
import { CSRF } from './catalog';
import { expect } from './fixtures';

// The app-level radio player in the e2e specs (radio-player, volume): the test stations of
// radio-server.mjs, the mini player and the listen sessions.

/** Relay start (ffmpeg, the first segments) plus the first title poll (10 s). */
export const LIVE_TIMEOUT_MS = 30_000;

/** Ends every listen session the administration sees (the fixture knows only playback sessions). */
export async function stopRadio(owner: APIRequestContext): Promise<void> {
  const response = await owner.get('/api/v1/admin/radio/relays');
  if (!response.ok()) return;
  const { relays } = (await response.json()) as { relays: { listeners: { listenId: string }[] }[] };
  for (const listener of relays.flatMap((relay) => relay.listeners)) {
    await owner.delete(`/api/v1/admin/radio/listeners/${listener.listenId}`, { headers: CSRF });
  }
}

/** No stored filter: /radio opens as it does for a new profile. */
export async function resetRadioPreferences(owner: APIRequestContext): Promise<void> {
  const response = await owner.put('/api/v1/radio/preferences', {
    headers: CSRF,
    data: { defaultCountry: 'CH', lastFilter: {}, lastSort: 'popular' },
  });
  expect(response.status(), await response.text()).toBe(200);
}

export const miniPlayer = (page: Page) => page.getByTestId('radio-mini-player');
export const miniOpen = (page: Page) => page.getByTestId('radio-mini-open');
export const audio = (page: Page) => page.getByTestId('radio-audio');

/** Opens /radio on `station` and plays it from its tile. */
export async function playStation(page: Page, station: string): Promise<void> {
  await page.goto(`/radio?q=${encodeURIComponent(station)}`);
  const tile = page
    .getByRole('list', { name: 'Treffer' })
    .getByRole('button', { name: `${station} abspielen` });
  await tile.click();
  await expect(miniPlayer(page)).toBeVisible();
  await expect(miniOpen(page)).toHaveAccessibleName(`Jetzt läuft öffnen: ${station}`);
  await expect(miniPlayer(page)).toHaveAttribute('data-state', 'live', {
    timeout: LIVE_TIMEOUT_MS,
  });
}
