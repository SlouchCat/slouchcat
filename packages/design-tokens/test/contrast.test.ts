import { describe, expect, it } from 'vitest';
import { accentNames, tokens } from '../dist/index.js';
import {
  type Env,
  type Rgba,
  type Viewport,
  colorAt,
  declarationsOf,
  gradients,
  length,
  paddingOf,
  paint,
  parseRules,
  rootVars,
  splitTop,
  tokensCss,
  valueOf,
  webCss,
} from './css.js';

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

// Every surface text, frames and controls can sit on, not only the base background: the opaque
// backgrounds (bg.overlay is a translucent scrim) and every surface step, read from the tokens.
const opaque = (value: string) => /^#[0-9a-f]{6}$/i.test(value);
const surfaces: Record<string, string> = Object.fromEntries([
  ...Object.entries(tokens.color.bg)
    .filter(([, value]) => opaque(value))
    .map(([name, value]) => [`bg.${name}`, value]),
  ...Object.entries(tokens.color.surface).map(([name, value]) => [`surface.${name}`, value]),
]);
// Every text colour; on-accent is text on an accent fill, checked with the accents.
const textTokens: Record<string, string> = Object.fromEntries(
  Object.entries(text)
    .filter(([name]) => name !== 'onAccent')
    .map(([name, value]) => [`text.${name}`, value]),
);

/** Colours as text on every surface: name / surface rows. */
function onSurfaces(colours: Record<string, string>): Row[] {
  return Object.entries(colours).flatMap(([name, foreground]) =>
    Object.entries(surfaces).map(([surfaceName, background]) =>
      measure(`${name} / ${surfaceName}`, foreground, background),
    ),
  );
}

/** A border token (translucent white) against each surface: its painted colour on that surface. */
function borderRows(name: string, token: string): Row[] {
  return Object.entries(surfaces).map(([surfaceName, surface]) =>
    measure(`${name} / ${surfaceName}`, over(token, surface), surface),
  );
}

describe('text contrast (concept 24, Z-1: at least 7:1)', () => {
  it('reads every opaque background, every surface and every text colour', () => {
    expect(Object.keys(surfaces)).toEqual([
      'bg.base',
      'bg.elevated',
      'surface.1',
      'surface.2',
      'surface.3',
    ]);
    expect(Object.keys(textTokens)).toEqual(['text.primary', 'text.secondary', 'text.muted']);
  });

  it('keeps every text token at 7:1 on every surface', () => {
    expect(failing(onSurfaces(textTokens), 7)).toEqual([]);
  });
});

// State colours and profile accents as text (a Chip, a StatusDot's word, a link, a danger
// notice) on every surface, Z-1: at least 4.5:1.
describe('state and accent colours as text (concept 24, Z-1: at least 4.5:1)', () => {
  it('keeps danger, warning and success at 4.5:1 on every surface', () => {
    const { danger, warning, success } = tokens.color.state;
    expect(
      failing(
        onSurfaces({ 'state.danger': danger, 'state.warning': warning, 'state.success': success }),
        4.5,
      ),
    ).toEqual([]);
  });

  it('keeps every profile accent at 4.5:1 on every surface', () => {
    const accents = Object.fromEntries(
      accentNames.map((name) => [`accent.${name}`, tokens.color.accent[name].base]),
    );
    expect(failing(onSurfaces(accents), 4.5)).toEqual([]);
  });
});

