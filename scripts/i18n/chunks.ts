// Working files for the translation agents (concept 22, 4.6): split en.json and de.json into
// chunks of one area each, and merge translated chunks back into one locale file.
//
//   node scripts/i18n/chunks.ts split <outDir> [--lang <code>] [--max 250] [--missing-only]
//                                              [--no-context]
//   node scripts/i18n/chunks.ts merge <code> <translatedDir> [--out <file>] [--fresh]
//
// split writes <outDir>/<nn>-<area>.json (one top-level area each; an area above --max texts is cut
// along its sub-areas into parts of at most --max) and <outDir>/index.json. Each chunk lists its
// texts with English, German and where they are used; with --lang it also names the plural forms
// that language needs, and --missing-only leaves out what web/src/locales/<code>.json already has.
//
// A translator returns, per chunk, a JSON object of the same name in <translatedDir>, flat
// ({"area.key": "text", "area.plural_few": "text"}) or nested; merge reads every *.json there,
// lays it over the existing locale file (none with --fresh), and writes it in en.json key order.
// Keys en.json does not have, or plural forms the language lacks, stop the merge.
import { mkdirSync, readdirSync, writeFileSync, existsSync } from 'node:fs';
import { join } from 'node:path';
import { fileURLToPath } from 'node:url';
import * as prettier from 'prettier';
import { collectUsage, type Use } from './context.ts';
import {
  flatten,
  groups,
  localePath,
  localesDir,
  parseArgs,
  readJson,
  requiredForms,
  splitPlural,
  unflatten,
  type Group,
  type Tree,
} from './lib.ts';

type Entry =
  | { en: string; de: string; used: string[] }
  | {
      plural: true;
      bare?: true;
      forms?: string[];
      en: Record<string, string>;
      de: Record<string, string>;
      used: string[];
    };

const enGroups = groups(flatten(readJson(localePath('en'))));
const deText = new Map(flatten(readJson(localePath('de'))));

/** Cuts groups (in en.json order) into parts of at most `max` texts along the key hierarchy. */
function cut(items: Group[], depth: number, max: number): Group[][] {
  const size = (list: Group[]) => list.reduce((n, g) => n + (g.forms?.size ?? 1), 0);
  if (size(items) <= max) return [items];
  // Children in order of first appearance; a leaf directly at this level is its own child.
  const children = new Map<string, Group[]>();
  for (const group of items) {
    const parts = group.base.split('.');
    const name = parts.length > depth + 1 ? parts[depth]! : `\0${group.base}`;
    children.set(name, [...(children.get(name) ?? []), group]);
  }
  if (children.size === 1) {
    // One sub-area holds everything: go deeper, or, at the leaves, cut the list in order.
    const only = [...children.values()][0]!;
    if (only.some((g) => g.base.split('.').length > depth + 2)) return cut(only, depth + 1, max);
    const parts: Group[][] = [[]];
    for (const group of only) {
      const last = parts.at(-1)!;
      if (last.length && size(last) + (group.forms?.size ?? 1) > max) parts.push([group]);
      else last.push(group);
    }
    return parts;
  }
  // Pack consecutive children into parts; a child above `max` is cut on its own.
  const out: Group[][] = [];
  let part: Group[] = [];
  for (const child of children.values()) {
    if (size(child) > max) {
      if (part.length) out.push(part);
      part = [];
      out.push(...cut(child, depth + 1, max));
      continue;
    }
    if (part.length && size(part) + size(child) > max) {
      out.push(part);
      part = [];
    }
    part.push(...child);
  }
  if (part.length) out.push(part);
  return out;
}

function usedText(uses: Use[] | undefined): string[] {
  if (!uses) return [];
  const lines = uses.map(
    (u) => `${u.file}:${u.line} ${u.component}${u.dynamic ? ' (built key)' : ''}`,
  );
  return lines.length > 4 ? [...lines.slice(0, 4), `… ${lines.length - 4} more`] : lines;
}

function split(
  outDir: string,
  lang: string | undefined,
  max: number,
  missingOnly: boolean,
  withContext: boolean,
) {
  const existing =
    lang && missingOnly && existsSync(localePath(lang))
      ? new Set(flatten(readJson(localePath(lang))).map(([key]) => key))
      : new Set<string>();
  const usage = withContext ? collectUsage() : new Map<string, Use[]>();
  const todo = enGroups.filter((group) => {
    if (!group.forms) return !existing.has(group.base);
    const need = lang ? requiredForms(lang, group.forms) : [...group.forms.keys()];
    const has = (form: string) =>
      existing.has(`${group.base}_${form}`) ||
      (form === 'one' && !!group.bare && existing.has(group.base));
    return need.some((form) => !has(form));
  });

  const areas = new Map<string, Group[]>();
  for (const group of todo) {
    const area = group.base.split('.')[0]!;
    areas.set(area, [...(areas.get(area) ?? []), group]);
  }
  mkdirSync(outDir, { recursive: true });
  const index: { file: string; area: string; prefixes: string[]; texts: number }[] = [];
  let n = 0;
  for (const [area, items] of areas) {
    const parts = cut(items, 1, max);
    parts.forEach((part, i) => {
      const name = parts.length > 1 ? `${area}-${String(i + 1).padStart(2, '0')}` : area;
      const file = `${String(++n).padStart(2, '0')}-${name}.json`;
      const texts: Record<string, Entry> = {};
      const prefixes = new Set<string>();
      for (const group of part) {
        prefixes.add(group.base.split('.').slice(0, 2).join('.'));
        if (!group.forms) {
          texts[group.base] = {
            en: group.text!,
            de: deText.get(group.base) ?? '',
            used: usedText(usage.get(group.base)),
          };
          continue;
        }
        const en = Object.fromEntries(group.forms);
        const deKey = (form: string) =>
          form === 'one' && group.bare && !deText.has(`${group.base}_one`)
            ? group.base
            : `${group.base}_${form}`;
        const de = Object.fromEntries(
          [...group.forms.keys()].map((form) => [form, deText.get(deKey(form)) ?? '']),
        );
        const first = group.bare ? group.base : `${group.base}_${[...group.forms.keys()][0]}`;
        texts[group.base] = {
          plural: true,
          // en.json keeps "one" under the key without suffix; translations give <key>_one.
          ...(group.bare ? { bare: true as const } : {}),
          ...(lang ? { forms: requiredForms(lang, group.forms) } : {}),
          en,
          de,
          used: usedText(usage.get(first)),
        };
      }
      const count = part.reduce((sum, g) => sum + (g.forms?.size ?? 1), 0);
      const chunk = {
        chunk: name,
        ...(lang ? { lang } : {}),
        returns: `${file}: {"<key>": "<text>"}, plural keys as "<key>_<form>" for each of "forms"`,
        texts,
      };
      writeFileSync(join(outDir, file), `${JSON.stringify(chunk, null, 2)}\n`);
      index.push({ file, area, prefixes: [...prefixes], texts: count });
    });
  }
  writeFileSync(join(outDir, 'index.json'), `${JSON.stringify(index, null, 2)}\n`);
  const total = index.reduce((sum, c) => sum + c.texts, 0);
  console.error(`${index.length} chunks, ${total} English texts, in ${outDir}`);
}

