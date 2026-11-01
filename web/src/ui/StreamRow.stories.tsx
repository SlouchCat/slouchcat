import type { Meta, StoryObj } from '@storybook/react-vite';
import { StreamRow } from './StreamRow';
import { art } from './stories/fixtures';

// A running stream on the overview's card «Jetzt läuft» (concept 11, 11.1).

const meta = {
  title: 'UI/StreamRow',
  component: StreamRow,
  args: {
    title: 'Die Stunde der Dämmerung',
    posterUrl: art.posterDusk,
    profile: 'Anna',
    device: 'Wohnzimmer-TV',
    mode: 'Transcodiert (NVENC) · 1080p',
    progress: 42,
    progressLabel: 'Fortschritt Die Stunde der Dämmerung',
    progressText: '0:50:24 / 2:00:00',
  },
  decorators: [
    (Story) => (
      <div style={{ width: '32rem' }}>
        <Story />
      </div>
    ),
  ],
} satisfies Meta<typeof StreamRow>;

export default meta;
type Story = StoryObj<typeof meta>;

/** Poster, title, profile and device, how it plays, the progress. */
export const Default: Story = {};

/** A title without a poster shows the neutral tile; it plays directly. */
export const WithoutPoster: Story = {
  args: {
    title: 'Ein sehr langer Titel einer Folge, der in der Zeile gekürzt werden muss',
    posterUrl: undefined,
    device: 'iPhone',
    mode: 'Direkt',
    progress: 8,
    progressText: '0:03:40 / 0:45:00',
  },
};
