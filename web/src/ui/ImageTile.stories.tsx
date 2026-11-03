import type { Meta, StoryObj } from '@storybook/react-vite';
import { Chip } from './Chip';
import { ImageTile } from './ImageTile';
import { ProgressBar } from './ProgressBar';
import { Stack } from './Stack';
import { art } from './stories/fixtures';

const meta = {
  title: 'UI/ImageTile',
  component: ImageTile,
  args: { src: art.backdropDusk, title: 'Die Stunde der Dämmerung', shape: 'landscape' },
  render: (args) => (
    <Stack direction="row" gap={4}>
      <div style={{ width: '20rem' }}>
        <ImageTile {...args} />
      </div>
      <div style={{ width: '20rem' }}>
        <ImageTile {...args} src={undefined} />
      </div>
    </Stack>
  ),
} satisfies Meta<typeof ImageTile>;

export default meta;
type Story = StoryObj<typeof meta>;

/** A backdrop across, and the fallback with the title when there is none. */
export const Landscape: Story = {};

/** Posters upright (top titles, collection members). */
export const Portrait: Story = {
  args: { src: art.posterDusk, shape: 'portrait' },
  render: (args) => (
    <Stack direction="row" gap={4}>
      <div style={{ width: '10rem' }}>
        <ImageTile {...args} />
      </div>
      <div style={{ width: '10rem' }}>
        <ImageTile {...args} src={undefined} />
      </div>
    </Stack>
  ),
};

/** A stream tile: mode and progress over the image. */
export const WithOverlay: Story = {
  args: {
    children: (
      <>
        <Chip tone="info">Transcode (HW)</Chip>
        <ProgressBar value={42} label="42 % gesehen" />
      </>
    ),
  },
};

// No compact story: nothing here reads a component token, both densities look the same (ADR 0020).
