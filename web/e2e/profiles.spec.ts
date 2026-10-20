import type { Page } from '@playwright/test';
import { createProfile, ensureEmptyLibrary, listMyProfiles, loginOwner } from './accounts';
import { choose } from './controls';
import { OWNER_PROFILE } from './env';
import { expect, test } from './fixtures';
import { expectFocusOn, expectNoFocusTrap, expectVisibleFocus, tabTo } from './keyboard';

// Profile picker (K-4), profile management (K-2) and the language switch without reload (C-3).
// Each test logs in once with a session of its own: a profile switch changes the session on the
// server, and the shared storage-state session of the other specs must stay on the primary profile.
// (Logins and PIN attempts count against the server's limit of 10 per minute, so they are few.)
test.use({ storageState: { cookies: [], origins: [] } });

const tile = (page: Page, name: string) =>
  page.getByRole('list', { name: 'Profile' }).getByRole('button', { name: new RegExp(`^${name}`) });
const primaryTile = (page: Page) => tile(page, OWNER_PROFILE);
const switchLink = (page: Page, name: string) =>
  page.getByRole('link', { name: `Profil wechseln, aktuell ${name}` });

/** Logs in; with two or more profiles the session has none yet and `/` leads to the picker. */
async function startAtPicker(page: Page) {
  await loginOwner(page.request);
  await page.goto('/');
  await expect(page).toHaveURL(/\/profiles$/);
  await expect(page.getByRole('heading', { level: 1, name: 'Wer schaut?' })).toBeVisible();
}

