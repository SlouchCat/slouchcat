import type { Meta, StoryObj } from '@storybook/react-vite';
import { Button } from './Button';
import { StreamTile } from './StreamTile';
import { art } from './stories/fixtures';

// A running stream on the streams page (concept 12, mockup Str-D2; concept 18, 17.3: one form
// per picture, the backdrop across, the square cover and the station logo).

const meta = {
  title: 'UI/StreamTile',
  component: StreamTile,
  args: {
    label: 'Anna: Die Stunde der Dämmerung',
    title: 'Die Stunde der Dämmerung',
    kind: 'movie',
    kindLabel: 'Film',
    image: { form: 'backdrop', src: art.backdropDusk },
    mode: 'Transcode (HW)',
    modeTone: 'warning',
    profile: 'Anna',
    account: 'anna@example.ch',
    facts: [
      { key: 'device', label: 'Gerät', value: 'Wohnzimmer-TV (Chrome)', icon: 'monitor' },
      { key: 'quality', label: 'Qualität', value: '1080p · 9,8 Mbit/s', icon: 'film' },
    ],
    progress: 42,
    progressLabel: 'Fortschritt Die Stunde der Dämmerung',
    position: '0:50:24 von 2:00:00',
    since: 'Seit 01.11.2026, 14:29',
    action: (
      <Button variant="danger" aria-label="Stream von Anna beenden">
        Beenden
      </Button>
    ),
  },
  decorators: [
    (Story) => (
      <div style={{ width: '22rem' }}>
        <Story />
      </div>
    ),
  ],
} satisfies Meta<typeof StreamTile>;

export default meta;
type Story = StoryObj<typeof meta>;

/**
 * Backdrop with the kind badge and the mode pill, title, profile and account, device, quality,
 * progress, action.
 */
export const Default: Story = {};

/**
 * Without a backdrop the fallback surface carries the title; a direct stream whose length is
 * unknown shows its position without a bar, and long texts wrap instead of being clipped.
 */
export const WithoutBackdrop: Story = {
  args: {
    title: 'Ein sehr langer Titel einer Folge, der in der Kachel umbricht · S01E04',
    kind: 'episode',
    kindLabel: 'Serie',
    image: { form: 'backdrop' },
    mode: 'Direct',
    modeTone: 'success',
    facts: [
      { key: 'device', label: 'Gerät', value: 'Unbekanntes Gerät', icon: 'monitor' },
      { key: 'quality', label: 'Qualität', value: 'Original', icon: 'film' },
    ],
    progress: undefined,
    position: '3:40',
  },
};

/**
 * An audiobook: its square cover in the middle on a blurred, darkened copy, the chapter in the
 * title, the format with the number of parts and the position in the whole book.
 */
export const Cover: Story = {
  args: {
    label: 'Kids: Moos und Stein · Kapitel 7: Der Sturm',
    title: 'Moos und Stein · Kapitel 7: Der Sturm',
    kind: 'audiobook',
    kindLabel: 'Hörbuch',
    image: { form: 'cover', src: art.posterMoss },
    mode: 'Remux',
    modeTone: 'neutral',
    profile: 'Kids',
    account: 'anna@example.ch',
    facts: [
      { key: 'device', label: 'Gerät', value: 'Kinderzimmer (iPad)', icon: 'monitor' },
      { key: 'format', label: 'Format', value: 'AAC · 64 kbit/s · 12 Teile', icon: 'audioTrack' },
    ],
    progress: 42,
    progressLabel: 'Fortschritt Moos und Stein',
    position: '3:41:10 von 8:46:00',
  },
};

/** A podcast episode from the feed without any cover: the initials of its title stand in. */
export const CoverWithoutImage: Story = {
  args: {
    ...Cover.args,
    label: 'Lena: Apropos · Folge vom 9. November',
    title: 'Apropos · Folge vom 9. November',
    kind: 'podcast',
    kindLabel: 'Podcast',
    image: { form: 'cover' },
    mode: 'Direct',
    modeTone: 'success',
    facts: [
      { key: 'device', label: 'Gerät', value: 'Wohnzimmer (Firefox)', icon: 'monitor' },
      { key: 'format', label: 'Format', value: 'MP3', icon: 'audioTrack' },
      { key: 'source', label: 'Quelle', value: 'Feed', icon: 'external' },
    ],
    progress: 35,
    position: '7:12 von 20:31',
  },
};

/** A radio listener: the station logo like a cover, no mode pill and no bar, only its state. */
export const Logo: Story = {
  args: {
    label: 'Marco: Radio Swiss Pop',
    title: 'Radio Swiss Pop',
    kind: 'radio',
    kindLabel: 'Radio',
    image: { form: 'logo', src: art.posterEmber },
    mode: undefined,
    profile: 'Marco',
    account: 'marco@example.ch',
    facts: [
      { key: 'device', label: 'Gerät', value: 'Küche (iPhone)', icon: 'monitor' },
      { key: 'format', label: 'Format', value: 'AAC · 128 kbit/s · umgepackt', icon: 'radio' },
      { key: 'listeners', label: 'Hörer', value: '1 Hörer dieses Senders', icon: 'users' },
    ],
    progress: undefined,
    progressLabel: 'Fortschritt Radio Swiss Pop',
    position: 'Hört',
  },
};

/** A station without logo: its initials on its accent; here its relay failed. */
export const LogoWithoutImage: Story = {
  args: {
    ...Logo.args,
    title: 'Kanal K',
    label: 'Marco: Kanal K',
    image: { form: 'logo' },
    position: 'Relais gestört',
  },
};

/** In the administration's compact density (ADR 0020): the action is a compact button. */
export const Compact: Story = { globals: { density: 'compact' } };
