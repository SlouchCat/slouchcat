import type { Meta, StoryObj } from '@storybook/react-vite';
import { StreamProgress } from './StreamProgress';

// The progress of a running stream, shared by StreamRow (overview) and StreamTile (streams page).

const meta = {
  title: 'UI/StreamProgress',
  component: StreamProgress,
  args: {
    value: 42,
    label: 'Fortschritt Die Stunde der Dämmerung',
    text: '0:50:24 / 2:00:00',
  },
  decorators: [
    (Story) => (
      <div style={{ width: '20rem', display: 'flex', flexDirection: 'column' }}>
        <Story />
      </div>
    ),
  ],
} satisfies Meta<typeof StreamProgress>;

export default meta;
type Story = StoryObj<typeof meta>;

/** The bar and the position next to it. */
export const Default: Story = {};

/** The length is unknown: the position alone, no bar. */
export const WithoutLength: Story = { args: { value: undefined, text: '3:40' } };

// No compact story: nothing here reads a component token, both densities look the same (ADR 0020).
