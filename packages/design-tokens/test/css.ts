// A small reader of the web app's CSS for the contrast test of text over artwork (concept 24,
// Z-6): rules with their @media and @container conditions, custom properties, lengths (calc, min,
// max, clamp, var) and linear gradients, evaluated at one point for one viewport. It reads only
// what those stylesheets use; anything else throws, so a new construct fails loudly instead of
// being measured wrong.
import { readFileSync } from 'node:fs';

/** A stylesheet of the web app, by its path below `web/src/`. */
export const webCss = (file: string): string =>
  readFileSync(new URL(`../../../web/src/${file}`, import.meta.url), 'utf8');

/** The compiled tokens (`dist/tokens.css`). */
export const tokensCss = (): string =>
  readFileSync(new URL('../dist/tokens.css', import.meta.url), 'utf8');

export interface Rule {
  selectors: string[];
  /** The preludes of the enclosing @media and @container rules, outermost first. */
  conditions: string[];
  /** Declarations in source order. */
  declarations: [string, string][];
}

/** Splits at `separator` outside parentheses. */
export function splitTop(text: string, separator: string): string[] {
  const parts: string[] = [];
  let depth = 0;
  let start = 0;
  for (let index = 0; index < text.length; index += 1) {
    const char = text[index];
    if (char === '(') depth += 1;
    else if (char === ')') depth -= 1;
    else if (depth === 0 && (separator === ' ' ? /\s/.test(char ?? '') : char === separator)) {
      parts.push(text.slice(start, index));
      start = index + 1;
    }
  }
  parts.push(text.slice(start));
  return parts.map((part) => part.trim()).filter((part) => part !== '');
}

