import type { Meta, StoryObj } from '@storybook/react-vite';
import { BarChart, type BarDatum } from './BarChart';

// Thirty days of watch time (K-5) with a fixed pattern, so the screenshot stays stable.
const minutes = [
  0, 12, 45, 30, 0, 0, 62, 88, 20, 15, 0, 40, 55, 60, 75, 10, 0, 0, 35, 50, 95, 60, 25, 0, 5, 30,
  45, 70, 60, 18,
];
const bars: BarDatum[] = minutes.map((value, index) => {
  const day = String((index % 30) + 1).padStart(2, '0');
  return {
    key: `2026-09-${day}`,
    label: `${day}.09.2026`,
    tick: `${day}.09.`,
    value,
    valueText: `${value} min`,
    tooltip: `${day}.09.2026: ${value} min`,
  };
});

const meta = {
  title: 'UI/BarChart',
  component: BarChart,
  args: {
    title: 'Schauzeit pro Tag',
    bars,
    columns: ['Tag', 'Minuten'],
  },
  decorators: [
    (Story) => (
      <div style={{ width: '40rem', padding: 'var(--sc-space-6)' }}>
        <Story />
      </div>
    ),
  ],
} satisfies Meta<typeof BarChart>;

export default meta;
type Story = StoryObj<typeof meta>;

export const Plain: Story = {};

/** With a daily limit: days above it in the warning colour. */
export const WithThreshold: Story = { args: { threshold: { value: 60, label: 'Limit 60 min' } } };
