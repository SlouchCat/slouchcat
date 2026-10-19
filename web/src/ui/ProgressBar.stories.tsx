import type { Meta, StoryObj } from '@storybook/react-vite';
import { ProgressBar } from './ProgressBar';
import { Stack } from './Stack';

const meta = {
  title: 'UI/ProgressBar',
  component: ProgressBar,
  args: { value: 36, label: '36 % gesehen' },
} satisfies Meta<typeof ProgressBar>;

export default meta;
type Story = StoryObj<typeof meta>;

export const Sizes: Story = {
  render: () => (
    <Stack gap={4}>
      <ProgressBar value={0} label="0 % gesehen" />
      <ProgressBar value={36} label="36 % gesehen" />
      <ProgressBar value={100} label="100 % gesehen" />
      <ProgressBar value={62} label="Scan 62 %" size="regular" />
    </Stack>
  ),
};
