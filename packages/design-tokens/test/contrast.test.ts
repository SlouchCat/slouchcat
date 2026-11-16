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

/** A translucent `rgba(r, g, b, a)` token laid over an opaque `#rrggbb`, as the browser paints it. */
function over(top: string, bottom: string): string {
  const match = /^rgba\((\d+), (\d+), (\d+), ([\d.]+)\)$/.exec(top);
  if (!match) throw new Error(`expected an rgba() colour, got ${top}`);
  const [r, g, b, alpha] = match.slice(1).map(Number) as [number, number, number, number];
  const base = [1, 3, 5].map((start) => parseInt(bottom.slice(start, start + 2), 16));
  return `#${[r, g, b]
    .map((channel, index) => Math.round(channel * alpha + (base[index] as number) * (1 - alpha)))
    .map((channel) => channel.toString(16).padStart(2, '0'))
    .join('')}`;
}

/** Layers listed top first, laid over an opaque ground. */
function stack(layers: string[], ground: string): string {
  return layers.reduceRight((below, layer) => over(layer, below), ground);
}

type Row = { pair: string; ratio: number };

function measure(pair: string, foreground: string, background: string): Row {
  return { pair, ratio: Number(contrast(foreground, background).toFixed(2)) };
}

function failing(rows: Row[], min: number, max = Infinity): string[] {
  console.table(rows);
  return rows.filter((row) => row.ratio < min || row.ratio > max).map((row) => row.pair);
}

const bg = tokens.color.bg.base;
const { border, scrim, text } = tokens.color;

// Every surface text, frames and controls can sit on, not only the base background.
const surfaces: Record<string, string> = {
  'bg.base': bg,
  'bg.elevated': tokens.color.bg.elevated,
  'surface.1': tokens.color.surface['1'],
  'surface.2': tokens.color.surface['2'],
  'surface.3': tokens.color.surface['3'],
};
const textTokens: Record<string, string> = {
  'text.primary': text.primary,
  'text.secondary': text.secondary,
  'text.muted': text.muted,
};

/** A border token (translucent white) against each surface: its painted colour on that surface. */
function borderRows(name: string, token: string): Row[] {
  return Object.entries(surfaces).map(([surfaceName, surface]) =>
    measure(`${name} / ${surfaceName}`, over(token, surface), surface),
  );
}

describe('text contrast (concept 24, Z-1: at least 7:1)', () => {
  it('keeps every text token at 7:1 on every surface', () => {
    const rows = Object.entries(textTokens).flatMap(([textName, foreground]) =>
      Object.entries(surfaces).map(([surfaceName, background]) =>
        measure(`${textName} / ${surfaceName}`, foreground, background),
      ),
    );
    expect(failing(rows, 7)).toEqual([]);
  });
});

describe('accent contrast (WCAG 2.x, minimum 4.5:1)', () => {
  it('defines exactly eight accent colours', () => {
    expect(accentNames).toHaveLength(8);
  });

  it('meets 4.5:1 for every accent as text and its text on the accent', () => {
    const rows = accentNames.flatMap((name) => {
      const accent = tokens.color.accent[name];
      return [
        measure(`accent.${name} / bg.base`, accent.base, bg),
        measure(`accent.${name}.on / accent.${name}`, accent.on, accent.base),
      ];
    });
    expect(failing(rows, 4.5)).toEqual([]);
  });
});

// Three border strengths (concept 24, 6.2): a divider quieter than a frame, a frame quieter than
// the outline of a control. Each is translucent white, so it is measured on every surface.
describe('border contrast (concept 24, Z-2 to Z-4)', () => {
  it('frames a panel, table, band, notice or save bar at 3:1 (Z-2)', () => {
    expect(failing(borderRows('border.frame', border.frame), 3)).toEqual([]);
  });

  it('outlines a control at 3:1, WCAG 2.2 1.4.11 (Z-3)', () => {
    expect(failing(borderRows('border.control', border.control), 3)).toEqual([]);
  });

  it('draws a divider at about 2:1 (Z-4)', () => {
    expect(failing(borderRows('border.subtle', border.subtle), 1.8, 2.5)).toEqual([]);
  });

  it('keeps divider < frame < control on every surface', () => {
    const order = Object.entries(surfaces).filter(([, surface]) => {
      const [divider, frame, control] = [border.subtle, border.frame, border.control].map((token) =>
        contrast(over(token, surface), surface),
      ) as [number, number, number];
      return !(divider < frame && frame < control);
    });
    expect(order.map(([name]) => name)).toEqual([]);
  });
});

// Text over artwork (concept 24, Z-6): at least 4.5:1 over a white picture, the worst case. Each
// place lists, top first, the layers under its weakest text (the point of the text farthest from
// the dark corner of its gradients), mirroring the CSS named in the case; change them together.
const white = '#ffffff';
const overImage: { place: string; layers: string[]; text: Record<string, string> }[] = [
  {
    // LibraryTile.module.css, .tile::after: the name and the count at the bottom of the gradient.
    place: 'library tile label',
    layers: [tokens.color.tile.labelGround, tokens.color.tile.shade],
    text: { 'text.primary': text.primary, 'text.secondary': text.secondary },
  },
  {
    // HeroCarousel.module.css, .scrim: the text block reaches half the stage's height and 55 % of
    // its width; there both gradients are at scrim.medium (stops at 50 % and 55 %).
    place: 'spotlight',
    layers: [scrim.medium, scrim.medium],
    text: { 'text.primary': text.primary, 'text.secondary': text.secondary },
  },
  {
    // Backdrop.module.css, .scrim: the text column of a detail page starts 60 % up the backdrop
    // (header plus 28vh under an 80vh backdrop), where the bottom gradient is at scrim.strong; the
    // left gradient has faded out at the column's right end.
    place: 'detail header',
    layers: [scrim.strong],
    text: {
      'text.primary': text.primary,
      'text.secondary': text.secondary,
      'text.muted': text.muted,
    },
  },
  {
    // PlayerChrome.module.css, .top and .bottom: scrim.strong under the content, the fade only in
    // the padding towards the picture.
    place: 'player controls',
    layers: [scrim.strong],
    text: { 'text.primary': text.primary, 'text.secondary': text.secondary },
  },
  {
    // Player.module.css, .pauseView: the pause info spans up to half the width, still on
    // scrim.strong (stop at 50 %).
    place: 'player pause view',
    layers: [scrim.strong],
    text: { 'text.primary': text.primary, 'text.secondary': text.secondary },
  },
];

describe('text over artwork (concept 24, Z-6: at least 4.5:1 over a white picture)', () => {
  it('keeps the text of every place with artwork readable', () => {
    const rows = overImage.flatMap(({ place, layers, text: used }) => {
      const ground = stack(layers, white);
      return Object.entries(used).map(([name, foreground]) =>
        measure(`${name} / ${place} over white`, foreground, ground),
      );
    });
    expect(failing(rows, 4.5)).toEqual([]);
  });
});
