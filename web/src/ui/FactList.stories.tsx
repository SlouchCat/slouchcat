import type { Meta, StoryObj } from '@storybook/react-vite';
import { FactList } from './FactList';

const meta = {
  title: 'UI/FactList',
  component: FactList,
  args: {
    facts: [
      ['Container', 'MKV'],
      ['Auflösung', '1920 × 1080 · 1080p'],
      ['Video', 'HEVC'],
      ['Audio', 'EAC3'],
      ['Grösse', '4.2 GB'],
      ['Version', 'Director’s Cut'],
    ],
  },
  decorators: [
    (Story) => (
      <div style={{ width: '40rem', padding: 'var(--sc-space-6)' }}>
        <Story />
      </div>
    ),
  ],
} satisfies Meta<typeof FactList>;

export default meta;
type Story = StoryObj<typeof meta>;

/** Detail and administration pages: the facts side by side. */
export const Grid: Story = {};

/** The player's stream info: one fact per line, values in monospace. */
export const Rows: Story = {
  args: {
    layout: 'rows',
    facts: [
      ['Modus', 'HLS'],
      ['Umwandlung', 'Transcode'],
      ['Backend', 'qsv'],
      ['Grund', 'Videocodec'],
      ['Datei-ID', '01JB8Z6Q4M7N9P2R3S5T7V9X0A'],
      ['Auflösung', '3840 × 2160 · 4K'],
    ],
  },
  decorators: [
    (Story) => (
      <div style={{ width: '22rem' }}>
        <Story />
      </div>
    ),
  ],
};
