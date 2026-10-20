import { feedEpisodes, feedKey, feedUrl, PODCAST_TITLE, subscribeFeed, unsubscribe } from './audio';
import { FEED_BASE } from './env';
import { expect, test } from './fixtures';
import {
  expectFocusOn,
  expectNoFocusTrap,
  expectVisibleFocus,
  resetFocus,
  tabTo,
} from './keyboard';
import { arrowTo } from './playback';

// /admin/podcasts (M6, H-2) by keyboard (rule 8): subscribe through the dialog (a feed the
// server cannot fetch is reported at the URL field, an address without http(s) already in the
// browser), "Jetzt abrufen" and "Abo löschen" with its confirmation; visible focus throughout,
// no focus trap.

test('podcasts page by keyboard: subscribe, fetch now, unsubscribe; errors at the field', async ({
  page,
  owner,
}, testInfo) => {
  test.setTimeout(90_000);
  const key = feedKey(testInfo);
  await feedEpisodes(key, 2);
  let created: string | undefined;
  try {
    await page.goto('/admin/podcasts');
    await expect(page.getByRole('heading', { level: 1, name: 'Podcasts' })).toBeVisible();
    await expect(
      page
        .getByRole('navigation', { name: 'Hauptnavigation' })
        .getByRole('link', { name: 'Podcasts' }),
    ).toHaveAttribute('aria-current', 'page');
    // The feed of the global setup is listed with its URL.
    await expect(page.getByRole('table', { name: 'Abonnierte Podcasts' })).toBeVisible();
    await expect(page.getByText(`${FEED_BASE}/feed.xml`, { exact: true })).toBeVisible();

    await resetFocus(page);
    const subscribe = page.getByRole('button', { name: 'Feed abonnieren' });
    await tabTo(page, subscribe);
    await page.keyboard.press('Enter');
    const dialog = page.getByRole('dialog', { name: 'Feed abonnieren' });
    await expect(dialog).toBeVisible();
    const url = dialog.getByRole('textbox', { name: 'Feed-Adresse' });
    await expectFocusOn(page, url);

    // No http(s): refused in the browser, nothing is sent.
    await page.keyboard.type('ftp://127.0.0.1/feed.xml');
    await page.keyboard.press('Enter');
    await expect(url).toHaveAttribute('aria-invalid', 'true');
    await expect(dialog.getByText('Gib eine Adresse mit http:// oder https:// an.')).toBeVisible();

    // A feed the server cannot fetch: the problem shows at the field.
    await url.fill(`${FEED_BASE}/f/${key}/missing.xml`);
    const failed = page.waitForResponse(
      (response) =>
        response.request().method() === 'POST' && response.url().endsWith('/admin/podcasts'),
    );
    await page.keyboard.press('Enter');
    expect((await failed).status()).toBe(422);
    await expect(url).toHaveAttribute('aria-invalid', 'true');
    await expect(
      dialog.getByText(/Der Feed ist nicht erreichbar|kein RSS-Podcast-Feed/),
    ).toBeVisible();

    // The real feed: the podcast is subscribed and listed; the focus returns to the button.
    await url.fill(feedUrl(key));
    await expectFocusOn(page, url);
    const done = page.waitForResponse(
      (response) =>
        response.request().method() === 'POST' && response.url().endsWith('/admin/podcasts'),
    );
    await page.keyboard.press('Enter');
    const answer = await done;
    expect(answer.status()).toBe(201);
    created = ((await answer.json()) as { id: string }).id;
    await expect(dialog).toHaveCount(0);
    await expect(page.getByText(`${PODCAST_TITLE} ist abonniert.`)).toBeVisible();
    await expectFocusOn(page, subscribe);
    const row = page
      .getByRole('row')
      .filter({ has: page.getByRole('rowheader').filter({ hasText: feedUrl(key) }) });
    await expect(row).toBeVisible();
    await expect(row.getByRole('cell').filter({ hasText: /^2$/ })).toBeVisible();

    // "Jetzt abrufen" by keyboard.
    const fetchButton = row.getByRole('button', { name: `${PODCAST_TITLE} jetzt abrufen` });
    await tabTo(page, fetchButton, 60);
    await page.keyboard.press('Enter');
    await expect(page.getByText(`${PODCAST_TITLE} wird abgerufen.`)).toBeVisible();
    await expectVisibleFocus(page);

    // "Abo löschen": Tab to it, confirm in the dialog; the row goes, the focus to the heading.
    await page.keyboard.press('Tab');
    await expectFocusOn(page, row.getByRole('button', { name: `Abo ${PODCAST_TITLE} löschen` }));
    await page.keyboard.press('Enter');
    const confirm = page.getByRole('alertdialog', { name: `Abo ${PODCAST_TITLE} löschen?` });
    await expect(confirm).toBeVisible();
    await expectFocusOn(page, confirm.getByRole('button', { name: 'Abo löschen' }));
    await page.keyboard.press('Enter');
    await expect(row).toHaveCount(0);
    created = undefined;
    await expect(page.getByText(`Das Abo ${PODCAST_TITLE} ist gelöscht.`)).toBeVisible();
    await expect(page.getByRole('heading', { level: 1, name: 'Podcasts' })).toBeVisible();
    await expectNoFocusTrap(page);
  } finally {
    if (created) await unsubscribe(owner, created);
  }
});

