import type { Meta, StoryObj } from '@storybook/react-vite';
import { PlayerStep } from './PlayerStep';

const noop = () => undefined;

const meta = {
  title: 'UI/PlayerStep',
  component: PlayerStep,
  args: { icon: 'chapterNext', label: 'Nächstes Kapitel', onPress: noop },
  // Room above the button for its hint and card, on the dark of the player.
  decorators: [
    (Story) => (
      <div
        style={{
          display: 'flex',
          alignItems: 'flex-end',
          height: '20rem',
          padding: '1rem',
          backgroundColor: 'var(--sc-color-media-backdrop)',
        }}
      >
        <Story />
      </div>
    ),
  ],
} satisfies Meta<typeof PlayerStep>;

export default meta;
type Story = StoryObj<typeof meta>;

/** A chapter button of the transport (concept 23, 3.1). */
export const Chapter: Story = {};

/** Greyed in place: focused by keyboard, its hint over it (also its description). */
export const Greyed: Story = {
  args: { unavailable: 'Letztes Kapitel' },
  tags: ['press:Tab'],
};

/** An episode button focused by keyboard: the card with the episode, its length and the key. */
export const EpisodeCard: Story = {
  args: {
    icon: 'next',
    label: 'Nächste Folge: S01E05 · Die Flamme',
    cardAlign: 'start',
    card: { heading: 'Nächste Folge', name: 'S01E05 · Die Flamme', detail: '52 Min.', keys: 'N' },
  },
  tags: ['press:Tab'],
};

/** Off for real during a switch: not focusable, no hint. */
export const Disabled: Story = { args: { isDisabled: true, unavailable: 'Keine Kapitel' } };
