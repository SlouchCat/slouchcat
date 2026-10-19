import type { Page } from '@playwright/test';
import { CSRF, H264_TITLE, idOf, itemPath } from './catalog';
import { expect, test } from './fixtures';
import { expectFocusOn, expectNoFocusTrap, tabTo } from './keyboard';

// Administration view of a title (B-6, B-8, B-9): state, files, the match dialog (the e2e server
// has no TMDB key: the search answers with the provider error), manual trailers.

let detail = '';
let admin = '';

const trailers = (page: Page) => page.getByRole('region', { name: 'Trailer' });

test.beforeEach(async ({ page }) => {
  if (!admin) {
    detail = await itemPath(page, H264_TITLE);
    admin = `/admin/items/${idOf(detail)}`;
  }
  await page.goto(admin);
  await expect(page.getByRole('heading', { level: 1, name: H264_TITLE })).toBeVisible();
});

test.afterEach(async ({ owner }) => {
  // A trailer left by a failed run would make "Löschen" ambiguous here and add a trailer button
  // to the detail page of the other specs.
  if (!admin) return;
  const item = (await (await owner.get(`/api/v1${admin}`)).json()) as {
    trailers: { id: number }[];
  };
  for (const trailer of item.trailers) {
    await owner.delete(`/api/v1${admin}/trailers/${trailer.id}`, { headers: CSRF });
  }
});

test('shows kind, state, both version files and the trailer state', async ({ page }) => {
  const facts = page.getByRole('region', { name: 'Metadaten' });
  await expect(facts.getByRole('term').filter({ hasText: /^Art$/ })).toBeVisible();
  await expect(facts.getByRole('definition').filter({ hasText: /^Film$/ })).toBeVisible();
  const files = page.getByRole('region', { name: 'Dateien' });
  await expect(files.getByRole('listitem')).toHaveCount(2);
  await expect(files.getByText(/Slouchcat Test H264 \(2024\)\.mp4$/)).toBeVisible();
  await expect(files.getByText(/Slouchcat Test H264 \(2024\) - 720p\.mp4$/)).toBeVisible();
  await expect(trailers(page)).toBeVisible();
  await expect(page.getByRole('link', { name: 'Zur Detailseite' })).toHaveAttribute('href', detail);
});

test('the detail page links here with "Verwalten"', async ({ page }) => {
  await page.goto(detail);
  const manage = page.getByRole('link', { name: 'Verwalten' });
  await manage.focus();
  await page.keyboard.press('Enter');
  await expect(page).toHaveURL(new RegExp(`${admin}$`));
});

test('match dialog: the search without a key shows the provider error; Escape closes', async ({
  page,
}) => {
  const match = page.getByRole('button', { name: 'Zuordnen' });
  await tabTo(page, match);
  await page.keyboard.press('Enter');
  const dialog = page.getByRole('dialog', { name: 'Titel zuordnen' });
  await expect(dialog).toBeVisible();
  const query = dialog.getByLabel('Suchbegriff');
  await expectFocusOn(page, query);
  await expect(query).toHaveValue(H264_TITLE);
  await page.keyboard.press('Enter');
  await expect(
    dialog.getByText(/TMDB hat den Schlüssel abgelehnt|Dieser Anbieter ist nicht eingerichtet/),
  ).toBeVisible();
  await page.keyboard.press('Escape');
  await expect(dialog).toHaveCount(0);
  await expectFocusOn(page, match);
});

test('add a manual trailer by keyboard, then delete it after confirmation', async ({ page }) => {
  const add = trailers(page).getByRole('button', { name: 'Hinzufügen' });
  await add.focus();
  await page.keyboard.press('Enter');
  const dialog = page.getByRole('dialog', { name: 'Trailer hinzufügen' });
  const source = dialog.getByLabel('YouTube-Link oder Video-ID');
  await expectFocusOn(page, source);
  await page.keyboard.type('https://www.youtube.com/watch?v=dQw4w9WgXcQ');
  await page.keyboard.press('Enter');
  await expect(dialog).toHaveCount(0);
  await expect(trailers(page).getByText('dQw4w9WgXcQ')).toBeVisible();
  await expect(trailers(page).getByText('Manuell')).toBeVisible();

  const remove = trailers(page).getByRole('button', { name: 'Löschen' });
  await remove.focus();
  await page.keyboard.press('Enter');
  const confirm = page.getByRole('alertdialog', { name: 'Trailer löschen?' });
  await expectFocusOn(page, confirm.getByRole('button', { name: 'Löschen' }));
  await page.keyboard.press('Enter');
  await expect(trailers(page).getByText('dQw4w9WgXcQ')).toHaveCount(0);
});

test('an invalid YouTube link is reported at the field', async ({ page }) => {
  await trailers(page).getByRole('button', { name: 'Hinzufügen' }).click();
  const dialog = page.getByRole('dialog', { name: 'Trailer hinzufügen' });
  await dialog.getByLabel('YouTube-Link oder Video-ID').fill('https://example.com/video');
  await dialog.getByRole('button', { name: 'Hinzufügen' }).click();
  await expect(dialog.getByLabel('YouTube-Link oder Video-ID')).toHaveAttribute(
    'aria-invalid',
    'true',
  );
  await page.keyboard.press('Escape');
  await expect(dialog).toHaveCount(0);
});

test('Tab: header, actions, rating, files, trailers; no focus trap', async ({ page }) => {
  await tabTo(page, page.getByRole('link', { name: 'Pflege' }));
  await tabTo(page, page.getByRole('button', { name: 'Zuordnen' }), 6);
  await tabTo(page, page.getByRole('button', { name: 'Aktualisieren' }), 2);
  await tabTo(page, page.getByRole('link', { name: 'Zur Detailseite' }), 2);
  await tabTo(page, trailers(page).getByRole('button', { name: 'Hinzufügen' }), 12);
  await expectNoFocusTrap(page);
});