test('podcasts table: arrows move between the rows; browser back leaves the page', async ({
  page,
  owner,
}, testInfo) => {
  const key = feedKey(testInfo);
  const second = await subscribeFeed(owner, key);
  try {
    // Both pages must have rendered before going back (Firefox, see admin-libraries.spec).
    await page.goto('/');
    await expect(page.getByRole('heading', { level: 2, name: 'Neu hinzugefügt' })).toBeVisible();
    await page.goto('/admin/podcasts');
    const table = page.getByRole('table', { name: 'Abonnierte Podcasts' });
    const row = (url: string) =>
      table.getByRole('row').filter({ has: page.getByRole('rowheader').filter({ hasText: url }) });
    const fetchOf = (url: string) =>
      row(url).getByRole('button', { name: `${PODCAST_TITLE} jetzt abrufen` });
    await expect(row(feedUrl(key))).toBeVisible();
    // ↓/↑ move between the rows like in the other admin tables (DataTable); the newest
    // subscription is listed first, the global setup's feed below it.
    await fetchOf(feedUrl(key)).focus();
    await arrowTo(page, fetchOf(`${FEED_BASE}/feed.xml`), 'ArrowDown');
    await expectVisibleFocus(page);
    await arrowTo(page, fetchOf(feedUrl(key)), 'ArrowUp');
    await expectVisibleFocus(page);
    await page.goBack();
    await expect(page).toHaveURL(/\/$/);
  } finally {
    await unsubscribe(owner, second.id);
  }
});

test('libraries page: the audio libraries show kind and default rating', async ({ page }) => {
  await page.goto('/admin/libraries');
  const books = page.getByRole('article', { name: 'Hörbücher' });
  await expect(books.getByText('Hörbücher', { exact: true })).toHaveCount(2);
  await expect(books.getByText('Standard: FSK 0')).toBeVisible();
  const podcasts = page.getByRole('article', { name: 'Podcasts' });
  await expect(podcasts.getByText('Standard: FSK 0')).toBeVisible();
  await expect(
    podcasts.getByText('Kein Ordner: Die Titel kommen aus abonnierten Feeds.'),
  ).toBeVisible();
  // "Standardstufe ändern" opens the dialog with the current rating; Escape leaves it unchanged.
  await books.getByRole('button', { name: 'Standardstufe von Hörbücher ändern' }).focus();
  await page.keyboard.press('Enter');
  const dialog = page.getByRole('dialog', { name: 'Standardstufe von Hörbücher' });
  await expect(dialog.getByRole('radio', { name: 'FSK 0' })).toBeChecked();
  await expectVisibleFocus(page);
  await page.keyboard.press('Escape');
  await expect(dialog).toHaveCount(0);
  await expectFocusOn(
    page,
    books.getByRole('button', { name: 'Standardstufe von Hörbücher ändern' }),
  );
});
