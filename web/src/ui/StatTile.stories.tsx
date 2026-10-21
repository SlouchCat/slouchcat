import type { Meta, StoryObj } from '@storybook/react-vite';
import { Stack } from './Stack';
import { StatTile } from './StatTile';

const meta = {
  title: 'UI/StatTile',
  component: StatTile,
  args: { label: 'Laufende Streams', value: '2' },
  decorators: [
    (Story) => (
      <div style={{ width: '32rem', padding: 'var(--sc-space-6)' }}>
        <Story />
      </div>
    ),
  ],
} satisfies Meta<typeof StatTile>;

export default meta;
type Story = StoryObj<typeof meta>;

export const Default: Story = {};

/** The job counters of the dashboard: failures marked in red. */
export const JobCounters: Story = {
  render: () => (
    <Stack direction="row" gap={2}>
      <StatTile label="Wartend" value="12" />
      <StatTile label="Laufend" value="1" tone="success" />
      <StatTile label="Fehlgeschlagen" value="3" tone="danger" />
    </Stack>
  ),
};

export const Warning: Story = { args: { label: 'inotify belegt', value: '86 %', tone: 'warning' } };