/** The style rules of a stylesheet, nested in @media and @container; other at-rules are skipped. */
export function parseRules(css: string): Rule[] {
  const source = css.replace(/\/\*[\s\S]*?\*\//g, '');
  const rules: Rule[] = [];
  const walk = (text: string, conditions: string[]) => {
    let index = 0;
    while (index < text.length) {
      const open = text.indexOf('{', index);
      if (open < 0) break;
      const prelude = text.slice(index, open).trim();
      let depth = 1;
      let close = open + 1;
      for (; close < text.length && depth > 0; close += 1) {
        if (text[close] === '{') depth += 1;
        else if (text[close] === '}') depth -= 1;
      }
      const body = text.slice(open + 1, close - 1);
      index = close;
      if (prelude.startsWith('@media') || prelude.startsWith('@container')) {
        walk(body, [...conditions, prelude]);
      } else if (!prelude.startsWith('@')) {
        rules.push({
          selectors: splitTop(prelude, ',').map((selector) => selector.replace(/\s+/g, ' ')),
          conditions,
          declarations: splitTop(body, ';').map((declaration) => {
            const colon = declaration.indexOf(':');
            return [declaration.slice(0, colon).trim(), declaration.slice(colon + 1).trim()] as [
              string,
              string,
            ];
          }),
        });
      }
    }
  };
  walk(source, []);
  return rules;
}

export interface Viewport {
  width: number;
  height: number;
}

/** Where a value is computed: the viewport and, for @container and cqi, the container's width. */
export interface Env {
  viewport: Viewport;
  container?: number;
  vars: Record<string, string>;
}

const REM = 16;

/** Whether an @media or @container prelude holds; only width conditions are understood. */
function holds(prelude: string, env: Env): boolean {
  const isContainer = prelude.startsWith('@container');
  const features = [...prelude.matchAll(/\(([^()]*)\)/g)].map((match) => (match[1] ?? '').trim());
  if (features.length === 0) throw new Error(`no condition in ${prelude}`);
  return features.every((feature) => {
    const match = /^width\s*(<=|>=|<|>)\s*(.+)$/.exec(feature);
    // Hover, motion and the like: not part of a viewport, so the block does not apply.
    if (!match) return false;
    const width = isContainer ? env.container : env.viewport.width;
    if (width === undefined) throw new Error(`${prelude} needs a container width`);
    const limit = length(match[2] ?? '', env);
    switch (match[1]) {
      case '<':
        return width < limit;
      case '<=':
        return width <= limit;
      case '>':
        return width > limit;
      default:
        return width >= limit;
    }
  });
}

/** The declarations that apply to `selector` in `env`, in source order. */
export function declarationsOf(rules: Rule[], selector: string, env: Env): [string, string][] {
  return rules
    .filter((rule) => rule.selectors.includes(selector))
    .filter((rule) => rule.conditions.every((prelude) => holds(prelude, env)))
    .flatMap((rule) => rule.declarations);
}

/** The last value of a property, or undefined. */
export function valueOf(declarations: [string, string][], property: string): string | undefined {
  return declarations.filter(([name]) => name === property).at(-1)?.[1];
}

const SIDES = { top: 0, right: 1, bottom: 2, left: 3 } as const;

/** One side of `padding` (shorthand or longhand, the later one wins). */
export function paddingOf(
  declarations: [string, string][],
  side: keyof typeof SIDES,
): string | undefined {
  let found: string | undefined;
  for (const [name, value] of declarations) {
    if (name === `padding-${side}`) found = value;
    if (name === 'padding') {
      const parts = splitTop(value, ' ');
      const [a, b = a, c = a, d = b] = parts;
      found = [a, b, c, d][SIDES[side]];
    }
  }
  return found;
}

/** The custom properties of `:root` in a stylesheet for `env`, over the ones given. */
export function rootVars(rules: Rule[], env: Env): Record<string, string> {
  const vars = { ...env.vars };
  for (const [name, value] of declarationsOf(rules, ':root', env))
    if (name.startsWith('--')) vars[name] = value;
  return vars;
}

function resolveVar(inner: string, env: Env): string {
  const [name = '', fallback] = splitTop(inner, ',');
  const value = env.vars[name] ?? fallback;
  if (value === undefined) throw new Error(`unknown custom property ${name}`);
  return value;
}

/**
 * A length in px. `basis` is what 100 % is; vw, vh, cqi and rem come from the env. Lengths are
 * calc(), min(), max(), clamp() and var() of px, rem, %, vw, vh, dvh, cqi and plain numbers.
 */
export function length(text: string, env: Env, basis?: number): number {
  const tokens = text.match(/-?\d*\.?\d+[a-z%]*|[a-z-]+\(|--[\w-]+|[()+*/,-]/gi) ?? [];
  let index = 0;
  const peek = () => tokens[index];
  const next = () => {
    const token = tokens[index];
    index += 1;
    if (token === undefined) throw new Error(`unexpected end of ${text}`);
    return token;
  };
  const expect = (token: string) => {
    if (next() !== token) throw new Error(`expected ${token} in ${text}`);
  };
  const args = (): number[] => {
    const values = [sum()];
    while (peek() === ',') {
      next();
      values.push(sum());
    }
    expect(')');
    return values;
  };
  const unit = (value: number, suffix: string): number => {
    switch (suffix) {
      case '':
      case 'px':
        return value;
      case 'rem':
        return value * REM;
      case 'vw':
        return (value * env.viewport.width) / 100;
      case 'vh':
      case 'dvh':
        return (value * env.viewport.height) / 100;
      case 'cqi':
      case 'cqw':
        if (env.container === undefined) throw new Error(`${text} needs a container width`);
        return (value * env.container) / 100;
      case '%':
        if (basis === undefined) throw new Error(`${text} needs a basis for %`);
        return (value * basis) / 100;
      default:
        throw new Error(`unknown unit ${suffix} in ${text}`);
    }
  };
  function factor(): number {
    const token = next();
    if (token === '(') {
      const value = sum();
      expect(')');
      return value;
    }
    if (token === '-') return -factor();
    if (token === 'var(') {
      let depth = 1;
      const start = index;
      while (depth > 0) {
        const part = next();
        if (part.endsWith('(')) depth += 1;
        else if (part === ')') depth -= 1;
      }
      return length(resolveVar(tokens.slice(start, index - 1).join(' '), env), env, basis);
    }
    if (token === 'calc(') {
      const value = sum();
      expect(')');
      return value;
    }
    if (token === 'min(') return Math.min(...args());
    if (token === 'max(') return Math.max(...args());
    if (token === 'clamp(') {
      const [low = 0, value = 0, high = 0] = args();
      return Math.max(low, Math.min(value, high));
    }
    const match = /^(-?\d*\.?\d+)([a-z%]*)$/i.exec(token);
    if (!match) throw new Error(`cannot read ${token} in ${text}`);
    return unit(Number(match[1]), (match[2] ?? '').toLowerCase());
  }
  function product(): number {
    let value = factor();
    while (peek() === '*' || peek() === '/') {
      const op = next();
      const right = factor();
      value = op === '*' ? value * right : value / right;
    }
    return value;
  }
  function sum(): number {
    let value = product();
    while (peek() === '+' || peek() === '-') {
      const op = next();
      const right = product();
      value = op === '+' ? value + right : value - right;
    }
    return value;
  }
  const value = sum();
  if (index !== tokens.length) throw new Error(`cannot read the rest of ${text}`);
  return value;
}

export interface Rgba {
  r: number;
  g: number;
  b: number;
  a: number;
}

/** A colour: #rrggbb, #rrggbbaa, rgb(), rgba() or transparent, or a var() of one. */
export function color(text: string, env: Env): Rgba {
  const value = text.trim();
  if (value === 'transparent') return { r: 0, g: 0, b: 0, a: 0 };
  const variable = /^var\((.*)\)$/.exec(value);
  if (variable) return color(resolveVar(variable[1] ?? '', env), env);
  const hex = /^#([0-9a-f]{2})([0-9a-f]{2})([0-9a-f]{2})([0-9a-f]{2})?$/i.exec(value);
  if (hex) {
    const [r, g, b] = [hex[1], hex[2], hex[3]].map((part) => parseInt(part ?? '0', 16)) as [
      number,
      number,
      number,
    ];
    return { r, g, b, a: hex[4] ? parseInt(hex[4], 16) / 255 : 1 };
  }
  const rgb = /^rgba?\((\d+),\s*(\d+),\s*(\d+)(?:,\s*([\d.]+))?\)$/.exec(value);
  if (rgb) {
    return {
      r: Number(rgb[1]),
      g: Number(rgb[2]),
      b: Number(rgb[3]),
      a: rgb[4] === undefined ? 1 : Number(rgb[4]),
    };
  }
  throw new Error(`cannot read the colour ${text}`);
}

export interface Gradient {
  /** `to top`, `to bottom`, `to right` or `to left`. */
  direction: string;
  stops: { color: string; position?: string }[];
}

/** The linear gradients of a `background-image`, top layer first; `none` is none. */
export function gradients(backgroundImage: string | undefined): Gradient[] {
  if (backgroundImage === undefined || backgroundImage === 'none') return [];
  return splitTop(backgroundImage, ',').map((layer) => {
    const match = /^linear-gradient\(([\s\S]*)\)$/.exec(layer);
    if (!match) throw new Error(`only linear-gradient() is read, got ${layer}`);
    const [direction = '', ...stops] = splitTop(match[1] ?? '', ',');
    if (!/^to (top|bottom|left|right)$/.test(direction))
      throw new Error(`only to top, bottom, left or right is read, got ${direction}`);
    return {
      direction,
      stops: stops.map((stop) => {
        const [colour = '', ...position] = splitTop(stop, ' ');
        return position.length > 0
          ? { color: colour, position: position.join(' ') }
          : { color: colour };
      }),
    };
  });
}

/**
 * The colour of a gradient `offset` px along its direction, the gradient being `size` px long
 * (CSS Images 3: positions in order, the first defaults to 0, the last to 100 %; the colours are
 * interpolated premultiplied).
 */
export function colorAt(gradient: Gradient, offset: number, size: number, env: Env): Rgba {
  const last = gradient.stops.length - 1;
  const positions: number[] = [];
  gradient.stops.forEach((stop, index) => {
    const raw =
      stop.position !== undefined
        ? length(stop.position, env, size)
        : index === 0
          ? 0
          : index === last
            ? size
            : undefined;
    if (raw === undefined) throw new Error('a middle stop without a position is not read');
    positions.push(Math.max(raw, positions.at(-1) ?? -Infinity));
  });
  const colours = gradient.stops.map((stop) => color(stop.color, env));
  const first = colours[0] as Rgba;
  if (offset <= (positions[0] as number)) return first;
  for (let index = 1; index <= last; index += 1) {
    const end = positions[index] as number;
    if (offset > end) continue;
    const start = positions[index - 1] as number;
    const t = end === start ? 1 : (offset - start) / (end - start);
    const from = colours[index - 1] as Rgba;
    const to = colours[index] as Rgba;
    const a = from.a + (to.a - from.a) * t;
    const channel = (key: 'r' | 'g' | 'b') =>
      a === 0 ? 0 : (from[key] * from.a + (to[key] * to.a - from[key] * from.a) * t) / a;
    return { r: channel('r'), g: channel('g'), b: channel('b'), a };
  }
  return colours[last] as Rgba;
}

/** Paints `layers` (top first) over an opaque ground; returns #rrggbb. */
export function paint(layers: Rgba[], ground: string): string {
  const base = color(ground, { viewport: { width: 0, height: 0 }, vars: {} });
  const result = layers.reduceRight(
    (below, layer) => ({
      r: layer.r * layer.a + below.r * (1 - layer.a),
      g: layer.g * layer.a + below.g * (1 - layer.a),
      b: layer.b * layer.a + below.b * (1 - layer.a),
    }),
    { r: base.r, g: base.g, b: base.b },
  );
  return `#${[result.r, result.g, result.b]
    .map((channel) => Math.round(channel).toString(16).padStart(2, '0'))
    .join('')}`;
}
