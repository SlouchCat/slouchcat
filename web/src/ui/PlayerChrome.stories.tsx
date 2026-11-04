import type { Meta, StoryObj } from '@storybook/react-vite';
import { PlayerChrome } from './PlayerChrome';

// A film of two hours, 54 minutes in: four chapters, the third without a title.
const chapters = [
  { startMs: 0, title: 'Anfang' },
  { startMs: 1_620_000, title: 'Die Farm' },
  { startMs: 2_700_000 },
  { startMs: 5_640_000, title: 'Der Damm' },
];

const noop = () => undefined;
const panel = (label: string, text?: string) => ({ label, text, expanded: false, onPress: noop });
const menu = (label: string, buttonLabel = label) => ({
  label,
  buttonLabel,
  options: [{ id: 'a', label }],
  selectedKey: 'a',
  onSelect: noop,
});

const meta = {
  title: 'UI/PlayerChrome',
  component: PlayerChrome,
  args: {
    title: 'Blade Runner 2049',
    headingDetail: '2017 · Science-Fiction, Drama',
    lineLabel: 'Blade Runner 2049 · Kapitel 3',
    onBack: noop,
    currentMs: 3_240_000,
    durationMs: 7_200_000,
    buffered: [[3_000_000, 3_900_000]],
    timeDisplay: 'elapsed',
    onTimeDisplayChange: noop,
    chapters,
    segments: { intro: { startMs: 60_000, endMs: 300_000 } },
    onSeek: noop,
    playing: true,
    onTogglePlay: noop,
    onSkip: noop,
    previous: { label: 'Vorheriges Kapitel', onPress: noop },
    next: { label: 'Nächstes Kapitel', onPress: noop },
    volume: 0.6,
    muted: false,
    onVolumeChange: noop,
    onToggleMute: noop,
    fullscreen: false,
    onToggleFullscreen: noop,
    pictureInPicture: { active: false, onToggle: noop },
    subtitles: panel('Untertitel: Aus', 'Aus'),
    audio: panel('Ton: Deutsch', 'Deutsch'),
    chapterMenu: menu('Kapitel'),
    settings: panel('Einstellungen'),
    onToggleHelp: noop,
  },
  decorators: [
    (Story) => (
      <div
        style={{
          position: 'relative',
          width: '64rem',
          height: '36rem',
          backgroundColor: 'var(--sc-color-media-backdrop)',
        }}
      >
        <Story />
      </div>
    ),
  ],
} satisfies Meta<typeof PlayerChrome>;

export default meta;
type Story = StoryObj<typeof meta>;

/** A film: head with year and genres, the line with chapter and time, timeline and buttons. */
export const Default: Story = {};

/** An episode: the series in the head, the episode under it; forward goes to the next episode. */
export const Episode: Story = {
  args: {
    title: 'Silo',
    headingDetail: 'S01E04 · Wahrheit',
    lineLabel: 'Silo',
    chapters: [],
    previous: undefined,
    next: { label: 'Nächste Folge', onPress: noop },
    pictureInPicture: undefined,
    chapterMenu: undefined,
  },
};

/** The time switched to the remaining time and the end time (V-4). */
export const RemainingTime: Story = { args: { timeDisplay: 'remaining' } };

/** Offline: the page leaves out the panels it cannot offer (F-15). */
export const Offline: Story = {
  args: { subtitles: undefined, audio: undefined },
};

/** The sound panel is open: its button says so (aria-expanded) and keeps its look. */
export const PanelOpen: Story = {
  args: { audio: { ...panel('Ton: Englisch', 'Englisch'), expanded: true } },
};

/**
 * The row is one Tab stop (roving tab index): Tab from the head reaches play/pause, → moves on to
 * +30 s.
 */
export const RowFocused: Story = {
  args: { playing: false },
  tags: ['press:Tab+Tab+Tab+Tab+Tab+Tab+ArrowRight'],
};
