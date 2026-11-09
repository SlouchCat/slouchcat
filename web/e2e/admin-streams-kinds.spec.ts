import type { Page, Route } from '@playwright/test';
import { expect, test } from './fixtures';
import {
  expectFocusOn,
  expectNoFocusTrap,
  expectVisibleFocus,
  resetFocus,
  tabTo,
} from './keyboard';

// Concept 18, 17.3 and 17.6: «Läuft gerade» lists everything that runs in one list, newest first:
// a film, an episode, an extra, an audiobook, a podcast episode from a file and one from the feed,
// and a radio listener, each with its kind. The filter is a toolbar of toggle buttons with counts
// (←/→ between them, Tab into the tiles), it holds across the 5-s poll also when its kind runs
// out, and a failed half keeps the other with a notice and «–» for its counts. «Beenden» stops a
// playback and a listener after the confirmation (rule 8: keyboard, visible focus, no trap,
// browser back). The two lists are stubbed: the e2e server has no audiobook, feed and radio
// sessions running at once, and real streams are tested in admin-streams.spec.ts.

// The stubs must see every request: without the service worker, which would fetch past them.
test.use({ serviceWorkers: 'block' });

const at = (minute: number) => `2026-11-09T18:${String(minute).padStart(2, '0')}:00Z`;

const base = {
  accountEmail: 'owner@example.test',
  device: { id: 'dev-1', name: 'Wohnzimmer (Chrome)' },
  hardware: false,
  lastActivityAt: at(59),
  mode: 'direct',
  source: 'file',
};

/** One playback of every kind, oldest last (the server answers newest first, the page sorts). */
const playbacks = [
  {
    ...base,
    id: '01JS0000000000000000000006',
    profileName: 'Gast',
    itemId: 'film',
    itemTitle: 'A Beautiful Mind',
    mediaFileId: 'mf-film',
    kind: 'movie',
    startedAt: at(45),
    positionMs: 1_660_000,
    durationMs: 8_130_000,
  },
  {
    ...base,
    id: '01JS0000000000000000000005',
    profileName: 'Marco',
    itemId: 'episode',
    itemTitle: 'Two and a Half Men · S09E12 · Fortyish Lady',
    mediaFileId: 'mf-episode',
    kind: 'episode',
    mode: 'transcode',
    hardware: true,
    reason: 'audio_codec',
    quality: '1080p',
    bitrate: 8_000_000,
    startedAt: at(31),
    positionMs: 692_000,
    durationMs: 1_264_000,
  },
  {
    ...base,
    id: '01JS0000000000000000000004',
    profileName: 'Kim',
    itemId: 'extra',
    itemTitle: 'Making-of',
    parentTitle: 'A Beautiful Mind',
    mediaFileId: 'mf-extra',
    kind: 'extra',
    startedAt: at(25),
  },
  {
    ...base,
    id: '01JS0000000000000000000003',
    profileName: 'Kids',
    itemId: 'book',
    itemTitle: 'Beispiel-Hörbuch',
    chapterTitle: 'Kapitel 7: Der Sturm',
    partCount: 12,
    audioCodec: 'aac',
    audioBitrate: 64_000,
    mediaFileId: 'mf-book',
    kind: 'audiobook',
    mode: 'remux',
    startedAt: at(20),
    positionMs: 13_270_000,
    durationMs: 31_560_000,
  },
  {
    ...base,
    id: '01JS0000000000000000000002',
    profileName: 'Lena',
    itemId: 'pod-file',
    itemTitle: 'Neueste Folge',
    parentTitle: 'Bad Friends',
    audioCodec: 'mp3',
    audioBitrate: 192_000,
    mediaFileId: 'mf-pod',
    kind: 'podcast_episode',
    startedAt: at(12),
    positionMs: 3_063_000,
    durationMs: 4_360_000,
  },
  {
    ...base,
    id: '01JS0000000000000000000001',
    profileName: 'Lena',
    itemId: 'pod-feed',
    itemTitle: 'Folge vom 9. November',
    parentTitle: 'Apropos',
    audioCodec: 'mp3',
    mediaFileId: '',
    source: 'feed',
    kind: 'podcast_episode',
    startedAt: at(39),
    positionMs: 432_000,
    durationMs: 1_231_000,
  },
];

const radio = {
  relays: [
    {
      stationId: 7,
      name: 'Radio SRF 3',
      codec: 'AAC',
      bitrateKbps: 128,
      transcoded: false,
      startedAt: Date.parse(at(40)),
      bufferSeconds: 600,
      bufferBytes: 9_600_000,
      state: 'live',
      listeners: [
        {
          listenId: '01JR0000000000000000000001',
          profileName: 'Marco',
          accountEmail: 'owner@example.test',
          playing: true,
          since: Date.parse(at(42)),
          device: { id: 'dev-2', name: 'Küche (iPhone)' },
        },
      ],
    },
  ],
  load: {
    relays: 1,
    maxRelays: 10,
    bufferBytes: 9_600_000,
    budgetBytes: 512_000_000,
    uploadKbps: 128,
    bufferMode: 'tmpfs',
  },
};

