import {
  useEffect,
  useRef,
  useState,
  type ComponentProps,
  type FocusEvent,
  type KeyboardEvent,
  type ReactNode,
  type Ref,
} from 'react';
import { useTranslation } from 'react-i18next';
import { formatPlayerTime } from '@/lib/format';
import { AriaButton } from './AriaButton';
import type { IconName } from './Icon';
import { MenuButton } from './Menu';
import styles from './PlayerChrome.module.css';
import { SeekBar, type SeekBarChapter, type SeekBarSegment } from './SeekBar';
import { TitleOrLogo } from './TitleOrLogo';
import { VolumeControl } from './VolumeControl';

/** The end time moves on while the player is paused: it is computed again this often. */
const CLOCK_TICK_MS = 10_000;

/** What is left to watch at `rate`, in real time (V-4: «−20:59 · endet um 23:41»). */
export function playerRemainingMs(currentMs: number, durationMs: number, rate = 1): number {
  const left = Math.max(0, durationMs - currentMs);
  return rate > 0 ? left / rate : left;
}

/** When the title ends if it plays on from `nowMs` at `rate`. */
export function playerEndTime(
  nowMs: number,
  currentMs: number,
  durationMs: number,
  rate = 1,
): Date {
  return new Date(nowMs + playerRemainingMs(currentMs, durationMs, rate));
}

/** A choice of the bar that is still a react-aria menu (the chapters until step 5). */
export type PlayerChromeMenu = Omit<
  ComponentProps<typeof MenuButton>,
  'excludeFromTabOrder' | 'portalContainer'
>;

/**
 * A button of the row that opens a panel (sound, subtitles, the gear). The page renders the panel;
 * PlayerPanel gives the focus back to this button when it closes.
 */
export interface PlayerChromePanelButton {
  /** Spoken name: «Ton: Deutsch», «Einstellungen». */
  label: string;
  /** The word beside the icon: the language chosen («Deutsch», «Aus»); none for the gear. */
  text?: string;
  /** The panel is open (aria-expanded). */
  expanded: boolean;
  onPress: () => void;
  isDisabled?: boolean;
}

/** A step of the transport: chapter or episode back or forward. */
export interface PlayerChromeStep {
  label: string;
  onPress: () => void;
  isDisabled?: boolean;
}

type RowKey =
  | 'previous'
  | 'back'
  | 'play'
  | 'forward'
  | 'next'
  | 'mute'
  | 'subtitles'
  | 'audio'
  | 'chapters'
  | 'settings'
  | 'help';

interface RowEntry {
  key: RowKey;
  disabled: boolean;
  render: (excluded: boolean) => ReactNode;
}

const modified = (event: KeyboardEvent) =>
  event.altKey || event.ctrlKey || event.metaKey || event.shiftKey;

interface PlayerChromeProps {
  /** The title, or the series of an episode. */
  title: string;
  /** Logo of the title or the series (ADR 0018); the text title shows until it loads. */
  logoUrl?: string;
  /** Under the title: «S01E04 · Wahrheit», the year and genres of a film. */
  headingDetail?: string;
  /** Above the timeline on the left: the title and the chapter playing. */
  lineLabel: string;
  onBack: () => void;
  /** The controls faded out (no input while playing). */
  hidden?: boolean;

  currentMs: number;
  durationMs: number;
  /** Buffered ranges as [startMs, endMs]. */
  buffered: readonly (readonly [number, number])[];
  /** Playback speed: the remaining time and the end time count with it. */
  rate?: number;
  /** Elapsed and total, or remaining and the end time; a click on the time switches. */
  timeDisplay: 'elapsed' | 'remaining';
  onTimeDisplayChange: (display: 'elapsed' | 'remaining') => void;
  chapters?: readonly SeekBarChapter[];
  segments?: { intro?: SeekBarSegment; outro?: SeekBarSegment };
  onSeek: (ms: number) => void;
  /** Shift+←/→ on the timeline. */
  onPreviousChapter?: () => void;
  onNextChapter?: () => void;