async function merge(code: string, dir: string, out: string, fresh: boolean) {
  const translated = new Map<string, string>();
  const files = readdirSync(dir).filter((name) => name.endsWith('.json') && name !== 'index.json');
  for (const name of files.sort()) {
    const data = readJson<Tree>(join(dir, name));
    for (const [key, text] of flatten(data)) {
      if (translated.has(key) && translated.get(key) !== text) {
        throw new Error(`${key}: two different texts (second in ${name})`);
      }
      translated.set(key, text);
    }
  }

  const plain = new Set<string>();
  const plural = new Map<string, string[]>();
  for (const group of enGroups) {
    if (!group.forms) plain.add(group.base);
    else {
      const forms = requiredForms(code, group.forms);
      plural.set(group.base, forms);
      // A bare key may stand for `_one` (lib.ts, Group); keep it where the language has "one".
      if (group.bare && forms.includes('one')) plain.add(group.base);
    }
  }
  const problems: string[] = [];
  for (const key of translated.keys()) {
    if (plain.has(key)) continue;
    const { base, form } = splitPlural(key);
    const forms = plural.get(base);
    if (!forms) problems.push(`${key}: not in en.json`);
    else if (!form || !forms.includes(form)) {
      problems.push(`${key}: ${code} needs ${forms.map((f) => `${base}_${f}`).join(', ')}`);
    }
  }
  if (problems.length) {
    for (const problem of problems) console.error(problem);
    throw new Error(`${problems.length} keys do not fit ${code}; nothing written`);
  }

  const base =
    !fresh && existsSync(out) ? new Map(flatten(readJson(out))) : new Map<string, string>();
  const merged = new Map([...base, ...translated]);
  const ordered: [string, string][] = [];
  let stale = 0;
  for (const group of enGroups) {
    if (!group.forms) {
      const text = merged.get(group.base);
      if (text !== undefined) ordered.push([group.base, text]);
      continue;
    }
    const bare = group.bare ? merged.get(group.base) : undefined;
    if (bare !== undefined && plain.has(group.base)) ordered.push([group.base, bare]);
    for (const form of plural.get(group.base)!) {
      const text = merged.get(`${group.base}_${form}`);
      if (text !== undefined) ordered.push([`${group.base}_${form}`, text]);
    }
  }
  const kept = new Set(ordered.map(([key]) => key));
  for (const key of merged.keys()) if (!kept.has(key)) stale++;
  // Same layout as the files Prettier keeps in web/src/locales.
  const options = (await prettier.resolveConfig(join(localesDir, 'en.json'))) ?? {};
  const json = JSON.stringify(unflatten(ordered), null, 2);
  writeFileSync(out, await prettier.format(json, { ...options, parser: 'json' }));
  console.error(
    `${out}: ${ordered.length} texts (${translated.size} from ${files.length} ` +
      `files${stale ? `, ${stale} no longer in en.json dropped` : ''})`,
  );
}

// Run as a script, not when another module imports this one.
if (process.argv[1] === fileURLToPath(import.meta.url)) {
  const { flags, positional } = parseArgs(process.argv.slice(2), ['lang', 'max', 'out']);
  const [command, ...rest] = positional;
  const value = (name: string) => {
    const v = flags.get(name);
    return typeof v === 'string' ? v : undefined;
  };
  try {
    if (command === 'split' && rest.length === 1) {
      const max = Number(value('max') ?? 250);
      if (!Number.isInteger(max) || max < 10) throw new Error('--max must be a whole number ≥ 10');
      split(rest[0]!, value('lang'), max, flags.has('missing-only'), !flags.has('no-context'));
    } else if (command === 'merge' && rest.length === 2) {
      const [code, dir] = rest as [string, string];
      await merge(code, dir, value('out') ?? localePath(code), flags.has('fresh'));
    } else {
      console.error(
        'usage: chunks.ts split <outDir> [--lang <code>] [--max 250] [--missing-only] [--no-context]\n' +
          '       chunks.ts merge <code> <translatedDir> [--out <file>] [--fresh]',
      );
      process.exit(2);
    }
  } catch (error) {
    console.error((error as Error).message);
    process.exit(1);
  }
}
