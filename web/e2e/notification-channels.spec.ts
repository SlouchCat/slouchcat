import { appendFileSync, constants, copyFileSync, readdirSync, rmSync } from 'node:fs';
import { join } from 'node:path';
import type { Page, TestInfo } from '@playwright/test';
import { introSeason, setFavorite } from './discover';
import { OWNER, OWNER_PROFILE } from './env';
import { expect, test } from './fixtures';
import { expectFocusOn, expectNoFocusTrap, resetFocus, tabTo } from './keyboard';
import {
  addPushSubscription,
  botMessages,
  channel,
  mailsTo,
  newPushSubscription,
  pushes,
  resetChannels,
  setChannel,
  smtpRejectAuth,
  telegramDrop,
  telegramStart,
} from './mocks';
import { INTRO_SHOW_TITLE, itemIdByTitle } from './playback';
import { SMTP_FROM } from './smtp-server.mjs';
import { BOT_USERNAME, CHAT_ID } from './telegram-server.mjs';

// Notification channels (A-6, E-8 channel part) against the server and the mocks of
// start-server.mjs: e-mail through the SMTP mock, Telegram through the bot API mock, Web Push
// through the push service mock (headless browsers cannot receive pushes, so the spec registers a
// subscription of the mock through the API, as the page would). The settings page by keyboard
// (rule 8). Every test leaves the main account's channels off again.

test.afterEach(async ({ owner }) => {
  await resetChannels(owner);
});

const card = (page: Page, name: 'E-Mail' | 'Web-Push' | 'Telegram') =>
  page.getByRole('article', { name });

async function openSettings(page: Page): Promise<void> {
  await page.goto('/settings/notifications');
  await expect(
    page.getByRole('heading', { level: 1, name: 'Benachrichtigungskanäle' }),
  ).toBeVisible();
}

/** Switches the channel of `name` on and stores it with its events as they are. */
async function switchOn(page: Page, name: 'E-Mail' | 'Web-Push' | 'Telegram'): Promise<void> {
  const box = card(page, name);
  const toggle = box.getByRole('switch', { name: 'Aktiv' });
  if (!(await toggle.isChecked())) await toggle.click();
  await box.getByRole('button', { name: 'Speichern' }).click();
  await expect(box.getByText('Gespeichert.')).toBeVisible();
  await expect(toggle).toBeChecked();
}

test('e-mail: the test message reaches the mail server; a refused login shows the failure', async ({
  page,
}) => {
  await openSettings(page);
  const mail = card(page, 'E-Mail');
  await expect(mail.getByLabel('E-Mail-Adresse')).toHaveValue(OWNER.email);
  await expect(mail.getByRole('checkbox', { name: 'Fehler' })).toBeVisible();
  await switchOn(page, 'E-Mail');

  await mail.getByRole('button', { name: 'Testnachricht' }).click();
  await expect(mail.getByText('Testnachricht gesendet.')).toBeVisible();
  await expect.poll(async () => (await mailsTo(OWNER.email)).length).toBe(1);
  const [sent] = await mailsTo(OWNER.email);
  expect(sent?.from).toBe(SMTP_FROM);
  expect(sent?.subject).not.toBe('');

  // The mail server refuses the (right) password once: the send fails with the password in play,
  // and the log scan of the run (global-teardown.ts) checks that it did not reach the log (A-9).
  await smtpRejectAuth(1);
  await mail.getByRole('button', { name: 'Testnachricht' }).click();
  await expect(mail.getByText(/Die Nachricht ging nicht weg/)).toBeVisible();
  expect(await mailsTo(OWNER.email)).toHaveLength(1);
});

