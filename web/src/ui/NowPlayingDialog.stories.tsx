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

const shift = {
  label: 'Zeitversatz, bis 30 Minuten zurück',
  maxSeconds: 1800,
  offset: 750,
  onSeek: noop,
  valueText: () => '12 Minuten 30 Sekunden zurück',
  shownText: () => '−12:30',
  startLabel: '−30:00',
  endLabel: 'Live',
};

const played = {
  heading: 'Zuvor gespielt',
  empty: 'Vor diesem Titel hat der Sender noch keine Titel gemeldet.',
  entries: [
    {
      key: '2',
      title: 'Flowers',
      subtitle: 'Miley Cyrus',
      time: '14:28',
      dateTime: '2026-11-08T13:28:00.000Z',
      coverUrl: art.posterEmber,
    },
    {
      key: '1',
      title: 'Hello',
      subtitle: 'Adele',
      time: '14:24',
      dateTime: '2026-11-08T13:24:00.000Z',
    },
    {
      key: '0',
      title: 'Radio Swiss Pop – Die Musik',
      time: '14:21',
      dateTime: '2026-11-08T13:21:00.000Z',
    },
  ],
};

/**
 * Stage 2: 12:30 behind live with the slider, the cover large and blurred behind the view, and
 * «Zuvor gespielt» (an entry without a cover shows the logo).
 */
export const TimeShiftWithCover: Story = {
  args: {
    coverUrl: art.posterDusk,
    isLive: false,
    timeShift: shift,
    played,
  },
};

/** At live: «Live» pressed, the slider at its right end. */
export const AtLive: Story = {
  args: {
    coverUrl: art.posterDusk,
    isLive: true,
    timeShift: { ...shift, offset: 0, valueText: () => 'Live', shownText: () => 'Live' },
    played: { ...played, entries: [] },
  },
};

/** The pause lasted longer than the window: the note over the controls. */
export const LongPause: Story = {
  args: {
    isLive: false,
    timeShift: { ...shift, offset: 1796, shownText: () => '−29:56' },
    notice: 'Die Pause war länger als 30 Minuten. Weiter ab dem ältesten Stand.',
    played,
  },
};

/** The server keeps no buffer (D-9): no slider, «Live» only. */
export const WithoutTimeShift: Story = {
  args: { isLive: true, played },
};
