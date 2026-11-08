import type { Meta, StoryObj } from '@storybook/react-vite';
import { CoverageBars } from './CoverageBars';

const row = (key: string, label: string, assigned: number, total = 38912) => ({
  key,
  label,
  assigned,
  total,
  percentText: `${Math.round((assigned / total) * 100)} %`,
  countText: `${assigned.toLocaleString('de-CH')} von ${total.toLocaleString('de-CH')} Sendern`,
});

const meta = {
  title: 'UI/CoverageBars',
  component: CoverageBars,
  args: {
    label: 'Abdeckung je Filter',
    rows: [
      row('genre', 'Genre', 35410),
      row('kind', 'Sendeart', 30120),
      row('mood', 'Stimmung', 9840),
      row('region', 'Region', 26100),
      row('language', 'Sprache', 37200),
    ],
  },
  decorators: [
    (Story) => (
      <div style={{ maxWidth: '32rem' }}>
        <Story />
      </div>
    ),
  ],
} satisfies Meta<typeof CoverageBars>;

export default meta;
type Story = StoryObj<typeof meta>;

/** Below 70 % warning, from 85 % success, in between neutral. */
export const Mixed: Story = {};

/** An empty catalog: every bar at 0. */
export const Empty: Story = {
  args: { rows: [row('genre', 'Genre', 0, 0), row('kind', 'Sendeart', 0, 0)] },
};
