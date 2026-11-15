import { readFileSync } from 'node:fs';
import type { Page } from '@playwright/test';
import { languages } from '../src/i18n/languages';
import { listMyProfiles, loginOwner } from './accounts';
import { CSRF } from './catalog';
import { choose } from './controls';
import { expect, test } from './fixtures';
import { expectFocusOn, expectNoFocusTrap, tabTo } from './keyboard';

// Concept 22 section 4.9: every shipped language once through the login page, the start page and
// the profile form, and the language button of the login page by keyboard (rule 8). The texts come
// from the language files, so a new shipped language joins the loop without a new line here.

test.use({ storageState: { cookies: [], origins: [] } });

type Texts = { [key: string]: string | Texts };
const files = new Map<string, Texts>();

/** The text `key` of `code` straight from web/src/locales/<code>.json, without its chain. */
function own(code: string, key: string): string | undefined {
  let file = files.get(code);
  if (!file) {
    file = JSON.parse(
      readFileSync(new URL(`../src/locales/${code}.json`, import.meta.url), 'utf8'),
    ) as Texts;
    files.set(code, file);
  }
  let value: string | Texts | undefined = file;
  for (const part of key.split('.')) value = typeof value === 'object' ? value[part] : undefined;
  return typeof value === 'string' ? value : undefined;
}

/**
 * The text `key` of `code` along its chain (concept 22 section 4.2): de-DE overlays de, and
 * English stands in for a text a language lacks between two releases (4.8).
 */
function text(code: string, key: string): string {
  const value = own(code, key) ?? (code === 'de-DE' ? own('de', key) : undefined) ?? own('en', key);
  if (value === undefined) throw new Error(`${code}.json has no text ${key}`);
  return value;
}

/** Opens the profile settings in `code`. */
async function openProfiles(page: Page, code: string) {
  await page.goto('/settings/profiles');
  await expect(
    page.getByRole('heading', { level: 1, name: text(code, 'profiles.settings.title') }),
  ).toBeVisible();
}

for (const { code, name } of languages) {
  test.describe(code, () => {
    // The browser asks for the language (a Swiss browser sends de-CH; a bare de is Germany's).
    test.use({ locale: code === 'de' ? 'de-CH' : code });

    test(`${code}: login page, start page and profile form in that language`, async ({ page }) => {
      // The login page in the language the browser asks for.
      await page.goto('/login');
      await expect(
        page.getByRole('heading', { level: 1, name: text(code, 'login.title') }),
      ).toBeVisible();
      await expect(page.locator('html')).toHaveAttribute('lang', code);

      await loginOwner(page.request);
      const primary = (await listMyProfiles(page.request)).find((profile) => profile.isPrimary);
      if (!primary) throw new Error('no primary profile');
      try {
        // The profile form of the primary profile, in German, switches it to `code`.
        await openProfiles(page, 'de');
        await page
          .getByRole('button', { name: text('de', 'profiles.settings.edit') })
          .first()
          .click();
        const form = page.getByRole('dialog', {
          name: text('de', 'profiles.settings.form.editTitle'),
        });
        await choose(form, 'radio', name);
        await form.getByRole('button', { name: text('de', 'profiles.settings.form.save') }).click();
        await expect(form).toBeHidden();
        await expect(page.locator('html')).toHaveAttribute('lang', code);
        await expect(page.locator('html')).toHaveAttribute('dir', 'ltr');

        // The start page in `code`.
        await page.goto('/');
        await expect(
          page.getByRole('heading', { level: 2, name: text(code, 'home.rows.recent') }),
        ).toBeVisible();
        await expect(page.locator('html')).toHaveAttribute('lang', code);

        // The form in `code`: its title, and the profile's language named in its own language.
        await openProfiles(page, code);
        await page.getByRole('button', { name: text(code, 'profiles.settings.add') }).click();
        const create = page.getByRole('dialog', {
          name: text(code, 'profiles.settings.form.createTitle'),
        });
        await expect(create.getByRole('radio', { name, exact: true })).toBeChecked();
        await page.keyboard.press('Escape');
        await expect(create).toBeHidden();
      } finally {
        const reset = await page.request.patch(`/api/v1/profiles/${primary.id}`, {
          headers: CSRF,
          data: { language: 'de' },
        });
        expect(reset.status()).toBe(200);
      }
    });
  });
}

