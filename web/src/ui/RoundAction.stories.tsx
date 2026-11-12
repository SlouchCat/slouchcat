import type { Meta, StoryObj } from '@storybook/react-vite';
import { RoundAction } from './RoundAction';

const meta = {
  title: 'UI/RoundAction',
  component: RoundAction,
  args: { label: 'Trailer', icon: 'play', onPress: () => undefined },
  decorators: [
    (Story) => (
      <div style={{ padding: 'var(--sc-space-6)' }}>
        <Story />
      </div>
    ),
  ],
} satisfies Meta<typeof RoundAction>;

export default meta;
type Story = StoryObj<typeof meta>;

export const Default: Story = {};

/** A switch that is on (Merken → Gemerkt): the circle in the accent with a check. */
export const Pressed: Story = { args: { label: 'Gemerkt', icon: 'plus', pressed: true } };

/** «Gesehen» of a partly watched series (concept 20 F-12): half the circle in the accent. */
export const Mixed: Story = { args: { label: 'Gesehen', icon: 'check', pressed: 'mixed' } };

/** Offline or without a playable file. */
export const Disabled: Story = {
  args: { label: 'Herunterladen', icon: 'download', isDisabled: true },
};

/** Focus: the ring around circle and label, the label in the primary text colour. */
export const Focused: Story = { tags: ['press:Tab'], args: { label: 'Von vorne', icon: 'replay' } };

/**
 * «Empfehlen» once pressed (concept 18 part B): the title is in the user picks; a second press
 * takes the own pick back.
 */
export const Recommended: Story = {
  args: { label: 'Empfohlen ✓', icon: 'star', pressed: true },
};

/** The row of the detail page: labels of different length side by side. */
export const Row: Story = {
  render: () => (
    <div
      style={{
        display: 'flex',
        flexWrap: 'wrap',
        gap: 'var(--sc-space-2)',
        alignItems: 'flex-start',
      }}
    >
      <RoundAction label="Trailer" icon="play" onPress={() => undefined} />
      <RoundAction label="Von vorne" icon="replay" href="#" />
      <RoundAction label="Gemerkt" icon="plus" pressed onPress={() => undefined} />
      <RoundAction label="Favorit" icon="heart" pressed={false} onPress={() => undefined} />
      <RoundAction label="Empfehlen" icon="star" pressed={false} onPress={() => undefined} />
      <RoundAction label="Herunterladen" icon="download" onPress={() => undefined} />
    </div>
  ),
};
