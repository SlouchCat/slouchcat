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

const weekdays = [
  ['Mo', 'Montag', 95],
  ['Di', 'Dienstag', 40],
  ['Mi', 'Mittwoch', 62],
  ['Do', 'Donnerstag', 20],
  ['Fr', 'Freitag', 140],
  ['Sa', 'Samstag', 210],
  ['So', 'Sonntag', 180],
] as const;

/** Horizontal bars per day of the week (concept 12, «Wann geschaut wird»). */
export const Horizontal: Story = {
  args: {
    title: 'Nach Wochentag',
    columns: ['Wochentag', 'Minuten'],
    orientation: 'horizontal',
    bars: weekdays.map(([tick, label, value]) => ({
      key: tick,
      label,
      tick,
      value,
      valueText: `${value} min`,
      tooltip: `${label}: ${value} min`,
    })),
  },
};

// No compact story: nothing here reads a component token, both densities look the same (ADR 0020).