  playing: boolean;
  onTogglePlay: () => void;
  /** −10 s and +30 s. */
  onSkip: (seconds: number) => void;
  /** A stable ref: the player gives its first focus through it (focusOnMount). */
  playButtonRef?: Ref<HTMLButtonElement>;
  /** Chapter back; none without chapters. */
  previous?: PlayerChromeStep;
  /** Chapter forward, or the next episode after the last chapter. */
  next?: PlayerChromeStep;
  /** Play, pause and the steps are off (a switch is on its way, the server ended the playback). */
  transportDisabled?: boolean;

  /** 0 … 1. */
  volume: number;
  muted: boolean;
  onVolumeChange: (volume: number) => void;
  onToggleMute: () => void;
  /** The slider goes up to 150 % (Web Audio gain; not in Safari, 11.15). */
  volumeBoost?: boolean;
  /** Only the mute button: Safari on the iPhone does not let a page set the volume (11.2). */
  muteOnly?: boolean;

  fullscreen: boolean;
  onToggleFullscreen: () => void;
  /** Only where the browser offers it (`document.pictureInPictureEnabled`). */
  pictureInPicture?: { active: boolean; onToggle: () => void };

  /**
   * The panels on the right (11.2): subtitles and sound with the language chosen, the gear with
   * quality, version, speed and the rest. A page leaves out what it cannot offer (the offline
   * player has no server to switch tracks, F-15).
   */
  subtitles?: PlayerChromePanelButton;
  audio?: PlayerChromePanelButton;
  settings?: PlayerChromePanelButton;
  /** The chapters, a menu until the chapter tab of step 5. */
  chapterMenu?: PlayerChromeMenu;
  /** The element the menus render into (the player, for full screen). */
  portalContainer?: Element;
  helpOpen?: boolean;
  onToggleHelp?: () => void;
  /** The tabs under the bar (step 5): ↓ from the buttons goes there, ↑ comes back. */
  tabs?: ReactNode;
}

/**
 * The controls of the video player (concept 13, 11.2): a head with back, the title or its logo and
 * the episode, picture in picture and full screen; a line with the title and chapter and the time
 * (a click switches to the remaining time and the end time); the timeline; and a row of buttons:
 * transport and volume on the left, subtitles, sound, the gear and «?» on the right. It shows state
 * and reports actions; the page owns the media element and the key map.
 *
 * Focus model: the row is one Tab stop with a roving tab index. ←/→ go from button to button,
 * ↑ goes to the timeline, ↓ to the tabs. On the timeline ↓ comes back to the row; its other keys go
 * on to the page (±10 s, volume). The volume slider is a Tab stop after its mute button while the
 * button is the row's stop, and keeps ←/→ for the volume.
 */