test('Telegram: the link code to the bot, "Verknüpft" within 10 s, then the test message', async ({
  page,
}) => {
  await openSettings(page);
  const bot = card(page, 'Telegram');
  await expect(bot.getByText('Nicht verknüpft')).toBeVisible();
  await bot.getByRole('button', { name: 'Verknüpfen' }).click();
  const instruction = bot.getByText(`Sende dem Bot @${BOT_USERNAME} die Nachricht /start`);
  await expect(instruction).toBeVisible();
  const code = /\/start ([A-Z0-9]{6})$/.exec((await instruction.textContent()) ?? '')?.[1];
  expect(code, 'link code in the instruction').toBeDefined();
  await expect(bot.getByRole('link', { name: /In Telegram öffnen/ })).toHaveAttribute(
    'href',
    `https://t.me/${BOT_USERNAME}?start=${code}`,
  );

  // The user sends "/start CODE"; the server polls the bot, links the chat and confirms there.
  await telegramStart(code!);
  await expect(bot.getByText(/^Verknüpft/)).toBeVisible({ timeout: 10_000 });
  await expect(instruction).toHaveCount(0);
  await expect
    .poll(
      async () =>
        (await botMessages()).filter((message) => message.chatId === String(CHAT_ID)).length,
    )
    .toBeGreaterThanOrEqual(1);
  await switchOn(page, 'Telegram');

  // A lost connection to the bot API (the token is in its URL, A-9): the send fails once.
  await telegramDrop(1, 'sendMessage');
  await bot.getByRole('button', { name: 'Testnachricht' }).click();
  await expect(bot.getByText(/Die Nachricht ging nicht weg/)).toBeVisible();

  const before = (await botMessages()).length;
  await bot.getByRole('button', { name: 'Testnachricht' }).click();
  await expect(bot.getByText('Testnachricht gesendet.')).toBeVisible();
  await expect.poll(async () => (await botMessages()).length).toBe(before + 1);
  expect((await botMessages()).at(-1)?.chatId).toBe(String(CHAT_ID));

  await bot.getByRole('button', { name: 'Verknüpfung lösen' }).click();
  await expect(bot.getByText('Nicht verknüpft')).toBeVisible();
});

test('Web Push: the test message reaches the push service encrypted; an expired one goes', async ({
  page,
  owner,
}) => {
  const subscription = await newPushSubscription();
  await addPushSubscription(owner, subscription, 'E2E-Gerät');
  await openSettings(page);
  const web = card(page, 'Web-Push');
  await expect(web.getByRole('list', { name: 'Angemeldete Geräte' })).toContainText('E2E-Gerät');
  await switchOn(page, 'Web-Push');

  await web.getByRole('button', { name: 'Testnachricht' }).click();
  await expect(web.getByText('Testnachricht gesendet.')).toBeVisible();
  await expect
    .poll(async () => (await pushes()).filter((push) => push.status === 201).length)
    .toBe(1);
  const [delivered] = (await pushes()).filter((push) => push.status === 201);
  expect(delivered?.error, 'the mock decrypts what the server encrypted').toBeNull();
  expect(delivered?.payload).toMatchObject({ title: expect.any(String) });

  // A subscription its push service reports gone (410) is deleted by the server.
  await addPushSubscription(owner, await newPushSubscription(true), 'Abgelaufen');
  await web.getByRole('button', { name: 'Testnachricht' }).click();
  await expect.poll(async () => (await pushes()).some((push) => push.status === 410)).toBe(true);
  await expect
    .poll(async () =>
      ((await channel(owner, 'push')).push?.subscriptions ?? []).map((entry) => entry.deviceName),
    )
    .toEqual(['E2E-Gerät']);
});

/** Copies of E-8 (S01E13, S01E14, S01E18, S01E19): numbers notifications.spec.ts does not use. */
function copyTarget(testInfo: TestInfo): string {
  const project = Math.max(0, ['chromium', 'firefox', 'webkit'].indexOf(testInfo.project.name));
  return String(13 + project * 5 + Math.min(testInfo.retry, 1)).padStart(2, '0');
}

