import { describe, expect, it } from 'vitest';
import { accentNames, tokens } from '../dist/index.js';

// WCAG 2.x relative luminance and contrast ratio.
// https://www.w3.org/TR/WCAG21/#dfn-relative-luminance
function luminance(hex: string): number {
  const match = /^#([0-9a-f]{2})([0-9a-f]{2})([0-9a-f]{2})$/i.exec(hex);
  if (!match) throw new Error(`expected an opaque #rrggbb colour, got ${hex}`);
  const [r, g, b] = match.slice(1).map((part) => {
    const channel = parseInt(part, 16) / 255;
    return channel <= 0.04045 ? channel / 12.92 : ((channel + 0.055) / 1.055) ** 2.4;
  }) as [number, number, number];
  return 0.2126 * r + 0.7152 * g + 0.0722 * b;
}

function contrast(foreground: string, background: string): number {
  const [light, dark] = [luminance(foreground), luminance(background)].sort((a, b) => b - a) as [
    number,
    number,
  ];
  return (light + 0.05) / (dark + 0.05);
}

const MIN_RATIO = 4.5;
const bg = tokens.color.bg.base;

// Text must stay readable on every surface it can sit on, not only on the base background.
const surfaces: Record<string, string> = {
  'bg.base': bg,
  'bg.elevated': tokens.color.bg.elevated,
  'surface.1': tokens.color.surface['1'],
  'surface.2': tokens.color.surface['2'],
  'surface.3': tokens.color.surface['3'],
};
const textTokens: Record<string, string> = {
  'text.primary': tokens.color.text.primary,
  'text.secondary': tokens.color.text.secondary,
  'text.muted': tokens.color.text.muted,
};

const pairs: { pair: string; foreground: string; background: string }[] = [
  ...Object.entries(textTokens).flatMap(([textName, foreground]) =>
    Object.entries(surfaces).map(([surfaceName, background]) => ({
      pair: `${textName} / ${surfaceName}`,
      foreground,
      background,
    })),
  ),
  ...accentNames.flatMap((name) => {
    const accent = tokens.color.accent[name];
    return [
      { pair: `accent.${name} / bg.base`, foreground: accent.base, background: bg },
      {
        pair: `accent.${name}.on / accent.${name}`,
        foreground: accent.on,
        background: accent.base,
      },
    ];
  }),
];

describe('colour contrast (WCAG 2.x, minimum 4.5:1)', () => {
  it('defines exactly eight accent colours', () => {
    expect(accentNames).toHaveLength(8);
  });

  it('meets the minimum ratio for every text and accent pair', () => {
    const rows = pairs.map(({ pair, foreground, background }) => ({
      pair,
      foreground,
      background,
      ratio: Number(contrast(foreground, background).toFixed(2)),
    }));
    console.table(rows);
    const failing = rows.filter((row) => row.ratio < MIN_RATIO).map((row) => row.pair);
    expect(failing).toEqual([]);
  });
});
