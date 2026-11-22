// The Kotlin output for the Android apps (build-kotlin.mjs, concept 27 5.5) against the CSS output
// of the web app: samples of every conversion, so both platforms keep the same values.
import { execFileSync } from 'node:child_process';
import { mkdtempSync, readFileSync, rmSync } from 'node:fs';
import { tmpdir } from 'node:os';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import { afterAll, beforeAll, describe, expect, it } from 'vitest';
import { tokensCss } from './css.js';

let kotlin = '';
let dir = '';

beforeAll(() => {
  dir = mkdtempSync(path.join(tmpdir(), 'tokens-kotlin-'));
  execFileSync(process.execPath, [
    fileURLToPath(new URL('../build-kotlin.mjs', import.meta.url)),
    dir,
  ]);
  kotlin = readFileSync(path.join(dir, 'com/slouchcat/core/design/SlouchcatTokens.kt'), 'utf8');
});

afterAll(() => rmSync(dir, { recursive: true, force: true }));

/** The value of a CSS custom property of dist/tokens.css. */
function css(name: string): string {
  const match = new RegExp(`--sc-${name}:\\s*([^;]+);`).exec(tokensCss());
  if (!match?.[1]) throw new Error(`--sc-${name} not in tokens.css`);
  return match[1].trim();
}

/** The expression after `val <name> =` inside the object path, e.g. ['Color', 'Bg', 'Overlay']. */
function kt(pathNames: string[]): string {
  let at = 0;
  for (const name of pathNames.slice(0, -1)) {
    at = kotlin.indexOf(`object ${name} {`, at);
    if (at < 0) throw new Error(`object ${name} missing`);
  }
  const match = new RegExp(`val ${pathNames.at(-1)} = (.+)`).exec(kotlin.slice(at));
  if (!match?.[1]) throw new Error(`${pathNames.join('.')} missing`);
  return match[1].trim();
}

/** [r, g, b, alpha 0..255] of a CSS colour, `#rrggbb` or `rgba(r, g, b, a)`. */
function rgbaOf(value: string): number[] {
  const hex = /^#([0-9a-f]{2})([0-9a-f]{2})([0-9a-f]{2})$/i.exec(value);
  if (hex) return [...hex.slice(1).map((part) => parseInt(part, 16)), 255];
  const rgba = /^rgba\((\d+), (\d+), (\d+), ([\d.]+)\)$/.exec(value);
  if (!rgba) throw new Error(`unexpected CSS colour ${value}`);
  return [Number(rgba[1]), Number(rgba[2]), Number(rgba[3]), Math.round(Number(rgba[4]) * 255)];
}

/** [r, g, b, alpha 0..255] of a generated `Color(0xAARRGGBB)`. */
function argbOf(expression: string): number[] {
  const match = /Color\(0x([0-9A-F]{2})([0-9A-F]{2})([0-9A-F]{2})([0-9A-F]{2})\)$/.exec(expression);
  if (!match) throw new Error(`unexpected Kotlin colour ${expression}`);
  const [a, r, g, b] = match.slice(1).map((part) => parseInt(part, 16));
  return [r, g, b, a] as number[];
}

describe('Kotlin tokens', () => {
  it('turns the alpha of #RRGGBBAA to the front, as Android reads it', () => {
    expect(kt(['Color', 'Bg', 'Overlay'])).toBe('androidx.compose.ui.graphics.Color(0xD90B0C10)');
    const samples: [string, string[]][] = [
      ['color-bg-overlay', ['Color', 'Bg', 'Overlay']],
      ['color-text-muted', ['Color', 'Text', 'Muted']],
      ['color-border-frame', ['Color', 'Border', 'Frame']],
      ['color-accent-ember-base', ['Color', 'Accent', 'Ember', 'Base']],
      ['color-scrim-clear', ['Color', 'Scrim', 'Clear']],
    ];
    for (const [cssName, ktPath] of samples) {
      expect(argbOf(kt(ktPath)), cssName).toEqual(rgbaOf(css(cssName)));
    }
  });

  it('resolves references like the CSS output', () => {
    expect(kt(['Color', 'Text', 'OnAccent'])).toBe(kt(['Color', 'Bg', 'Base']));
  });

  it('writes px as dp, rem as dp (x16) and font sizes as sp', () => {
    expect(css('space-4')).toBe('16px');
    expect(kt(['Space', 'S4'])).toBe('16.dp');
    expect(css('size-poster-sm')).toBe('8.5rem');
    expect(kt(['Size', 'Poster', 'Sm'])).toBe('136.dp');
    expect(css('font-size-md')).toBe('1rem');
    expect(kt(['Font', 'Size', 'Md'])).toBe('16.sp');
    // A component token that refers to a font size follows the user's font size as well.
    expect(kt(['Control', 'FontSize'])).toBe('16.sp');
  });

  it('writes durations, curves, weights and numbers', () => {
    expect(css('motion-duration-fast')).toBe('120ms');
    expect(kt(['Motion', 'Duration', 'Fast'])).toBe('120');
    expect(kt(['Motion', 'Easing', 'Standard'])).toBe(
      'androidx.compose.animation.core.CubicBezierEasing(0.2f, 0f, 0f, 1f)',
    );
    expect(kt(['Font', 'Weight', 'Bold'])).toBe('androidx.compose.ui.text.font.FontWeight(700)');
    expect(kt(['Focus', 'Scale'])).toBe('1.06f');
  });

  it('writes the ten-foot tokens of the TV app (tokens-tv) only into Kotlin', () => {
    // Half of the mockup's 1920 x 1080 canvas: 26 px there is 13 sp on the TV's 960 x 540 dp.
    expect(kt(['Tv', 'Font', 'Size', 'S'])).toBe('13.sp');
    expect(kt(['Tv', 'Space', 'SafeX'])).toBe('48.dp');
    expect(kt(['Tv', 'Scale', 'Focus'])).toBe('1.06f');
    expect(tokensCss()).not.toMatch(/--sc-tv-/);
  });

  it('lists what Compose has no counterpart for instead of writing it', () => {
    expect(kotlin).toMatch(/\/\/ left out: font\.family\.sans/);
    expect(kotlin).toMatch(/\/\/ left out: library-tile\.label-size-fluid/);
    expect(kotlin).not.toMatch(/val Sans =/);
  });
});
