import type { Meta, StoryObj } from '@storybook/react-vite';
import { CandidateCard } from './CandidateCard';
import { Chip } from './Chip';
import { art } from './stories/fixtures';

const meta = {
  title: 'UI/CandidateCard',
  component: CandidateCard,
  args: {
    title: 'Alien (1979)',
    subtitle: 'Alien',
    overview:
      'Die Besatzung des Raumfrachters Nostromo empfängt auf dem Heimweg ein Signal von einem unbekannten Planeten und landet, um nachzusehen.',
    posterUrl: art.posterDusk,
    'aria-label': 'Alien (1979) zuordnen',
    onPress: () => undefined,
  },
  decorators: [
    (Story) => (
      <div style={{ width: '32rem', padding: 'var(--sc-space-6)' }}>
        <Story />
      </div>
    ),
  ],
} satisfies Meta<typeof CandidateCard>;

export default meta;
type Story = StoryObj<typeof meta>;

export const Default: Story = {};

/** The provider has no poster and no overview. */
export const WithoutPoster: Story = {
  args: { posterUrl: undefined, overview: undefined, subtitle: undefined },
};

/** While the chosen candidate is stored. */
export const Disabled: Story = { args: { isDisabled: true } };

export const Focused: Story = { tags: ['press:Tab'] };

/** An audiobook entry (concept 19 D5.2): square cover, people and facts, the length badge. */
export const Book: Story = {
  args: {
    title: 'Die Gefährten – Der Herr der Ringe 1',
    subtitle: undefined,
    overview: undefined,
    posterUrl: art.posterDusk,
    shape: 'square',
    details: [
      'J. R. R. Tolkien · gelesen von Achim Höppner',
      'Der Hörverlag · 2002 · ASIN B0TEST0001',
    ],
    badge: (
      <Chip tone="success" icon="check">
        22 Std. 54 Min.
      </Chip>
    ),
    'aria-label': 'Die Gefährten wählen',
  },
};

/** The picked entry whose comparison is open. */
export const BookSelected: Story = { args: { ...Book.args, isSelected: true } };
