import type { Meta, StoryObj } from '@storybook/react-vite';
import { LibraryTile } from './LibraryTile';
import { accents, art } from './stories/fixtures';

const meta = {
  title: 'UI/LibraryTile',
  component: LibraryTile,
  args: {
    href: '#',
    name: 'Filme',
    caption: '1234 Titel',
    covers: [
      { posterUrl: art.posterDusk },
      { posterUrl: art.posterEmber },
      { posterUrl: art.posterMoss },
      { posterUrl: art.posterDusk },
    ],
    accent: accents.dusk,
    eager: true,
  },
  decorators: [
    (Story) => (
      <div
        style={{
          width: 'calc(2 * var(--sc-size-poster-lg) + var(--sc-space-4))',
          padding: 'var(--sc-space-6)',
        }}
      >
        <Story />
      </div>
    ),
  ],
} satisfies Meta<typeof LibraryTile>;

export default meta;
type Story = StoryObj<typeof meta>;

export const Default: Story = {};

/** Focus: ring and scale like a poster card. */
export const Focused: Story = { tags: ['press:Tab'] };

/** A library with two titles: the collage has two slices; one has no poster yet. */
export const FewTitles: Story = {
  args: {
    name: 'Hörbücher',
    caption: '2 Hörbücher',
    covers: [{ posterUrl: art.posterEmber }, {}],
    accent: accents.ember,
  },
};

/** A long name is cut with an ellipsis; the count stays whole. */
export const LongName: Story = {
  args: { name: 'Dokumentarfilme aus aller Welt und noch mehr', caption: '87 Titel' },
};
