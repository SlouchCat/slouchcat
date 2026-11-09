import type { Meta, StoryObj } from '@storybook/react-vite';
import { SeekBar } from './SeekBar';
import { art } from './stories/fixtures';

// A film of two hours: eight chapters, the third without a title.
const chapters = [
  { startMs: 0, title: 'Anfang' },
  { startMs: 840_000, title: 'Die Farm' },
  { startMs: 1_620_000 },
  { startMs: 2_700_000, title: 'Los Angeles' },
  { startMs: 3_600_000, title: 'Das Archiv' },
  { startMs: 4_500_000, title: 'Las Vegas' },
  { startMs: 5_640_000, title: 'Der Damm' },
  { startMs: 6_600_000, title: 'Schnee' },
];

const noop = () => undefined;

const meta = {
  title: 'UI/SeekBar',
  component: SeekBar,
  args: {
    label: 'Wiedergabeposition',
    valueText: '0:54:00 von 2:00:00',
    currentMs: 3_240_000,
    durationMs: 7_200_000,
    buffered: [[3_000_000, 3_900_000]],
    onSeek: noop,
  },
  decorators: [
    (Story) => (
      <div style={{ width: '48rem', paddingTop: '6rem' }}>
        <Story />
      </div>
    ),
  ],
} satisfies Meta<typeof SeekBar>;

export default meta;
type Story = StoryObj<typeof meta>;

/** Played part in the accent, the buffer ahead lighter; no chapters. */
export const Default: Story = {};

/** Chapter marks as notches, a lighter intro at the start and outro at the end. */
export const Chapters: Story = {
  args: {
    chapters,
    segments: {
      intro: { startMs: 60_000, endMs: 300_000 },
      outro: { startMs: 6_900_000, endMs: 7_200_000 },
    },
  },
};

/** Focused by keyboard: the track thickens and shows the thumb. */
export const Focused: Story = { args: { chapters }, tags: ['press:Tab'] };

/** Rests the pointer over the bar at a fraction of its width. */
const pointAt =
  (at: number) =>
  ({ canvasElement }: { canvasElement: HTMLElement }) => {
    const slider = canvasElement.querySelector<HTMLElement>('[role="slider"]');
    if (!slider) return;
    const box = slider.getBoundingClientRect();
    slider.dispatchEvent(
      new PointerEvent('pointermove', {
        bubbles: true,
        pointerType: 'mouse',
        clientX: box.left + box.width * at,
        clientY: box.top + box.height / 2,
      }),
    );
  };

/**
 * The pointer rests over the third chapter: the field shows the time and «Kapitel 3», which has no
 * title of its own.
 */
export const Preview: Story = { args: { chapters }, play: pointAt(0.3) };

/** Room above the bar for a field with a picture. */
const roomForPicture = (Story: () => React.JSX.Element) => (
  <div style={{ paddingTop: '6rem' }}>
    <Story />
  </div>
);

/**
 * Stage 3: the chapters have images, and the field shows the image of the chapter under the
 * pointer («Los Angeles») above the time and the name.
 */
export const PreviewImage: Story = {
  args: { chapters: chapters.map((chapter) => ({ ...chapter, imageUrl: art.backdropDusk })) },
  decorators: [roomForPicture],
  play: pointAt(0.4),
};

/**
 * A drawn sheet of 10 x 10 tiles of 320 x 180 (no film frames): each tile its own hue, with a
 * lighter band whose place moves with the tile, so the right cut-out is visible.
 */
function drawnSheet(rows: number): string {
  const tiles = Array.from({ length: rows * 10 }, (_, k) => {
    const x = (k % 10) * 320;
    const y = Math.floor(k / 10) * 180;
    return (
      `<rect x='${x}' y='${y}' width='320' height='180' fill='hsl(${(k * 37) % 360} 45% 32%)'/>` +
      `<rect x='${x + 20 + (k % 10) * 26}' y='${y + 30}' width='24' height='120' fill='hsl(${(k * 37) % 360} 60% 70%)'/>`
    );
  }).join('');
  const svg = `<svg xmlns='http://www.w3.org/2000/svg' viewBox='0 0 3200 ${rows * 180}'>${tiles}</svg>`;
  return `data:image/svg+xml;utf8,${encodeURIComponent(svg)}`;
}

// Two hours, a point every 10 s: 720 tiles on eight sheets, the last with 2 rows.
const trickplay = {
  tileWidth: 320,
  tileHeight: 180,
  columns: 10,
  rows: 10,
  pointsMs: Array.from({ length: 720 }, (_, k) => k * 10_000),
  sheets: Array.from({ length: 8 }, (_, index) => ({
    index,
    url: drawnSheet(index === 7 ? 2 : 10),
  })),
};

/**
 * Concept 18 part A: the field shows the preview tile of the position (here 0:48:00, tile 288 on
 * sheet 2) above the time and the chapter, in the tile's aspect ratio.
 */
export const PreviewTile: Story = {
  args: { chapters, trickplay },
  decorators: [roomForPicture],
  play: pointAt(0.4),
};

/**
 * While the set is made, a sheet that is not there yet shows the chapter image in the box of the
 * tiles, so the field keeps its height (2.40:1 here).
 */
export const PreviewTilePending: Story = {
  args: {
    chapters: chapters.map((chapter) => ({ ...chapter, imageUrl: art.backdropDusk })),
    trickplay: { ...trickplay, tileHeight: 134, sheets: [] },
  },
  decorators: [roomForPicture],
  play: pointAt(0.4),
};
