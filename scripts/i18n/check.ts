// Checks a locale file against web/src/locales/en.json (concept 22, 4.9).
//
//   node scripts/i18n/check.ts <code> [--file <path>] [--strict] [--verbose]
//   node scripts/i18n/check.ts <code> --missing      list the keys still to translate
//   node scripts/i18n/check.ts --all [--strict]      every shipped language but en (languages.ts)
//   node scripts/i18n/check.ts --all --quiet         errors only (CI and check:fast): structure
//                                                    errors fail, missing texts are counted
//   node scripts/i18n/check.ts --all --missing       missing count per shipped language
//
// Errors (exit 1): keys en.json does not have, plural forms that are not exactly the language's
// categories (Intl.PluralRules, plus `_zero` where en has it), placeholders, `$t()` nestings or
// <Trans> markup that differ from English, empty texts, and a missing number in a plural form
// that covers more than 1, and leading or trailing white space that differs from English (" · ",
// " or "; ja, zh-CN and zh-TW are exempt, their separators go without spaces: "、"; Korean writes
// spaces between words and is not). Missing
// keys are warnings, errors with --strict (the release gate).
// Warnings: text identical to English, labels longer than 1.5× English.
//
// `de` is checked like any language. `de-DE` is an overlay (4.2): only keys de.json has, and
// every text must differ from de.json.
import { existsSync, readdirSync } from 'node:fs';
import { basename } from 'node:path';
import { fileURLToPath } from 'node:url';
import { supportedLanguages } from '../../web/src/i18n/languages.ts';
import {
  flatten,
  groups,
  localePath,
  localesDir,
  markup,
  nestings,
  oneCoversMore,
  parseArgs,
  placeholders,
  readJson,
  requiredForms,
  splitPlural,
  width,
  type Group,
} from './lib.ts';

type Report = { errors: string[]; missing: string[]; identical: string[]; long: string[] };

const enEntries = flatten(readJson(localePath('en')));
const enGroups = groups(enEntries);
const enText = new Map(enEntries);

// Leaf names that hold labels, not sentences, when the English text is short too.
const sentenceKey = /(intro|lead|hint|help|body|text|description|note|summary|explain|warning)/i;
const looksLikeLabel = (key: string, en: string) =>
  width(en) <= 30 && !/[.!?:]$/.test(en.trim()) && !sentenceKey.test(key.split('.').pop()!);

function sameList(a: string[], b: string[]): boolean {
  return a.length === b.length && a.every((value, i) => value === b[i]);
}

/** Languages whose separators and joining words go without spaces around them ("、", "または"). */
const ownSpacing = new Set(['ja', 'zh-CN', 'zh-TW']);

/** Whether the text starts and ends with white space like English (" · ", ", ", " › "). */
function sameEdges(text: string, en: string): boolean {
  return /^\s/.test(text) === /^\s/.test(en) && /\s$/.test(text) === /\s$/.test(en);
}

function sameMarkup(a: Map<string, number>, b: Map<string, number>): boolean {
  return a.size === b.size && [...a].every(([name, n]) => b.get(name) === n);
}

function compareText(
  code: string,
  key: string,
  text: string,
  en: string,
  report: Report,
  enForPlaceholders = en,
  why = '',
) {
  // In ja, zh-CN and zh-TW " · " may lose its spaces, never its mark (an empty text).
  if (text.trim() === '') {
    report.errors.push(`${key}: empty text`);
    return;
  }
  if (!ownSpacing.has(code) && !sameEdges(text, en)) {
    report.errors.push(
      `${key}: white space at the start or end differs from English (${JSON.stringify(text)}, English ${JSON.stringify(en)})`,
    );
  }
  const want = placeholders(enForPlaceholders);
  const got = placeholders(text);
  if (!sameList(got, want)) {
    report.errors.push(`${key}: placeholders ${fmt(got)}, needs ${fmt(want)}${why}`);
  }
  if (!sameList(nestings(text), nestings(en))) report.errors.push(`${key}: $t() nestings differ`);
  if (!sameMarkup(markup(text), markup(en))) {
    report.errors.push(
      `${key}: markup ${fmtTags(markup(text))}, English has ${fmtTags(markup(en))}`,
    );
  }
  if (text === en && /\p{L}{2,}/u.test(en.replace(/\{\{[^}]*\}\}/g, '')))
    report.identical.push(key);
  if (looksLikeLabel(key, en) && width(text) > 1.5 * width(en) && width(text) - width(en) >= 4) {
    report.long.push(`${key}: ${width(text)} vs ${width(en)} (“${text}” / “${en}”)`);
  }
}

