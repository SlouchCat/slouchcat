import {
  useRef,
  useState,
  type HTMLAttributes,
  type PointerEvent,
  type ReactNode,
  type Ref,
} from 'react';
import { useTranslation } from 'react-i18next';
import { itemAccentStyle } from './accent';
import { AriaButton } from './AriaButton';
import styles from './AudioPlayer.module.css';
import { MenuButton } from './Menu';
import { PosterFallback } from './PosterFallback';

/** The menus of the audio player; the page opens them by key as well (C, T). */
export type AudioPlayerMenu = 'chapters' | 'speed' | 'sleep';

/** A chapter on the title's single timeline (contract `Chapter`). */
export interface AudioPlayerChapter {
  startMs: number;
  endMs: number;
  title?: string;
}

/** Speeds of the speed menu (H-3). */
const RATES = [0.5, 0.75, 1, 1.25, 1.5, 1.75, 2, 2.5, 3] as const;
/** Choices of the sleep timer menu; minutes as text, `chapter` = end of the chapter. */
const SLEEP_CHOICES = ['off', '15', '30', '45', '60', 'chapter'] as const;

interface AudioPlayerProps extends Omit<HTMLAttributes<HTMLElement>, 'title' | 'children'> {
  title: string;
  /** The author of a book, the podcast of an episode. */
  byline?: string;
  /** Square cover (w400 poster URL), or undefined for the lettered fallback. */
  coverUrl?: string;
  /** The title's accent colour (`#rrggbb`): the glow behind the cover. */
  accent?: string;
  chapters: readonly AudioPlayerChapter[];
  /** The playing chapter (-1: none), the time into it and its length. */
  chapter: { index: number; offsetMs: number; lengthMs: number };
  currentMs: number;
  durationMs: number;
  playing: boolean;
  /** Playback speed, 0.5 … 3. */
  rate: number;
  /** 0 … 1. */
  volume: number;
  muted: boolean;
  /** Menu key of the sleep timer (`off`, `15` … `60`, `chapter`) and the time it has left. */
  sleepChoice: string;
  sleepRemainingMs: number | null;
  openMenu: AudioPlayerMenu | null;
  onOpenMenuChange: (menu: AudioPlayerMenu | null) => void;
  onTogglePlay: () => void;
  onSeek: (ms: number) => void;
  /** −15 s or +30 s. */
  onSkip: (seconds: number) => void;
  onPreviousChapter: () => void;
  onNextChapter: () => void;
  onRate: (rate: number) => void;
  onSleep: (choice: string) => void;
  onVolume: (volume: number) => void;
  onToggleMute: () => void;
  onBack: () => void;
  /** The playback was ended (watch-time limit, the server): only "back" stays. */
  isDisabled?: boolean;
  playButtonRef?: Ref<HTMLButtonElement>;
  /** Why the playback stopped (failed, ended, watch-time limit), over the player. */
  alert?: ReactNode;
  /** A short confirmation of a key (volume, speed, sleep timer). */
  status?: string | null;
  /** The media element. */
  children?: ReactNode;
}

const pad = (value: number) => String(value).padStart(2, '0');

/** `h:mm:ss` from an hour on (of the longer of both times), else `m:ss`. */
function clock(ms: number, longest: number): string {
  const total = Math.max(0, Math.floor(ms / 1000));
  const hours = Math.floor(total / 3600);
  const minutes = Math.floor((total % 3600) / 60);
  const seconds = pad(total % 60);
  return longest >= 3_600_000 ? `${hours}:${pad(minutes)}:${seconds}` : `${minutes}:${seconds}`;
}

/** Book-wide seek bar (an ARIA slider); the page's key map moves it with the arrow keys. */
function SeekSlider({
  label,
  valueText,
  currentMs,
  durationMs,
  onSeek,
  isDisabled,
}: {
  label: string;
  valueText: string;
  currentMs: number;
  durationMs: number;
  onSeek: (ms: number) => void;
  isDisabled: boolean;
}) {
  const track = useRef<HTMLDivElement>(null);
  const [dragging, setDragging] = useState(false);
  const fraction = durationMs > 0 ? Math.min(1, Math.max(0, currentMs / durationMs)) : 0;
  const seekTo = (event: PointerEvent<HTMLDivElement>) => {
    const rect = track.current?.getBoundingClientRect();
    if (isDisabled || !rect || rect.width === 0 || durationMs <= 0) return;
    onSeek(Math.min(1, Math.max(0, (event.clientX - rect.left) / rect.width)) * durationMs);
  };
  return (
    <div
      ref={track}
      className={styles.seek}
      role="slider"
      tabIndex={0}
      aria-label={label}
      aria-valuemin={0}
      aria-valuemax={Math.round(durationMs / 1000)}
      aria-valuenow={Math.round(currentMs / 1000)}
      aria-valuetext={valueText}
      aria-disabled={isDisabled || undefined}
      data-dragging={dragging || undefined}
      onPointerDown={(event) => {
        event.currentTarget.setPointerCapture(event.pointerId);
        setDragging(true);
        seekTo(event);
      }}
      onPointerMove={(event) => {
        if (dragging) seekTo(event);
      }}
      onPointerUp={(event) => {
        event.currentTarget.releasePointerCapture(event.pointerId);
        setDragging(false);
      }}
      onPointerCancel={() => setDragging(false)}
    >
      <div className={styles.seekTrack}>
        <span className={styles.seekPlayed} style={{ width: `${fraction * 100}%` }} />
        <span className={styles.seekThumb} style={{ left: `${fraction * 100}%` }} />
      </div>
    </div>
  );
}

