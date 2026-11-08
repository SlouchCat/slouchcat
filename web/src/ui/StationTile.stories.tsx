import type { Meta, StoryObj } from '@storybook/react-vite';
import type { ComponentProps } from 'react';
import { StationTile } from './StationTile';
import { art } from './stories/fixtures';

const noop = () => undefined;

const base: ComponentProps<typeof StationTile> = {
  name: 'Radio Swiss Pop',
  logoUrl: art.posterMoss,
  caption: 'Schweiz · Zürich',
  playLabel: 'Radio Swiss Pop abspielen',
  onPlay: noop,
  favorite: { pressed: false, label: 'Radio Swiss Pop in Meine Sender', onPress: noop },
};

const meta = {
  title: 'UI/StationTile',
  component: StationTile,
  args: base,
  decorators: [
    (Story) => (
      <div style={{ width: '10rem' }}>
        <Story />
      </div>
    ),
  ],
} satisfies Meta<typeof StationTile>;

export default meta;
type Story = StoryObj<typeof meta>;

export const Default: Story = {};

/** The radio plays this station; it is a favourite. */
export const Playing: Story = {
  args: {
    state: 'playing',
    stateLabel: 'Läuft',
    playLabel: 'Radio Swiss Pop pausieren',
    favorite: { pressed: true, label: 'Radio Swiss Pop in Meine Sender', onPress: noop },
  },
};

/** Without a logo the initials; a station that left the catalog says until when it stays. */
export const Missing: Story = {
  args: {
    name: 'Kanal K',
    logoUrl: undefined,
    caption: 'Schweiz · Aargau',
    note: 'Nicht mehr im Katalog, verschwindet am 14.11.2026',
  },
};

/** The focused tile grows a little, as the posters do. */
export const Focused: Story = { tags: ['press:Tab'] };

/**
 * A tile of «Meine Sender» on the start page (concept 16, 10.2): a favourite whose relay runs,
 * with the title at its live edge; the first tile loads its logo at once (`priority`).
 */
export const HomeRowWithTitle: Story = {
  args: {
    caption: 'Dua Lipa – Houdini',
    priority: true,
    favorite: { pressed: true, label: 'Radio Swiss Pop in Meine Sender', onPress: noop },
  },
};
