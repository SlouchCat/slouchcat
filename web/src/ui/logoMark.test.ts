import { readFileSync } from 'node:fs';
import { join } from 'node:path';
import { describe, expect, it } from 'vitest';
import { LOGO_HEAD, LOGO_PLAY, LOGO_VIEW_BOX } from './logoMark';

/*
 * The sign sat 24 of 512 units below the middle of its canvas, and every copy had inherited it:
 * the launcher icon, the PWA icons, the logo next to the name, the TV banner. So two things are
 * checked here: the sign is in the middle, and every file that cannot import logoMark.ts carries
 * the same two paths.
 */

const repo = join(import.meta.dirname, '../../..');

/** The files outside this module that draw the sign. */
const copies = [
  'web/public/icon.svg',
  'web/public/spike/icon.svg',
  'docs/store/grafiken/app-icon.svg',
  'docs/store/grafiken/feature-graphic.svg',
  'docs/store/grafiken/tv-banner.svg',
  'android/app-mobile/src/main/res/drawable/ic_launcher_foreground.xml',
  'android/app-mobile/src/main/res/drawable/ic_launcher_monochrome.xml',
  'android/app-tv/src/main/res/drawable/icon.xml',
  'android/app-tv/src/main/res/drawable/banner.xml',
];

interface Point {
  x: number;
  y: number;
}

/** How many numbers each command takes; Z takes none and moves nothing that counts here. */
const STEP: Record<string, number> = { M: 2, L: 2, H: 1, V: 1, A: 7 };

/**
 * The end points of a path of M, L, H, V, A and Z commands, absolute or relative. Their box is
 * the box of the sign: its two arcs are rounded corners, which reach no further than their ends.
 * Reads numbers as logoMark.ts writes them (digits, an optional fraction, a sign).
 */
function endPoints(d: string): Point[] {
  const points: Point[] = [];
  let x = 0;
  let y = 0;
  for (const match of d.matchAll(/([MLHVAZ])([^MLHVAZ]*)/gi)) {
    const command = match[1] ?? '';
    const numbers = (match[2]?.match(/-?\d+(?:\.\d+)?/g) ?? []).map(Number);
    const kind = command.toUpperCase();
    const step = STEP[kind];
    if (step === undefined) continue;
    const relative = command !== kind;
    for (let i = 0; i + step <= numbers.length; i += step) {
      const last = numbers[i + step - 1] ?? 0;
      const secondLast = numbers[i + step - 2] ?? 0;
      if (kind === 'H') x = last + (relative ? x : 0);
      else if (kind === 'V') y = last + (relative ? y : 0);
      else {
        x = secondLast + (relative ? x : 0);
        y = last + (relative ? y : 0);
      }
      points.push({ x, y });
    }
  }
  return points;
}

function box(d: string) {
  const points = endPoints(d);
  const xs = points.map((point) => point.x);
  const ys = points.map((point) => point.y);
  return {
    left: Math.min(...xs),
    right: Math.max(...xs),
    top: Math.min(...ys),
    bottom: Math.max(...ys),
  };
}

/** The canvas is a square from 0 0. */
const CANVAS = 512;

describe('the Slouchcat sign', () => {
  it('sits in the middle of its canvas', () => {
    expect(LOGO_VIEW_BOX).toBe(`0 0 ${CANVAS} ${CANVAS}`);
    const head = box(LOGO_HEAD);
    expect(head).toEqual({ left: 136, right: 376, top: 112, bottom: 400 });
    expect(head.left).toBe(CANVAS - head.right);
    expect(head.top).toBe(CANVAS - head.bottom);
  });

  it('has the play triangle centred on its centroid, in the head below the ears', () => {
    const corners = endPoints(LOGO_PLAY);
    expect(corners).toHaveLength(3);
    const centroid = {
      x: corners.reduce((sum, corner) => sum + corner.x, 0) / 3,
      y: corners.reduce((sum, corner) => sum + corner.y, 0) / 3,
    };
    // Sideways the middle of the canvas; in height the middle between the notch of the ears
    // (y 184) and the lower edge of the head (y 400).
    expect(centroid).toEqual({ x: CANVAS / 2, y: (184 + 400) / 2 });
  });

  it.each(copies)('%s draws the same sign', (file) => {
    const text = readFileSync(join(repo, file), 'utf8');
    expect(text).toContain(LOGO_HEAD);
    expect(text).toContain(LOGO_PLAY);
  });
});
