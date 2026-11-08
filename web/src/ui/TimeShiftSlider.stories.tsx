import type { Meta, StoryObj } from '@storybook/react-vite';
import { useState } from 'react';
import { TimeShiftSlider } from './TimeShiftSlider';

const pad = (value: number) => String(value).padStart(2, '0');
const shown = (offset: number) =>
  offset === 0 ? 'Live' : `−${Math.floor(offset / 60)}:${pad(offset % 60)}`;
const spoken = (offset: number) =>
  offset === 0 ? 'Live' : `${Math.floor(offset / 60)} Minuten ${offset % 60} Sekunden zurück`;

const meta = {
  title: 'UI/TimeShiftSlider',
  component: TimeShiftSlider,
  args: {
    label: 'Zeitversatz, bis 30 Minuten zurück',
    maxSeconds: 1800,
    offset: 750,
    onSeek: () => undefined,
    valueText: spoken,
    shownText: shown,
    startLabel: '−30:00',
    endLabel: 'Live',
  },
  decorators: [
    (Story) => (
      <div style={{ maxWidth: '32rem' }}>
        <Story />
      </div>
    ),
  ],
} satisfies Meta<typeof TimeShiftSlider>;

export default meta;
type Story = StoryObj<typeof meta>;

/** 12:30 behind live. */
export const Behind: Story = {};

/** At live: «Live» in the live red. */
export const AtLive: Story = { args: { offset: 0 } };

/** A 10-minute window: the slider goes back to −10:00. */
export const ShortWindow: Story = {
  args: {
    maxSeconds: 600,
    offset: 125,
    label: 'Zeitversatz, bis 10 Minuten zurück',
    startLabel: '−10:00',
  },
};

/** Moves and seeks as in the player (arrow keys in steps of 5 s). */
export const Interactive: Story = {
  render: function Render(args) {
    const [offset, setOffset] = useState(args.offset);
    return <TimeShiftSlider {...args} offset={offset} onSeek={setOffset} />;
  },
};
