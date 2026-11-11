import { mkdirSync, rmSync } from 'node:fs';
import { join } from 'node:path';
import type { APIRequestContext } from '@playwright/test';
import {
  feedEpisodes,
  feedKey,
  feedUrl,
  openPodcastRow,
  PODCAST_TITLE,
  podcastIdByFeed,
  podcastLibraryId,
  podcastRow,
  podcastToggle,
  subscribeFeed,
  unsubscribe,
} from './audio';
import { CSRF } from './catalog';
import { FEED_BASE, MEDIA_DIR } from './env';
import { expect, test } from './fixtures';
import {
  expectFocusOn,
  expectNoFocusTrap,
  expectVisibleFocus,
  resetFocus,
  tabTo,
} from './keyboard';
import { arrowTo } from './playback';

// /admin/podcasts (M6, H-2; concept 12, variant C; concept 19 part A, 12.2 variant A) by keyboard
// (rule 8): tabs «Alle» and one per podcast library, each podcast a row with its cover that folds
// out (Enter) into its feed address and actions and links to its page;
// subscribe through the dialog (a feed the server cannot fetch is reported at the URL field, an
// address without http(s) already in the browser), "Jetzt abrufen" and "Abo löschen" with its
// confirmation; visible focus throughout, no focus trap.

test('podcasts page by keyboard: subscribe, fetch now, unsubscribe; errors at the field @keyboard', async ({
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
    // The row of the list with covers (concept 19 part A, 12.2): title first, two episodes.
    await expect(row.getByRole('rowheader')).toContainText(PODCAST_TITLE);
    await expect(row.getByText('2', { exact: true })).toBeVisible();

    // The row folds out by keyboard: the whole feed address, the library, the actions.
    const detail = await openPodcastRow(page, created);
    await expect(detail.getByText(feedUrl(key), { exact: true })).toBeVisible();
    await expect(detail.getByText('Podcasts', { exact: true })).toBeVisible();

    // "Jetzt abrufen" by keyboard: the page polls every 2 s for a while. The row's link to the
    // page of the podcast comes first.
    await page.keyboard.press('Tab');
    await expectFocusOn(
      page,
      row.getByRole('link', { name: `Folgen von ${PODCAST_TITLE} ansehen` }),
    );
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

/** The setting of stored episodes the library «Podcasts» has ab Werk (O-2). */
const LINK_ONLY = { mode: 'link', scope: 'new', lastN: 10, parallel: 2 } as const;

/** Removes a folder of a library; a scan that still runs refuses it for a moment (409). */
async function removeRoot(
  owner: APIRequestContext,
  libraryId: number,
  rootId: number,
  path: string,
) {
  await expect
    .poll(
      async () =>
        (
          await owner.post(`/api/v1/admin/libraries/${libraryId}/roots/${rootId}/removal`, {
            headers: CSRF,
            data: { confirmPath: path },
          })
        ).status(),
      { timeout: 30_000 },
    )
    .toBe(200);
}

/** The downloads of a podcast as the list of the administration counts them. */
async function podcastDownloads(owner: APIRequestContext, podcastId: string) {
  const response = await owner.get('/api/v1/admin/podcasts');
  expect(response.status(), await response.text()).toBe(200);
  const podcast = (
    (await response.json()) as {
      id: string;
      storedBytes: number;
      downloads: { queued: number; running: number; failed: number };
    }[]
  ).find((entry) => entry.id === podcastId);
  return podcast
    ? { active: podcast.downloads.queued + podcast.downloads.running, ...podcast }
    : { active: 0, storedBytes: 0, downloads: { queued: 0, running: 0, failed: 0 } };
}

// Stored podcast episodes (concept 19 part A, P-12 and P-13) by keyboard: the band «Neue Folgen»
// folds out by Enter, its groups take the arrow keys and a small change applies at once; the
// setting of one podcast asks first when it loads once, «Abbrechen» keeps the old scope and
// «Laden» saves the new one. The library «Podcasts» gets a writable podcast folder for the time
// of the test, and only the test's own podcast stores, so the feed of the global setup stays a link.
test('podcasts page: band «Neue Folgen» and the confirmation by keyboard @keyboard', async ({
  page,
  owner,
}, testInfo) => {
  test.setTimeout(90_000);
  const key = feedKey(testInfo);
  const dir = join(MEDIA_DIR, '.e2e', `podcast-store-${key}`);
  mkdirSync(dir, { recursive: true });
  const libraryId = await podcastLibraryId(owner);
  const podcast = await subscribeFeed(owner, key);
  const added = await owner.post(`/api/v1/admin/libraries/${libraryId}/roots`, {
    headers: CSRF,
    data: { roots: [{ path: dir, content: 'podcast', providers: ['local'] }] },
  });
  expect(added.status(), await added.text()).toBe(201);
  const root = ((await added.json()) as { roots: { id: number; path: string }[] }).roots.find(
    (entry) => entry.path === dir,
  );
  try {
    await page.goto('/admin/podcasts');
    await expect(podcastRow(page, podcast.id)).toBeVisible();
    // Folded: the band is one line with the summary.
    const toggle = page.getByRole('button', { name: /^Neue Folgen(: Podcasts)?$/ });
    await expect(page.getByText('Nur Link · nichts wird gespeichert').first()).toBeVisible();
    await expect(toggle).toHaveAttribute('aria-expanded', 'false');
    await resetFocus(page);
    await tabTo(page, toggle);
    await page.keyboard.press('Enter');
    await expect(toggle).toHaveAttribute('aria-expanded', 'true');
    const band = page.locator(`[id="${await toggle.getAttribute('aria-controls')}"]`);
    // The folder check of the new folder passes: «Speichern» could be chosen.
    await expect(band.getByText(dir, { exact: true })).toBeVisible();
    await expect(band.getByText(/^beschreibbar/)).toBeVisible();
    const modes = band.getByRole('radiogroup', { name: 'Neue Folgen' });
    await expect(modes.getByRole('radio', { name: 'Speichern' })).toBeEnabled();

    // «Gleichzeitig laden» by arrow key: it applies at once («Gespeichert»).
    const parallel = band.getByRole('radiogroup', { name: 'Gleichzeitig laden' });
    await tabTo(page, parallel.getByRole('radio', { name: '2' }));
    await expectVisibleFocus(page);
    const saved = page.waitForResponse(
      (response) =>
        response.request().method() === 'PATCH' && /\/admin\/libraries\/\d+$/.test(response.url()),
    );
    await page.keyboard.press('ArrowRight');
    expect((await saved).status()).toBe(200);
    await expect(parallel.getByRole('radio', { name: '3' })).toBeChecked();
    await expect(band.getByText('Gespeichert.')).toBeVisible();
    await expect(page.getByText('Nur Link · nichts wird gespeichert').first()).toBeVisible();

    // The podcast's own setting: «Speichern» with the library's «Nur neue Folgen» loads nothing
    // once and applies at once.
    const detail = await openPodcastRow(page, podcast.id);
    const mode = detail.getByRole('radiogroup', { name: 'Neue Folgen dieses Podcasts' });
    await tabTo(page, mode.getByRole('radio', { name: 'Wie Bibliothek' }));
    const ownSaved = page.waitForResponse(
      (response) =>
        response.request().method() === 'PATCH' &&
        response.url().endsWith(`/admin/podcasts/${podcast.id}`),
    );
    await page.keyboard.press('ArrowRight');
    expect((await ownSaved).status()).toBe(200);
    await expect(detail.getByText('Gilt: Speichern · Nur neue Folgen')).toBeVisible();

    // «Alle Folgen» loads the two episodes once: the estimate comes first.
    const scope = detail.getByRole('radiogroup', { name: 'Umfang dieses Podcasts' });
    await tabTo(page, scope.getByRole('radio', { name: 'Wie Bibliothek' }));
    await page.keyboard.press('ArrowLeft'); // wraps to «Alle Folgen»
    await expect(scope.getByRole('radio', { name: 'Alle Folgen' })).toBeChecked();
    await expect(detail.getByText(/^Das lädt einmalig rund 2 Folgen/)).toBeVisible();
    await expectFocusOn(page, scope.getByRole('radio', { name: 'Alle Folgen' }));
    // «Abbrechen»: the old scope stays, the focus goes back to the group.
    await tabTo(page, detail.getByRole('button', { name: 'Abbrechen', exact: true }));
    await expectVisibleFocus(page);
    await page.keyboard.press('Enter');
    await expect(detail.getByText(/^Das lädt einmalig/)).toHaveCount(0);
    await expect(scope.getByRole('radio', { name: 'Wie Bibliothek' })).toBeChecked();
    await expectFocusOn(page, scope.getByRole('radio', { name: 'Alle Folgen' }));

    // Again, and «Laden»: the scope is saved and the downloads are queued.
    await page.keyboard.press('ArrowLeft');
    await page.keyboard.press('ArrowRight'); // back to «Alle Folgen»
    // Checked again means the earlier question has gone; the estimate is the new one.
    await expect(scope.getByRole('radio', { name: 'Alle Folgen' })).toBeChecked();
    await expect(detail.getByText(/^Das lädt einmalig rund 2 Folgen/)).toBeVisible();
    await tabTo(page, detail.getByRole('button', { name: 'Laden', exact: true }));
    const loaded = page.waitForResponse(
      (response) =>
        response.request().method() === 'PATCH' &&
        response.url().endsWith(`/admin/podcasts/${podcast.id}`),
    );
    await page.keyboard.press('Enter');
    const answer = await loaded;
    expect(answer.status()).toBe(200);
    expect(((await answer.json()) as { effectiveScope: string }).effectiveScope).toBe('all');
    await expect(detail.getByText('Gilt: Speichern · Alle Folgen')).toBeVisible();
    await expectFocusOn(page, scope.getByRole('radio', { name: 'Alle Folgen' }));
    await expectNoFocusTrap(page);
    // The loading begins: the planner queues the two episodes (they wait, run, fail or are there).
    await expect
      .poll(
        async () => {
          const state = await podcastDownloads(owner, podcast.id);
          return state.active + state.downloads.failed + (state.storedBytes > 0 ? 1 : 0);
        },
        { timeout: 30_000 },
      )
      .toBeGreaterThan(0);
  } finally {
    // «Nur Link» cancels what still loads, so nothing writes into the folder that goes next.
    await owner.patch(`/api/v1/admin/podcasts/${podcast.id}`, {
      headers: CSRF,
      data: { mode: 'link' },
    });
    await expect
      .poll(async () => (await podcastDownloads(owner, podcast.id)).active, { timeout: 30_000 })
      .toBe(0);
    await unsubscribe(owner, podcast.id);
    if (root) await removeRoot(owner, libraryId, root.id, dir);
    await owner.patch(`/api/v1/admin/libraries/${libraryId}`, {
      headers: CSRF,
      data: { podcastDownloads: LINK_ONLY },
    });
    rmSync(dir, { recursive: true, force: true });
  }
});
