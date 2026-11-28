#!/usr/bin/env node
// Drives the two browser MCP servers over stdio, the way Claude Code does, against the test page of
// scripts/design-tools-check.sh: open, click, type, resize, screenshot, console, network, and for
// Chrome DevTools a performance trace and a Lighthouse audit. Proves the servers work without a
// model in between.
//
//   sc-mcp-check <url of defect.html> [--only playwright|chrome-devtools]
//
// The page must hold a button "Open dialog" that shows "Dialog open", a text field "Name" and an
// image missing.png that does not exist. Exit code 1 when a step fails.
import { dirname, resolve } from 'node:path';
import { fileURLToPath } from 'node:url';
import { parseArgs } from 'node:util';
import { Client } from '@modelcontextprotocol/sdk/client/index.js';
import { StdioClientTransport } from '@modelcontextprotocol/sdk/client/stdio.js';

const { values, positionals } = parseArgs({
  allowPositionals: true,
  options: { only: { type: 'string' } },
});
const url = positionals[0];
if (!url) {
  console.error('usage: sc-mcp-check <url of defect.html> [--only playwright|chrome-devtools]');
  process.exit(2);
}

const bin = resolve(dirname(fileURLToPath(import.meta.url)), '..', '..', 'bin');
let failures = 0;

async function connect(command) {
  const client = new Client({ name: 'sc-mcp-check', version: '1.0.0' });
  await client.connect(new StdioClientTransport({ command, args: [], stderr: 'ignore' }));
  return client;
}

// All text parts of a tool result, joined. A tool error throws.
async function call(client, name, args = {}) {
  const result = await client.callTool({ name, arguments: args }, undefined, { timeout: 120_000 });
  const text = result.content
    .filter((part) => part.type === 'text')
    .map((part) => part.text)
    .join('\n');
  if (result.isError) throw new Error(`${name}: ${text.slice(0, 300)}`);
  return { text, images: result.content.filter((part) => part.type === 'image').length };
}

async function step(label, run) {
  try {
    const detail = await run();
    console.log(`ok    ${label}${detail ? ` (${detail})` : ''}`);
  } catch (error) {
    failures += 1;
    console.log(`FAIL  ${label}: ${String(error.message ?? error).split('\n')[0]}`);
  }
}

function expect(condition, message) {
  if (!condition) throw new Error(message);
}

async function playwright() {
  const client = await connect(resolve(bin, 'sc-mcp-playwright'));
  try {
    let snapshot = '';
    await step('playwright: tools listed', async () => {
      const { tools } = await client.listTools();
      const names = tools.map((tool) => tool.name);
      for (const name of ['browser_navigate', 'browser_click', 'browser_take_screenshot']) {
        expect(names.includes(name), `tool ${name} missing`);
      }
      return `${names.length} tools`;
    });
    await step('playwright: page opened', async () => {
      await call(client, 'browser_navigate', { url });
      snapshot = (await call(client, 'browser_snapshot')).text;
      expect(snapshot.includes('Open dialog'), 'button not in the snapshot');
    });
    await step('playwright: button clicked, dialog shown', async () => {
      const ref = /button "Open dialog"[^\n]*\[ref=([^\]]+)\]/.exec(snapshot)?.[1];
      expect(ref, 'no ref for the button');
      await call(client, 'browser_click', { element: 'Open dialog button', target: ref });
      const after = (await call(client, 'browser_snapshot')).text;
      expect(after.includes('Dialog open'), 'dialog text not in the snapshot');
      await call(client, 'browser_press_key', { key: 'Escape' });
    });
    await step('playwright: form field filled', async () => {
      const ref = /textbox "Name"[^\n]*\[ref=([^\]]+)\]/.exec(snapshot)?.[1];
      expect(ref, 'no ref for the text field');
      await call(client, 'browser_type', { element: 'Name field', target: ref, text: 'Slouchcat' });
      const value = await call(client, 'browser_evaluate', {
        function: '() => document.getElementById("name").value',
      });
      expect(value.text.includes('Slouchcat'), 'value not set');
    });
    await step('playwright: phone size and screenshot', async () => {
      await call(client, 'browser_resize', { width: 390, height: 844 });
      const width = await call(client, 'browser_evaluate', { function: '() => window.innerWidth' });
      expect(width.text.includes('390'), `inner width is not 390: ${width.text.slice(0, 80)}`);
      const shot = await call(client, 'browser_take_screenshot', { type: 'png', scale: 'css' });
      expect(shot.images > 0 || /\.png/.test(shot.text), 'no image in the answer');
    });
    await step('playwright: console error seen', async () => {
      const messages = await call(client, 'browser_console_messages', { level: 'error' });
      expect(/404|Failed to load/i.test(messages.text), 'no error for the missing image');
    });
    await step('playwright: failed request seen', async () => {
      const requests = await call(client, 'browser_network_requests', { static: true });
      expect(requests.text.includes('missing.png'), 'missing.png not in the request list');
    });
    await step('playwright: broken image found', async () => {
      const broken = await call(client, 'browser_evaluate', {
        function:
          '() => [...document.images].filter((i) => i.complete && i.naturalWidth === 0).length',
      });
      expect(/\b1\b/.test(broken.text), `expected one broken image: ${broken.text.slice(0, 80)}`);
    });
  } finally {
    await client.close();
  }
}

