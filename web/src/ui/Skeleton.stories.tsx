import type { Meta, StoryObj } from '@storybook/react-vite';
import { Skeleton } from './Skeleton';
import { Stack } from './Stack';

const meta = {
  title: 'UI/Skeleton',
  component: Skeleton,
} satisfies Meta<typeof Skeleton>;

export default meta;
type Story = StoryObj<typeof meta>;

export const Shapes: Story = {
  render: () => (
    <Stack gap={4}>
      <Skeleton shape="title" />
      <Skeleton shape="line" />
      <Skeleton shape="block" />
      <div style={{ width: 'var(--sc-size-poster-md)' }}>
        <Skeleton shape="poster" />
      </div>
    </Stack>
  ),
};

export const Hero: Story = {
  parameters: { layout: 'fullscreen' },
  args: { shape: 'hero' },
};
