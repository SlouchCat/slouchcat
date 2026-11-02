import type { Meta, StoryObj } from '@storybook/react-vite';
import { HeatGrid } from './HeatGrid';

// Minutes per hour of the week, Monday 00:00 first, with a fixed pattern: evenings, and the
// weekend from the afternoon on.
const values = Array.from({ length: 7 * 24 }, (_, index) => {
  const day = Math.floor(index / 24);
  const hour = index % 24;
  const evening = hour >= 19 && hour <= 23 ? 30 + (hour - 19) * 12 : 0;
  const weekend = day >= 5 && hour >= 14 && hour < 19 ? 25 + (hour - 14) * 5 : 0;
  return evening + weekend + (hour === 7 ? 10 : 0);
});

const meta = {
  title: 'UI/HeatGrid',
  component: HeatGrid,
  args: {
    title: 'Wann geschaut wird',
    values,
    formatValue: (minutes: number) => `${minutes} min`,
  },
  decorators: [
    (Story) => (
      <div style={{ width: '40rem' }}>
        <Story />
      </div>
    ),
  ],
} satisfies Meta<typeof HeatGrid>;

export default meta;
type Story = StoryObj<typeof meta>;

/** Seven days, 24 hours; the accent the stronger the more was watched. */
export const Default: Story = {};

export const Compact: Story = { globals: { density: 'compact' } };
