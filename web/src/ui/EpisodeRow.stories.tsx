import type { Meta, StoryObj } from '@storybook/react-vite';
import { EpisodeRow } from './EpisodeRow';

const meta = {
  title: 'UI/EpisodeRow',
  component: EpisodeRow,
  args: {
    href: '#',
    title: 'Folge 12: Ein langer Abend',
    caption: '13. Okt. 2025 · 41 min',
    playHref: '#',
    playLabel: 'Folge 12: Ein langer Abend abspielen',
  },
  decorators: [
    (Story) => (
      <div style={{ width: '40rem', padding: 'var(--sc-space-6)' }}>
        <Story />
      </div>
    ),
  ],
} satisfies Meta<typeof EpisodeRow>;

export default meta;
type Story = StoryObj<typeof meta>;

export const Default: Story = {};

/** Half heard: the progress under the date. */
export const WithProgress: Story = {
  args: { progress: { percent: 48, label: '48 % gehört' } },
};

/** Focus on the episode link (Tab), then on its play button (Tab again). */
export const Focused: Story = { tags: ['press:Tab'] };

export const PlayFocused: Story = { tags: ['press:Tab+Tab'] };
