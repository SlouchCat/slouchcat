import { acceptInvitation, accountRow, createInvitation, openAccounts } from './accounts';
import { GUEST_PASSWORD } from './env';
import { expect, test } from './fixtures';
import { expectFocusOn, expectNoFocusTrap, tabTo } from './keyboard';
import { createDownload, openOfflineLibrary, storedDownloads, waitForComplete } from './offline';
import { H264_TITLE, itemIdByTitle } from './playback';

// K-7 and AN-4 for downloads: the administration removes an account; its device learns that at
// the next contact with the server (its session is gone, so it asks with the device cookie,
// checkOfflineRevocations) and deletes the download with its files. The removal dialog says that a
// device which never comes online again keeps its downloads.

const GUEST = { email: 'gast-offline-e2e@slouchcat.test', password: GUEST_PASSWORD };

test('a removed account’s download disappears from the device at the next contact', async ({
  page,
  browser,
  baseURL,
  extraHTTPHeaders,
  owner,
}) => {
  test.setTimeout(240_000);
  const invitation = await createInvitation(owner);
  const guestContext = await browser.newContext({
    baseURL,
    locale: 'de-CH',
    extraHTTPHeaders,
    storageState: { cookies: [], origins: [] },
  });
  try {
    await acceptInvitation(guestContext.request, invitation.token, {
      email: GUEST.email,
      password: GUEST.password,
      profileName: 'Gast Offline',
      language: 'de',
    });
    const guest = await guestContext.newPage();
    await guest.goto('/downloads');
    await expect(guest.getByRole('heading', { level: 1, name: 'Downloads' })).toBeVisible();
    await createDownload(guest, await itemIdByTitle(owner, H264_TITLE));
    const [download] = await waitForComplete(guest);

    // The administration removes the account; the dialog names the limit of AN-4.
    await openAccounts(page);
    const row = accountRow(page, GUEST.email);
    const remove = row.getByRole('button', { name: `${GUEST.email} entfernen` });
    await tabTo(page, remove, 80);
    await page.keyboard.press('Enter');
    const dialog = page.getByRole('alertdialog', { name: `Konto ${GUEST.email} entfernen?` });
    await expect(dialog).toContainText('Laufende Sitzungen und Wiedergaben enden sofort');
    await expect(dialog).toContainText(
      'Downloads auf den Geräten des Kontos verschwinden beim nächsten Kontakt mit dem Server. Ein Gerät, das nie mehr online kommt, behält sie.',
    );
    await expectFocusOn(page, dialog.getByRole('button', { name: 'Konto entfernen' }));
    await page.keyboard.press('Enter');
    await expect(row).toHaveCount(0);

    // The guest's device at its next contact: the session is gone, the download goes too.
    await guest.goto('/');
    await expect(guest).toHaveURL(/\/login$/);
    await expect
      .poll(async () => (await storedDownloads(guest)).length, { timeout: 30_000 })
      .toBe(0);
    const folders = await guest.evaluate(async () => {
      const root = await navigator.storage.getDirectory();
      const dir = (await root.getDirectoryHandle('downloads', { create: true })) as unknown as {
        keys(): AsyncIterable<string>;
      };
      const names: string[] = [];
      for await (const name of dir.keys()) names.push(name);
      return names;
    });
    expect(folders).not.toContain(download.id);

    // The offline library of that device is empty now.
    await openOfflineLibrary(guest);
    await expect(guest.getByText('Keine Downloads auf diesem Gerät')).toBeVisible();
    await expectNoFocusTrap(guest);
  } finally {
    await guestContext.close();
  }
});
