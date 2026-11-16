import { readdirSync, readFileSync } from 'node:fs';
import { join } from 'node:path';
import { tokens } from '@slouchcat/design-tokens';
import { describe, expect, it } from 'vitest';

/*
 * Every colour token the web app reads must exist in the token source (concept 24, 6.5). A
 * misspelt or removed name falls back to nothing in the browser, and stylelint does not catch it
 * in shorthands like `border` (the case of `var(--sc-color-border)` in Integrations.module.css).
 */

/** The custom property names style-dictionary emits: `--sc-` plus the kebab-cased token path. */
function propertyNames(node: unknown, path: string[] = []): string[] {
  if (typeof node !== 'object' || node === null) {
    return [
      `--sc-${path.map((key) => key.replace(/[A-Z]/g, (c) => `-${c.toLowerCase()}`)).join('-')}`,
    ];
  }
  return Object.entries(node).flatMap(([key, child]) => propertyNames(child, [...path, key]));
}

function sources(dir: string): string[] {
  return readdirSync(dir, { withFileTypes: true }).flatMap((entry) => {
    const path = join(dir, entry.name);
    if (entry.isDirectory()) return sources(path);
    return /\.(css|tsx?)$/.test(entry.name) && !/\.test\.tsx?$/.test(entry.name) ? [path] : [];
  });
}

const defined = new Set(propertyNames({ color: tokens.color }));
const root = join(import.meta.dirname, '..');

describe('colour tokens used by the web app', () => {
  it('finds the token names of the source', () => {
    expect(defined).toContain('--sc-color-border-frame');
    expect(defined).toContain('--sc-color-tile-label-ground');
  });

  it('uses only colour tokens the source defines', () => {
    const unknown = sources(root).flatMap((file) =>
      [
        ...readFileSync(file, 'utf8').matchAll(
          /var\(\s*(--sc-color-[a-z0-9-]*[a-z0-9])(?![a-z0-9$-])/g,
        ),
      ]
        .map((match) => match[1] as string)
        .filter((name) => !defined.has(name))
        .map((name) => `${file.slice(root.length + 1)}: ${name}`),
    );
    expect(unknown).toEqual([]);
  });
});
