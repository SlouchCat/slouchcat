import { existsSync, readFileSync } from 'node:fs';
import { BACKUP_PASSPHRASE, EXTERNAL_BASE_URL, OWNER, SERVER_LOG } from './env';
import { scanLog, scannedLines } from './log-scan';
import { pushSubscriptionSecrets } from './mocks';
import { SMTP_PASSWORD } from './smtp-server.mjs';
import { TELEGRAM_BOT_TOKEN } from './telegram-server.mjs';

/**
 * A-9: no secret in the logs. After the last spec (the server still runs), the server's log of the
 * whole run (start-server.mjs tees it into node_modules/.playwright/server.log) is scanned for
 *
 * - the fake secrets of the run: the SMTP password, the Telegram bot token (whole and its secret
 *   part), the main account's password, the backup passphrase, and the auth secrets and endpoint
 *   ids of the push subscriptions the push mock handed out (capability URLs), and
 * - patterns of secrets whatever their value (log-scan.ts): stream and download tokens, API keys,
 *   bearer tokens, bot API paths, invitation links and gitleaks-like `password=…` assignments.
 *
 * The specs put these secrets in play where a leak would show: the e-mail spec sends with the SMTP
 * password and has one AUTH refused, the Telegram spec has one bot request fail with the token in
 * its URL, every spec logs in with the account password. Any hit fails the run, with the line
 * numbers and the rule (the secret itself masked). Against a server of your own
 * (SLOUCHCAT_E2E_BASE_URL) there is no log to scan.
 */
export default async function globalTeardown(): Promise<void> {
  if (EXTERNAL_BASE_URL) {
    console.log('A-9 log scan: skipped, the server runs outside the suite');
    return;
  }
  if (!existsSync(SERVER_LOG)) throw new Error(`A-9 log scan: ${SERVER_LOG} is missing`);
  const log = readFileSync(SERVER_LOG, 'utf8');
  const secrets: Record<string, string> = {
    smtpPassword: SMTP_PASSWORD,
    telegramBotToken: TELEGRAM_BOT_TOKEN,
    telegramBotSecret: TELEGRAM_BOT_TOKEN.split(':')[1] ?? TELEGRAM_BOT_TOKEN,
    ownerPassword: OWNER.password,
    backupPassphrase: BACKUP_PASSPHRASE,
  };
  for (const [index, subscription] of (await pushSubscriptionSecrets()).entries()) {
    secrets[`pushAuth${index}`] = subscription.auth;
    secrets[`pushEndpoint${index}`] = subscription.id;
  }
  const findings = scanLog(log, secrets);
  console.log(
    `A-9 log scan: ${scannedLines(log)} lines of ${SERVER_LOG}, ` +
      `${Object.keys(secrets).length} secrets of the run, ${findings.length} findings`,
  );
  if (findings.length > 0) {
    const lines = findings.map(
      (finding) => `  line ${finding.line} (${finding.rule}): ${finding.excerpt}`,
    );
    throw new Error(`A-9: secrets in the server log\n${lines.join('\n')}`);
  }
}
