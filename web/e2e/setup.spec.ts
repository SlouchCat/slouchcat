import { expect, test, type Page } from '@playwright/test';
import { MOVIES_DIR } from './env';
import { startFreshServer, type FreshServer } from './fresh-server';
import {
  expectFocusOn,
  expectNoFocusTrap,
  expectVisibleFocus,
  resetFocus,
  tabTo,
} from './keyboard';

// The guided wizard of concept 22 part A, step by step with the keyboard (rule 8): the list of
// steps first, then the step, then its foot; Enter on «Weiter»; the heading takes the focus on
// every step; no focus trap. It sets up a server of its own (fresh-server.ts): the wizard creates
// the owner and changes the settings, which the shared server of the run must keep. The steps
// run in order on one page, as an owner would go through them.

test.describe.configure({ mode: 'serial' });
test.use({ storageState: { cookies: [], origins: [] } });

const OWNER = { email: 'wizard@example.test', password: 'wizard-password-0815', profile: 'Marco' };

let server: FreshServer;
let page: Page;

test.beforeAll(async ({ browser }) => {
  server = await startFreshServer();
  page = await browser.newPage({ locale: 'de-CH' });
});

test.afterAll(async () => {
  await page.close();
  await server.stop();
});

const heading = (name: string) => page.getByRole('heading', { level: 1, name });
const stepLink = (name: RegExp) =>
  page.getByRole('navigation', { name: 'Einrichtung' }).getByRole('link', { name });

/** Enter on «Weiter»: the next step's heading has the focus. */
async function next(title: string) {
  await tabTo(page, page.getByRole('button', { name: 'Weiter', exact: true }));
  await expectVisibleFocus(page);
  await page.keyboard.press('Enter');
  await expectFocusOn(page, heading(title));
}

test('step 1, language: list, language button, search, the languages, then «Weiter» @keyboard', async () => {
  await page.goto(`${server.url}/admin`);
  // Every path leads to the first step while there is no account.
  await expect(page).toHaveURL(/\/setup\/language$/);
  await expectFocusOn(page, heading('Sprache wählen'));
  await resetFocus(page);
  await page.keyboard.press('Tab');
  await expectFocusOn(page, stepLink(/^1\. Sprache$/));
  // The list is one Tab stop; the arrows move inside it.
  await page.keyboard.press('ArrowDown');
  await expectFocusOn(page, stepLink(/^2\. Konto$/));
  await page.keyboard.press('Tab');
  await expectFocusOn(page, page.getByRole('button', { name: /^Sprache: / }));
  await page.keyboard.press('Tab');
  await expectFocusOn(page, page.getByRole('searchbox', { name: 'Sprache suchen' }));
  await page.keyboard.press('Tab');
  await expectFocusOn(page, page.getByRole('radio', { name: /^Deutsch \(Schweiz\)/ }));
  await expect(page.getByRole('button', { name: 'Überspringen' })).toHaveCount(0);
  await expectNoFocusTrap(page);
  await next('Konto anlegen');
});

test('step 2, account: the fields in order, Enter creates the owner @keyboard', async () => {
  await expect(page).toHaveURL(/\/setup\/account$/);
  await page.keyboard.press('Tab');
  await expectFocusOn(page, page.getByLabel(/^E-Mail-Adresse/));
  await page.keyboard.type(OWNER.email);
  await page.keyboard.press('Tab');
  await page.keyboard.type(OWNER.password);
  await page.keyboard.press('Tab');
  await expectFocusOn(page, page.getByLabel(/^Passwort wiederholen/));
  await page.keyboard.type(OWNER.password);
  await page.keyboard.press('Tab');
  await expectFocusOn(page, page.getByLabel(/^Name des Profils/));
  await page.keyboard.type(OWNER.profile);
  await page.keyboard.press('Tab');
  // The colour: one Tab stop, the arrows choose.
  await expect(page.getByRole('radiogroup', { name: 'Farbe' }).locator(':focus')).toHaveCount(1);
  await page.keyboard.press('ArrowRight');
  await expectNoFocusTrap(page);
  await next('Region und Metadaten');
  // Signed in now: the list ends with «Später fortsetzen», and the account cannot be gone back to.
  await expect(page).toHaveURL(/\/setup\/region$/);
  await expect(page.getByRole('button', { name: 'Später fortsetzen' })).toBeVisible();
});

