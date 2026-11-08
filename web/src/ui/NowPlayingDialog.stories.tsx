import type { Meta, StoryObj } from '@storybook/react-vite';
import type { ComponentProps } from 'react';
import { NowPlayingDialog } from './NowPlayingDialog';
import { art } from './stories/fixtures';

const noop = () => undefined;

const base: ComponentProps<typeof NowPlayingDialog> = {
  isOpen: true,
  onOpenChange: noop,
  heading: 'Jetzt läuft',
  closeLabel: 'Jetzt läuft schliessen',
  stationName: 'Radio Swiss Pop',
  logoUrl: art.posterMoss,
  place: 'Schweiz · Zürich',
  title: 'Houdini',
  subtitle: 'Dua Lipa',
  toggle: { pressed: true, label: 'Pause', onPress: noop },
  liveLabel: 'Live',
  onLive: noop,
  favorite: { pressed: true, label: 'Radio Swiss Pop in Meine Sender', onPress: noop },
  facts: [
    { label: 'Qualität', value: 'AAC 128 kbit/s' },
    { label: 'Genre', value: 'Pop' },
    { label: 'Sprache', value: 'Deutsch' },
  ],
  hint: 'Über Slouchcat, Standort des Servers',
};

const meta = {
  title: 'UI/NowPlayingDialog',
  component: NowPlayingDialog,
  args: base,
} satisfies Meta<typeof NowPlayingDialog>;

export default meta;
type Story = StoryObj<typeof meta>;

/** A station that sends titles. */
export const WithTitle: Story = {};

/** A station without titles and without a logo: its programme, the initials. */
export const WithoutTitle: Story = {
  args: {
    stationName: 'Kanal K',
    logoUrl: undefined,
    place: 'Schweiz · Aargau',
    title: 'Live-Programm',
    subtitle: 'Ohne Titelangaben vom Sender',
    toggle: { pressed: false, label: 'Weiter abspielen', onPress: noop },
    favorite: { pressed: false, label: 'Kanal K in Meine Sender', onPress: noop },
  },
};
