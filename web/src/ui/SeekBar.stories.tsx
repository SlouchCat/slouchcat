import type { Meta, StoryObj } from '@storybook/react-vite';
import { SeekBar } from './SeekBar';

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

/**
 * The pointer rests over the third chapter: the field shows the time and «Kapitel 3», which has no
 * title of its own.
 */
export const Preview: Story = {
  args: { chapters },
  play: ({ canvasElement }) => {
    const slider = canvasElement.querySelector<HTMLElement>('[role="slider"]');
    if (!slider) return;
    const box = slider.getBoundingClientRect();
    slider.dispatchEvent(
      new PointerEvent('pointermove', {
        bubbles: true,
        pointerType: 'mouse',
        clientX: box.left + box.width * 0.3,
        clientY: box.top + box.height / 2,
      }),
    );
  },
};
