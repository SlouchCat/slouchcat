// Shared helpers of the translation tools (concept 22, stage 4). The locale files are i18next v4
// JSON: nested objects with string leaves, plural forms as `<key>_<category>`.
import { readFileSync } from 'node:fs';
import { dirname, join, resolve } from 'node:path';
import { fileURLToPath } from 'node:url';

export const repoRoot = resolve(dirname(fileURLToPath(import.meta.url)), '../..');
export const localesDir = join(repoRoot, 'web/src/locales');

export type Tree = { [key: string]: string | Tree };

/** CLDR plural categories in the order i18next and Weblate list them. */
export const pluralOrder = ['zero', 'one', 'two', 'few', 'many', 'other'] as const;
const pluralSuffix = /_(zero|one|two|few|many|other)$/;

export function readJson<T = Tree>(path: string): T {
  return JSON.parse(readFileSync(path, 'utf8')) as T;
}

export function localePath(code: string): string {
  return join(localesDir, `${code}.json`);
}

/** Leaves of a locale tree in file order, as dotted keys. */
export function flatten(tree: Tree, prefix = ''): [string, string][] {
  const out: [string, string][] = [];
  for (const [key, value] of Object.entries(tree)) {
    const path = prefix ? `${prefix}.${key}` : key;
    if (typeof value === 'string') out.push([path, value]);
    else if (value !== null && typeof value === 'object') out.push(...flatten(value, path));
    else throw new Error(`${path}: leaf is ${JSON.stringify(value)}, not a string`);
  }
  return out;
}

/** Rebuilds the nested tree from dotted keys; insertion order is kept. */
export function unflatten(entries: Iterable<[string, string]>): Tree {
  const root: Tree = {};
  for (const [key, value] of entries) {
    const parts = key.split('.');
    let node = root;
    for (const part of parts.slice(0, -1)) {
      const child = node[part];
      if (typeof child === 'string') throw new Error(`${key}: ${part} is a text and a group`);
      node = child ?? (node[part] = {});
    }
    node[parts[parts.length - 1]!] = value;
  }
  return root;
}

/** Splits `a.b_one` into base `a.b` and form `one`; keys without a plural suffix have no form. */
export function splitPlural(key: string): { base: string; form?: string } {
  const match = pluralSuffix.exec(key);
  return match ? { base: key.slice(0, match.index), form: match[1] } : { base: key };
}

/**
 * The source keys as groups: a plain text, or a plural base with its English forms. Order is the
 * order of en.json; a plural group sits where its first form (or its bare key) is.
 *
 * `bare`: en.json keeps the singular under the key without suffix (`admin.home.browse.hanging`
 * next to `hanging_other`). i18next falls back to it when `_one` is missing, so it counts as the
 * `one` form; a translation may give `_one` or the bare key.
 */
export type Group = { base: string; forms?: Map<string, string>; text?: string; bare?: true };

export function groups(entries: [string, string][]): Group[] {
  const out: Group[] = [];
  const byKey = new Map<string, Group>();
  const pluralBases = new Set(
    entries
      .map(([key]) => splitPlural(key))
      .filter((k) => k.form)
      .map((k) => k.base),
  );
  for (const [key, text] of entries) {
    const { base, form } = splitPlural(key);
    if (!form && !pluralBases.has(key)) {
      out.push({ base: key, text });
      continue;
    }
    let group = byKey.get(base);
    if (!group) {
      group = { base, forms: new Map() };
      byKey.set(base, group);
      out.push(group);
    }
    if (form) group.forms!.set(form, text);
    else {
      group.bare = true;
      if (!group.forms!.has('one')) group.forms!.set('one', text);
    }
  }
  // Forms in CLDR order, whatever order the file has.
  for (const group of out) {
    if (group.forms)
      group.forms = new Map(
        pluralOrder.filter((f) => group.forms!.has(f)).map((f) => [f, group.forms!.get(f)!]),
      );
  }
  return out;
}

/** The plural categories a language needs, in CLDR order. */
export function pluralCategories(code: string): string[] {
  const categories = new Intl.PluralRules(code).resolvedOptions().pluralCategories as string[];
  return pluralOrder.filter((category) => categories.includes(category));
}

