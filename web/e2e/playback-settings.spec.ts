import type { APIRequestContext, Locator, Page } from '@playwright/test';
import { CSRF } from './catalog';
import { expect, test } from './fixtures';
import {
  expectFocusOn,
  expectNoFocusTrap,
  expectVisibleFocus,
  resetFocus,
  tabTo,
} from './keyboard';

// «Meine Wiedergabe» (concept 13 11.4, F-13) by keyboard (rule 8): the audio languages in order
// (add, pick and swap with the arrows on a closed select, remove), the subtitle rule with the
// arrows, saving with Enter, the series memory reset behind a confirmation, Back to the page
// before, a visible ring on every stop and no focus trap. The owner's preferences go back as
// they were after each test (the track choice of the other specs reads them).

interface Prefs {
  audioLanguages: string[];
  subtitleMode: string;
  subtitleLanguages: string[];
}

async function prefsOf(api: APIRequestContext): Promise<Prefs> {
  const response = await api.get('/api/v1/me/playback-prefs');
  expect(response.status(), await response.text()).toBe(200);
  return (await response.json()) as Prefs;
}

async function restore(api: APIRequestContext, prefs: Prefs): Promise<void> {
  const response = await api.put('/api/v1/me/playback-prefs', { data: prefs, headers: CSRF });
  expect(response.status(), await response.text()).toBe(200);
}

const audioList = (page: Page) => page.getByRole('group', { name: 'Tonsprachen' });
const place = (list: Locator, n: number) =>
  list.getByRole('combobox', { name: `${n}. Tonsprache`, exact: true });
const placesOf = (list: Locator) =>
  list
    .getByRole('combobox')
    .evaluateAll((selects) => selects.map((select) => (select as HTMLSelectElement).value));

/** Picks `value` in the focused closed select with ↑ or ↓ only, one step per option. */
async function pickByKeys(page: Page, select: Locator, value: string): Promise<void> {
  await expect(select).toBeFocused();
  const { from, to } = await select.evaluate((element, wanted) => {
    const values = [...(element as HTMLSelectElement).options].map((option) => option.value);
    return {
      from: values.indexOf((element as HTMLSelectElement).value),
      to: values.indexOf(wanted),
    };
  }, value);
  expect(to, `${value} is a choice`).toBeGreaterThanOrEqual(0);
  for (let step = 0; step < Math.abs(to - from); step += 1) {
    await page.keyboard.press(to > from ? 'ArrowDown' : 'ArrowUp');
  }
  await expect(select).toHaveValue(value);
}

async function tabToFromTop(page: Page, target: Locator): Promise<void> {
  await resetFocus(page);
  await tabTo(page, target, 200);
}

test('«Meine Wiedergabe» by keyboard: languages, subtitle rule, save, series reset, Back @keyboard', async ({
  page,
}) => {
  test.setTimeout(60_000);
  const before = await prefsOf(page.request);
  try {
    // The sidebar leads there for every profile; Enter follows the link.
    await page.goto('/');
    const link = page.getByRole('link', { name: 'Meine Wiedergabe' });
    await tabToFromTop(page, link);
    await page.keyboard.press('Enter');
    await expect(page).toHaveURL(/\/settings\/playback$/);
    await expect(page.getByRole('heading', { level: 1, name: 'Meine Wiedergabe' })).toBeVisible();
    const list = audioList(page);
    await expect(place(list, 1)).toBeVisible();
    const start = await placesOf(list);
    expect(start).toEqual(before.audioLanguages);

    // «Weitere Tonsprache» appends a place and focuses it; Japanisch goes there by the arrows.
    const more = list.getByRole('button', { name: 'Weitere Tonsprache' });
    await tabToFromTop(page, more);
    await page.keyboard.press('Enter');
    const added = place(list, start.length + 1);
    await expectFocusOn(page, added);
    await pickByKeys(page, added, 'ja');
    // Japanisch onto place 1: the two places swap, the select keeps the focus.
    const first = place(list, 1);
    await tabToFromTop(page, first);
    await pickByKeys(page, first, 'ja');
    await expectFocusOn(page, first);
    expect(await placesOf(list)).toEqual(['ja', ...start.slice(1), start[0]]);

    // The subtitle rule: Tab onto the chosen radio, ↓ to the next one, which is chosen.
    const rule = page.getByRole('radiogroup', { name: 'Untertitel' });
    const checked = rule.getByRole('radio', { checked: true });
    await tabToFromTop(page, checked);
    await expectVisibleFocus(page);
    const always = rule.getByRole('radio', { name: 'Immer' });
    for (let presses = 0; presses < 4 && !(await always.isChecked()); presses += 1) {
      await page.keyboard.press('ArrowDown');
    }
    await expect(always).toBeChecked();
    await expect(
      page.getByText(/Immer volle Untertitel in der ersten Untertitelsprache/),
    ).toBeVisible();

    // Enter on «Speichern» stores all three fields at once.
    const save = page.getByRole('button', { name: 'Speichern' });
    await tabToFromTop(page, save);
    const stored = page.waitForResponse(
      (response) =>
        response.request().method() === 'PUT' && response.url().endsWith('/me/playback-prefs'),
    );
    await page.keyboard.press('Enter');
    expect((await stored).status()).toBe(200);
    await expect(page.getByText('Gespeichert.')).toBeVisible();
    const saved = await prefsOf(page.request);
    expect(saved.audioLanguages).toEqual(['ja', ...start.slice(1), start[0]]);
    expect(saved.subtitleMode).toBe('always');
    expect(saved.subtitleLanguages).toEqual(before.subtitleLanguages);

    // The removal button of a place, by keyboard: the list is as before once Japanisch is gone.
    await tabToFromTop(page, first);
    await pickByKeys(page, first, start[0]);
    const remove = list.getByRole('button', {
      name: `${start.length + 1}. Tonsprache (Japanisch) entfernen`,
    });
    await tabToFromTop(page, remove);
    await page.keyboard.press('Enter');
    await expect(list.getByRole('combobox')).toHaveCount(start.length);
    expect(await placesOf(list)).toEqual(start);

    // The series memory: Enter opens the confirmation on «Zurücksetzen»; Escape closes it and
    // the focus returns to the button; Enter twice resets.
    const reset = page.getByRole('button', { name: 'Serienmerker zurücksetzen' });
    await tabToFromTop(page, reset);
    await page.keyboard.press('Enter');
    const dialog = page.getByRole('alertdialog', { name: 'Serienmerker zurücksetzen?' });
    await expect(dialog).toBeVisible();
    await expectFocusOn(page, dialog.getByRole('button', { name: 'Zurücksetzen' }));
    await page.keyboard.press('Escape');
    await expect(dialog).toBeHidden();
    await expectFocusOn(page, reset);
    await page.keyboard.press('Enter');
    await expectFocusOn(page, dialog.getByRole('button', { name: 'Zurücksetzen' }));
    const deleted = page.waitForResponse(
      (response) =>
        response.request().method() === 'DELETE' &&
        response.url().endsWith('/me/playback-prefs/series'),
    );
    await page.keyboard.press('Enter');
    expect((await deleted).status()).toBe(204);
    await expect(page.getByText('Serienmerker zurückgesetzt.')).toBeVisible();

    await expectNoFocusTrap(page, 200);

    // Back leads to the page before.
    await page.goBack();
    await expect(page).toHaveURL(/\/$/);
  } finally {
    await restore(page.request, before);
  }
});
