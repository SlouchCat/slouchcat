import type { Meta, StoryObj } from '@storybook/react-vite';
import type { ComponentProps } from 'react';
import { MiniPlayer } from './MiniPlayer';
import { art } from './stories/fixtures';

const noop = () => undefined;

const base: ComponentProps<typeof MiniPlayer> = {
  label: 'Radio-Wiedergabe',
  stationName: 'Radio Swiss Pop',
  logoUrl: art.posterMoss,
  line: 'Dua Lipa – Houdini',
  state: 'live',
  stateLabel: 'Live',
  openLabel: 'Jetzt läuft öffnen: Radio Swiss Pop',
  onOpen: noop,
  toggle: { pressed: true, label: 'Pause', onPress: noop },
  favorite: { pressed: true, label: 'Radio Swiss Pop in Meine Sender', onPress: noop },
  closeLabel: 'Radio beenden',
  onClose: noop,
};

const meta = {
  title: 'UI/MiniPlayer',
  component: MiniPlayer,
  args: base,
  decorators: [
    (Story) => (
      <div style={{ maxWidth: '40rem' }}>
        <Story />
      </div>
    ),
  ],
} satisfies Meta<typeof MiniPlayer>;

export default meta;
type Story = StoryObj<typeof meta>;

/** Live, with the title the stream announced. */
export const LiveWithTitle: Story = {};

/** Live, the station sends no titles: «Live» in their place, initials instead of a logo. */
export const LiveWithoutTitle: Story = {
  args: {
    stationName: 'Kanal K',
    logoUrl: undefined,
    line: 'Live',
    openLabel: 'Jetzt läuft öffnen: Kanal K',
    favorite: { pressed: false, label: 'Kanal K in Meine Sender', onPress: noop },
  },
};

export const Paused: Story = {
  args: {
    state: 'paused',
    stateLabel: 'Pausiert',
    toggle: { pressed: false, label: 'Weiter abspielen', onPress: noop },
  },
};

export const Starting: Story = {
  args: { state: 'starting', stateLabel: 'Verbindet …', line: 'Live' },
};

/** The server refused the station: the message in place of the title. */
export const Failed: Story = {
  args: {
    state: 'error',
    stateLabel: undefined,
    line: 'Gerade laufen schon zu viele Sender über den Server.',
    toggle: { pressed: false, label: 'Nochmals versuchen', onPress: noop },
    favorite: undefined,
  },
};