/**
 * The audio player (M6, design 4): a large cover with a glow in the title's accent, title and
 * author, the current chapter with the time into it, a seek bar over the whole book, transport
 * (−15 s, play/pause, +30 s, chapter back and forward), speed, chapter list, sleep timer and
 * volume. It shows state and reports choices; the page owns the media element, the key map and
 * the timers.
 */
export function AudioPlayer({
  title,
  byline,
  coverUrl,
  accent,
  chapters,
  chapter,
  currentMs,
  durationMs,
  playing,
  rate,
  volume,
  muted,
  sleepChoice,
  sleepRemainingMs,
  openMenu,
  onOpenMenuChange,
  onTogglePlay,
  onSeek,
  onSkip,
  onPreviousChapter,
  onNextChapter,
  onRate,
  onSleep,
  onVolume,
  onToggleMute,
  onBack,
  isDisabled = false,
  playButtonRef,
  alert,
  status,
  children,
  ...rest
}: AudioPlayerProps) {
  const { t, i18n } = useTranslation();
  const rateText = (value: number) =>
    t('player.audio.rate', {
      rate: new Intl.NumberFormat(i18n.language, { maximumFractionDigits: 2 }).format(value),
    });
  const chapterTitle = (index: number) =>
    chapters[index]?.title ?? t('player.chapters.untitled', { n: index + 1 });
  const current = chapters[chapter.index];
  const hasChapters = chapters.length > 0;
  const elapsed = clock(currentMs, durationMs);
  const total = clock(durationMs, durationMs);
  const sleepLeft =
    sleepRemainingMs === null ? null : clock(sleepRemainingMs, Math.max(sleepRemainingMs, 1));
  const menuProps = (menu: AudioPlayerMenu) => ({
    isDisabled,
    isOpen: openMenu === menu,
    onOpenChange: (isOpen: boolean) =>
      onOpenMenuChange(isOpen ? menu : openMenu === menu ? null : openMenu),
  });
  const presetKey = RATES.find((value) => Math.abs(value - rate) < 0.001);

  return (
    <main className={styles.player} aria-label={t('player.audioRegion')} {...rest}>
      {children}
      {alert && (
        <div className={styles.overlay} role="alert">
          {alert}
        </div>
      )}
      {status && (
        <div className={styles.status} role="status">
          {status}
        </div>
      )}
      <div className={styles.top}>
        <AriaButton
          variant="ghost"
          icon="back"
          iconOnly
          aria-label={t('player.back')}
          onPress={onBack}
        />
      </div>

      <div className={styles.stage}>
        <div className={styles.cover} style={itemAccentStyle(accent)}>
          {coverUrl ? (
            <img className={styles.image} src={coverUrl} alt="" width={400} height={400} />
          ) : (
            <span className={styles.image}>
              <PosterFallback />
            </span>
          )}
        </div>

        <div className={styles.panel}>
          <div className={styles.heading}>
            <h1 className={styles.title}>{title}</h1>
            {byline && <p className={styles.byline}>{byline}</p>}
            {current && (
              <p className={styles.chapter} data-chapter-index={chapter.index}>
                {t('player.audio.chapterLine', {
                  title: chapterTitle(chapter.index),
                  position: clock(chapter.offsetMs, chapter.lengthMs),
                  length: clock(chapter.lengthMs, chapter.lengthMs),
                })}
              </p>
            )}
          </div>

          <div className={styles.timeline}>
            <SeekSlider
              label={t('player.seek')}
              valueText={t('player.time', { elapsed, total })}
              currentMs={currentMs}
              durationMs={durationMs}
              onSeek={onSeek}
              isDisabled={isDisabled}
            />
            <div className={styles.times} aria-hidden="true">
              <span>{elapsed}</span>
              <span>{total}</span>
            </div>
          </div>

          <div className={styles.transport}>
            {hasChapters && (
              <AriaButton
                variant="ghost"
                icon="previous"
                iconOnly
                aria-label={t('player.audio.previousChapter')}
                onPress={onPreviousChapter}
                isDisabled={isDisabled}
              />
            )}
            <AriaButton
              variant="ghost"
              icon="replay"
              iconOnly
              aria-label={t('player.audio.back15')}
              onPress={() => onSkip(-15)}
              isDisabled={isDisabled}
            />
            <AriaButton
              ref={playButtonRef}
              variant="primary"
              size="lg"
              icon={playing ? 'pause' : 'play'}
              iconOnly
              aria-label={playing ? t('player.pause') : t('player.play')}
              onPress={onTogglePlay}
              isDisabled={isDisabled}
              autoFocus
            />
            <AriaButton
              variant="ghost"
              icon="forward"
              iconOnly
              aria-label={t('player.audio.forward30')}
              onPress={() => onSkip(30)}
              isDisabled={isDisabled}
            />
            {hasChapters && (
              <AriaButton
                variant="ghost"
                icon="next"
                iconOnly
                aria-label={t('player.audio.nextChapter')}
                onPress={onNextChapter}
                isDisabled={isDisabled || chapter.index >= chapters.length - 1}
              />
            )}
          </div>

          <div className={styles.extras}>
            <span className={styles.group}>
              <AriaButton
                variant="ghost"
                aria-label={t('player.audio.slower')}
                onPress={() => onRate(rate - 0.1)}
                isDisabled={isDisabled || rate <= 0.5}
              >
                {t('player.audio.minus')}
              </AriaButton>
              <MenuButton
                buttonLabel={t('player.audio.speedButton', { rate: rateText(rate) })}
                label={rateText(rate)}
                options={RATES.map((value) => ({ id: String(value), label: rateText(value) }))}
                selectedKey={presetKey === undefined ? undefined : String(presetKey)}
                onSelect={(id) => onRate(Number(id))}
                {...menuProps('speed')}
              />
              <AriaButton
                variant="ghost"
                aria-label={t('player.audio.faster')}
                onPress={() => onRate(rate + 0.1)}
                isDisabled={isDisabled || rate >= 3}
              >
                {t('player.audio.plus')}
              </AriaButton>
            </span>
            {hasChapters && (
              <MenuButton
                icon="list"
                buttonLabel={t('player.chapters.button')}
                label={t('player.chapters.button')}
                options={chapters.map((entry, index) => ({
                  id: String(index),
                  label: t('player.chapters.item', {
                    title: chapterTitle(index),
                    time: clock(entry.startMs, durationMs),
                  }),
                }))}
                selectedKey={chapter.index >= 0 ? String(chapter.index) : undefined}
                reselect
                onSelect={(id) => {
                  const entry = chapters[Number(id)];
                  if (entry) onSeek(entry.startMs);
                }}
                {...menuProps('chapters')}
              />
            )}
            <MenuButton
              icon="moon"
              buttonLabel={
                sleepLeft === null
                  ? t('player.audio.sleep.button')
                  : t('player.audio.sleep.buttonActive', { left: sleepLeft })
              }
              label={sleepLeft ?? t('player.audio.sleep.short')}
              options={SLEEP_CHOICES.map((choice) => ({
                id: choice,
                label:
                  choice === 'off'
                    ? t('player.audio.sleep.off')
                    : choice === 'chapter'
                      ? t('player.audio.sleep.chapter')
                      : t('player.audio.sleep.minutes', { count: Number(choice) }),
              }))}
              selectedKey={sleepChoice}
              onSelect={onSleep}
              {...menuProps('sleep')}
            />
            <span className={styles.group}>
              <AriaButton
                variant="ghost"
                icon={muted || volume === 0 ? 'volumeMute' : 'volume'}
                iconOnly
                aria-label={t('player.audio.mute')}
                aria-pressed={muted}
                onPress={onToggleMute}
              />
              <input
                className={styles.volume}
                type="range"
                min={0}
                max={1}
                step={0.05}
                value={muted ? 0 : volume}
                aria-label={t('player.audio.volume')}
                aria-valuetext={t('player.volume', {
                  percent: Math.round((muted ? 0 : volume) * 100),
                })}
                onChange={(event) => onVolume(Number(event.currentTarget.value))}
              />
            </span>
          </div>
        </div>
      </div>
    </main>
  );
}
