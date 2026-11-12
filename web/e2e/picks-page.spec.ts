import type { Page } from '@playwright/test';
import { acceptInvitation, createInvitation } from './accounts';
import { EXTRAS_TITLE, H264_TITLE, itemPath } from './catalog';
import { GUEST_PASSWORD } from './env';
import { ROW_TITLES } from './discover';
import { expect, test } from './fixtures';
import { expectFocusOn, expectNoFocusTrap, resetFocus, tabTo } from './keyboard';

// Concept 18 part B (14.4, 14.9): the star «Empfehlen» of the title page puts the title at the
// front of the user picks; the page lists every pick once, newest first, without a note and
// without «Mehr laden» (16.4); the start page row links it with "Alle anzeigen ›" (concept 08).
// The `owner` fixture leaves only the baseline pick (the H264 film) and removes the one made here.
const grid = (page: Page) => page.getByRole('list', { name: ROW_TITLES.picks });
const cards = (page: Page) => grid(page).getByRole('link');

test('User picks by keyboard: the star, arrows, Enter, back; no focus trap @keyboard', async ({
  page,
  owner,
}) => {
  // The star of the title page by keyboard: Enter recommends, the hint names the retention.
  await page.goto(await itemPath(page, EXTRAS_TITLE));
  await expect(page.getByRole('heading', { level: 1, name: EXTRAS_TITLE })).toBeVisible();
  await resetFocus(page);
  const star = page.getByRole('button', { name: 'Empfehlen' });
  // The round actions in their order: «Merken», «Gesehen» (concept 20 P-5), then the star (a film
  // has no «Favorit»).
  await tabTo(page, page.getByRole('button', { name: /^(Merken|Gemerkt)$/ }), 40);
  await page.keyboard.press('Tab');
  await expectFocusOn(page, page.getByRole('button', { name: 'Gesehen', exact: true }));
  await page.keyboard.press('Tab');
  await expectFocusOn(page, star);
  await expect(star).toHaveAttribute('aria-pressed', 'false');
  await page.keyboard.press('Enter');
  await expect(
    page.getByText('Zu den User-Empfehlungen hinzugefügt. Bleibt 60 Tage.'),
  ).toBeVisible();
  const done = page.getByRole('button', { name: 'Empfohlen ✓' });
  await expect(done).toHaveAttribute('aria-pressed', 'true');
  await expectFocusOn(page, done);

  // The start page row links the page.
  await page.goto('/');
  const row = page.getByRole('region', { name: ROW_TITLES.picks });
  const all = row.getByRole('link', { name: 'Alle anzeigen ›' });
  await expect(all).toHaveAttribute('href', '/picks');
  await all.click();
  await expect(page).toHaveURL('/picks');
  // Fresh load for the keyboard part: after the pointer click Firefox draws no :focus-visible ring
  // on the first script focus, so the arrow steps below would read as rings missing.
  await page.goto('/picks');
  await expect(page.getByRole('heading', { level: 1, name: ROW_TITLES.picks })).toBeVisible();
  await expect(page.getByText('2 Titel', { exact: true })).toBeVisible();
  // Newest pick first: the new one, then the baseline pick; no note under the posters.
  await expect(cards(page)).toHaveCount(2);
  await expect(cards(page).first()).toContainText(EXTRAS_TITLE);
  await expect(cards(page).nth(1)).toContainText(H264_TITLE);
  await expect(page.getByRole('button', { name: /Mehr laden/ })).toHaveCount(0);

  // The first arrow lands on the first card; right and left move between the cards.
  await resetFocus(page);
  await page.keyboard.press('ArrowDown');
  await expectFocusOn(page, cards(page).first());
  await page.keyboard.press('ArrowRight');
  await expectFocusOn(page, cards(page).nth(1));
  await page.keyboard.press('ArrowLeft');
  await expectFocusOn(page, cards(page).first());
  await page.keyboard.press('ArrowRight');

  // Enter opens the title; the browser's back lands on the same card.
  const href = await cards(page).nth(1).getAttribute('href');
  await page.keyboard.press('Enter');
  await expect(page).toHaveURL(href ?? /\/items\//);
  await page.goBack();
  await expect(page).toHaveURL('/picks');
  // The card gets the focus by script after the navigation; Firefox draws no :focus-visible ring
  // for that until the next key, so the ring is checked after an arrow step.
  await expect(cards(page).nth(1)).toBeFocused();
  await page.keyboard.press('ArrowLeft');
  await expectFocusOn(page, cards(page).first());

  // Tab from a fresh load: header, "Zurück", the cards; then the end of the page.
  await page.reload();
  await expect(cards(page)).toHaveCount(2);
  await resetFocus(page);
  await tabTo(page, page.getByRole('button', { name: 'Zurück' }));
  await tabTo(page, cards(page).first(), 3);
  // Then the card's check mark (concept 20 P-5) and its watchlist button (concept 17 D-9).
  await page.keyboard.press('Tab');
  await expectFocusOn(
    page,
    grid(page)
      .getByRole('listitem')
      .first()
      .getByRole('button', { name: /^(Gesehen|Gehört): / }),
  );
  await page.keyboard.press('Tab');
  await expectFocusOn(
    page,
    grid(page)
      .getByRole('listitem')
      .first()
      .getByRole('button', { name: / merken$/ }),
  );
  await expectNoFocusTrap(page);

  // Escape goes back; opened directly, that is the start page.
  await resetFocus(page);
  await page.keyboard.press('Escape');
  await expect(page).toHaveURL((url) => url.pathname === '/');

  // Enter on «Empfohlen ✓» takes the own pick back (U-F7); the title leaves the list.
  await page.goto(await itemPath(page, EXTRAS_TITLE));
  await resetFocus(page);
  await tabTo(page, done, 40);
  await page.keyboard.press('Enter');
  await expect(page.getByText('Empfehlung zurückgenommen.')).toBeVisible();
  await expect(star).toHaveAttribute('aria-pressed', 'false');
  await page.goto('/picks');
  await expect(cards(page)).toHaveCount(1);
  await expect(cards(page).first()).toContainText(H264_TITLE);
  // Nothing left behind for the next spec (the `owner` fixture checks the same).
  expect(
    ((await (await owner.get('/api/v1/admin/picks')).json()) as { title: string }[]).map(
      (pick) => pick.title,
    ),
  ).not.toContain(EXTRAS_TITLE);
});

// U-F10 (plan 03 E-2): a guest account reads the user picks but has no star on the title page.
test('a guest account sees the user picks without the star', async ({
  browser,
  baseURL,
  extraHTTPHeaders,
  owner,
}) => {
  const invitation = await createInvitation(owner);
  const guestContext = await browser.newContext({
    baseURL,
    locale: 'de-CH',
    extraHTTPHeaders,
    storageState: { cookies: [], origins: [] },
  });
  try {
    await acceptInvitation(guestContext.request, invitation.token, {
      email: 'empfehlungen-gast@slouchcat.test',
      password: GUEST_PASSWORD,
      profileName: 'Gast',
      language: 'de',
    });
    const guest = await guestContext.newPage();
    await guest.goto(await itemPath(guest, H264_TITLE));
    await expect(guest.getByRole('heading', { level: 1, name: H264_TITLE })).toBeVisible();
    await expect(guest.getByRole('button', { name: /^(Merken|Gemerkt)$/ })).toBeVisible();
    await expect(guest.getByRole('button', { name: /^(Empfehlen|Empfohlen ✓)$/ })).toHaveCount(0);

    // The list itself is there for every account: the baseline pick.
    await guest.goto('/picks');
    await expect(cards(guest)).toHaveCount(1);
    await expect(cards(guest).first()).toContainText(H264_TITLE);
  } finally {
    await guestContext.close();
  }
});