test('E-8: a new episode of a favourite reaches the e-mail channel with the profile name', async ({
  owner,
}, testInfo) => {
  // Copy to mail: the scan, the dispatch through the job queue and the SMTP delivery (plan A-6).
  test.setTimeout(180_000);
  await setChannel(owner, 'email', { enabled: true, events: ['new_content'] });
  const series = await itemIdByTitle(owner, INTRO_SHOW_TITLE);
  await setFavorite(owner, series, true);

  const season = introSeason();
  const source = readdirSync(season).find((name) => /S01E01\.\w+$/.test(name));
  if (!source) throw new Error(`no S01E01 in ${season}`);
  const number = copyTarget(testInfo);
  const target = join(season, source.replace('S01E01', `S01E${number}`));
  // Bytes of its own at the end: the scan must not take the copy for a file of an earlier copy
  // that moved (equal size and partial hash, B-9).
  copyFileSync(join(season, source), target, constants.COPYFILE_EXCL);
  appendFileSync(target, `slouchcat-e2e ${testInfo.project.name} ${Date.now()}`);
  try {
    await expect
      .poll(async () => (await mailsTo(OWNER.email)).map((mail) => mail.subject), {
        timeout: 120_000,
        intervals: [2000],
      })
      .toContainEqual(expect.stringMatching(new RegExp(`^\\[${OWNER_PROFILE}\\] `)));
  } finally {
    rmSync(target, { force: true });
  }
});

test('keyboard: arrows to "Kanäle einstellen", Tab/Space/Enter on the cards, back; no trap', async ({
  page,
}) => {
  // The inbox is a browsing page: the arrows reach the link next to "Zurück".
  await page.goto('/notifications');
  await expect(page.getByRole('heading', { level: 1, name: 'Benachrichtigungen' })).toBeVisible();
  const link = page.getByRole('link', { name: 'Kanäle einstellen' });
  await resetFocus(page);
  await page.keyboard.press('ArrowDown');
  for (let presses = 0; presses < 3; presses += 1) {
    if (await link.evaluate((element) => element === document.activeElement)) break;
    await page.keyboard.press('ArrowRight');
  }
  await expectFocusOn(page, link);
  await page.keyboard.press('Enter');
  await expect(page).toHaveURL(/\/settings\/notifications$/);
  await expect(
    page.getByRole('heading', { level: 1, name: 'Benachrichtigungskanäle' }),
  ).toBeVisible();

  // E-mail card: Space switches, Tab reaches the address, the events and "Speichern".
  const mail = card(page, 'E-Mail');
  const toggle = mail.getByRole('switch', { name: 'Aktiv' });
  await resetFocus(page);
  await tabTo(page, toggle);
  await page.keyboard.press('Space');
  await expect(toggle).toBeChecked();
  await page.keyboard.press('Tab');
  await expectFocusOn(page, mail.getByLabel('E-Mail-Adresse'));
  // Arrows move the caret inside the field, not the focus.
  await page.keyboard.press('ArrowLeft');
  await expect(mail.getByLabel('E-Mail-Adresse')).toBeFocused();
  const newContent = mail.getByRole('checkbox', { name: 'Neue Inhalte' });
  await tabTo(page, newContent);
  await page.keyboard.press('Space');
  await expect(newContent).toBeChecked();
  await tabTo(page, mail.getByRole('button', { name: 'Speichern' }));
  await page.keyboard.press('Enter');
  await expect(mail.getByText('Gespeichert.')).toBeVisible();
  await page.keyboard.press('Tab');
  await expectFocusOn(page, mail.getByRole('button', { name: 'Testnachricht' }));

  // On through the other cards to the end of the page.
  await tabTo(page, card(page, 'Telegram').getByRole('button', { name: 'Verknüpfen' }), 60);
  await expectNoFocusTrap(page);

  // The browser's back returns to the inbox.
  await page.goBack();
  await expect(page).toHaveURL(/\/notifications$/);
  const stored = await channel(page.request, 'email');
  expect(stored).toMatchObject({ enabled: true });
  expect(stored.events).toContain('new_content');
});