test.describe('picker', () => {
  test('arrow keys move between tiles, Enter picks, back returns; no focus trap', async ({
    page,
    owner,
  }) => {
    await createProfile(owner, { name: 'Zweit', accent: 'sky', language: 'de' });
    await createProfile(owner, { name: 'Dritt', accent: 'rose', language: 'de' });
    await startAtPicker(page);

    // The first tile has the focus on arrival, with a visible ring.
    await expectFocusOn(page, primaryTile(page));
    await page.keyboard.press('ArrowRight');
    await expectFocusOn(page, tile(page, 'Zweit'));
    await page.keyboard.press('ArrowRight');
    await expectFocusOn(page, tile(page, 'Dritt'));
    await page.keyboard.press('ArrowLeft');
    await expectFocusOn(page, tile(page, 'Zweit'));

    // Down to the actions, up again to a tile.
    await page.keyboard.press('ArrowDown');
    await expectFocusOn(page, page.getByRole('button', { name: 'Abmelden' }));
    await page.keyboard.press('ArrowUp');
    await expect(page.getByRole('list', { name: 'Profile' }).locator(':focus')).toHaveCount(1);

    // No profile is active yet, so nothing is marked and nothing can be managed.
    await expect(page.locator('[aria-current="true"]')).toHaveCount(0);
    await expect(page.getByRole('link', { name: 'Profile verwalten' })).toHaveCount(0);

    await tile(page, 'Zweit').focus();
    const selected = page.waitForResponse((response) => response.url().endsWith('/select'));
    await page.keyboard.press('Enter');
    expect((await selected).status()).toBe(200);
    await expect(page).toHaveURL(/\/$/);
    await expect(switchLink(page, 'Zweit')).toBeVisible();

    // Back leads to the picker, which now marks the active profile and offers no management:
    // "Zweit" is not the primary profile.
    await page.goBack();
    await expect(page).toHaveURL(/\/profiles$/);
    await expect(tile(page, 'Zweit')).toHaveAttribute('aria-current', 'true');
    await expect(page.getByRole('link', { name: 'Profile verwalten' })).toHaveCount(0);

    await expectNoFocusTrap(page);
  });

  test('the primary profile is marked and offers "Profile verwalten"', async ({ page, owner }) => {
    await createProfile(owner, { name: 'Zweit' });
    await startAtPicker(page);
    await primaryTile(page).focus();
    await page.keyboard.press('Enter');
    await expect(page).toHaveURL(/\/$/);

    await switchLink(page, OWNER_PROFILE).click();
    await expect(page).toHaveURL(/\/profiles$/);
    await expect(primaryTile(page)).toHaveAttribute('aria-current', 'true');
    await expect(primaryTile(page)).toContainText('Hauptprofil');
    const manage = page.getByRole('link', { name: 'Profile verwalten' });
    await manage.focus();
    await page.keyboard.press('Enter');
    await expect(page).toHaveURL(/\/settings\/profiles$/);
  });

  test('a deep link survives the picker', async ({ page, owner }) => {
    await createProfile(owner, { name: 'Zweit' });
    await loginOwner(page.request);
    await page.goto('/admin/libraries');
    await expect(page).toHaveURL(/\/profiles\?next=(%2F|\/)admin(%2F|\/)libraries$/);
    await primaryTile(page).focus();
    await page.keyboard.press('Enter');
    await expect(page).toHaveURL(/\/admin\/libraries$/);
  });

  test('PIN dialog: focus, Escape, wrong PIN, lock with countdown, right PIN', async ({
    page,
    owner,
  }) => {
    await createProfile(owner, { name: 'Geschuetzt', pin: '4711' });
    await createProfile(owner, { name: 'Offen', pin: '1357' });
    await startAtPicker(page);
    const locked = tile(page, 'Geschuetzt');
    await expect(locked).toContainText('Geschuetzt');

    // Enter opens the dialog; the PIN field has the focus and masks the digits.
    await locked.focus();
    await page.keyboard.press('Enter');
    const dialog = page.getByRole('dialog', { name: 'PIN für Geschuetzt' });
    await expect(dialog).toBeVisible();
    const field = dialog.getByLabel('PIN');
    await expectFocusOn(page, field);
    await expect(field).toHaveAttribute('type', 'password');
    await expect(field).toHaveAttribute('inputmode', 'numeric');

    // Escape closes it and focus returns to the tile.
    await page.keyboard.press('Escape');
    await expect(dialog).toBeHidden();
    await expectFocusOn(page, locked);

    // A wrong PIN is reported (403), the field is empty again and keeps the focus.
    await page.keyboard.press('Enter');
    await expectFocusOn(page, field);
    await page.keyboard.type('0000');
    const wrong = page.waitForResponse((response) => response.url().endsWith('/select'));
    await page.keyboard.press('Enter');
    expect((await wrong).status()).toBe(403);
    await expect(dialog.getByRole('alert')).toHaveText(
      'Die PIN stimmt nicht. Versuche es noch einmal.',
    );
    await expect(field).toHaveValue('');
    await expectFocusOn(page, field);

    // Wrong PINs add up and lock the profile for this device (AN-11): countdown, submit disabled,
    // no more attempts. The field empties when each answer has arrived.
    const status = dialog.getByRole('status');
    const answers: number[] = [];
    page.on('response', (response) => {
      if (response.url().endsWith('/select')) answers.push(response.status());
    });
    for (let attempt = 0; attempt < 3 && !(await status.isVisible()); attempt += 1) {
      await page.keyboard.type('1111');
      await page.keyboard.press('Enter');
      await expect(field).toHaveValue('');
    }
    // The wrong PIN that locks the profile already answers 429.
    expect(answers.at(-1)).toBe(429);
    await expect(status).toContainText('Zu viele Fehlversuche. Neuer Versuch in');
    await expect(status).toContainText(/0:\d\d/);
    await expect(dialog.getByRole('button', { name: 'Öffnen' })).toBeDisabled();
    await page.keyboard.type('4711');
    await page.keyboard.press('Enter');
    await expect(dialog).toBeVisible();
    await expect(page).toHaveURL(/\/profiles$/);

    await page.keyboard.press('Escape');
    await expect(dialog).toBeHidden();
    await expectFocusOn(page, locked);

    // Another profile is not locked: its right PIN opens it.
    await tile(page, 'Offen').focus();
    await page.keyboard.press('Enter');
    await expectFocusOn(
      page,
      page.getByRole('dialog', { name: 'PIN für Offen' }).getByLabel('PIN'),
    );
    await page.keyboard.type('1357');
    await page.keyboard.press('Enter');
    await expect(page).toHaveURL(/\/$/);
    await expect(switchLink(page, 'Offen')).toBeVisible();
  });

  // Arrow keys belong to the dialog: norigin must not move the focus to the tiles behind it.
  test('arrow keys inside the PIN dialog keep the focus in the dialog', async ({ page, owner }) => {
    await createProfile(owner, { name: 'Geschuetzt', pin: '4711' });
    await createProfile(owner, { name: 'Offen' });
    await startAtPicker(page);
    await tile(page, 'Geschuetzt').focus();
    await page.keyboard.press('Enter');
    const dialog = page.getByRole('dialog', { name: 'PIN für Geschuetzt' });
    await expectFocusOn(page, dialog.getByLabel('PIN'));
    await page.keyboard.press('Tab');
    await expectFocusOn(page, dialog.getByRole('button', { name: 'Abbrechen' }));
    // Not even for a moment: record every focus that lands outside the dialog.
    await page.evaluate(() => {
      const w = window as unknown as { __outside: string[] };
      w.__outside = [];
      document.addEventListener('focusin', (event) => {
        const target = event.target as HTMLElement;
        if (!target.closest('[role="dialog"]')) w.__outside.push(target.textContent ?? '');
      });
    });
    for (const key of ['ArrowRight', 'ArrowLeft', 'ArrowDown', 'ArrowUp']) {
      await page.keyboard.press(key);
      await expectFocusOn(page, dialog.getByRole('button', { name: 'Abbrechen' }));
    }
    expect(
      await page.evaluate(() => (window as unknown as { __outside: string[] }).__outside),
    ).toEqual([]);
    await page.keyboard.press('Escape');
    await expect(dialog).toBeHidden();
    await expectFocusOn(page, tile(page, 'Geschuetzt'));
    // The tiles navigate by arrow keys again once the dialog is closed.
    await page.keyboard.press('ArrowRight');
    await expect(page.getByRole('list', { name: 'Profile' }).locator(':focus')).toHaveCount(1);
    await expect(tile(page, 'Geschuetzt')).not.toBeFocused();
  });
});

