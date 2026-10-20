import type { Meta, StoryObj } from '@storybook/react-vite';
import { CandidateCard } from './CandidateCard';
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
