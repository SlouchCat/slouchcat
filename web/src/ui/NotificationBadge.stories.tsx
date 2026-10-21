import type { Meta, StoryObj } from '@storybook/react-vite';
import { NotificationBadge } from './NotificationBadge';
import { Stack } from './Stack';

const meta = {
  title: 'UI/NotificationBadge',
  component: NotificationBadge,
  args: { count: 3 },
  decorators: [
    (Story) => (
      <div style={{ padding: 'var(--sc-space-6)' }}>
        <Story />
      </div>
    ),
  ],
} satisfies Meta<typeof NotificationBadge>;

export default meta;
type Story = StoryObj<typeof meta>;

/** One digit, two digits and the cap. */
export const Counts: Story = {
  render: () => (
    <Stack direction="row" gap={4}>
      <NotificationBadge count={3} />
      <NotificationBadge count={42} />
      <NotificationBadge count={250} />
    </Stack>
  ),
};