// C-3: the UI language follows the profile at once; a marker on `window` proves the page was
// not reloaded.
test('switching to an English profile changes the start page without a reload and back', async ({
  page,
  owner,
}) => {
  await createProfile(owner, { name: 'English', language: 'en', accent: 'teal' });
  await startAtPicker(page);
  await primaryTile(page).click();
  await expect(page.getByRole('heading', { level: 2, name: 'Neu hinzugefügt' })).toBeVisible();
  await page.evaluate(() => {
    (window as unknown as { __noReload: boolean }).__noReload = true;
  });

  await switchLink(page, OWNER_PROFILE).click();
  await expect(page).toHaveURL(/\/profiles$/);
  await tile(page, 'English').click();
  await expect(page).toHaveURL(/\/$/);
  await expect(page.getByRole('heading', { level: 2, name: 'Recently added' })).toBeVisible();
  await expect(page.getByRole('heading', { level: 1, name: 'Home' })).toBeAttached();
  await expect(page.getByRole('link', { name: 'Switch profile, now English' })).toBeVisible();
  await expect(page.locator('html')).toHaveAttribute('lang', 'en');
  // The profile's colour: the accent follows the profile's token.
  const accent = await page.evaluate(() =>
    document.documentElement.style.getPropertyValue('--sc-accent'),
  );
  expect(accent).toBe('var(--sc-color-accent-teal-base)');

  await page.getByRole('link', { name: 'Switch profile, now English' }).click();
  await tile(page, OWNER_PROFILE).click();
  await expect(page.getByRole('heading', { level: 2, name: 'Neu hinzugefügt' })).toBeVisible();
  await expect(page.locator('html')).toHaveAttribute('lang', 'de');

  expect(
    await page.evaluate(() => (window as unknown as { __noReload?: boolean }).__noReload),
  ).toBe(true);
});