/** The article names (profile: title) in the order of the list. */
const order = [
  'Gast: A Beautiful Mind',
  'Marco: Radio SRF 3',
  'Lena: Apropos · Folge vom 9. November',
  'Marco: Two and a Half Men · S09E12 · Fortyish Lady',
  'Kim: A Beautiful Mind · Making-of',
  'Kids: Beispiel-Hörbuch · Kapitel 7: Der Sturm',
  'Lena: Bad Friends · Neueste Folge',
];

interface Stubs {
  streams: typeof playbacks;
  radio: typeof radio;
  radioFails: boolean;
  deleted: string[];
}

/** Stubs both lists and the two stops; images of the stubs answer 404 (the fallbacks show). */
async function stub(page: Page): Promise<Stubs> {
  const state: Stubs = { streams: [...playbacks], radio, radioFails: false, deleted: [] };
  await page.route('**/api/v1/admin/streams', (route) => route.fulfill({ json: state.streams }));
  await page.route('**/api/v1/admin/streams/*', async (route: Route) => {
    if (route.request().method() !== 'DELETE') return route.fallback();
    const id = route.request().url().split('/').pop()!;
    state.deleted.push(id);
    state.streams = state.streams.filter((stream) => stream.id !== id);
    await route.fulfill({ status: 204 });
  });
  await page.route('**/api/v1/admin/radio/relays', (route) =>
    state.radioFails
      ? route.fulfill({
          status: 500,
          contentType: 'application/problem+json',
          json: { type: 'about:blank', title: 'internal', status: 500, code: 'internal' },
        })
      : route.fulfill({ json: state.radio }),
  );
  await page.route('**/api/v1/admin/radio/listeners/*', async (route) => {
    if (route.request().method() !== 'DELETE') return route.fallback();
    const id = route.request().url().split('/').pop()!;
    state.deleted.push(id);
    state.radio = {
      ...state.radio,
      relays: state.radio.relays.map((relay) => ({
        ...relay,
        listeners: relay.listeners.filter((listener) => listener.listenId !== id),
      })),
    };
    await route.fulfill({ status: 204 });
  });
  return state;
}

