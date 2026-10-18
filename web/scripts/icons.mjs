// Generates the PWA icons in public/icons/ from public/icon.svg with ffmpeg (librsvg decoder).
//
//   icon-192.png, icon-512.png   the SVG as is (purpose "any")
//   icon-512-maskable.png        the SVG scaled to 80 % on the token background (purpose "maskable"),
//                                so the safe zone of every platform mask keeps the whole mark
//
// Run with `pnpm run icons` after changing icon.svg or the tokens; the PNGs are committed.
import { execFileSync } from 'node:child_process';
import { mkdirSync, readFileSync } from 'node:fs';
import { fileURLToPath } from 'node:url';
import { tokens } from '@slouchcat/design-tokens';

const publicDir = fileURLToPath(new URL('../public/', import.meta.url));
const svgPath = `${publicDir}icon.svg`;
const outDir = `${publicDir}icons/`;

const background = tokens.color.bg.base;
const allowed = new Set([background, tokens.color.accent.ember.base].map((c) => c.toLowerCase()));

// The SVG cannot reference CSS variables when it is used as an app icon, so it carries the token
// values literally. Fail loudly when it drifts from the tokens.
const used = readFileSync(svgPath, 'utf8').match(/#[0-9a-f]{6}\b/gi) ?? [];
const drift = used.filter((c) => !allowed.has(c.toLowerCase()));
if (drift.length > 0) {
  throw new Error(`icon.svg uses colours that are not design tokens: ${drift.join(', ')}`);
}

mkdirSync(outDir, { recursive: true });

function ffmpeg(args) {
  execFileSync('ffmpeg', ['-hide_banner', '-loglevel', 'error', '-y', ...args], {
    stdio: 'inherit',
  });
}

function plain(size) {
  const out = `${outDir}icon-${size}.png`;
  ffmpeg(['-width', String(size), '-height', String(size), '-i', svgPath, '-frames:v', '1', out]);
  console.log(`wrote ${out}`);
}

function maskable(size) {
  const inner = Math.round(size * 0.8);
  const out = `${outDir}icon-${size}-maskable.png`;
  const hex = background.replace('#', '0x');
  ffmpeg([
    '-f',
    'lavfi',
    '-i',
    `color=c=${hex}:s=${size}x${size},format=rgba`,
    '-width',
    String(inner),
    '-height',
    String(inner),
    '-i',
    svgPath,
    '-filter_complex',
    '[0:v][1:v]overlay=(W-w)/2:(H-h)/2:format=rgb,format=rgb24',
    '-frames:v',
    '1',
    out,
  ]);
  console.log(`wrote ${out}`);
}

plain(192);
plain(512);
maskable(512);
