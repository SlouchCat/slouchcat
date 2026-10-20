import type { Meta, StoryObj } from '@storybook/react-vite';
import { StorageMeter } from './StorageMeter';

const GB = 1e9;

const meta = {
  title: 'UI/StorageMeter',
  component: StorageMeter,
  args: {
    label: 'Speicherbelegung',
    valueText: '3,2 GB von 14 GB belegt',
    total: 14 * GB,
    parts: [
      { key: 'downloads', value: 2.8 * GB, label: 'Downloads: 2,8 GB', tone: 'accent' },
      { key: 'other', value: 0.4 * GB, label: 'Andere Daten: 0,4 GB', tone: 'neutral' },
    ],
    caption: 'Frei: 10,8 GB von 14 GB',
  },
  decorators: [
    (Story) => (
      <div style={{ width: '32rem', padding: 'var(--sc-space-6)' }}>
        <Story />
      </div>
    ),
  ],
} satisfies Meta<typeof StorageMeter>;

export default meta;
type Story = StoryObj<typeof meta>;

export const Default: Story = {};

/** Nearly full: the downloads take most of the quota. */
export const NearlyFull: Story = {
  args: {
    valueText: '13,5 GB von 14 GB belegt',
    parts: [
      { key: 'downloads', value: 12.9 * GB, label: 'Downloads: 12,9 GB', tone: 'accent' },
      { key: 'other', value: 0.6 * GB, label: 'Andere Daten: 0,6 GB', tone: 'neutral' },
    ],
    caption: 'Frei: 0,5 GB von 14 GB',
  },
};

/** Nothing downloaded yet. */
export const Empty: Story = {
  args: {
    valueText: '0,1 GB von 14 GB belegt',
    parts: [
      { key: 'downloads', value: 0, label: 'Downloads: 0 GB', tone: 'accent' },
      { key: 'other', value: 0.1 * GB, label: 'Andere Daten: 0,1 GB', tone: 'neutral' },
    ],
    caption: 'Frei: 13,9 GB von 14 GB',
  },
};
