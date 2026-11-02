import type { Meta, StoryObj } from '@storybook/react-vite';
import { Button } from './Button';
import { StreamTile } from './StreamTile';
import { art } from './stories/fixtures';

// A running stream on the streams page (concept 12, mockup Str-D2).

const meta = {
  title: 'UI/StreamTile',
  component: StreamTile,
  args: {
    title: 'Die Stunde der Dämmerung',
    backdropUrl: art.backdropDusk,
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

/** Backdrop with the mode pill, title, profile and account, device, quality, progress, action. */
export const Default: Story = {};

/**
 * Without a backdrop the fallback surface carries the title; a direct stream whose length is
 * unknown shows its position without a bar, and long texts wrap instead of being clipped.
 */
export const WithoutBackdrop: Story = {
  args: {
    title: 'Ein sehr langer Titel einer Folge, der in der Kachel umbricht · S01E04',
    backdropUrl: undefined,
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

// No compact story: nothing here reads a component token, both densities look the same (ADR 0020).
