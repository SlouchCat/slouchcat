import type { Meta, StoryObj } from '@storybook/react-vite';
import { CollectionCard } from './CollectionCard';
import { accents, art } from './stories/fixtures';

const meta = {
  title: 'UI/CollectionCard',
  component: CollectionCard,
  args: {
    href: '#',
    name: 'Die Dämmerungs-Trilogie',
    caption: '3 Titel',
    posterUrl: art.posterDusk,
    accent: accents.dusk,
    eager: true,
  },
  decorators: [
    (Story) => (
      <div style={{ width: 'var(--sc-size-poster-lg)', padding: 'var(--sc-space-6)' }}>
        <Story />
      </div>
    ),
  ],
} satisfies Meta<typeof CollectionCard>;

export default meta;
type Story = StoryObj<typeof meta>;

export const Default: Story = {};

/** Focus: ring and scale like a poster card. */
export const Focused: Story = { tags: ['press:Tab'] };

/** The collection's own poster (every member visible) stands before the cover title's. */
export const WithOwnPoster: Story = { args: { imageUrl: art.posterEmber } };

/** No visible member has a poster yet: the lettered fallback tile on the stack. */
export const WithoutPoster: Story = { args: { posterUrl: undefined, name: 'Meine Sammlung' } };

/** An audiobook series (concept 19 D4.2): square, the number of volumes, the share heard. */
export const SquareSeries: Story = {
  args: {
    shape: 'square',
    posterUrl: undefined,
    name: 'Die Dämmerungs-Reihe',
    caption: 'Mara Lindqvist · 6 Bände',
    badge: '6 Bände',
    progress: { percent: 50, label: '3 von 6 Bänden gehört' },
  },
};

/** A filter is on: only some volumes pass it. */
export const SquareFiltered: Story = {
  args: {
    shape: 'square',
    posterUrl: undefined,
    name: 'Die Dämmerungs-Reihe',
    caption: 'Mara Lindqvist · 6 Bände',
    badge: '2 von 6 Bänden',
  },
};
