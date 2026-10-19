import type { Meta, StoryObj } from '@storybook/react-vite';
import { PosterCard } from './PosterCard';
import { accents, art } from './stories/fixtures';

const meta = {
  title: 'UI/PosterCard',
  component: PosterCard,
  args: {
    href: '#',
    title: 'Die Stunde der Dämmerung',
    caption: '2023',
    posterUrl: art.posterDusk,
    eager: true,
  },
  decorators: [
    (Story) => (
      <div style={{ width: 'var(--sc-size-poster-lg)', padding: 'var(--sc-space-6)' }}>
        <Story />
      </div>
    ),
  ],
} satisfies Meta<typeof PosterCard>;

export default meta;
type Story = StoryObj<typeof meta>;

export const Default: Story = { args: { accent: accents.dusk } };

/** Focus: ring, scale (focus.scale) and a glow in the title's accent, all in 180 ms. */
export const FocusedWithAccent: Story = {
  tags: ['press:Tab'],
  args: { accent: accents.dusk },
};

/** A title without accent glows in the profile accent. */
export const FocusedWithoutAccent: Story = {
  tags: ['press:Tab'],
  args: { title: 'Glut', caption: '2019', posterUrl: art.posterEmber },
};

/** While the poster waits for the hero (P-10) the frame carries a tint of the accent. */
export const WaitingForImage: Story = {
  args: { title: 'Moos und Stein', caption: '2021', accent: accents.moss, deferImage: true },
};

export const WithProgress: Story = {
  args: {
    title: 'Glut',
    caption: 'Noch 1 h 12 min',
    posterUrl: art.posterEmber,
    accent: accents.ember,
    progress: { percent: 36, label: '36 % gesehen' },
  },
};

export const WithoutPoster: Story = {
  args: { title: 'Slouchcat Test H264 (2024)', caption: '2024', posterUrl: undefined },
};
