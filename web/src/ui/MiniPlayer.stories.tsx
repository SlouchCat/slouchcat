import type { Meta, StoryObj } from '@storybook/react-vite';
import type { ComponentProps } from 'react';
import { MiniPlayer } from './MiniPlayer';
import { art } from './stories/fixtures';
import { VolumeControl } from './VolumeControl';

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

/** Behind live (concept 16 stage 2): the offset in place of «Live», the title's cover. */
export const TimeShifted: Story = {
  args: {
    state: 'shifted',
    stateLabel: '−12:30',
    coverUrl: art.posterDusk,
    line: 'Adele – Hello',
  },
};

/** The pause lasted longer than the window: the note with «Live» over the bar. */
export const LongPause: Story = {
  args: {
    state: 'shifted',
    stateLabel: '−29:56',
    hint: {
      text: 'Die Pause war länger als 30 Minuten. Weiter ab dem ältesten Stand.',
      actionLabel: 'Live',
      onAction: noop,
    },
  },
};

/** The volume beside play/pause (concept 19 E3): at rest only the mute button. */
export const WithVolume: Story = {
  args: {
    volume: {
      control: (
        <VolumeControl
          volume={0.8}
          muted={false}
          onVolumeChange={noop}
          onToggleMute={noop}
          placement="above"
        />
      ),
      onToggleMute: noop,
    },
  },
  decorators: [
    (Story) => (
      <div style={{ paddingTop: '4rem' }}>
        <Story />
      </div>
    ),
  ],
};

/** Tab to the mute button (open, play, mute): the slider opens above the bar. */
export const VolumeOpen: Story = { ...WithVolume, tags: ['press:Tab+Tab+Tab'] };

/** A phone without a pointer: the slider open on a line of its own under the station. */
export const VolumeInline: Story = {
  args: {
    volume: {
      control: (
        <VolumeControl
          volume={0.8}
          muted={false}
          onVolumeChange={noop}
          onToggleMute={noop}
          inline
        />
      ),
      onToggleMute: noop,
    },
  },
};