async function chromeDevtools() {
  const client = await connect(resolve(bin, 'sc-mcp-chrome-devtools'));
  try {
    let pageId;
    await step('chrome-devtools: tools listed', async () => {
      const { tools } = await client.listTools();
      const names = tools.map((tool) => tool.name);
      for (const name of ['new_page', 'performance_start_trace', 'lighthouse_audit']) {
        expect(names.includes(name), `tool ${name} missing`);
      }
      return `${names.length} tools`;
    });
    await step('chrome-devtools: page opened', async () => {
      await call(client, 'new_page', { url });
      const pages = (await call(client, 'list_pages')).text;
      const line = pages.split('\n').find((entry) => entry.includes(url));
      pageId = Number(/(\d+)/.exec(line ?? '')?.[1]);
      expect(Number.isInteger(pageId), `no page id in: ${pages.slice(0, 200)}`);
      const snapshot = await call(client, 'take_snapshot', { pageId });
      expect(snapshot.text.includes('Open dialog'), 'button not in the snapshot');
    });
    await step('chrome-devtools: console and network read', async () => {
      const requests = await call(client, 'list_network_requests', { pageId });
      expect(requests.text.includes('missing.png'), 'missing.png not in the request list');
      const messages = await call(client, 'list_console_messages', { pageId });
      expect(/404|Failed to load/i.test(messages.text), 'no console error for the missing image');
    });
    await step('chrome-devtools: phone size and screenshot', async () => {
      await call(client, 'resize_page', { pageId, width: 390, height: 844 });
      const shot = await call(client, 'take_screenshot', { pageId, format: 'png' });
      expect(shot.images > 0 || /\.png/.test(shot.text), 'no image in the answer');
    });
    await step('chrome-devtools: performance trace analysed', async () => {
      const trace = await call(client, 'performance_start_trace', {
        pageId,
        reload: true,
        autoStop: true,
      });
      expect(/LCP|CLS|insight/i.test(trace.text), `no metrics in: ${trace.text.slice(0, 200)}`);
      return /LCP[^\n]*/.exec(trace.text)?.[0].slice(0, 60);
    });
    await step('chrome-devtools: Lighthouse audit run', async () => {
      const audit = await call(client, 'lighthouse_audit', {
        pageId,
        mode: 'navigation',
        device: 'desktop',
      });
      expect(/accessibility/i.test(audit.text), `no scores in: ${audit.text.slice(0, 200)}`);
    });
  } finally {
    await client.close();
  }
}

if (!values.only || values.only === 'playwright') await playwright();
if (!values.only || values.only === 'chrome-devtools') await chromeDevtools();
process.exit(failures > 0 ? 1 : 0);
