import type { Meta, StoryObj } from '@storybook/react-vite';
import { ChapterCard } from './ChapterCard';
import { PlayerChrome, type PlayerChromeTab } from './PlayerChrome';

// A film of two hours, 54 minutes in: four chapters, the third without a title.
const chapters = [
  { startMs: 0, title: 'Anfang' },
  { startMs: 1_620_000, title: 'Die Farm' },
  { startMs: 2_700_000 },
  { startMs: 5_640_000, title: 'Der Damm' },
];

const noop = () => undefined;
const panel = (label: string, text?: string) => ({ label, text, expanded: false, onPress: noop });
const DURATION_MS = 7_200_000;
// The tabs under the bar (11.2): chapters as cards, the third one playing.
const tabs: PlayerChromeTab[] = [
  {
    id: 'chapters',
    label: 'Kapitel',
    cards: {
      count: chapters.length,
      current: 2,
      render: (index, excluded) => (
        <ChapterCard
          index={index}
          title={chapters[index]!.title}
          startMs={chapters[index]!.startMs}
          durationMs={DURATION_MS}
          progress={index < 2 ? 100 : index === 2 ? 60 : undefined}
          isCurrent={index === 2}
          onPress={noop}
          excludeFromTabOrder={excluded}
        />
      ),
    },
  },
  {
    id: 'info',
    label: 'Info',
    content: (
      <p>Ein Blade Runner findet ein Geheimnis, das die Gesellschaft ins Chaos stürzen könnte.</p>
    ),
  },
];

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
    tabs,
    settings: panel('Einstellungen'),
    onToggleHelp: noop,
  },
  // The frame of the picture: 64rem × 36rem, a story sets another one in `parameters.frame`.
  decorators: [
    (Story, { parameters }) => {
      const frame = (parameters.frame ?? {}) as { width?: string; height?: string };
      return (
        <div
          style={{
            position: 'relative',
            width: frame.width ?? '64rem',
            height: frame.height ?? '36rem',
            backgroundColor: 'var(--sc-color-media-backdrop)',
          }}
        >
          <Story />
        </div>
      );
    },
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
    tabs: [],
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

/** The tab «Kapitel» is open: a row of chapter cards under the bar, the chapter playing marked. */
export const ChaptersTab: Story = { args: { playing: false, openTab: 'chapters' } };

/** The tab «Info»: the description, one Tab stop that scrolls. */
export const InfoTab: Story = { args: { playing: false, openTab: 'info' } };

/**
 * A phone (11.2): −10 s, play/pause and +10 s large in the middle; back on the left and volume,
 * subtitles, sound and the gear on the right at the top, the title in a line under them; picture
 * in picture and full screen beside the time above the timeline; no tabs, no shortcut help. On an
 * iPhone only the mute button.
 */
export const Phone: Story = {
  args: { layout: 'phone', muteOnly: true, playing: false },
  parameters: { frame: { width: '24rem', height: '44rem' } },
};
