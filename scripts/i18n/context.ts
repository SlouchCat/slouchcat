// Where each text of web/src/locales/en.json is used, so a translation knows whether "Play" is a
// button, a heading or a state (concept 22, 4.6). Best effort: string literals that name a key
// count as exact uses; template literals and literal prefixes ("kind.", `kind.${x}`) count as
// dynamic uses for every key they can build.
//
//   node scripts/i18n/context.ts [--out <file>] [--unresolved]
//
// Writes { "<key>": [{ file, line, component, dynamic? }] } in en.json order (stdout without
// --out); keys with no use get []. --unresolved prints the keys without any use instead.
// The counts go to stderr.
import { readdirSync, readFileSync, writeFileSync } from 'node:fs';
import { join, relative } from 'node:path';
import { fileURLToPath } from 'node:url';
import { flatten, localePath, parseArgs, readJson, repoRoot, splitPlural } from './lib.ts';

export type Use = { file: string; line: number; component: string; dynamic?: true };

const sourceRoot = join(repoRoot, 'web/src');
const skipped = /\.(test|stories)\.tsx?$|\.d\.ts$/;

function sourceFiles(dir: string): string[] {
  return readdirSync(dir, { withFileTypes: true }).flatMap((entry) => {
    const path = join(dir, entry.name);
    if (entry.isDirectory()) return entry.name === 'locales' ? [] : sourceFiles(path);
    return /\.tsx?$/.test(entry.name) && !skipped.test(entry.name) ? [path] : [];
  });
}

// String literals on one line ('…', "…") and template literals (`…`, may span lines).
const literal = /'((?:[^'\\\n]|\\.)*)'|"((?:[^"\\\n]|\\.)*)"|`((?:[^`\\]|\\.)*)`/g;
// Top-level declarations that name the enclosing component or function.
const declaration =
  /^(?:export\s+)?(?:default\s+)?(?:async\s+)?(?:function\s*\*?\s*([A-Za-z_$][\w$]*)|(?:const|let)\s+([A-Za-z_$][\w$]*))/;

function escapeRegExp(text: string): string {
  return text.replace(/[.*+?^${}()|[\]\\]/g, '\\$&');
}

/** Every use of every key; keys come from en.json, plural forms resolve through their base. */
export function collectUsage(): Map<string, Use[]> {
  const keys = flatten(readJson(localePath('en'))).map(([key]) => key);
  const usage = new Map<string, Use[]>(keys.map((key) => [key, []]));
  const byBase = new Map<string, string[]>();
  for (const key of keys) {
    const { base } = splitPlural(key);
    byBase.set(base, [...(byBase.get(base) ?? []), key]);
    if (base !== key) byBase.set(key, [key]);
  }
  const looksLikeKey = /^[A-Za-z][\w-]*(\.[\w-]+)+\.?$/;

  for (const path of sourceFiles(sourceRoot)) {
    const source = readFileSync(path, 'utf8');
    const file = relative(repoRoot, path);
    const lineStarts = [0];
    for (let i = 0; i < source.length; i++) if (source[i] === '\n') lineStarts.push(i + 1);
    const lineOf = (offset: number) => {
      let lo = 0;
      let hi = lineStarts.length - 1;
      while (lo < hi) {
        const mid = (lo + hi + 1) >> 1;
        if (lineStarts[mid]! <= offset) lo = mid;
        else hi = mid - 1;
      }
      return lo + 1;
    };
    const lines = source.split('\n');
    const componentAt = (line: number) => {
      for (let i = line - 1; i >= 0; i--) {
        const m = declaration.exec(lines[i]!);
        if (m) return m[1] ?? m[2]!;
      }
      return '(module)';
    };
    const add = (key: string, offset: number, dynamic: boolean) => {
      const line = lineOf(offset);
      const uses = usage.get(key)!;
      if (uses.some((use) => use.file === file && use.line === line)) return;
      uses.push({
        file,
        line,
        component: componentAt(line),
        ...(dynamic ? { dynamic: true } : {}),
      });
    };

    const expression = /\$\{((?:[^{}]|\{[^{}]*\})*)\}/g;
    const scan = (code: string, base: number) => {
      for (const m of code.matchAll(literal)) visit(m, base + m.index);
    };
    const visit = (m: RegExpMatchArray, offset: number) => {
      const template = m[3];
      if (template !== undefined && template.includes('${')) {
        // Literals inside the expressions (`${t('a.b')} …`) are uses of their own.
        for (const e of template.matchAll(expression)) scan(e[1]!, offset + 1 + e.index + 2);
        // `a.b.${x}` or `a.${x}.c`: every key the static parts allow. A template that starts with
        // an expression (`${base}.title`) says nothing reliable and is skipped.
        if (template.startsWith('${')) return;
        const parts = template.split(expression);
        const statics = parts.filter((_, i) => i % 2 === 0);
        if (!statics[0]!.includes('.')) return;
        const pattern = new RegExp(`^${statics.map(escapeRegExp).join('[\\w.-]+?')}(_[a-z]+)?$`);
        for (const key of keys) if (pattern.test(key)) add(key, offset, true);
        return;
      }
      const text = m[1] ?? m[2] ?? template ?? '';
      if (!looksLikeKey.test(text)) return;
      const exact = byBase.get(text);
      if (exact) {
        for (const key of exact) add(key, offset, false);
        return;
      }
      // 'admin.jobs.types.' + id, or a group name handed on and completed elsewhere.
      const prefix = text.endsWith('.') ? text : `${text}.`;
      for (const key of keys) if (key.startsWith(prefix)) add(key, offset, true);
    };
    scan(source, 0);
  }
  return usage;
}

// Run as a script, not when another module imports this one.
if (process.argv[1] === fileURLToPath(import.meta.url)) {
  const { flags } = parseArgs(process.argv.slice(2), ['out']);
  const usage = collectUsage();
  let exact = 0;
  let dynamic = 0;
  const unresolved: string[] = [];
  for (const [key, uses] of usage) {
    if (uses.length === 0) unresolved.push(key);
    else if (uses.some((use) => !use.dynamic)) exact++;
    else dynamic++;
  }
  if (flags.has('unresolved')) {
    process.stdout.write(unresolved.map((key) => `${key}\n`).join(''));
  } else {
    const json = `${JSON.stringify(Object.fromEntries(usage), null, 2)}\n`;
    const out = flags.get('out');
    if (typeof out === 'string') writeFileSync(out, json);
    else process.stdout.write(json);
  }
  process.stderr.write(
    `${usage.size} keys: ${exact} used literally, ${dynamic} only through a built key, ` +
      `${unresolved.length} without a use found\n`,
  );
}
