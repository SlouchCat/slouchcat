import type { Meta, StoryObj } from '@storybook/react-vite';
import { StatusStrip } from './StatusStrip';

const meta = {
  title: 'UI/StatusStrip',
  component: StatusStrip,
  args: {
    label: 'Zusammenfassung',
    tiles: [
      { key: 'running', label: 'Laufend', value: '1', tone: 'success' },
      { key: 'queued', label: 'Wartend', value: '12' },
      { key: 'planned', label: 'Geplant', value: '6' },
      { key: 'paused', label: 'Pausiert', value: '2', tone: 'warning' },
      { key: 'failed', label: 'Fehlgeschlagen', value: '265', tone: 'danger' },
    ],
  },
  decorators: [
    (Story) => (
      <div style={{ width: '60rem' }}>
        <Story />
      </div>
    ),
  ],
} satisfies Meta<typeof StatusStrip>;

export default meta;
type Story = StoryObj<typeof meta>;

/** The five counters of the jobs page (JOB-1) as compact tiles, 56 px high. */
export const Default: Story = {};

// No compact story: the strip's tiles are 56 px in both densities (S-4), the shots are equal.