test('the language button of the login page: Tab, Enter, arrows, Enter; no focus trap @keyboard', async ({
  page,
}) => {
  await page.goto('/login');
  await expect(page.getByRole('heading', { level: 1, name: 'Anmelden' })).toBeVisible();
  const button = page.getByRole('button', { name: 'Sprache: Deutsch (Schweiz)' });
  await tabTo(page, button);
  await expectFocusOn(page, button);
  await page.keyboard.press('Enter');
  const menu = page.getByRole('menu');
  await expect(menu.getByRole('menuitemradio')).toHaveText(languages.map(({ name }) => name));
  await expect(menu.getByRole('menuitemradio', { name: 'Deutsch (Schweiz)' })).toBeChecked();
  // Escape closes without a change and gives the focus back.
  await page.keyboard.press('Escape');
  await expect(menu).toBeHidden();
  await expectFocusOn(page, button);

  await page.keyboard.press('Enter');
  await expect(menu).toBeVisible();
  await page.keyboard.press('ArrowDown');
  await expectFocusOn(page, menu.getByRole('menuitemradio', { name: 'Deutsch (Deutschland)' }));
  await page.keyboard.press('ArrowDown');
  await expectFocusOn(page, menu.getByRole('menuitemradio', { name: 'English' }));
  await page.keyboard.press('Enter');
  await expect(menu).toBeHidden();
  await expect(page.getByRole('heading', { level: 1, name: 'Sign in' })).toBeVisible();
  await expect(page.locator('html')).toHaveAttribute('lang', 'en');
  await expectFocusOn(page, page.getByRole('button', { name: 'Language: English' }));
  await expectNoFocusTrap(page);
});

// Concept 22 step 1: the wizard's language step switches the interface at once, German
// (Germany) formats like Germany and German (Switzerland) like Switzerland; arrows choose, Tab
// leaves the group. The run's server has an account, so the answers before one are mocked.
test('the language step of the wizard: arrows switch the interface and the formats @keyboard', async ({
  page,
}) => {
  await page.route('**/api/v1/auth/session', (route) =>
    route.fulfill({
      status: 401,
      contentType: 'application/problem+json',
      json: { type: 'about:blank', title: 'unauthenticated', status: 401, code: 'unauthenticated' },
    }),
  );
  await page.route('**/api/v1/setup', (route) => route.fulfill({ json: { setupRequired: true } }));
  await page.goto('/setup/language');
  const heading = page.getByRole('heading', {
    level: 1,
    name: text('de', 'setup.steps.language.title'),
  });
  await expectFocusOn(page, heading);
  const swiss = page.getByRole('radio', { name: /^Deutsch \(Schweiz\)/ });
  await tabTo(page, swiss);
  await expect(swiss).toBeChecked();
  await expect(page.getByText(new Intl.NumberFormat('de-CH').format(1234567.5))).toBeVisible();
  await page.keyboard.press('ArrowDown');
  await expectFocusOn(page, page.getByRole('radio', { name: /^Deutsch \(Deutschland\)/ }));
  await expect(page.locator('html')).toHaveAttribute('lang', 'de-DE');
  await expect(page.getByText('1.234.567,5')).toBeVisible();
  // de-DE overlays de: a text with ß comes from its own file, the rest from de.json.
  await expect(
    page.getByRole('button', { name: text('de-DE', 'setup.actions.next') }),
  ).toBeVisible();
  await page.keyboard.press('ArrowDown');
  await expect(page.locator('html')).toHaveAttribute('lang', 'en');
  await expect(
    page.getByRole('heading', { level: 1, name: text('en', 'setup.steps.language.title') }),
  ).toBeVisible();
  await page.keyboard.press('ArrowUp');
  await page.keyboard.press('ArrowUp');
  await expect(page.locator('html')).toHaveAttribute('lang', 'de');
  await expectNoFocusTrap(page);
});
