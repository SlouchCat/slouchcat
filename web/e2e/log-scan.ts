// The secret scan of the server log (A-9), without file or network access so it can be tried on
// any text: global-teardown.ts feeds it the log of the run and the fake secrets of the run.

/**
 * Patterns that mean a secret reached the log in clear, whatever its value. Each one leaves
 * `[redacted]` alone, the server's mask (internal/logging Redacted).
 *
 * - `st=` (stream token, plan 5.4) and `dt=` (download token, M5) in a query string or URL
 * - `api_key=` / `apikey=` (TMDB, TVDB, YouTube)
 * - `Bearer <token>` (an Authorization header)
 * - `/bot<id>:<secret>` (the Telegram bot token in a bot API URL, M8)
 * - `/invitations/<token>` and `/invite/<token>` (invitation links, K-6)
 * - gitleaks-like: `token`, `secret`, `password`, `passphrase` or `api key` followed by `=`, `:`,
 *   a quote or a space and 16 or more token characters; `key=` followed by 32 or more
 */
export const SECRET_PATTERNS: readonly { rule: string; pattern: RegExp }[] = [
  { rule: 'stream token', pattern: /(?:^|[?&;\s"'])st=(?!\[redacted\])[^&;#\s"']+/ },
  { rule: 'download token', pattern: /(?:^|[?&;\s"'])dt=(?!\[redacted\])[^&;#\s"']+/ },
  { rule: 'api key parameter', pattern: /(?:^|[^A-Za-z0-9])api_?key=(?!\[redacted\])[^&;#\s"']+/i },
  { rule: 'bearer token', pattern: /\bBearer\s+(?!\[redacted\])[A-Za-z0-9._~+/-]{16,}/i },
  { rule: 'telegram bot token', pattern: /\/bot\d+:[A-Za-z0-9_-]+/ },
  {
    rule: 'invitation token',
    pattern: /\/(?:invitations|invite)\/(?!\[redacted\])[A-Za-z0-9_-]{16,}/i,
  },
  {
    rule: 'credential assignment',
    pattern:
      /(?:token|secret|password|passphrase|api[_-]?key)["'=: ]+(?!\[redacted\])[A-Za-z0-9_-]{16,}/i,
  },
  { rule: 'key assignment', pattern: /(?:^|[^A-Za-z0-9_])key=[A-Za-z0-9_-]{32,}/ },
];

export interface Finding {
  /** 1-based line number in the log. */
  line: number;
  rule: string;
  /** The line with every known secret masked, at most 200 characters. */
  excerpt: string;
}

/**
 * Every line of `log` that holds one of `secrets` (exact values of this run, named by key) or
 * matches a secret pattern. Secrets shorter than 8 characters are ignored (they would match by
 * accident).
 */
export function scanLog(log: string, secrets: Record<string, string>): Finding[] {
  const known = Object.entries(secrets).filter(([, value]) => value.length >= 8);
  const mask = (text: string) =>
    known.reduce((masked, [name, value]) => masked.split(value).join(`<${name}>`), text);
  const findings: Finding[] = [];
  log.split('\n').forEach((text, index) => {
    if (text === '') return;
    const add = (rule: string) =>
      findings.push({ line: index + 1, rule, excerpt: mask(text).slice(0, 200) });
    for (const [name, value] of known) {
      if (text.includes(value)) add(`secret ${name}`);
    }
    for (const { rule, pattern } of SECRET_PATTERNS) {
      if (pattern.test(text)) add(rule);
    }
  });
  return findings;
}

/** Lines of `log` the scan looked at (not empty). */
export const scannedLines = (log: string) => log.split('\n').filter((line) => line !== '').length;
