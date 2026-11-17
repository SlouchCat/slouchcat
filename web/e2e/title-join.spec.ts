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
import { useFreshProfile } from './playback';
import { closeWishes, openWishes, seerrRequests, type WishesSetup } from './wishes';

// Concept 25 stage 4 (F-7 «Mitwünschen», functional concept 4.3, M-04): a film another profile of
// the household already wished. The preview page shows that wish's state, never who wished it, and
// «Benachrichtige mich» joins it without a call to Seerr; with «Bereits angeforderte Titel: Nur
// Zustand zeigen» only the state shows. By keyboard (rule 8), against the real server: the owner's
// primary profile wishes the film, a fresh profile of the same account opens it.

const FILM = '/titles/movie/9003';
const content = (page: Page) => page.locator('#content');

/** The owner's primary profile wishes the film. */
async function wishAsOwner(owner: APIRequestContext) {
  const made = await owner.post('/api/v1/wishes', {
    headers: CSRF,
    data: { media: 'movie', tmdbId: 9003, source: 'detail' },
  });
  expect(made.status(), await made.text()).toBe(201);
}

/**
 * The page profile's own wishes go first: a joined wish whose first wisher withdraws passes to the
 * joiner and would stay open (closeWishes only withdraws the owner's primary profile's wishes).
 */
async function withdrawPageWishes(page: Page) {
  const open = (await (await page.request.get('/api/v1/wishes?state=open&limit=100')).json()) as {
    items?: { id: number }[];
  };
  for (const wish of open.items ?? []) {
    await page.request.delete(`/api/v1/wishes/${wish.id}`, { headers: CSRF });
  }
}

/** The fresh profile of the page may request. */
async function allowRequests(owner: APIRequestContext, profileId: number) {
  const allowed = await owner.patch(`/api/v1/profiles/${profileId}`, {
    headers: CSRF,
    data: { canRequest: true },
  });
  expect(allowed.status(), await allowed.text()).toBe(200);
}

test('«Benachrichtige mich»: the state of the other wish, Tab reaches the button, Enter joins, the focus stays on the state @keyboard', async ({
  page,
  owner,
}) => {
  let setup: WishesSetup | undefined;
  try {
    setup = await openWishes(owner);
    await wishAsOwner(owner);
    const profile = await useFreshProfile(page, owner, 'E2E Mitwünschen');
    await allowRequests(owner, profile.id);
    const before = (await seerrRequests()).length;

    await page.goto(FILM);
    await expect(page.getByRole('heading', { level: 1, name: 'Glasfabrik' })).toBeVisible();
    // Pending in Seerr, or still in the outbox when Seerr took longer.
    await expect(
      page.getByText(/^Bereits angefordert · (Freigabe ausstehend|Wird übermittelt)$/),
    ).toBeVisible();
    await expect(page.getByRole('button', { name: 'Film anfordern' })).toHaveCount(0);
    await expect(content(page)).not.toContainText('{{');

    // Tab after the side navigation: the back arrow, then «Benachrichtige mich».
    const join = page.getByRole('button', { name: 'Benachrichtige mich' });
    await resetFocus(page);
    await tabTo(page, page.getByRole('button', { name: 'Zurück', exact: true }), 60);
    await page.keyboard.press('Tab');
    await expectFocusOn(page, join);

    // Enter joins: the button becomes the state of the joined wish and keeps the focus.
    await page.keyboard.press('Enter');
    const state = page.getByRole('link', { name: /^(Freigabe ausstehend|Wird übermittelt)$/ });
    await expect(state).toBeVisible();
    await expectFocusOn(page, state);
    await expectVisibleFocus(page);
    await expect(page.getByText('Du wirst benachrichtigt, sobald der Titel da ist.')).toBeVisible();
    const card = page.getByRole('region', { name: 'Dein Wunsch' });
    await expect(card.getByText(/du wünschst mit/)).toBeVisible();
    await expect(card.getByRole('button', { name: 'Nicht mehr mitwünschen' })).toBeVisible();
    await expect(page.getByText(/^Bereits angefordert · /)).toHaveCount(0);
    await expect(content(page)).not.toContainText('{{');
    // Joining asks Seerr nothing.
    expect((await seerrRequests()).length).toBe(before);
    await expectNoFocusTrap(page, 60);
  } finally {
    await withdrawPageWishes(page);
    await closeWishes(owner, setup);
  }
});

test('«Nur Zustand zeigen»: the state of the other wish without a button', async ({
  page,
  owner,
}) => {
  let setup: WishesSetup | undefined;
  try {
    setup = await openWishes(owner);
    const status = await owner.put('/api/v1/admin/wishes/settings', {
      headers: CSRF,
      data: { alreadyRequested: 'status' },
    });
    expect(status.ok(), await status.text()).toBe(true);
    await wishAsOwner(owner);
    const profile = await useFreshProfile(page, owner, 'E2E Mitwünschen');
    await allowRequests(owner, profile.id);

    await page.goto(FILM);
    await expect(page.getByRole('heading', { level: 1, name: 'Glasfabrik' })).toBeVisible();
    await expect(
      page.getByText(/^Bereits angefordert · (Freigabe ausstehend|Wird übermittelt)$/),
    ).toBeVisible();
    await expect(page.getByText(/Sobald er abspielbar ist, findest du ihn/)).toBeVisible();
    await expect(page.getByRole('button', { name: 'Benachrichtige mich' })).toHaveCount(0);
    await expect(page.getByRole('button', { name: 'Film anfordern' })).toHaveCount(0);
    await expect(content(page)).not.toContainText('{{');
  } finally {
    await withdrawPageWishes(page);
    await closeWishes(owner, setup);
  }
});