const fmt = (list: string[]) => (list.length ? list.map((p) => `{{${p}}}`).join(' ') : 'none');
const fmtTags = (tags: Map<string, number>) =>
  tags.size ? [...tags].map(([name, n]) => `<${name}>×${n}`).join(' ') : 'none';

/** Placeholders a plural form must carry, taken from the English form it stands for. */
function placeholderSource(code: string, group: Group, form: string): string {
  const forms = group.forms!;
  const other = forms.get('other') ?? [...forms.values()].at(-1)!;
  // A category that covers more than 1 must show the number, even where en's `_one` spells "1".
  if (form === 'one' && oneCoversMore(code)) return other;
  return forms.get(form) ?? other;
}

export function checkLocale(code: string, target: Map<string, string>): Report {
  const report: Report = { errors: [], missing: [], identical: [], long: [] };
  const seen = new Set<string>();
  for (const group of enGroups) {
    if (!group.forms) {
      seen.add(group.base);
      const text = target.get(group.base);
      if (text === undefined) report.missing.push(group.base);
      else compareText(code, group.base, text, group.text!, report);
      continue;
    }
    const required = requiredForms(code, group.forms);
    // The bare key stands for `_one` where en.json uses it that way (see lib.ts, Group).
    const formKey = (form: string) =>
      form === 'one' && group.bare && !target.has(`${group.base}_one`) && target.has(group.base)
        ? group.base
        : `${group.base}_${form}`;
    const present = required.filter((form) => target.has(formKey(form)));
    for (const key of target.keys()) {
      const { base, form } = splitPlural(key);
      if (base === group.base && form) seen.add(key);
    }
    if (present.length === 0) {
      report.missing.push(group.base);
      continue;
    }
    for (const form of required) {
      const key = formKey(form);
      const text = target.get(key);
      if (text === undefined) {
        report.errors.push(`${key}: plural form missing (${code} needs ${required.join(', ')})`);
        continue;
      }
      const en = group.forms.get(form) ?? group.forms.get('other')!;
      const source = placeholderSource(code, group, form);
      const why =
        source !== en ? ` ("${form}" covers more than 1 in ${code}: like en "other")` : '';
      compareText(code, key, text, en, report, source, why);
    }
    for (const key of target.keys()) {
      const { base, form } = splitPlural(key);
      if (base === group.base && form && !required.includes(form)) {
        report.errors.push(`${key}: ${code} has no plural form "${form}" (${required.join(', ')})`);
      }
    }
    if (group.bare && target.has(group.base) && !required.includes('one')) {
      report.errors.push(`${group.base}: ${code} has no form "one"; give _other only`);
    }
  }
  for (const key of target.keys()) {
    if (!seen.has(key) && !enText.has(key)) report.errors.push(`${key}: not in en.json`);
  }
  return report;
}

function checkOverlay(target: Map<string, string>): Report {
  const report: Report = { errors: [], missing: [], identical: [], long: [] };
  const de = new Map(flatten(readJson(localePath('de'))));
  for (const [key, text] of target) {
    const base = de.get(key);
    if (base === undefined) report.errors.push(`${key}: not in de.json`);
    else if (text === base)
      report.errors.push(`${key}: same as de.json (drop it from the overlay)`);
    else compareText('de-DE', key, text, base, report);
  }
  report.identical = [];
  return report;
}