export function PlayerChrome({
  title,
  logoUrl,
  headingDetail,
  lineLabel,
  onBack,
  hidden = false,
  currentMs,
  durationMs,
  buffered,
  rate = 1,
  timeDisplay,
  onTimeDisplayChange,
  chapters,
  segments,
  onSeek,
  onPreviousChapter,
  onNextChapter,
  playing,
  onTogglePlay,
  onSkip,
  playButtonRef,
  previous,
  next,
  transportDisabled = false,
  volume,
  muted,
  onVolumeChange,
  onToggleMute,
  volumeBoost = false,
  muteOnly = false,
  fullscreen,
  onToggleFullscreen,
  pictureInPicture,
  subtitles,
  audio,
  settings,
  chapterMenu,
  portalContainer,
  helpOpen = false,
  onToggleHelp,
  tabs,
}: PlayerChromeProps) {
  const { t, i18n } = useTranslation();
  const row = useRef<HTMLDivElement>(null);
  const seekZone = useRef<HTMLDivElement>(null);
  const tabsZone = useRef<HTMLDivElement>(null);
  const [active, setActive] = useState<RowKey>('play');
  const [now, setNow] = useState(() => Date.now());

  useEffect(() => {
    if (timeDisplay !== 'remaining') return;
    const timer = window.setInterval(() => setNow(Date.now()), CLOCK_TICK_MS);
    return () => window.clearInterval(timer);
  }, [timeDisplay]);

  const elapsed = formatPlayerTime(currentMs, durationMs);
  const total = formatPlayerTime(durationMs, durationMs);
  const remainingMs = playerRemainingMs(currentMs, durationMs, rate);
  const timeText =
    timeDisplay === 'remaining'
      ? t('player.chrome.timeRemaining', {
          remaining: formatPlayerTime(remainingMs, durationMs),
          end: new Intl.DateTimeFormat(i18n.language, {
            hour: '2-digit',
            minute: '2-digit',
          }).format(playerEndTime(now, currentMs, durationMs, rate)),
        })
      : t('player.chrome.timeElapsed', { elapsed, total });

  const panelButton = (key: RowKey, config: PlayerChromePanelButton | undefined, icon: IconName) =>
    config
      ? [
          {
            key,
            disabled: config.isDisabled ?? false,
            render: (excluded: boolean) => (
              <AriaButton
                variant="ghost"
                icon={icon}
                iconOnly={config.text === undefined}
                aria-label={config.label}
                aria-expanded={config.expanded}
                onPress={config.onPress}
                isDisabled={config.isDisabled}
                excludeFromTabOrder={excluded}
              >
                {config.text !== undefined && (
                  <span className={styles.panelText}>{config.text}</span>
                )}
              </AriaButton>
            ),
          },
        ]
      : [];
  const menu = (key: RowKey, config: PlayerChromeMenu | undefined) =>
    config
      ? [
          {
            key,
            disabled: config.isDisabled ?? false,
            render: (excluded: boolean) => (
              <MenuButton
                {...config}
                excludeFromTabOrder={excluded}
                portalContainer={portalContainer}
              />
            ),
          },
        ]
      : [];

  const left: RowEntry[] = [
    ...(previous
      ? [
          {
            key: 'previous' as const,
            disabled: transportDisabled || previous.isDisabled === true,
            render: (excluded: boolean) => (
              <AriaButton
                variant="ghost"
                icon="previous"
                iconOnly
                aria-label={previous.label}
                onPress={previous.onPress}
                isDisabled={transportDisabled || previous.isDisabled}
                excludeFromTabOrder={excluded}
              />
            ),
          },
        ]
      : []),
    {
      key: 'back',
      disabled: transportDisabled,
      render: (excluded) => (
        <AriaButton
          variant="ghost"
          icon="replay"
          iconOnly
          aria-label={t('player.chrome.back10')}
          onPress={() => onSkip(-10)}
          isDisabled={transportDisabled}
          excludeFromTabOrder={excluded}
        />
      ),
    },
    {
      key: 'play',
      disabled: transportDisabled,
      render: (excluded) => (
        <AriaButton
          ref={playButtonRef}
          variant="ghost"
          icon={playing ? 'pause' : 'play'}
          iconOnly
          aria-label={playing ? t('player.pause') : t('player.play')}
          onPress={onTogglePlay}
          isDisabled={transportDisabled}
          excludeFromTabOrder={excluded}
        />
      ),
    },
    {
      key: 'forward',
      disabled: transportDisabled,
      render: (excluded) => (
        <AriaButton
          variant="ghost"
          icon="forward"
          iconOnly
          aria-label={t('player.chrome.forward30')}
          onPress={() => onSkip(30)}
          isDisabled={transportDisabled}
          excludeFromTabOrder={excluded}
        />
      ),
    },
    ...(next
      ? [
          {
            key: 'next' as const,
            disabled: transportDisabled || next.isDisabled === true,
            render: (excluded: boolean) => (
              <AriaButton
                variant="ghost"
                icon="next"
                iconOnly
                aria-label={next.label}
                onPress={next.onPress}
                isDisabled={transportDisabled || next.isDisabled}
                excludeFromTabOrder={excluded}
              />
            ),
          },
        ]
      : []),
    {
      key: 'mute',
      disabled: false,
      render: (excluded) => (
        <VolumeControl
          volume={volume}
          muted={muted}
          onVolumeChange={onVolumeChange}
          onToggleMute={onToggleMute}
          boost={volumeBoost}
          muteOnly={muteOnly}
          tabIndex={excluded ? -1 : 0}
        />
      ),
    },
  ];
  const right: RowEntry[] = [
    ...panelButton('subtitles', subtitles, 'subtitles'),
    ...panelButton('audio', audio, 'audioTrack'),
    ...menu('chapters', chapterMenu),
    ...panelButton('settings', settings, 'settings'),
    ...(onToggleHelp
      ? [
          {
            key: 'help' as const,
            disabled: false,
            render: (excluded: boolean) => (
              <AriaButton
                variant="ghost"
                icon="help"
                iconOnly
                aria-label={t('player.chrome.help')}
                aria-expanded={helpOpen}
                onPress={onToggleHelp}
                excludeFromTabOrder={excluded}
              />
            ),
          },
        ]
      : []),
  ];

  // The row's Tab stop: the button that had the focus last, else play/pause, else the first
  // button that can take the focus (play is off during a switch).
  const enabled = [...left, ...right].filter((entry) => !entry.disabled);
  const stop = enabled.some((entry) => entry.key === active)
    ? active
    : (enabled.find((entry) => entry.key === 'play') ?? enabled[0])?.key;
  const renderEntry = (entry: RowEntry) => (
    <span key={entry.key} className={styles.item} data-row-key={entry.key}>
      {entry.render(entry.key !== stop)}
    </span>
  );

  /** The buttons of the row in their order, without the ones that cannot take the focus. */
  const rowButtons = () =>
    [...(row.current?.querySelectorAll<HTMLElement>('[data-row-key]') ?? [])]
      .map((item) => item.querySelector('button'))
      .filter((button): button is HTMLButtonElement => button !== null && !button.disabled);
  const focusRow = () => {
    const buttons = rowButtons();
    const target =
      row.current?.querySelector<HTMLButtonElement>(`[data-row-key="${stop}"] button`) ??
      buttons[0];
    target?.focus();
  };
  const focusSeek = () => seekZone.current?.querySelector<HTMLElement>('[role="slider"]')?.focus();
  const focusTabs = () =>
    tabsZone.current
      ?.querySelector<HTMLElement>('[tabindex="0"], button:not(:disabled), a[href]')
      ?.focus();

  // Capture phase: a menu button opens its menu on ↑/↓ itself, so the row takes those keys first.
  // Keys from a menu's popover reach here through the React tree (a portal) and are left alone.
  const onRowKeyDown = (event: KeyboardEvent<HTMLDivElement>) => {
    const target = event.target;
    if (!(target instanceof HTMLElement) || !event.currentTarget.contains(target)) return;
    if (modified(event)) return;
    const consume = () => {
      event.preventDefault();
      event.stopPropagation();
    };
    if (event.key === 'ArrowUp') {
      consume();
      focusSeek();
      return;
    }
    if (event.key === 'ArrowDown') {
      consume();
      focusTabs();
      return;
    }
    // The volume slider keeps ←/→, Home and End for the volume.
    if (!(target instanceof HTMLButtonElement)) return;
    const buttons = rowButtons();
    const index = buttons.indexOf(target);
    if (index < 0) return;
    const to =
      event.key === 'ArrowLeft'
        ? index - 1
        : event.key === 'ArrowRight'
          ? index + 1
          : event.key === 'Home'
            ? 0
            : event.key === 'End'
              ? buttons.length - 1
              : null;
    if (to === null) return;
    // At either end the key stays in the row: it neither wraps nor seeks.
    consume();
    buttons[Math.min(buttons.length - 1, Math.max(0, to))]?.focus();
  };

  const onRowFocus = (event: FocusEvent<HTMLDivElement>) => {
    const key = (event.target as HTMLElement)
      .closest('[data-row-key]')
      ?.getAttribute('data-row-key');
    if (key) setActive(key as RowKey);
  };

  return (
    <div className={styles.chrome} data-hidden={hidden || undefined}>
      <div className={styles.top}>
        <AriaButton
          variant="ghost"
          icon="back"
          iconOnly
          aria-label={t('player.back')}
          onPress={onBack}
        />
        <div className={styles.heading}>
          <h1 className={styles.title}>
            <TitleOrLogo title={title} logoUrl={logoUrl} className={styles.logo} />
          </h1>
          {headingDetail && <p className={styles.headingDetail}>{headingDetail}</p>}
        </div>
        {pictureInPicture && (
          <AriaButton
            variant="ghost"
            icon="pictureInPicture"
            iconOnly
            aria-label={
              pictureInPicture.active
                ? t('player.chrome.exitPictureInPicture')
                : t('player.chrome.pictureInPicture')
            }
            aria-pressed={pictureInPicture.active}
            onPress={pictureInPicture.onToggle}
          />
        )}
        <AriaButton
          variant="ghost"
          icon={fullscreen ? 'fullscreenExit' : 'fullscreen'}
          iconOnly
          aria-label={fullscreen ? t('player.exitFullscreen') : t('player.fullscreen')}
          onPress={onToggleFullscreen}
        />
      </div>

      <div className={styles.bottom}>
        <div className={styles.line}>
          <span className={styles.lineLabel}>{lineLabel}</span>
          <AriaButton
            variant="ghost"
            onPress={() => {
              setNow(Date.now());
              onTimeDisplayChange(timeDisplay === 'elapsed' ? 'remaining' : 'elapsed');
            }}
          >
            <span className={styles.time}>{timeText}</span>
          </AriaButton>
        </div>
        {/* ↓ on the timeline goes to the row; the slider passes every other plain arrow on. */}
        {/* eslint-disable-next-line jsx-a11y/no-static-element-interactions */}
        <div
          ref={seekZone}
          onKeyDown={(event) => {
            if (event.key !== 'ArrowDown' || modified(event)) return;
            event.preventDefault();
            event.stopPropagation();
            focusRow();
          }}
        >
          <SeekBar
            label={t('player.seek')}
            valueText={t('player.time', { elapsed, total })}
            currentMs={currentMs}
            durationMs={durationMs}
            buffered={buffered}
            onSeek={onSeek}
            chapters={chapters}
            segments={segments}
            onPreviousChapter={onPreviousChapter}
            onNextChapter={onNextChapter}
          />
        </div>
        <div
          ref={row}
          className={styles.bar}
          role="toolbar"
          aria-label={t('player.chrome.toolbar')}
          onKeyDownCapture={onRowKeyDown}
          onFocus={onRowFocus}
        >
          <span className={styles.group}>{left.map(renderEntry)}</span>
          <span className={styles.group}>{right.map(renderEntry)}</span>
        </div>
        {tabs && (
          // ↑ in the tabs goes back to the row, also from a card in a row of cards.
          <div
            ref={tabsZone}
            onKeyDownCapture={(event) => {
              if (event.key !== 'ArrowUp' || modified(event)) return;
              if (!(event.target instanceof Node) || !event.currentTarget.contains(event.target)) {
                return;
              }
              event.preventDefault();
              event.stopPropagation();
              focusRow();
            }}
          >
            {tabs}
          </div>
        )}
      </div>
    </div>
  );
}
