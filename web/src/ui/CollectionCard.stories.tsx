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

/** No visible member has a poster yet: the lettered fallback tile on the stack. */
export const WithoutPoster: Story = { args: { posterUrl: undefined, name: 'Meine Sammlung' } };
