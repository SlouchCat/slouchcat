import {
  feedEpisodes,
  feedKey,
  feedUrl,
  openPodcastRow,
  PODCAST_TITLE,
  podcastIdByFeed,
  podcastRow,
  podcastToggle,
  subscribeFeed,
  unsubscribe,
} from './audio';
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

// /admin/podcasts (M6, H-2; concept 12, variant C) by keyboard (rule 8): tabs «Alle» and one per
// podcast library, each podcast a row that folds out (Enter) into its feed address and actions;
// subscribe through the dialog (a feed the server cannot fetch is reported at the URL field, an
// address without http(s) already in the browser), "Jetzt abrufen" and "Abo löschen" with its
// confirmation; visible focus throughout, no focus trap.

test('podcasts page by keyboard: subscribe, fetch now, unsubscribe; errors at the field', async ({
  page,
  owner,
}, testInfo) => {
  test.setTimeout(90_000);
  const key = feedKey(testInfo);
  await feedEpisodes(key, 2);
  const setupId = await podcastIdByFeed(owner, `${FEED_BASE}/feed.xml`);
  let created: string | undefined;
  try {
    await page.goto('/admin/podcasts');
    const heading = page.getByRole('heading', { level: 1, name: 'Podcasts' });
    await expect(heading).toBeVisible();
    await expect(
      page.getByRole('navigation', { name: 'Verwaltung' }).getByRole('link', { name: 'Podcasts' }),
    ).toHaveAttribute('aria-current', 'page');
    await expect(
      page
        .getByRole('tablist', { name: 'Podcasts nach Bibliothek' })
        .getByRole('tab', { selected: true }),
    ).toHaveAccessibleName(/^Alle \d+ Podcasts?$/);
    // The feed of the global setup is listed; its fold-out shows the URL.
    await expect(page.getByRole('table', { name: 'Abonnierte Podcasts' })).toBeVisible();
    await resetFocus(page);
    const setupDetail = await openPodcastRow(page, setupId);
    await expect(setupDetail.getByText(`${FEED_BASE}/feed.xml`, { exact: true })).toBeVisible();
    await expectVisibleFocus(page);
    await page.keyboard.press('Enter');
    await expect(podcastToggle(page, setupId)).toHaveAttribute('aria-expanded', 'false');

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
    const row = podcastRow(page, created);
    await expect(row.getByRole('rowheader')).toHaveText(PODCAST_TITLE);
    await expect(row.getByRole('cell').filter({ hasText: /^2$/ })).toBeVisible();

    // The row folds out by keyboard: the whole feed address, the library, the actions.
    const detail = await openPodcastRow(page, created);
    await expect(detail.getByText(feedUrl(key), { exact: true })).toBeVisible();
    await expect(detail.getByText('Podcasts', { exact: true })).toBeVisible();

    // "Jetzt abrufen" by keyboard: the page polls every 2 s for a while.
    await page.keyboard.press('Tab');
    await expectFocusOn(
      page,
      detail.getByRole('button', { name: `${PODCAST_TITLE} jetzt abrufen` }),
    );
    await page.keyboard.press('Enter');
    await expect(page.getByText(`${PODCAST_TITLE} wird abgerufen.`)).toBeVisible();
    await expect(page.getByText('Live, alle 2 Sekunden')).toBeVisible();
    await expectVisibleFocus(page);

    // "Abo löschen": Tab to it, confirm in the dialog; the row goes, the focus to the heading.
    await page.keyboard.press('Tab');
    await expectFocusOn(page, detail.getByRole('button', { name: `Abo ${PODCAST_TITLE} löschen` }));
    await page.keyboard.press('Enter');
    const confirm = page.getByRole('alertdialog', { name: `Abo ${PODCAST_TITLE} löschen?` });
    await expect(confirm).toBeVisible();
    // An open confirmation holds the polling.
    await expect(page.getByText('Live, alle 2 Sekunden')).toHaveCount(0);
    await expectFocusOn(page, confirm.getByRole('button', { name: 'Abo löschen' }));
    await page.keyboard.press('Enter');
    await expect(row).toHaveCount(0);
    created = undefined;
    await expect(page.getByText(`Das Abo ${PODCAST_TITLE} ist gelöscht.`)).toBeVisible();
    await expect(heading).toBeFocused();
    await expectNoFocusTrap(page);
  } finally {
    if (created) await unsubscribe(owner, created);
  }
});

test('podcasts page: library tabs and rows by arrow keys; browser back leaves the page', async ({
  page,
  owner,
}, testInfo) => {
  const key = feedKey(testInfo);
  const second = await subscribeFeed(owner, key);
  const setupId = await podcastIdByFeed(owner, `${FEED_BASE}/feed.xml`);
  try {
    // Both pages must have rendered before going back (Firefox, see admin-libraries.spec).
    await page.goto('/');
    await expect(page.getByRole('heading', { level: 2, name: 'Neu hinzugefügt' })).toBeVisible();
    await page.goto('/admin/podcasts');
    await expect(podcastRow(page, second.id)).toBeVisible();

    // ←/→ move between the tabs and select them; the tab lives in the address (replaced).
    const tabs = page.getByRole('tablist', { name: 'Podcasts nach Bibliothek' });
    const all = tabs.getByRole('tab', { name: /^Alle/ });
    await resetFocus(page);
    await tabTo(page, all);
    await page.keyboard.press('ArrowRight');
    const library = tabs.getByRole('tab', { selected: true });
    await expect(library).not.toHaveAccessibleName(/^Alle/);
    await expect(library).toBeFocused();
    await expectVisibleFocus(page);
    await expect(page).toHaveURL(/\/admin\/podcasts\?tab=library-\d+$/);
    await expect(page.getByRole('table', { name: /^Abonnierte Podcasts in / })).toBeVisible();
    await page.keyboard.press('ArrowLeft');
    await expect(all).toHaveAttribute('aria-selected', 'true');
    await expectFocusOn(page, all);
    await expect(page).toHaveURL(/\/admin\/podcasts$/);

    // ↓/↑ move between the rows like in the other admin tables (DataTable); the newest
    // subscription is listed first, the global setup's feed below it.
    await tabTo(page, podcastToggle(page, second.id), 60);
    await arrowTo(page, podcastToggle(page, setupId), 'ArrowDown');
    await expectVisibleFocus(page);
    await arrowTo(page, podcastToggle(page, second.id), 'ArrowUp');
    await expectVisibleFocus(page);
    await page.goBack();
    await expect(page).toHaveURL(/\/$/);
  } finally {
    await unsubscribe(owner, second.id);
  }
});
