import type { Meta, StoryObj } from '@storybook/react-vite';
import { EpisodeCard } from './EpisodeCard';
import { accents, art } from './stories/fixtures';

const meta = {
  title: 'UI/EpisodeCard',
  component: EpisodeCard,
  args: {
    href: '#',
    code: 'S01E02',
    title: 'Die lange Nacht',
    caption: '45 min',
    stillUrl: art.backdropDusk,
    accent: accents.dusk,
  },
  decorators: [
    (Story) => (
      <div style={{ width: '20rem', padding: 'var(--sc-space-6)' }}>
        <Story />
      </div>
    ),
  ],
} satisfies Meta<typeof EpisodeCard>;

export default meta;
type Story = StoryObj<typeof meta>;

export const Default: Story = {};

export const WithProgress: Story = {
  args: { progress: { percent: 64, label: '64 % gesehen' } },
};

/** No still yet (no metadata match): the frame shows the accent tint. */
export const WithoutStill: Story = {
  args: { code: 'S01E03–E04', title: 'Episode 3', stillUrl: undefined, caption: undefined },
};

export const Focused: Story = { tags: ['press:Tab'] };