test('every kind in one list with the kind filter, a failed half and «Beenden» @keyboard', async ({
  page,
}) => {
  test.setTimeout(90_000);
  const state = await stub(page);
  await page.goto('/admin/streams');
  const heading = page.getByRole('heading', { level: 1, name: 'Streams' });
  await expect(heading).toBeVisible();
  const list = page.getByRole('region', { name: 'Läuft gerade' });
  const tiles = list.getByRole('article');
  await expect(tiles).toHaveCount(7);
  expect(
    await tiles.evaluateAll((all) => all.map((tile) => tile.getAttribute('aria-label'))),
  ).toEqual(order);
  // Each tile carries its kind; the radio names device, format and state, the book its chapter,
  // format and the whole book's length, the feed episode its source.
  const tile = (name: string) => list.getByRole('article', { name });
  for (const [name, kind] of [
    ['Gast: A Beautiful Mind', 'Film'],
    ['Marco: Two and a Half Men · S09E12 · Fortyish Lady', 'Serie'],
    ['Kim: A Beautiful Mind · Making-of', 'Extra'],
    ['Kids: Beispiel-Hörbuch · Kapitel 7: Der Sturm', 'Hörbuch'],
    ['Lena: Bad Friends · Neueste Folge', 'Podcast'],
    ['Marco: Radio SRF 3', 'Radio'],
  ] as const) {
    await expect(tile(name).getByText(kind, { exact: true })).toBeVisible();
  }
  const book = tile('Kids: Beispiel-Hörbuch · Kapitel 7: Der Sturm');
  await expect(book).toContainText(/AAC · 64\skbit\/s · 12 Teile/);
  await expect(book).toContainText('3:41:10 von 8:46:00');
  await expect(tile('Lena: Apropos · Folge vom 9. November')).toContainText('Feed');
  await expect(tile('Lena: Bad Friends · Neueste Folge')).toContainText('Datei');
  const srf = tile('Marco: Radio SRF 3');
  await expect(srf).toContainText('Küche (iPhone)');
  await expect(srf).toContainText(/AAC · 128\skbit\/s · umgepackt/);
  await expect(srf).toContainText('Hört');
  await expect(page.getByRole('region', { name: 'Radio' })).toHaveCount(0);

  // The counters: «Alle» is the sum of the others.
  const toolbar = page.getByRole('toolbar', { name: 'Nach Art filtern' });
  const filter = (label: string) => toolbar.getByRole('button', { name: new RegExp(`^${label}`) });
  for (const [label, count] of [
    ['Alle', '7'],
    ['Video', '3'],
    ['Hörbücher', '1'],
    ['Podcasts', '2'],
    ['Radio', '1'],
  ] as const) {
    await expect(filter(label)).toHaveText(`${label} ${count}`);
  }
  await expect(filter('Alle')).toHaveAttribute('aria-pressed', 'true');

  // Keyboard: Tab into the toolbar, → to «Hörbücher», Enter; Tab leads into the tiles.
  await resetFocus(page);
  await tabTo(page, filter('Alle'), 60);
  await expectVisibleFocus(page);
  await page.keyboard.press('ArrowRight');
  await expectFocusOn(page, filter('Video'));
  await page.keyboard.press('ArrowRight');
  await expectFocusOn(page, filter('Hörbücher'));
  await page.keyboard.press('Enter');
  await expect(filter('Hörbücher')).toHaveAttribute('aria-pressed', 'true');
  await expect(filter('Alle')).toHaveAttribute('aria-pressed', 'false');
  await expect(tiles).toHaveCount(1);
  await page.keyboard.press('Tab');
  await expectFocusOn(page, book.getByRole('button', { name: 'Stream von Kids beenden' }));
  // Shift+Tab returns to the pressed button; → and Enter pick «Podcasts».
  await page.keyboard.press('Shift+Tab');
  await expectFocusOn(page, filter('Hörbücher'));
  await page.keyboard.press('ArrowRight');
  await page.keyboard.press('Enter');
  await expect(tiles).toHaveCount(2);

  // «Beenden» of the feed episode by keyboard: Tab into its tile, Enter, the confirmation, Enter.
  await page.keyboard.press('Tab');
  const stopFeed = tile('Lena: Apropos · Folge vom 9. November').getByRole('button', {
    name: 'Stream von Lena beenden',
  });
  await expectFocusOn(page, stopFeed);
  await page.keyboard.press('Enter');
  const confirm = page.getByRole('alertdialog', { name: 'Stream von Lena beenden?' });
  await expect(confirm).toContainText('Folge vom 9. November');
  await expectFocusOn(page, confirm.getByRole('button', { name: 'Stream beenden' }));
  await page.keyboard.press('Enter');
  await expect(page.getByText('Stream von Lena beendet.')).toBeVisible();
  expect(state.deleted).toEqual(['01JS0000000000000000000001']);
  await expect(tiles).toHaveCount(1);
  await expect(filter('Podcasts')).toHaveText('Podcasts 1');

  // The last podcast ends on its own: the filter stays, its text says so, «Alle» still counts.
  state.streams = state.streams.filter((stream) => stream.kind !== 'podcast_episode');
  await expect(page.getByText('Gerade läuft nichts dieser Art.')).toBeVisible({ timeout: 10_000 });
  await expect(filter('Podcasts')).toHaveAttribute('aria-pressed', 'true');
  await expect(filter('Podcasts')).toHaveText('Podcasts 0');
  await expect(filter('Alle')).toHaveText('Alle 5');
  await expect(tiles).toHaveCount(0);

  // The radio fails: the playbacks stay with a notice, the radio's counts say «–».
  state.radioFails = true;
  await expect(
    page.getByText('Radiohörer fehlen in der Liste, bis die Anfrage wieder gelingt.'),
  ).toBeVisible({ timeout: 10_000 });
  await expect(filter('Radio')).toHaveText(/^Radio –/);
  await expect(filter('Alle')).toHaveText(/^Alle –/);
  await filter('Alle').click();
  await expect(tiles).toHaveCount(4);
  await expect(tile('Marco: Radio SRF 3')).toHaveCount(0);
  // Back again: the listener returns; its «Beenden» goes through the radio.
  state.radioFails = false;
  await expect(tile('Marco: Radio SRF 3')).toBeVisible({ timeout: 10_000 });
  await expect(filter('Alle')).toHaveText('Alle 5');
  await tile('Marco: Radio SRF 3').getByRole('button', { name: 'Radio von Marco beenden' }).focus();
  await page.keyboard.press('Enter');
  const confirmRadio = page.getByRole('alertdialog', { name: 'Radio von Marco beenden?' });
  await expect(confirmRadio).toContainText('Radio SRF 3');
  await page.keyboard.press('Enter');
  await expect(page.getByText('Radio von Marco beendet.')).toBeVisible();
  expect(state.deleted).toContain('01JR0000000000000000000001');
  await expect(tile('Marco: Radio SRF 3')).toHaveCount(0);
  await expect(heading).toBeFocused();
  await expectNoFocusTrap(page);

  // Back (rule 8): another page and the browser's Back return to the list.
  await page.goto('/admin/devices');
  await page.goBack();
  await expect(heading).toBeVisible();
  await expect(tiles.first()).toBeVisible();
});