test('step 3, region: country, rating country with its steps, languages, radio @keyboard', async () => {
  await resetFocus(page);
  await page.keyboard.press('Tab');
  await expectFocusOn(page, stepLink(/^3\. Region$/));
  await page.keyboard.press('Tab');
  await expectFocusOn(page, page.getByRole('button', { name: 'Später fortsetzen' }));
  await page.keyboard.press('Tab');
  await expectFocusOn(page, page.getByRole('combobox', { name: 'Land' }));
  await expect(page.getByRole('combobox', { name: 'Land' })).toHaveValue('CH');
  await page.keyboard.press('Tab');
  // Every rating country of the server's table (concept 22 stage 3), Switzerland suggested; the
  // steps of the choice stand below it, as the household step offers them.
  const rating = page.getByRole('combobox', { name: 'Altersfreigaben nach' });
  await expectFocusOn(page, rating);
  await expect(rating).toHaveValue('CH');
  await expect(rating.getByRole('option')).toHaveCount(25);
  await expect(
    page.getByText(/^Altersstufen der Profile: ab 0 · ab 6 · ab 7 · ab 8/),
  ).toBeVisible();
  await rating.selectOption('FR');
  await expect(
    page.getByText('Altersstufen der Profile: Tous publics · -10 · -12 · -16 · -18'),
  ).toBeVisible();
  await expectFocusOn(page, rating);
  await expect(
    page.getByRole('switch', { name: 'Schweiz ins Radio-Angebot aufnehmen' }),
  ).toBeChecked();
  await expectNoFocusTrap(page);
  await next('Dienste verbinden');
  await expect(stepLink(/^3\. Region, erledigt$/)).toBeVisible();
});

test('step 4, services: the cards with their links, «Überspringen» goes on @keyboard', async () => {
  await expect(page.getByRole('heading', { level: 2, name: 'Empfohlen' })).toBeVisible();
  const links = page.getByRole('link', { name: /^Schlüssel bei .* holen$/ });
  await expect(links.first()).toBeVisible();
  await tabTo(page, page.getByRole('button', { name: 'Überspringen' }));
  await expectVisibleFocus(page);
  await page.keyboard.press('Enter');
  await expectFocusOn(page, heading('Wiedergabe'));
  await expect(stepLink(/^4\. Dienste, übersprungen$/)).toBeVisible();
});

test('F-11: a new sign-in before the end leads to the first open step', async ({ browser }) => {
  const other = await browser.newPage({ locale: 'de-CH' });
  await other.goto(`${server.url}/login`);
  await other.getByLabel(/^E-Mail-Adresse/).fill(OWNER.email);
  await other.getByLabel(/^Passwort/).fill(OWNER.password);
  await other.getByRole('button', { name: 'Anmelden' }).press('Enter');
  await expect(other).toHaveURL(/\/setup\/playback$/);
  await expect(other.getByRole('heading', { level: 1, name: 'Wiedergabe' })).toBeVisible();
  // «Später fortsetzen» lets go for this sign-in; the overview says what is still open.
  await other.getByRole('button', { name: 'Später fortsetzen' }).press('Enter');
  await expect(other).toHaveURL(`${server.url}/`);
  await other.goto(`${server.url}/admin`);
  await expect(other.getByText('Einrichtung nicht abgeschlossen')).toBeVisible();
  await expect(other.getByRole('link', { name: 'Einrichtung fortsetzen' })).toHaveAttribute(
    'href',
    '/setup/playback',
  );
  await other.close();
});

test('step 5, playback: the backends by arrows, the trickplay default @keyboard', async () => {
  const group = page.getByRole('radiogroup', { name: 'Grafikchip für das Umwandeln' });
  await expect(group.getByRole('radio', { name: 'Automatisch (empfohlen)' })).toBeChecked();
  await tabTo(page, group.getByRole('radio', { name: 'Automatisch (empfohlen)' }));
  await expectVisibleFocus(page);
  // The arrows pass over a backend whose probe failed and store the choice at once.
  const probed = await group.getByRole('radio', { disabled: false }).count();
  if (probed === 2) {
    await page.keyboard.press('ArrowDown');
    await expectFocusOn(page, group.getByRole('radio', { name: 'Nur Prozessor' }));
    await expect(page.getByText('Gespeichert: Nur Prozessor')).toBeVisible();
    await page.keyboard.press('ArrowUp');
    await expect(page.getByText('Gespeichert: Automatisch (empfohlen)')).toBeVisible();
  }
  await page.keyboard.press('Tab');
  await expectFocusOn(page, page.getByRole('switch', { name: /Vorschau beim Spulen/ }));
  await page.keyboard.press('Space');
  await expect(page.getByRole('switch', { name: /Vorschau beim Spulen/ })).toBeChecked();
  await expectNoFocusTrap(page);
  await next('Bibliotheken');
});