/** Categories that some whole number from 2 up hits, plus `other` (i18next's last resort). */
export function categoriesFromTwo(code: string): Set<string> {
  const rules = new Intl.PluralRules(code);
  const hit = new Set<string>(['other']);
  for (let n = 2; n <= 1000; n++) hit.add(rules.select(n));
  hit.add(rules.select(1_000_000));
  return hit;
}

/** True when the `one` category covers more than the number 1 (fr and pt-BR: 0; ru: 21, 31 …). */
export function oneCoversMore(code: string): boolean {
  const rules = new Intl.PluralRules(code);
  for (let n = 0; n <= 1000; n++) if (n !== 1 && rules.select(n) === 'one') return true;
  return false;
}

/** The forms a translated plural group must have: the language's categories, `zero` if en has it. */
export function requiredForms(code: string, enForms: Map<string, string>): string[] {
  let categories = pluralCategories(code);
  // A text en only has as `_other` is never shown for 1 (admin.home.browse.hanging); the language
  // still needs every category that whole numbers from 2 up reach.
  if (!enForms.has('one')) {
    const fromTwo = categoriesFromTwo(code);
    categories = categories.filter((category) => fromTwo.has(category));
  }
  return enForms.has('zero') ? ['zero', ...categories] : categories;
}

/** Placeholders like `{{count, number}}`, normalised without spaces, sorted, duplicates kept. */
export function placeholders(text: string): string[] {
  return [...text.matchAll(/\{\{([^{}]+)\}\}/g)].map((m) => m[1]!.replace(/\s+/g, '')).sort();
}

/** Nested translations `$t(key)`, sorted. */
export function nestings(text: string): string[] {
  return [...text.matchAll(/\$t\(([^)]*)\)/g)].map((m) => m[1]!.trim()).sort();
}

/**
 * Markup of `<Trans>` components: tag names that open and close in the text (`<path>…</path>`),
 * with counts. Lone brackets such as `<deploy-folder>` or `<media@example.ch>` are text.
 */
export function markup(text: string): Map<string, number> {
  const opens = new Map<string, number>();
  const closes = new Set<string>();
  for (const m of text.matchAll(/<(\/?)([A-Za-z][\w-]*)\s*>/g)) {
    if (m[1]) closes.add(m[2]!);
    else opens.set(m[2]!, (opens.get(m[2]!) ?? 0) + 1);
  }
  return new Map([...opens].filter(([name]) => closes.has(name)));
}

/** Display width: East Asian wide and full-width characters count twice. */
export function width(text: string): number {
  let w = 0;
  for (const ch of text) {
    const cp = ch.codePointAt(0)!;
    const wide =
      (cp >= 0x1100 && cp <= 0x115f) ||
      (cp >= 0x2e80 && cp <= 0xa4cf) ||
      (cp >= 0xac00 && cp <= 0xd7a3) ||
      (cp >= 0xf900 && cp <= 0xfaff) ||
      (cp >= 0xfe30 && cp <= 0xfe4f) ||
      (cp >= 0xff00 && cp <= 0xff60) ||
      (cp >= 0xffe0 && cp <= 0xffe6);
    w += wide ? 2 : 1;
  }
  return w;
}

/** Command-line flags: `--name value`, `--flag`, and the remaining positional arguments. */
export function parseArgs(argv: string[], valued: string[]) {
  const flags = new Map<string, string | true>();
  const positional: string[] = [];
  for (let i = 0; i < argv.length; i++) {
    const arg = argv[i]!;
    if (!arg.startsWith('--')) {
      positional.push(arg);
      continue;
    }
    const [name, inline] = arg.slice(2).split('=', 2) as [string, string | undefined];
    if (inline !== undefined) flags.set(name, inline);
    else if (valued.includes(name)) {
      const value = argv[++i];
      if (value === undefined) throw new Error(`--${name} needs a value`);
      flags.set(name, value);
    } else flags.set(name, true);
  }
  return { flags, positional };
}
