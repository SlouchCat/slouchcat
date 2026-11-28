#!/usr/bin/env node
// Renders an SVG file to a PNG with resvg. Transparent unless --background is given.
//
//   sc-svg2png <in.svg> <out.png> [--width N | --height N | --zoom F] [--background '#rrggbb']
//              [--font file.ttf ...]
//
// System fonts are loaded; --font adds font files for text that is not yet converted to paths.
import { readFileSync, writeFileSync } from 'node:fs';
import { parseArgs } from 'node:util';
import { Resvg } from '@resvg/resvg-js';

const { values, positionals } = parseArgs({
  allowPositionals: true,
  options: {
    width: { type: 'string' },
    height: { type: 'string' },
    zoom: { type: 'string' },
    background: { type: 'string' },
    font: { type: 'string', multiple: true, default: [] },
  },
});

if (positionals.length !== 2) {
  console.error('usage: sc-svg2png <in.svg> <out.png> [--width N | --height N | --zoom F]');
  process.exit(2);
}

let fitTo = { mode: 'original' };
if (values.width) fitTo = { mode: 'width', value: Number(values.width) };
else if (values.height) fitTo = { mode: 'height', value: Number(values.height) };
else if (values.zoom) fitTo = { mode: 'zoom', value: Number(values.zoom) };

const resvg = new Resvg(readFileSync(positionals[0]), {
  fitTo,
  background: values.background,
  font: { loadSystemFonts: true, fontFiles: values.font },
});
const image = resvg.render();
writeFileSync(positionals[1], image.asPng());
console.log(`${positionals[1]}: ${image.width}x${image.height}`);