test.describe('profile management', () => {
  const article = (page: Page, name: string) => page.getByRole('article', { name });

  // One profile: the login picks it, so the session starts on the primary profile.
  test.beforeEach(async ({ page }) => {
    await loginOwner(page.request);
    await page.goto('/settings/profiles');
    await expect(page.getByRole('heading', { level: 1, name: 'Profile' })).toBeVisible();
    await expect(article(page, OWNER_PROFILE)).toBeVisible();
  });

  test('create, edit every rating, PIN set and remove, delete (K-2)', async ({ page }) => {
    // Create.
    await page.getByRole('button', { name: 'Profil hinzufügen' }).click();
    const form = page.getByRole('dialog', { name: 'Neues Profil' });
    await form.getByLabel('Name', { exact: true }).fill('Kind');
    for (const age of [0, 6, 12, 16, 18]) {
      await choose(form, 'radio', `FSK ${age}`);
      await expect(form.getByRole('radio', { name: `FSK ${age}` })).toBeChecked();
    }
    await choose(form, 'radio', 'FSK 6');
    await choose(form, 'radio', 'English');
    await choose(form, 'radio', 'Türkis');
    await form.getByRole('button', { name: 'Profil anlegen' }).click();
    await expect(form).toBeHidden();
    const kind = article(page, 'Kind');
    await expect(kind).toBeVisible();
    await expect(kind.getByText('FSK 6')).toBeVisible();
    await expect(kind.getByText('English')).toBeVisible();

    // Edit: every rating value can be chosen; the last one is saved.
    await kind.getByRole('button', { name: 'Bearbeiten' }).click();
    const edit = page.getByRole('dialog', { name: 'Profil bearbeiten' });
    await expect(edit.getByLabel('Name', { exact: true })).toHaveValue('Kind');
    await expect(edit.getByRole('radio', { name: 'FSK 6' })).toBeChecked();
    for (const age of [0, 6, 12, 16, 18, 12]) {
      await choose(edit, 'radio', `FSK ${age}`);
      await expect(edit.getByRole('radio', { name: `FSK ${age}` })).toBeChecked();
    }
    await edit.getByLabel('Name', { exact: true }).fill('Kind 2');
    await edit.getByRole('button', { name: 'Speichern' }).click();
    await expect(edit).toBeHidden();
    const renamed = article(page, 'Kind 2');
    await expect(renamed.getByText('FSK 12')).toBeVisible();

    // PIN: set (a short one is refused), shown, remove.
    await renamed.getByRole('button', { name: 'PIN setzen' }).click();
    const pin = page.getByRole('dialog', { name: 'PIN für Kind 2' });
    await pin.getByLabel('Neue PIN').fill('123');
    await pin.getByRole('button', { name: 'PIN speichern' }).click();
    await expect(pin.getByText('Die PIN besteht aus 4 bis 8 Ziffern.')).toBeVisible();
    await pin.getByLabel('Neue PIN').fill('2468');
    await pin.getByRole('button', { name: 'PIN speichern' }).click();
    await expect(pin).toBeHidden();
    await expect(renamed.getByText('PIN', { exact: true })).toBeVisible();
    await expect(renamed.getByRole('button', { name: 'PIN ändern' })).toBeVisible();
    await renamed.getByRole('button', { name: 'PIN entfernen' }).click();
    await page
      .getByRole('alertdialog', { name: 'PIN von Kind 2 entfernen?' })
      .getByRole('button', { name: 'PIN entfernen' })
      .click();
    await expect(renamed.getByText('PIN', { exact: true })).toHaveCount(0);
    await expect(renamed.getByRole('button', { name: 'PIN setzen' })).toBeVisible();

    // The primary profile has no delete button; the other one is deleted after confirmation.
    await expect(article(page, OWNER_PROFILE).getByRole('button', { name: 'Löschen' })).toHaveCount(
      0,
    );
    await renamed.getByRole('button', { name: 'Löschen' }).click();
    const confirm = page.getByRole('alertdialog', { name: 'Profil „Kind 2“ löschen?' });
    await confirm.getByRole('button', { name: 'Abbrechen' }).click();
    await expect(renamed).toBeVisible();
    await renamed.getByRole('button', { name: 'Löschen' }).click();
    await confirm.getByRole('button', { name: 'Profil löschen' }).click();
    await expect(renamed).toHaveCount(0);
    expect(await listMyProfiles(page.request)).toHaveLength(1);
  });

  test('the 11th profile is refused at the name field', async ({ page, owner }) => {
    for (let index = 2; index <= 10; index += 1) {
      await createProfile(owner, { name: `P${index}` });
    }
    await page.reload();
    await expect(article(page, 'P10')).toBeVisible();
    await page.getByRole('button', { name: 'Profil hinzufügen' }).click();
    const form = page.getByRole('dialog', { name: 'Neues Profil' });
    await form.getByLabel('Name', { exact: true }).fill('Elftes');
    await page.keyboard.press('Enter');
    await expect(form.getByText('Mehr als 10 Profile pro Konto sind nicht möglich.')).toBeVisible();
    await expect(form.getByLabel('Name', { exact: true })).toHaveAttribute('aria-invalid', 'true');
  });

  test('Tab order, radios with arrows, Enter submits, Escape closes; no focus trap', async ({
    page,
  }) => {
    await tabTo(page, page.getByRole('link', { name: 'Zugriff' }));
    await tabTo(page, page.getByRole('link', { name: 'Profile', exact: true }));
    await tabTo(page, page.getByRole('button', { name: 'Profil hinzufügen' }));

    // The same order as on the page: the add button, then the actions of each card.
    await page.keyboard.press('Tab');
    await expectFocusOn(
      page,
      article(page, OWNER_PROFILE).getByRole('button', { name: 'Bearbeiten' }),
    );
    await page.keyboard.press('Tab');
    await expectFocusOn(
      page,
      article(page, OWNER_PROFILE).getByRole('button', { name: 'Schauzeit anzeigen' }),
    );
    await page.keyboard.press('Tab');
    await expectFocusOn(
      page,
      article(page, OWNER_PROFILE).getByRole('button', { name: 'PIN setzen' }),
    );

    // Open the create dialog from the keyboard.
    await page.getByRole('button', { name: 'Profil hinzufügen' }).focus();
    await page.keyboard.press('Enter');
    const form = page.getByRole('dialog', { name: 'Neues Profil' });
    await expectFocusOn(page, form.getByLabel('Name', { exact: true }));
    await page.keyboard.type('Tastatur');

    // Rating: Tab lands on the checked radio, arrows move and select.
    await page.keyboard.press('Tab');
    await expectFocusOn(page, form.getByRole('radio', { name: 'FSK 18' }));
    await page.keyboard.press('ArrowLeft');
    await expectFocusOn(page, form.getByRole('radio', { name: 'FSK 16' }));
    await expect(form.getByRole('radio', { name: 'FSK 16' })).toBeChecked();
    await page.keyboard.press('ArrowLeft');
    await page.keyboard.press('ArrowLeft');
    await expectFocusOn(page, form.getByRole('radio', { name: 'FSK 6' }));

    // Language, accent, libraries, PIN, then the two buttons.
    await page.keyboard.press('Tab');
    await expectFocusOn(page, form.getByRole('radio', { name: 'Deutsch' }));
    await page.keyboard.press('ArrowRight');
    await expectFocusOn(page, form.getByRole('radio', { name: 'English' }));
    await page.keyboard.press('Tab');
    await expect(form.getByRole('radiogroup', { name: 'Farbe' }).locator(':focus')).toHaveCount(1);
    await expectVisibleFocus(page);
    await page.keyboard.press('ArrowRight');
    await expect(form.getByRole('radiogroup', { name: 'Farbe' }).locator(':focus')).toHaveCount(1);
    await page.keyboard.press('Tab');
    await expectFocusOn(page, form.getByRole('radio', { name: 'Alle', exact: true }));
    // The playback limits (W-8, K-5): the quality radios, then the three number fields.
    await page.keyboard.press('Tab');
    await expectFocusOn(page, form.getByRole('radio', { name: 'Ohne Limit' }));
    await page.keyboard.press('ArrowRight');
    await expectFocusOn(page, form.getByRole('radio', { name: '360p' }));
    await page.keyboard.press('ArrowLeft');
    await page.keyboard.press('Tab');
    await expectFocusOn(page, form.getByLabel('Bandbreite (kbit/s)'));
    await page.keyboard.press('Tab');
    await expectFocusOn(page, form.getByLabel('Gleichzeitige Streams'));
    await page.keyboard.press('Tab');
    await expectFocusOn(page, form.getByLabel('Schauzeit pro Tag (Minuten)'));
    await page.keyboard.press('Tab');
    await expectFocusOn(page, form.getByLabel(/^PIN/));
    await page.keyboard.press('Tab');
    await expectFocusOn(page, form.getByRole('button', { name: 'Abbrechen' }));
    await page.keyboard.press('Tab');
    await expectFocusOn(page, form.getByRole('button', { name: 'Profil anlegen' }));

    // Escape closes without saving and returns the focus to the add button.
    await page.keyboard.press('Escape');
    await expect(form).toBeHidden();
    await expectFocusOn(page, page.getByRole('button', { name: 'Profil hinzufügen' }));
    await expect(article(page, 'Tastatur')).toHaveCount(0);

    // Enter in the name field submits.
    await page.keyboard.press('Enter');
    await form.getByLabel('Name', { exact: true }).fill('Tastatur');
    await form.getByLabel('Name', { exact: true }).press('Enter');
    await expect(form).toBeHidden();
    await expect(article(page, 'Tastatur')).toBeVisible();

    await expectNoFocusTrap(page);
  });

  test('a library restriction round-trips and the restricted profile sees none of the other titles', async ({
    page,
    owner,
  }) => {
    await ensureEmptyLibrary(owner);
    await page.goto('/settings/profiles');
    await expect(article(page, OWNER_PROFILE)).toBeVisible();

    // Create a profile limited to the empty library.
    await page.getByRole('button', { name: 'Profil hinzufügen' }).click();
    const form = page.getByRole('dialog', { name: 'Neues Profil' });
    await form.getByLabel('Name', { exact: true }).fill('Eingeschraenkt');
    await choose(form, 'radio', 'Auswahl');
    await choose(form, 'checkbox', 'E2E leer');
    await form.getByRole('button', { name: 'Profil anlegen' }).click();
    await expect(form).toBeHidden();
    const card = article(page, 'Eingeschraenkt');
    await expect(card).toContainText('E2E leer');
    await expect(article(page, OWNER_PROFILE)).toContainText('Alle Bibliotheken');

    // Editing shows the restriction as stored and saving it unchanged keeps it.
    await card.getByRole('button', { name: 'Bearbeiten' }).click();
    const edit = page.getByRole('dialog', { name: 'Profil bearbeiten' });
    await expect(edit.getByRole('radio', { name: 'Auswahl' })).toBeChecked();
    await expect(edit.getByRole('checkbox', { name: 'E2E leer' })).toBeChecked();
    await edit.getByRole('button', { name: 'Speichern' }).click();
    await expect(edit).toBeHidden();
    await expect(card).toContainText('E2E leer');
    const stored = (await (await page.request.get('/api/v1/profiles')).json()) as {
      name: string;
      allLibraries: boolean;
      libraryIds: number[];
    }[];
    expect(stored.find((profile) => profile.name === 'Eingeschraenkt')).toMatchObject({
      allLibraries: false,
    });

    // The restricted profile sees none of the titles of the other library.
    await switchLink(page, OWNER_PROFILE).click();
    await tile(page, 'Eingeschraenkt').click();
    await expect(page).toHaveURL(/\/$/);
    await expect(
      page.getByRole('heading', { level: 2, name: 'Hier ist es noch leer' }),
    ).toBeVisible();
    await expect(page.getByRole('heading', { level: 2, name: 'Neu hinzugefügt' })).toHaveCount(0);
    const home = (await (await page.request.get('/api/v1/home')).json()) as {
      rows: { items: unknown[] }[];
    };
    expect(home.rows.flatMap((row) => row.items)).toHaveLength(0);
  });

  test('a non-primary profile gets a notice instead', async ({ page, owner }) => {
    const other = await createProfile(owner, { name: 'Andere' });
    // This session switches over the API (a switch counts as one attempt like a login).
    await page.request.post(`/api/v1/auth/profiles/${other.id}/select`, {
      headers: { 'X-Requested-With': 'Slouchcat' },
    });
    await page.goto('/settings/profiles');
    await expect(page.getByText('Nur im Hauptprofil')).toBeVisible();
    await expect(page.getByRole('link', { name: 'Profile', exact: true })).toHaveCount(0);
    await expect(page.getByRole('link', { name: 'Bibliotheken' })).toHaveCount(0);
    await page.getByRole('link', { name: 'Profil wechseln', exact: true }).click();
    await expect(page).toHaveURL(/\/profiles$/);
  });
});
