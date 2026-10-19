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

/** The profile picker while the list loads: a round avatar and a name per tile. */
export const Avatars: Story = {
  render: () => (
    <Stack direction="row" gap={6}>
      {[0, 1, 2].map((index) => (
        <Stack key={index} gap={2}>
          <Skeleton shape="avatar" />
          <Skeleton shape="line" />
        </Stack>
      ))}
    </Stack>
  ),
};

export const Hero: Story = {
  parameters: { layout: 'fullscreen' },
  args: { shape: 'hero' },
};
