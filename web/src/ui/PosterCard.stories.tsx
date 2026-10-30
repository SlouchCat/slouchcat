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

/** An extra of a title: no poster, its kind as a label (B-7). */
export const WithBadge: Story = {
  args: { title: 'Making Of', caption: undefined, posterUrl: undefined, badge: 'Featurette' },
};

/** An audiobook or a podcast: the cover is square (M6). */
export const Square: Story = {
  args: {
    title: 'Die Stunde der Dämmerung',
    caption: 'Mara Lindqvist',
    shape: 'square',
    accent: accents.dusk,
    progress: { percent: 52, label: '52 % gehört' },
  },
};

/** The library page: a finished title carries a check mark, its age rating sits top left. */
export const Watched: Story = {
  args: {
    title: 'Glut',
    caption: '2019',
    posterUrl: art.posterEmber,
    accent: accents.ember,
    badge: 'FSK 12',
    watchedLabel: 'Gesehen',
  },
};

/** The library page: a started title with its progress and its age rating. */
export const StartedWithRating: Story = {
  args: {
    title: 'Moos und Stein',
    caption: '2021',
    accent: accents.moss,
    badge: 'FSK 6',
    progress: { percent: 42, label: '42 % gesehen' },
  },
};

/** A finished audiobook: square cover with the check mark, no age rating. */
export const SquareHeard: Story = {
  args: {
    title: 'Die Stunde der Dämmerung',
    caption: 'Mara Lindqvist',
    shape: 'square',
    accent: accents.dusk,
    watchedLabel: 'Gehört',
  },
};

/**
 * Without `href` the card is a button (the spotlight row under the band, concept 08, F-5): the
 * chosen card carries a ring in the title's accent.
 */
export const ButtonActive: Story = {
  args: { href: undefined, onPress: () => undefined, active: true, accent: accents.dusk },
};

/** A card of that row that is not the chosen one: the poster at 70 %. */
export const ButtonDimmed: Story = {
  args: {
    href: undefined,
    onPress: () => undefined,
    dimmed: true,
    title: 'Glut',
    caption: '2019',
    posterUrl: art.posterEmber,
    accent: accents.ember,
  },
};

/** The chosen card with the band's time bar drawn inside its frame (a child of the card). */
export const ButtonWithTimeBar: Story = {
  args: {
    href: undefined,
    onPress: () => undefined,
    active: true,
    accent: accents.dusk,
    children: (
      <span
        style={{
          position: 'absolute',
          right: 0,
          bottom: 0,
          left: 0,
          width: '40%',
          height: 'var(--sc-space-1)',
          backgroundColor: 'var(--sc-color-text-primary)',
        }}
      />
    ),
  },
};
