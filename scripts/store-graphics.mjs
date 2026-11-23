// Renders the store graphics of docs/store/grafiken.md from the SVG sources in docs/store/grafiken/
// with the Chromium of the workspace's Playwright (no system tool needed). Called by
// scripts/store-graphics.sh, which checks the target folder.
//
//   node scripts/store-graphics.mjs <target folder>
//
// app-icon.png 512 x 512 (PNG 32 bit with alpha, as the Play Console asks), feature-graphic.png
// 1024 x 500 and tv-banner.png 1280 x 720 (PNG 24 bit without alpha). Chromium writes an opaque
// page as RGB, so the colour type is set here: the PNG is decoded and written again as RGBA or RGB
// (zlib of Node, no image library). Every file's size and colour type is checked after writing.
import { createRequire } from "node:module";
import { readFileSync, writeFileSync } from "node:fs";
import { deflateSync, inflateSync } from "node:zlib";
import path from "node:path";
import { fileURLToPath } from "node:url";

const root = path.resolve(path.dirname(fileURLToPath(import.meta.url)), "..");
const { chromium } = createRequire(path.join(root, "web/package.json"))("@playwright/test");

const graphics = [
  { source: "app-icon.svg", target: "app-icon.png", width: 512, height: 512, alpha: true },
  { source: "feature-graphic.svg", target: "feature-graphic.png", width: 1024, height: 500, alpha: false },
  { source: "tv-banner.svg", target: "tv-banner.png", width: 1280, height: 720, alpha: false },
];

const target = process.argv[2];
if (!target) {
  console.error("usage: node scripts/store-graphics.mjs <target folder>");
  process.exit(2);
}

const browser = await chromium.launch();
try {
  const page = await browser.newPage({ deviceScaleFactor: 1 });
  for (const g of graphics) {
    const svg = readFileSync(path.join(root, "docs/store/grafiken", g.source), "utf8");
    await page.setViewportSize({ width: g.width, height: g.height });
    await page.setContent(`<!doctype html><html><body style="margin:0">${svg}</body></html>`);
    const shot = await page.screenshot({ type: "png", clip: { x: 0, y: 0, width: g.width, height: g.height } });
    const out = recode(shot, g.alpha);
    writeFileSync(path.join(target, g.target), out);
    const info = header(out);
    console.log(`${g.target}: ${info.width} x ${info.height}, ${info.alpha ? "RGBA" : "RGB"}, ${out.length} bytes`);
    if (info.width !== g.width || info.height !== g.height || info.alpha !== g.alpha) {
      throw new Error(`${g.target}: expected ${g.width} x ${g.height} ${g.alpha ? "RGBA" : "RGB"}`);
    }
  }
} finally {
  await browser.close();
}

/** Width, height, channels and whether the PNG has an alpha channel (IHDR colour type 6). */
function header(png) {
  const type = png[25];
  const channels = { 2: 3, 6: 4 }[type];
  return { width: png.readUInt32BE(16), height: png.readUInt32BE(20), channels, alpha: type === 6 };
}

/** An 8-bit RGB or RGBA PNG written again as RGBA (`alpha`) or RGB; every pixel must be opaque. */
function recode(png, alpha) {
  const { width, height, channels } = header(png);
  if (png[24] !== 8 || !channels || png[28] !== 0) throw new Error("expected an 8-bit RGB or RGBA PNG without interlace");
  const idat = [];
  for (let at = 8; at < png.length; ) {
    const length = png.readUInt32BE(at);
    if (png.toString("latin1", at + 4, at + 8) === "IDAT") idat.push(png.subarray(at + 8, at + 8 + length));
    at += 12 + length;
  }
  // Undo the row filters (PNG specification, section 9).
  const raw = inflateSync(Buffer.concat(idat));
  const stride = width * channels;
  const pixels = Buffer.alloc(stride * height);
  for (let y = 0; y < height; y++) {
    const filter = raw[y * (stride + 1)];
    for (let x = 0; x < stride; x++) {
      const a = x >= channels ? pixels[y * stride + x - channels] : 0;
      const b = y > 0 ? pixels[(y - 1) * stride + x] : 0;
      const c = x >= channels && y > 0 ? pixels[(y - 1) * stride + x - channels] : 0;
      let v = raw[y * (stride + 1) + 1 + x];
      if (filter === 1) v += a;
      else if (filter === 2) v += b;
      else if (filter === 3) v += (a + b) >> 1;
      else if (filter === 4) {
        const p = a + b - c;
        const pa = Math.abs(p - a), pb = Math.abs(p - b), pc = Math.abs(p - c);
        v += pa <= pb && pa <= pc ? a : pb <= pc ? b : c;
      } else if (filter !== 0) throw new Error(`unknown PNG filter ${filter}`);
      pixels[y * stride + x] = v & 0xff;
    }
  }
  // Rows without filter in the wanted colour type.
  const out = alpha ? 4 : 3;
  const rows = Buffer.alloc((width * out + 1) * height);
  for (let y = 0; y < height; y++) {
    for (let x = 0; x < width; x++) {
      const from = y * stride + x * channels;
      if (channels === 4 && pixels[from + 3] !== 255) throw new Error(`pixel ${x},${y} is not opaque`);
      const to = y * (width * out + 1) + 1 + x * out;
      pixels.copy(rows, to, from, from + 3);
      if (alpha) rows[to + 3] = 255;
    }
  }
  const ihdr = Buffer.alloc(13);
  ihdr.writeUInt32BE(width, 0);
  ihdr.writeUInt32BE(height, 4);
  ihdr.set([8, alpha ? 6 : 2, 0, 0, 0], 8);
  return Buffer.concat([png.subarray(0, 8), chunk("IHDR", ihdr), chunk("IDAT", deflateSync(rows, { level: 9 })), chunk("IEND", Buffer.alloc(0))]);
}

function chunk(type, data) {
  const head = Buffer.alloc(8);
  head.writeUInt32BE(data.length, 0);
  head.write(type, 4, "latin1");
  const crc = Buffer.alloc(4);
  crc.writeUInt32BE(crc32(Buffer.concat([head.subarray(4), data])), 0);
  return Buffer.concat([head, data, crc]);
}

function crc32(bytes) {
  let crc = 0xffffffff;
  for (const byte of bytes) {
    crc ^= byte;
    for (let k = 0; k < 8; k++) crc = crc & 1 ? (crc >>> 1) ^ 0xedb88320 : crc >>> 1;
  }
  return (crc ^ 0xffffffff) >>> 0;
}
