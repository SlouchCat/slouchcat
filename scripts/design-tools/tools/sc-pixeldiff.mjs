#!/usr/bin/env node
// Compares two PNG files of the same size pixel by pixel and writes the difference as an image.
//
//   sc-pixeldiff <a.png> <b.png> [--out diff.png] [--threshold 0.1] [--max-ratio 0]
//
// Prints the number and share of differing pixels. Exit code 1 when the share is above
// --max-ratio or the sizes differ, 2 on usage errors.
import { readFileSync, writeFileSync } from 'node:fs';
import { parseArgs } from 'node:util';
import pixelmatch from 'pixelmatch';
import { PNG } from 'pngjs';

const { values, positionals } = parseArgs({
  allowPositionals: true,
  options: {
    out: { type: 'string' },
    threshold: { type: 'string', default: '0.1' },
    'max-ratio': { type: 'string', default: '0' },
  },
});

if (positionals.length !== 2) {
  console.error('usage: sc-pixeldiff <a.png> <b.png> [--out diff.png] [--max-ratio 0.001]');
  process.exit(2);
}

const [first, second] = positionals.map((file) => PNG.sync.read(readFileSync(file)));
if (first.width !== second.width || first.height !== second.height) {
  console.log(
    `sizes differ: ${first.width}x${first.height} against ${second.width}x${second.height}`,
  );
  process.exit(1);
}

const diff = new PNG({ width: first.width, height: first.height });
const pixels = pixelmatch(first.data, second.data, diff.data, first.width, first.height, {
  threshold: Number(values.threshold),
});
if (values.out) writeFileSync(values.out, PNG.sync.write(diff));

const ratio = pixels / (first.width * first.height);
console.log(`${pixels} differing pixels, ratio ${ratio.toFixed(6)}`);
process.exit(ratio > Number(values['max-ratio']) ? 1 : 0);
