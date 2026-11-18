import type { Page, Route } from '@playwright/test';
import { expect, test } from './fixtures';
import { expectFocusOn, expectNoFocusTrap, resetFocus, tabTo } from './keyboard';
import { closeWishes, openWishes, type WishesSetup } from './wishes';

// Concept 25 stage 4 (E-7, M-03, M-08): the download progress of wishes in procurement for
// viewers, in «Meine Wünsche» and on the wish card of the preview page, by keyboard (rule 8). The
// progress comes from Radarr and Sonarr through the server; the e2e run has no mocks of them on
// this branch yet, so the answers of listWishes and getExternalTitle are fed here (as the
// notifications and start page specs feed theirs). The page, its rights and Seerr are the real
// ones (openWishes).

// The fed answers must reach the page every time: no service worker answering from its cache.
test.use({ serviceWorkers: 'block' });

const readAt = new Date().toISOString();
const counts = { open: 5, pending: 0, processing: 5, partial: 0, available: 0, closed: 0 };

/** A wish in procurement as listWishes answers it. */
const procuring = (
  id: number,
  tmdbId: number,
  title: string,
  progress: Record<string, unknown> | undefined,
  extra: Record<string, unknown> = {},
) => ({
  id,
  media: 'movie',
  tmdbId,
  title,
  state: 'processing',
  origin: 'slouchcat',
  role: 'owner',
  seasons: [],
  canWithdraw: false,
  canRequestAgain: false,
  createdAt: readAt,
  updatedAt: readAt,
  events: [{ at: readAt, kind: 'created' }],
  ...(progress ? { progress } : {}),
  ...extra,
});

const WISHES = [
  procuring(9701, 9004, 'Sternenfeld', { stage: 'search', readAt }),
  procuring(9702, 9005, 'Rotes Ufer', { stage: 'download', percent: 45, etaSeconds: 720, readAt }),
  procuring(9703, 9006, 'Winterhaus', { stage: 'import', readAt }),
  procuring(
    9704,
    9100,
    'Nebelhafen',
    { stage: 'stalled', episodes: { done: 3, total: 8 }, readAt },
    { media: 'show', seasons: [{ number: 2, state: 'processing' }] },
  ),
  procuring(9705, 9003, 'Glasfabrik', undefined),
];

/** listWishes answers the wishes above (its count read too). */
async function feedWishes(page: Page) {
  await page.route(/\/api\/v1\/wishes(\?.*)?$/, async (route: Route) => {
    if (route.request().method() !== 'GET') return route.fallback();
    await route.fulfill({ json: { items: WISHES, total: WISHES.length, counts } });
  });
}

const row = (page: Page, title: string) => page.getByRole('article', { name: new RegExp(title) });

test('«Meine Wünsche»: the four stages under «In Beschaffung», nothing without progress; no focus stop @keyboard', async ({
  page,
  owner,
}) => {
  let setup: WishesSetup | undefined;
  try {
    setup = await openWishes(owner);
    await feedWishes(page);
    await page.goto('/wishes');
    await expect(page.getByRole('heading', { level: 1, name: 'Meine Wünsche' })).toBeVisible();
    await expect(row(page, 'Sternenfeld').getByText('Wird gesucht')).toBeVisible();
    const download = row(page, 'Rotes Ufer');
    const line = 'Wird heruntergeladen · 45 % · noch 12 min';
    await expect(download.getByText(line)).toBeVisible();
    await expect(download.getByRole('progressbar', { name: line })).toHaveAttribute(
      'aria-valuenow',
      '45',
    );
    await expect(row(page, 'Winterhaus').getByText('Wird eingelesen')).toBeVisible();
    await expect(
      row(page, 'Nebelhafen').getByText('Download verzögert · 3 von 8 Folgen da'),
    ).toBeVisible();
    await expect(row(page, 'Glasfabrik').getByText(/^(Wird |Download)/)).toHaveCount(0);
    // No service, no file: only the server's words.
    await expect(page.locator('#content')).not.toContainText(/Radarr|Sonarr|\{\{/);

    // Tab goes from «Zum Titel» of the row straight to its history: the progress is no stop.
    await resetFocus(page);
    const open = download.getByRole('link', { name: 'Zum Titel' });
    await tabTo(page, open, 60);
    await page.keyboard.press('Tab');
    await expectFocusOn(page, download.getByRole('button', { name: /^Verlauf: / }));
    await expect(download.locator('[role="progressbar"][tabindex]')).toHaveCount(0);
    await expectNoFocusTrap(page, 80);
  } finally {
    await page.unrouteAll({ behavior: 'ignoreErrors' });
    await closeWishes(owner, setup);
  }
});

test('the preview page: the wish card with its progress @keyboard', async ({ page, owner }) => {
  let setup: WishesSetup | undefined;
  try {
    setup = await openWishes(owner);
    await feedWishes(page);
    await page.route('**/api/v1/titles/movie/9005', async (route: Route) => {
      const response = await route.fetch();
      const body = (await response.json()) as Record<string, unknown>;
      await route.fulfill({
        response,
        json: {
          ...body,
          canRequest: false,
          request: { canRequest: false, reason: 'already_requested', autoApprove: false },
          wish: {
            id: 9702,
            state: 'processing',
            progress: { stage: 'download', percent: 60, etaSeconds: 300, readAt },
          },
        },
      });
    });
    await page.goto('/titles/movie/9005');
    await expect(page.getByRole('heading', { level: 1, name: 'Rotes Ufer' })).toBeVisible();
    const card = page.getByRole('region', { name: 'Dein Wunsch' });
    await expect(card.getByText('In Beschaffung', { exact: true })).toBeVisible();
    const line = 'Wird heruntergeladen · 60 % · noch 5 min';
    await expect(card.getByText(line)).toBeVisible();
    await expect(card.getByRole('progressbar', { name: line })).toBeVisible();
    await expect(page.locator('#content')).not.toContainText('{{');
    // Tab reaches the card's link; the progress on the way is no stop.
    await expect(page.getByRole('link', { name: 'In Beschaffung' })).toBeVisible();
    await resetFocus(page);
    await tabTo(page, card.getByRole('link', { name: 'Zu meinen Wünschen' }), 80);
    await expect(card.locator('[role="progressbar"][tabindex]')).toHaveCount(0);
    await expectNoFocusTrap(page, 60);
  } finally {
    await page.unrouteAll({ behavior: 'ignoreErrors' });
    await closeWishes(owner, setup);
  }
});