function load(code: string, file?: string): Map<string, string> {
  return new Map(flatten(readJson(file ?? localePath(code))));
}

function run(
  code: string,
  file: string | undefined,
  strict: boolean,
  verbose: boolean,
  quiet: boolean,
): boolean {
  const target = load(code, file);
  const report = code === 'de-DE' ? checkOverlay(target) : checkLocale(code, target);
  if (code === 'de' && [...target.values()].some((text) => text.includes('ß'))) {
    report.errors.push('de.json uses ß; Swiss spelling writes ss');
  }
  const errors = strict
    ? [...report.errors, ...report.missing.map((key) => `${key}: missing`)]
    : report.errors;
  const limit = verbose ? Infinity : 20;
  const list = (title: string, items: string[]) => {
    if (items.length === 0) return;
    console.log(`  ${title}: ${items.length}`);
    for (const item of items.slice(0, limit)) console.log(`    ${item}`);
    if (items.length > limit) console.log(`    … ${items.length - limit} more (--verbose)`);
  };
  if (quiet) {
    const missing = strict ? '' : `, ${report.missing.length} missing`;
    if (errors.length > 0 || report.missing.length > 0)
      console.log(`${code}: ${errors.length} errors${missing}`);
    list('errors', errors);
    return errors.length === 0;
  }
  console.log(`${code}: ${target.size} texts, ${errors.length} errors`);
  list('errors', errors);
  if (!strict) list('missing (warning)', report.missing);
  list('identical to English (warning)', report.identical);
  list('longer than 1.5× English (warning)', report.long);
  return errors.length === 0;
}

function missingKeys(code: string, file?: string): string[] {
  return code === 'de-DE' ? [] : checkLocale(code, load(code, file)).missing;
}

// Run as a script, not when another module imports this one.
if (process.argv[1] === fileURLToPath(import.meta.url)) {
  const { flags, positional } = parseArgs(process.argv.slice(2), ['file']);
  const file = flags.get('file');
  const path = typeof file === 'string' ? file : undefined;
  // --all checks what the app ships (web/src/i18n/languages.ts), not what happens to lie in the
  // folder: a shipped language without its file is an error, a file of no shipped language a note.
  const codes = flags.has('all')
    ? supportedLanguages.filter((code) => code !== 'en')
    : positional.slice(0, 1);
  if (flags.has('all')) {
    const unshipped = readdirSync(localesDir)
      .filter((name) => name.endsWith('.json'))
      .map((name) => basename(name, '.json'))
      .filter((code) => !(supportedLanguages as readonly string[]).includes(code));
    if (unshipped.length > 0) console.log(`not shipped, not checked: ${unshipped.join(', ')}`);
    const absent = codes.filter((code) => !existsSync(localePath(code)));
    if (absent.length > 0) {
      for (const code of absent)
        console.error(`${code}: shipped, but web/src/locales/${code}.json is missing`);
      process.exit(1);
    }
  }
  if (codes.length === 0) {
    console.error(
      'usage: check.ts <code> [--file <path>] [--strict] [--missing] [--verbose] | --all',
    );
    process.exit(2);
  }
  for (const code of codes) {
    try {
      new Intl.PluralRules(code);
    } catch {
      console.error(`${code}: not a language code Intl knows`);
      process.exit(2);
    }
  }
  if (flags.has('missing')) {
    for (const code of codes) {
      const missing = missingKeys(code, path);
      if (flags.has('all')) console.log(`${code}: ${missing.length} missing`);
      else for (const key of missing) console.log(key);
    }
    process.exit(0);
  }
  let ok = true;
  for (const code of codes) {
    ok = run(code, path, flags.has('strict'), flags.has('verbose'), flags.has('quiet')) && ok;
  }
  if (flags.has('quiet') && ok) console.log(`i18n: ${codes.length} language files, no errors`);
  process.exit(ok ? 0 : 1);
}
