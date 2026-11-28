// Shared browser start for the sc-* tools: the pinned Chrome for Testing of the toolbox, headless.
import { existsSync } from 'node:fs';
import { dirname, resolve } from 'node:path';
import { fileURLToPath } from 'node:url';
import { chromium } from 'playwright-core';

// Installed layout: <root>/node/tools/browser.mjs and <root>/bin/chrome.
export const toolboxRoot = resolve(dirname(fileURLToPath(import.meta.url)), '..', '..');

export function chromePath() {
  const path = process.env.SC_CHROME || resolve(toolboxRoot, 'bin', 'chrome');
  if (!existsSync(path)) {
    throw new Error(`Chrome not found at ${path}; run scripts/install-design-tools.sh`);
  }
  return path;
}

export async function launch() {
  // playwright-core turns Chrome's sandbox off unless asked; pages under test may hold outside content.
  return chromium.launch({ executablePath: chromePath(), headless: true, chromiumSandbox: true });
}

// "390x844" -> { width: 390, height: 844 }
export function parseViewport(text) {
  const match = /^(\d+)x(\d+)$/.exec(text.trim());
  if (!match) throw new Error(`viewport must look like 1440x900, got "${text}"`);
  return { width: Number(match[1]), height: Number(match[2]) };
}

// Phone and tablet widths get touch and a mobile viewport, like a real device would.
export function contextOptions(viewport, { colorScheme, locale, scale }) {
  const mobile = viewport.width < 600;
  return {
    viewport,
    deviceScaleFactor: scale,
    isMobile: mobile,
    hasTouch: viewport.width < 1100,
    colorScheme,
    locale,
    reducedMotion: 'reduce',
  };
}
