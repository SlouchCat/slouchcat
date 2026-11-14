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

/** «Radio» (concept 21 P-9): the logos of the favourites; a station without one by its initials. */
export const Radio: Story = {
  args: {
    name: 'Radio',
    caption: '3 Sender',
    covers: [],
    icon: 'radio',
    stations: [
      { name: 'Radio Swiss Pop', logoUrl: art.posterMoss },
      { name: 'Kanal K' },
      { name: 'Energy Zürich', logoUrl: art.posterEmber },
    ],
    accent: undefined,
  },
};

/** «Radio» without favourites (O-15): no logos, an invitation to discover stations. */
export const RadioDiscover: Story = {
  args: {
    name: 'Radio',
    caption: 'Sender entdecken',
    covers: [],
    icon: 'radio',
    accent: undefined,
  },
};

/** «Sammlungen» and «User-Empfehlungen»: no collage, a large icon. */
export const Plain: Story = {
  args: {
    name: 'Sammlungen',
    caption: '333 Sammlungen',
    covers: [],
    icon: 'stack',
    accent: undefined,
  },
};
