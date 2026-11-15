import { readFileSync } from 'node:fs';
import type { Page } from '@playwright/test';
import { languages } from '../src/i18n/languages';
import { listMyProfiles, loginOwner } from './accounts';
import { CSRF } from './catalog';
import { choose } from './controls';
import { expect, test } from './fixtures';
import { expectFocusOn, expectNoFocusTrap, tabTo } from './keyboard';

// Concept 22 section 4.9: every shipped language once through the start page and the profile
// form, and the language button of the login page by keyboard (rule 8). The texts come from the
// language files, so a new shipped language joins the loop without a new line here.

test.use({ storageState: { cookies: [], origins: [] } });

type Texts = { [key: string]: string | Texts };
const files = new Map<string, Texts>();

/** The text `key` of `code` straight from web/src/locales/<code>.json. */
function text(code: string, key: string): string {
  let file = files.get(code);
  if (!file) {
    file = JSON.parse(
      readFileSync(new URL(`../src/locales/${code}.json`, import.meta.url), 'utf8'),
    ) as Texts;
    files.set(code, file);
  }
  let value: string | Texts | undefined = file;
  for (const part of key.split('.')) value = typeof value === 'object' ? value[part] : undefined;
  if (typeof value !== 'string') throw new Error(`${code}.json has no text ${key}`);
  return value;
}

/** Opens the profile settings in `code`. */
async function openProfiles(page: Page, code: string) {
  await page.goto('/settings/profiles');
  await expect(
    page.getByRole('heading', { level: 1, name: text(code, 'profiles.settings.title') }),
  ).toBeVisible();
}

test('every shipped language: start page and profile form in that language', async ({ page }) => {
  await loginOwner(page.request);
  const primary = (await listMyProfiles(page.request)).find((profile) => profile.isPrimary);
  if (!primary) throw new Error('no primary profile');
  try {
    let shown = 'de';
    await openProfiles(page, shown);
    for (const { code, name } of languages) {
      // The profile form of the primary profile switches it to `code`, in the language shown now.
      await page
        .getByRole('button', { name: text(shown, 'profiles.settings.edit') })
        .first()
        .click();
      const form = page.getByRole('dialog', {
        name: text(shown, 'profiles.settings.form.editTitle'),
      });
      await choose(form, 'radio', name);
      await form.getByRole('button', { name: text(shown, 'profiles.settings.form.save') }).click();
      await expect(form).toBeHidden();
      shown = code;
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
    }
  } finally {
    const reset = await page.request.patch(`/api/v1/profiles/${primary.id}`, {
      headers: CSRF,
      data: { language: 'de' },
    });
    expect(reset.status()).toBe(200);
  }
});

test('the language button of the login page: Tab, Enter, arrows, Enter; no focus trap @keyboard', async ({
  page,
}) => {
  await page.goto('/login');
  await expect(page.getByRole('heading', { level: 1, name: 'Anmelden' })).toBeVisible();
  const button = page.getByRole('button', { name: 'Sprache: Deutsch' });
  await tabTo(page, button);
  await expectFocusOn(page, button);
  await page.keyboard.press('Enter');
  const menu = page.getByRole('menu');
  await expect(menu.getByRole('menuitemradio')).toHaveText(languages.map(({ name }) => name));
  await expect(menu.getByRole('menuitemradio', { name: 'Deutsch' })).toBeChecked();
  // Escape closes without a change and gives the focus back.
  await page.keyboard.press('Escape');
  await expect(menu).toBeHidden();
  await expectFocusOn(page, button);

  await page.keyboard.press('Enter');
  await expect(menu).toBeVisible();
  await page.keyboard.press('ArrowDown');
  await expectFocusOn(page, menu.getByRole('menuitemradio', { name: 'English' }));
  await page.keyboard.press('Enter');
  await expect(menu).toBeHidden();
  await expect(page.getByRole('heading', { level: 1, name: 'Sign in' })).toBeVisible();
  await expect(page.locator('html')).toHaveAttribute('lang', 'en');
  await expectFocusOn(page, page.getByRole('button', { name: 'Language: English' }));
  await expectNoFocusTrap(page);
});