describe('accent contrast (WCAG 2.x, minimum 4.5:1)', () => {
  it('defines exactly eight accent colours', () => {
    expect(accentNames).toHaveLength(8);
  });

  it('meets 4.5:1 for the text on every accent, its own and text.on-accent', () => {
    const rows = accentNames.flatMap((name) => {
      const accent = tokens.color.accent[name];
      return [
        measure(`accent.${name}.on / accent.${name}`, accent.on, accent.base),
        measure(`text.on-accent / accent.${name}`, text.onAccent, accent.base),
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

// Text over artwork (concept 24, Z-6): at least 4.5:1 over a white picture, the worst case. The
// gradients, their stops and the boxes the text sits in are read from the web app's stylesheets
// (css.ts), for each reference viewport; each place takes the point of its text farthest from the
// dark end of its gradients. What a stylesheet cannot say (how far a text block reaches) is named
// at the place and in ADR 0020, addendum concept 24.
const white = '#ffffff';
const viewports: Record<string, Viewport> = {
  phone: { width: 375, height: 812 },
  'phone landscape': { width: 812, height: 375 },
  tablet: { width: 768, height: 1024 },
  'small laptop': { width: 1024, height: 768 },
  laptop: { width: 1280, height: 800 },
  'laptop 1440': { width: 1440, height: 900 },
  'full HD': { width: 1920, height: 1080 },
  'portrait monitor': { width: 1080, height: 1920 },
  QHD: { width: 2560, height: 1440 },
  'ultra-wide': { width: 3440, height: 1440 },
};

const tokenRules = parseRules(tokensCss());
const globalRules = parseRules(webCss('ui/global.css'));
const hero = parseRules(webCss('ui/HeroCarousel.module.css'));
const backdrop = parseRules(webCss('ui/Backdrop.module.css'));
const detailHeader = parseRules(webCss('features/catalog/DetailHeader.module.css'));
const chrome = parseRules(webCss('ui/PlayerChrome.module.css'));
const player = parseRules(webCss('features/player/Player.module.css'));

/** The custom properties of the tokens and of global.css for a viewport. */
function envOf(viewport: Viewport, container?: number): Env {
  const base: Env = { viewport, vars: {} };
  const vars = rootVars(globalRules, { ...base, vars: rootVars(tokenRules, base) });
  return { viewport, vars, ...(container === undefined ? {} : { container }) };
}

const required = (value: string | undefined, what: string): string => {
  if (value === undefined) throw new Error(`no ${what} in the stylesheet`);
  return value;
};

type Point = { fromTop: number; fromLeft: number; width: number; height: number };

/** A gradient evaluated at a point of its box. */
function gradientAt(gradient: ReturnType<typeof gradients>[number], env: Env, point: Point): Rgba {
  switch (gradient.direction) {
    case 'to top':
      return colorAt(gradient, point.height - point.fromTop, point.height, env);
    case 'to bottom':
      return colorAt(gradient, point.fromTop, point.height, env);
    case 'to right':
      return colorAt(gradient, point.fromLeft, point.width, env);
    default:
      return colorAt(gradient, point.width - point.fromLeft, point.width, env);
  }
}

/**
 * The gradient layers of a rule, top first, each evaluated at its own point. A `mask-image` of
 * gradients scales the alpha of every layer by the mask's alpha at that point.
 */
function layersAt(
  rules: ReturnType<typeof parseRules>,
  selector: string,
  env: Env,
  point: Point,
): Rgba[] {
  const declarations = declarationsOf(rules, selector, env);
  const layers = gradients(valueOf(declarations, 'background-image')).map((gradient) =>
    gradientAt(gradient, env, point),
  );
  const fill = valueOf(declarations, 'background-color');
  const painted = fill === undefined ? layers : [...layers, colorAtFill(fill, env)];
  const mask = gradients(valueOf(declarations, 'mask-image')).reduce(
    (alpha, gradient) => alpha * gradientAt(gradient, env, point).a,
    1,
  );
  return painted.map((layer) => ({ ...layer, a: layer.a * mask }));
}

const colorAtFill = (fill: string, env: Env): Rgba =>
  colorAt({ direction: 'to top', stops: [{ color: fill }, { color: fill }] }, 0, 1, env);

interface Place {
  place: string;
  /** The ground under the weakest text, painted over white. */
  ground: string;
  text: Record<string, string>;
}

const heroText = { 'text.primary': text.primary, 'text.secondary': text.secondary };

/**
 * HeroCarousel.module.css: the stage is as wide as the page beside the side navigation (with or
 * without it from 768 px) and as high as `.stage` says. With a ground under the text block
 * (`.content::before`, a narrow stage) the ground reaches past the block by its negative `inset`
 * and fades out there; the text stands inside the block, so its weakest point is the block's right
 * edge just below its top padding, and the stage's scrims count only where they are weakest (the
 * stage's top right corner). Without a ground the text block reaches half the stage's height (not
 * readable from CSS: title, note and actions) and as far as `.content` is wide.
 */
function spotlightPlaces(): Place[] {
  return Object.entries(viewports).flatMap(([name, viewport]) => {
    const env = envOf(viewport);
    const gutter = length('var(--page-gutter)', env);
    const sidenav = length('var(--sc-size-sidenav-width)', env);
    const widths = viewport.width >= 768 ? [0, sidenav] : [0];
    return widths.map((nav) => {
      const width = viewport.width - nav - 2 * gutter;
      const stageEnv = envOf(viewport, width);
      const height = length(
        required(valueOf(declarationsOf(hero, '.stage', stageEnv), 'height'), '.stage height'),
        stageEnv,
      );
      const content = declarationsOf(hero, '.content', stageEnv);
      const ground = declarationsOf(hero, '.content::before', stageEnv);
      const stage = { width, height };
      const right = Math.min(
        width,
        length(required(valueOf(content, 'max-width'), '.content max-width'), stageEnv, width),
      );
      let layers: Rgba[];
      if (valueOf(ground, 'background-image') !== undefined) {
        // The ground reaches `up` above the block and `out` past its right edge; the block is at
        // most as high as the stage. The text starts below the block's top padding.
        const [up = '0', out = '0'] = splitTop(
          required(valueOf(ground, 'inset'), '.content::before inset'),
          ' ',
        );
        const above = -length(up, stageEnv);
        const beyond = -length(out, stageEnv, right);
        const top = length(required(paddingOf(content, 'top'), '.content padding'), stageEnv);
        layers = [
          ...layersAt(hero, '.content::before', stageEnv, {
            width: right + beyond,
            height: height + above,
            fromTop: above + top,
            fromLeft: right,
          }),
          ...layersAt(hero, '.scrim', stageEnv, { ...stage, fromTop: 0, fromLeft: width }),
        ];
      } else {
        layers = layersAt(hero, '.scrim', stageEnv, {
          ...stage,
          fromTop: height / 2,
          fromLeft: right,
        });
      }
      return {
        place: `spotlight, ${name}${nav ? ' beside the side navigation' : ''}`,
        ground: paint(layers, white),
        text: heroText,
      };
    });
  });
}

/**
 * Backdrop.module.css under DetailHeader.module.css: the backdrop is as high as `.backdrop` says,
 * the text column starts below the top padding of `.layout`; the left scrim counts where it has
 * faded out (the column's right end). Text that starts below the backdrop stands on the page.
 */
function detailPlaces(): Place[] {
  return Object.entries(viewports).flatMap(([name, viewport]) => {
    const env = envOf(viewport);
    const box = declarationsOf(backdrop, '.backdrop', env);
    const height = Math.max(
      length(required(valueOf(box, 'height'), '.backdrop height'), env),
      length(required(valueOf(box, 'min-height'), '.backdrop min-height'), env),
    );
    const top = length(
      required(paddingOf(declarationsOf(detailHeader, '.layout', env), 'top'), '.layout padding'),
      env,
    );
    if (top >= height) return [];
    const layers = layersAt(backdrop, '.scrim', env, {
      width: viewport.width,
      height,
      fromTop: top,
      fromLeft: viewport.width,
    });
    return [
      {
        place: `detail header, ${name}`,
        ground: paint(layers, white),
        text: {
          'text.primary': text.primary,
          'text.secondary': text.secondary,
          'text.muted': text.muted,
        },
      },
    ];
  });
}

/**
 * PlayerChrome.module.css, .top and .bottom: the text lies inside the padding; the edge of the
 * text towards the picture is the weakest point. The content's own height does not matter to the
 * stops, so one line of 48 px stands for it.
 */
function chromePlaces(): Place[] {
  return Object.entries(viewports).flatMap(([name, viewport]) => {
    const env = envOf(viewport);
    return (['.top', '.bottom'] as const).map((selector) => {
      const declarations = declarationsOf(chrome, selector, env);
      const top = length(required(paddingOf(declarations, 'top'), `${selector} padding`), env);
      const bottom = length(
        required(paddingOf(declarations, 'bottom'), `${selector} padding`),
        env,
      );
      const height = top + 48 + bottom;
      const fromTop = selector === '.top' ? height - bottom : top;
      const layers = layersAt(chrome, selector, env, {
        width: viewport.width,
        height,
        fromTop,
        fromLeft: 0,
      });
      return {
        place: `player controls ${selector.slice(1)}, ${name}`,
        ground: paint(layers, white),
        text: heroText,
      };
    });
  });
}

/**
 * Player.module.css, .pauseView over the whole player (the viewport): the pause info starts at
 * the view's left padding and is as wide as `.pauseInfo` allows; its right end is the weakest
 * point. With a ground under the info (`.pauseInfo::before`, a narrow screen) the ground reaches
 * past the info by its negative `inset`; the text's weakest point there is the info's top right
 * corner (the ground's fades up and down are alike).
 */
function pausePlaces(): Place[] {
  return Object.entries(viewports).map(([name, viewport]) => {
    const env = envOf(viewport);
    const view = declarationsOf(player, '.pauseView', env);
    const left = length(required(paddingOf(view, 'left'), '.pauseView padding'), env);
    const right = length(required(paddingOf(view, 'right'), '.pauseView padding'), env);
    const inner = viewport.width - left - right;
    const info = Math.min(
      inner,
      length(
        required(
          valueOf(declarationsOf(player, '.pauseInfo', env), 'max-width'),
          '.pauseInfo max-width',
        ),
        env,
        inner,
      ),
    );
    const layers = layersAt(player, '.pauseView', env, {
      width: viewport.width,
      height: viewport.height,
      fromTop: 0,
      fromLeft: left + info,
    });
    const ground = declarationsOf(player, '.pauseInfo::before', env);
    if (valueOf(ground, 'background-image') !== undefined) {
      const [up = '0', out = '0', down = '0', back = '0'] = splitTop(
        required(valueOf(ground, 'inset'), '.pauseInfo::before inset'),
        ' ',
      );
      const [above, beyond, below, before] = [up, out, down, back].map(
        (side) => -length(side, env, info),
      ) as [number, number, number, number];
      layers.unshift(
        ...layersAt(player, '.pauseInfo::before', env, {
          width: before + info + beyond,
          height: above + viewport.height + below,
          fromTop: above,
          fromLeft: before + info,
        }),
      );
    }
    return { place: `player pause view, ${name}`, ground: paint(layers, white), text: heroText };
  });
}

const overImage: Place[] = [
  {
    // LibraryTile.module.css, .tile::after: the name and the count at the bottom of the gradient.
    place: 'library tile label',
    ground: stack([tokens.color.tile.labelGround, tokens.color.tile.shade], white),
    text: heroText,
  },
  ...spotlightPlaces(),
  ...detailPlaces(),
  ...chromePlaces(),
  ...pausePlaces(),
];

describe('text over artwork (concept 24, Z-6: at least 4.5:1 over a white picture)', () => {
  it('keeps the text of every place with artwork readable on every reference viewport', () => {
    const rows = overImage.flatMap(({ place, ground, text: used }) =>
      Object.entries(used).map(([name, foreground]) =>
        measure(`${name} / ${place} over white`, foreground, ground),
      ),
    );
    expect(failing(rows, 4.5)).toEqual([]);
  });

  it('lays a ground under the spotlight text on a narrow stage and under the pause info on a narrow screen', () => {
    // A phone: the text block spans the stage, past where the left scrim fades out.
    const phone = envOf(viewports.phone as Viewport, 343);
    expect(
      valueOf(declarationsOf(hero, '.content::before', phone), 'background-image'),
    ).toBeDefined();
    const narrow = envOf(viewports.phone as Viewport);
    expect(
      valueOf(declarationsOf(player, '.pauseInfo::before', narrow), 'background-image'),
    ).toBeDefined();
  });
});
