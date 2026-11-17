import type { APIRequestContext, Page } from '@playwright/test';
import { CSRF } from './catalog';
import { expect, test } from './fixtures';
import {
  expectFocusOn,
  expectNoFocusTrap,
  expectVisibleFocus,
  resetFocus,
  tabTo,
} from './keyboard';
import {
  closeWishes,
  openWishes,
  seerrRequests,
  setSeerrRequest,
  type WishesSetup,
} from './wishes';
import { useFreshProfile } from './playback';

// Concept 25 stage 2, 5.3 (M-08): «Meine Wünsche» and the side navigation's entry «Wünsche» by
// keyboard (rule 8). The wishes are made through the API against the Seerr mock; a declined one
// comes from Seerr's state through «Jetzt abgleichen» (syncWishesNow).

const nav = (page: Page) => page.getByRole('navigation', { name: 'Hauptnavigation' });
const row = (page: Page, title: string) => page.getByRole('article', { name: new RegExp(title) });

async function wish(api: APIRequestContext, tmdbId: number) {
  const response = await api.post('/api/v1/wishes', {
    headers: CSRF,
    data: { media: 'movie', tmdbId, source: 'detail' },
  });
  expect(response.status(), await response.text()).toBe(201);
  return (await response.json()) as { id: number; state: string };
}

test('the page by keyboard: the entry with its count, rows, history, withdraw asks, the chips filter, empty @keyboard', async ({
  page,
  owner,
}) => {
  let setup: WishesSetup | undefined;
  try {
    setup = await openWishes(owner, { limit: 5 });
    // A profile of its own that may request: no other spec's wishes in its list.
    const profile = await useFreshProfile(page, owner, 'E2E Meine Wünsche');
    const allowed = await owner.patch(`/api/v1/profiles/${profile.id}`, {
      headers: CSRF,
      data: { canRequest: true },
    });
    expect(allowed.status(), await allowed.text()).toBe(200);
    await wish(page.request, 9001);
    await wish(page.request, 9002);
    // Seerr declines the second; the sync brings the state over.
    const copper = (await seerrRequests()).find((request) => request.mediaId === 9002);
    await setSeerrRequest(copper!.id, 3);
    const sync = await owner.post('/api/v1/admin/seerr/sync', { headers: CSRF });
    expect(sync.status()).toBe(202);
    await expect
      .poll(async () => {
        const list = (await (await page.request.get('/api/v1/wishes?state=closed')).json()) as {
          items: unknown[];
        };
        return list.items.length;
      })
      .toBe(1);

    // The entry «Wünsche» after «Gesehen», with the open count, leads to «Entdecken» (concept
    // 25, 5.4); its link «Meine Wünsche» with the same count to the page.
    await page.goto('/');
    const entry = nav(page).getByRole('link', { name: 'Wünsche, 1 offener Wunsch' });
    await expect(entry).toBeVisible();
    await tabTo(page, entry, 40);
    await page.keyboard.press('Enter');
    await expect(page).toHaveURL(/\/discover$/);
    const mine = page
      .getByRole('navigation', { name: 'Bereiche unter Wünsche' })
      .getByRole('link', { name: 'Meine Wünsche (1)' });
    await tabTo(page, mine, 40);
    await page.keyboard.press('Enter');
    await expect(page).toHaveURL(/\/wishes$/);
    await expect(mine).toHaveAttribute('aria-current', 'page');
    await expect(page.getByRole('heading', { level: 1, name: 'Meine Wünsche' })).toBeVisible();
    await expect(page.getByText('1 offener Wunsch')).toBeVisible();
    await expect(page.getByText(/^Limit: Filme: \d von 5 in 7 Tagen/)).toBeVisible();
    await expect(page.locator('#content')).not.toContainText('{{');

    // Newest first: the declined film has «Erneut anfordern», the pending one «Zurückziehen».
    const declined = row(page, 'Kupfermond');
    await expect(declined.getByText('Abgelehnt', { exact: true }).first()).toBeVisible();
    await expect(
      declined.getByRole('button', { name: 'Kupfermond erneut anfordern' }),
    ).toBeVisible();
    const pending = row(page, 'Glasvogel');
    // Pending in Seerr, or still in the outbox when Seerr took longer than 3 s.
    await expect(
      pending.getByText(/^(Freigabe ausstehend|Wird übermittelt)$/).first(),
    ).toBeVisible();

    // Tab through a row: «Zum Titel», «Zurückziehen», the folded history; Enter opens it.
    await resetFocus(page);
    const open = pending.getByRole('link', { name: 'Zum Titel' });
    await tabTo(page, open, 60);
    await page.keyboard.press('Tab');
    const withdraw = pending.getByRole('button', { name: 'Glasvogel: zurückziehen' });
    await expectFocusOn(page, withdraw);
    await page.keyboard.press('Tab');
    const history = pending.getByRole('button', { name: /^Verlauf: / });
    await expectFocusOn(page, history);
    await expect(history).toHaveAttribute('aria-expanded', 'false');
    await page.keyboard.press('Enter');
    await expect(history).toHaveAttribute('aria-expanded', 'true');
    await expect(pending.getByRole('list', { name: 'Verlauf' })).toBeVisible();

    // «Zurückziehen» asks; Escape keeps the wish and returns the focus.
    await page.keyboard.press('Shift+Tab');
    await expectFocusOn(page, withdraw);
    await page.keyboard.press('Enter');
    const question = page.getByRole('alertdialog', { name: 'Wunsch zurückziehen?' });
    await expectFocusOn(page, question.getByRole('button', { name: 'Abbrechen' }));
    await page.keyboard.press('Escape');
    await expect(question).toHaveCount(0);
    await expectFocusOn(page, withdraw);

    // The chip «Offen» filters (URL), then the withdrawal empties it.
    const openChip = page.getByRole('button', { name: 'Offen (1)' });
    await resetFocus(page);
    await tabTo(page, openChip, 40);
    await page.keyboard.press('Enter');
    await expect(page).toHaveURL(/\/wishes\?state=open$/);
    await expect(row(page, 'Kupfermond')).toHaveCount(0);
    await tabTo(page, withdraw, 10);
    await page.keyboard.press('Enter');
    await question.getByRole('button', { name: 'Zurückziehen' }).press('Enter');
    await expect(page.getByText('Keine offenen Wünsche.')).toBeVisible();
    await expectVisibleFocus(page);
    await expect(nav(page).getByRole('link', { name: 'Wünsche', exact: true })).toBeVisible();
    await expectNoFocusTrap(page, 80);
  } finally {
    await closeWishes(owner, setup);
  }
});

test('a profile without the request right and without wishes has no entry; its page says it is empty', async ({
  page,
  owner,
}) => {
  let setup: WishesSetup | undefined;
  try {
    setup = await openWishes(owner);
    // A profile of its own: the other test's closed wishes stay with the main profile.
    await useFreshProfile(page, owner, 'E2E Wünsche');
    await page.goto('/wishes');
    await expect(page.getByText('Noch keine Wünsche.')).toBeVisible();
    // A new profile may not request until the main profile allows it (F-9): no entry.
    await expect(nav(page).getByRole('link', { name: 'Gesehen', exact: true })).toBeVisible();
    await expect(nav(page).getByRole('link', { name: /^Wünsche/ })).toHaveCount(0);
  } finally {
    await closeWishes(owner, setup);
  }
});
