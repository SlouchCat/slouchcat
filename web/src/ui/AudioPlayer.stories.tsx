import type { Meta, StoryObj } from '@storybook/react-vite';
import { AudioPlayer } from './AudioPlayer';
import { accents, art } from './stories/fixtures';

// Twelve chapters of five minutes, like a short audiobook.
const chapters = Array.from({ length: 12 }, (_, index) => ({
  startMs: index * 300_000,
  endMs: (index + 1) * 300_000,
  title: `Kapitel ${index + 1}`,
}));

const noop = () => undefined;

const meta = {
  title: 'UI/AudioPlayer',
  component: AudioPlayer,
  parameters: { layout: 'fullscreen' },
  args: {
    title: 'Die Stunde der Dämmerung',
    byline: 'Mara Lindqvist',
    coverUrl: art.posterDusk,
    accent: accents.dusk,
    chapters,
    chapter: { index: 2, offsetMs: 134_000, lengthMs: 300_000 },
    currentMs: 734_000,
    durationMs: 3_600_000,
    playing: true,
    rate: 1,
    volume: 0.8,
    muted: false,
    sleepChoice: 'off',
    sleepRemainingMs: null,
    openMenu: null,
    onOpenMenuChange: noop,
    onTogglePlay: noop,
    onSeek: noop,
    onSkip: noop,
    onPreviousChapter: noop,
    onNextChapter: noop,
    onRate: noop,
    onSleep: noop,
    onVolume: noop,
    onToggleMute: noop,
    onBack: noop,
  },
} satisfies Meta<typeof AudioPlayer>;

export default meta;
type Story = StoryObj<typeof meta>;

/** Playing at 1.25×: chapter, time into it, book-wide position; the play button has the focus. */
export const Playing: Story = { args: { rate: 1.25 } };

export const Paused: Story = { args: { playing: false } };

/** The chapter list (key C): the playing chapter carries the tick. */
export const ChapterMenuOpen: Story = { args: { openMenu: 'chapters' } };

/** The speed menu with the presets; 2× is chosen. */
export const SpeedMenuOpen: Story = { args: { openMenu: 'speed', rate: 2 } };

/** A sleep timer runs: the button shows the time it has left (key T opens the menu). */
export const SleepTimer: Story = {
  args: { sleepChoice: '30', sleepRemainingMs: 1_781_000, openMenu: 'sleep' },
};

/** A podcast episode: no chapters, no cover of its own (fallback tile), muted. */
export const EpisodeWithoutCover: Story = {
  args: {
    title: 'Folge 12: Ein langer Abend',
    byline: 'Der Abend-Podcast',
    coverUrl: undefined,
    accent: accents.moss,
    chapters: [],
    chapter: { index: -1, offsetMs: 0, lengthMs: 0 },
    currentMs: 95_000,
    durationMs: 2_460_000,
    playing: false,
    muted: true,
  },
};

/** The server ended the playback: a notice over the player, the controls are off. */
export const Ended: Story = {
  args: {
    playing: false,
    isDisabled: true,
    alert: (
      <>
        <p>Wiedergabe beendet</p>
        <p>Die Wiedergabe wurde beendet.</p>
      </>
    ),
  },
};