test('step 6, libraries: none is refused, the dialog adds one @keyboard', async () => {
  await expect(page.getByRole('button', { name: 'Überspringen' })).toHaveCount(0);
  await tabTo(page, page.getByRole('button', { name: 'Weiter', exact: true }));
  await page.keyboard.press('Enter');
  await expect(page.getByText(/Noch keine Bibliothek/)).toBeVisible();
  await expectFocusOn(page, page.getByRole('button', { name: 'Weiter', exact: true }));

  await tabTo(page, page.getByRole('button', { name: 'Bibliothek hinzufügen' }));
  await page.keyboard.press('Enter');
  const dialog = page.getByRole('dialog', { name: 'Bibliothek hinzufügen' });
  await expect(dialog).toBeVisible();
  await dialog.getByLabel('Name', { exact: true }).fill('Filme');
  await dialog.getByLabel('Ordner', { exact: true }).fill(MOVIES_DIR);
  await dialog.getByRole('button', { name: 'Bibliothek mit 1 Ordner hinzufügen' }).press('Enter');
  await expect(dialog).toBeHidden();
  await expect(page.getByRole('region', { name: 'Filme' })).toBeVisible();
  await next('Haushalt');
});

test('step 7, household: the own profile, «Profil hinzufügen» with the French steps, then skip @keyboard', async () => {
  await expect(page.getByText(OWNER.profile).first()).toBeVisible();
  await tabTo(page, page.getByRole('button', { name: 'Profil hinzufügen' }));
  await expectVisibleFocus(page);
  // The ages are those of the rating country of step 3 (France, F-12).
  await page.keyboard.press('Enter');
  const form = page.getByRole('dialog', { name: 'Neues Profil' });
  const ages = form.getByRole('radiogroup', { name: 'Altersfreigabe' });
  await expect(ages.getByRole('radio')).toHaveCount(5);
  await expect(ages.getByRole('radio', { name: '-12' })).toBeVisible();
  await expect(ages.getByRole('radio', { name: 'Tous publics' })).toBeVisible();
  await page.keyboard.press('Escape');
  await expect(form).toBeHidden();
  await expectFocusOn(page, page.getByRole('button', { name: 'Profil hinzufügen' }));
  await tabTo(page, page.getByRole('button', { name: 'Überspringen' }));
  await page.keyboard.press('Enter');
  await expectFocusOn(page, heading('Zugriff'));
});

test('step 8, access: the address of this browser as the suggestion, the update check @keyboard', async () => {
  await expect(page.getByText(`Du greifst gerade über ${server.url} zu.`)).toBeVisible();
  await tabTo(page, page.getByRole('button', { name: 'Diese Adresse übernehmen' }));
  await page.keyboard.press('Enter');
  await expect(page.getByRole('textbox', { name: /^Adresse/ })).toHaveValue(server.url);
  await expect(
    page.getByRole('checkbox', { name: 'Täglich nach neuen Versionen suchen' }),
  ).toBeChecked();
  await expectNoFocusTrap(page);
  await next('Zusammenfassung');
});

test('step 9, summary: «Ändern» per step, finish, the first scan, the start page @keyboard', async () => {
  await expect(page.getByRole('link', { name: 'Region ändern' })).toBeVisible();
  await tabTo(page, page.getByRole('button', { name: 'Einrichtung abschliessen' }));
  await page.keyboard.press('Enter');
  const done = page.getByRole('heading', { name: 'Slouchcat ist eingerichtet' });
  await expectFocusOn(page, done);
  await expect(page.getByText('Filme', { exact: true })).toBeVisible();
  await tabTo(page, page.getByRole('button', { name: 'Zur Startseite' }));
  await page.keyboard.press('Enter');
  await expect(page).toHaveURL(`${server.url}/`);
  // Finished: the wizard is gone for good, also at the next visit.
  await page.goto(`${server.url}/setup/region`);
  await expect(page).toHaveURL(`${server.url}/`);
});
