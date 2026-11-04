import {
  useEffect,
  useLayoutEffect,
  useRef,
  useState,
  type FocusEvent,
  type KeyboardEvent,
  type ReactNode,
  type Ref,
  type RefObject,
} from 'react';
import { useTranslation } from 'react-i18next';
import { formatPlayerTime } from '@/lib/format';
import { AriaButton } from './AriaButton';
import { focusLost } from './focusAfter';
import type { IconName } from './Icon';
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
  /** The id of the panel (PlayerPanel `id`), for `aria-controls` while it is open. */
  controls?: string;
  onPress: () => void;
  isDisabled?: boolean;
}

/** A step of the transport: chapter or episode back or forward. */
export interface PlayerChromeStep {
  label: string;
  onPress: () => void;
  isDisabled?: boolean;
}

/**
 * A tab under the bar (11.2): Kapitel, Als Nächstes, Besetzung, Info. It opens either a row of
 * cards or free content (one Tab stop that scrolls).
 */
export interface PlayerChromeTab {
  id: string;
  label: string;
  /**
   * A row of cards with a roving tab index. `render` draws card `index`; `excluded` takes it out
   * of the Tab order (the row is one Tab stop, on `current` to begin with).
   */
  cards?: {
    count: number;
    current?: number;
    render: (index: number, excluded: boolean) => ReactNode;
    /**
     * Moves the focus to the current card whenever it changes (C opens the chapters on the chapter
     * playing, also when the tab is open already); undefined leaves the focus alone.
     */
    focusRequest?: number;
  };
  /** Free content instead of cards (the info of the title). */
  content?: ReactNode;
}

/**
 * Desktop and tablet, or a phone (11.2: narrower than 768 px, or a touch screen held sideways):
 * on a phone −10 s, play/pause and +10 s sit large in the middle, volume, subtitles, sound and the
 * gear at the top right, and there are no tabs. The focus order follows the layout.
 */
export type PlayerChromeLayout = 'desktop' | 'phone';

type RowKey =
  | 'previous'
  | 'back'
  | 'play'
  | 'forward'
  | 'next'
  | 'mute'
  | 'subtitles'
  | 'audio'
  | 'settings'
  | 'help';

interface RowEntry {
  key: RowKey;
  disabled: boolean;
  render: (excluded: boolean) => ReactNode;
}

const modified = (event: KeyboardEvent) =>
  event.altKey || event.ctrlKey || event.metaKey || event.shiftKey;

const consume = (event: KeyboardEvent) => {
  event.preventDefault();
  event.stopPropagation();
};

/** The index ←/→, Home and End lead to from `index` of `count`, or null for another key. */
function rovingTarget(key: string, index: number, count: number): number | null {
  const to =
    key === 'ArrowLeft'
      ? index - 1
      : key === 'ArrowRight'
        ? index + 1
        : key === 'Home'
          ? 0
          : key === 'End'
            ? count - 1
            : null;
  // At either end the key stays where it is: it neither wraps nor seeks.
  return to === null ? null : Math.min(count - 1, Math.max(0, to));
}

/** The Tab stop of a toolbar: the button marked as its stop, else its first enabled one. */
function focusToolbar(toolbar: HTMLElement | null) {
  const target =
    toolbar?.querySelector<HTMLButtonElement>('[data-stop] button') ??
    toolbar?.querySelector<HTMLButtonElement>('[data-row-key] button:not(:disabled)');
  target?.focus();
}

interface ToolbarProps {
  label: string;
  className: string | undefined;
  groupClassName: string | undefined;
  groups: readonly (readonly RowEntry[])[];
  toolbarRef: RefObject<HTMLDivElement | null>;
  onUp?: () => void;
  onDown?: () => void;
}

/**
 * A row of buttons that is one Tab stop with a roving tab index: ←/→, Home and End go from button
 * to button, ↑ and ↓ to the zones above and below. The stop is the button that had the focus last,
 * else play/pause, else the first button that can take the focus (play is off during a switch).
 * The volume slider is a Tab stop after its mute button while the button is the stop, and keeps
 * ←/→ for the volume.
 */
function Toolbar({
  label,
  className,
  groupClassName,
  groups,
  toolbarRef,
  onUp,
  onDown,
}: ToolbarProps) {
  const [active, setActive] = useState<RowKey>('play');
  const enabled = groups.flat().filter((entry) => !entry.disabled);
  const stop = enabled.some((entry) => entry.key === active)
    ? active
    : (enabled.find((entry) => entry.key === 'play') ?? enabled[0])?.key;

  /** The buttons of the row in their order, without the ones that cannot take the focus. */
  const buttons = () =>
    [...(toolbarRef.current?.querySelectorAll<HTMLElement>('[data-row-key]') ?? [])]
      .map((item) => item.querySelector('button'))
      .filter((button): button is HTMLButtonElement => button !== null && !button.disabled);

  // Capture phase: the row takes ↑/↓ before a button can act on them.
  const onKeyDown = (event: KeyboardEvent<HTMLDivElement>) => {
    const target = event.target;
    if (!(target instanceof HTMLElement) || !event.currentTarget.contains(target)) return;
    if (modified(event)) return;
    if (event.key === 'ArrowUp' || event.key === 'ArrowDown') {
      consume(event);
      (event.key === 'ArrowUp' ? onUp : onDown)?.();
      return;
    }
    // The volume slider keeps ←/→, Home and End for the volume.
    if (!(target instanceof HTMLButtonElement)) return;
    const list = buttons();
    const index = list.indexOf(target);
    if (index < 0) return;
    const to = rovingTarget(event.key, index, list.length);
    if (to === null) return;
    consume(event);
    list[to]?.focus();
  };

  const onFocus = (event: FocusEvent<HTMLDivElement>) => {
    const key = (event.target as HTMLElement)
      .closest('[data-row-key]')
      ?.getAttribute('data-row-key');
    if (key) setActive(key as RowKey);
  };

  return (
    <div
      ref={toolbarRef}
      className={className}
      role="toolbar"
      aria-label={label}
      onKeyDownCapture={onKeyDown}
      onFocus={onFocus}
    >
      {groups.map((group, index) => (
        // The groups are fixed per layout: their position names them.
        <span key={index} className={groupClassName}>
          {group.map((entry) => (
            <span
              key={entry.key}
              className={styles.item}
              data-row-key={entry.key}
              data-stop={entry.key === stop || undefined}
            >
              {entry.render(entry.key !== stop)}
            </span>
          ))}
        </span>
      ))}
    </div>
  );
}

/**
 * The cards of an open tab: one Tab stop with a roving tab index, on the current card (the chapter
 * playing, the episode playing) to begin with. ←/→, Home and End go from card to card and stay in
 * the row at its ends.
 */
function CardRow({
  label,
  cards,
}: {
  label: string;
  cards: NonNullable<PlayerChromeTab['cards']>;
}) {
  const list = useRef<HTMLUListElement>(null);
  const [active, setActive] = useState(() =>
    Math.min(Math.max(0, cards.current ?? 0), Math.max(0, cards.count - 1)),
  );
  const stop = Math.min(active, Math.max(0, cards.count - 1));
  const items = () => [...(list.current?.querySelectorAll<HTMLElement>(':scope > li') ?? [])];
  const focusable = (item: HTMLElement | undefined) =>
    item?.querySelector<HTMLElement>('button, a[href]');

  // The page asks for the focus on the current card; the card's focus makes it the row's stop.
  // Only a new request moves the focus, not the current card moving on during playback.
  const { focusRequest, current } = cards;
  const currentNow = useRef(current);
  useEffect(() => {
    currentNow.current = current;
  }, [current]);
  useEffect(() => {
    if (focusRequest === undefined) return;
    const all = [...(list.current?.querySelectorAll<HTMLElement>(':scope > li') ?? [])];
    all[currentNow.current ?? 0]?.querySelector<HTMLElement>('button, a[href]')?.focus();
  }, [focusRequest]);

  const onKeyDown = (event: KeyboardEvent<HTMLUListElement>) => {
    if (modified(event) || event.defaultPrevented) return;
    const all = items();
    const index = all.findIndex(
      (item) => event.target instanceof Node && item.contains(event.target),
    );
    if (index < 0) return;
    const to = rovingTarget(event.key, index, all.length);
    if (to === null) return;
    consume(event);
    focusable(all[to])?.focus();
  };

  return (
    // The arrows of the row move the focus between its cards (their own buttons).
    // eslint-disable-next-line jsx-a11y/no-noninteractive-element-interactions
    <ul
      ref={list}
      className={styles.cards}
      aria-label={label}
      onKeyDown={onKeyDown}
      onFocus={(event) => {
        const index = items().findIndex((item) => item.contains(event.target));
        if (index >= 0) setActive(index);
      }}
    >
      {Array.from({ length: cards.count }, (_, index) => (
        <li key={index} className={styles.card}>
          {cards.render(index, index !== stop)}
        </li>
      ))}
    </ul>
  );
}

interface PlayerChromeProps {
  /** Desktop (the default) or phone; the page decides by the media query of 11.2. */
  layout?: PlayerChromeLayout;
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
  /** −10 s and +30 s (on a phone +10 s). */
  onSkip: (seconds: number) => void;
  /** A stable ref: the player gives its first focus through it (focusOnMount). */
  playButtonRef?: Ref<HTMLButtonElement>;
  /** Chapter or episode back; none without either. Not on a phone. */
  previous?: PlayerChromeStep;
  /** Chapter forward, or the next episode after the last chapter. Not on a phone. */
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
  helpOpen?: boolean;
  /** The shortcut help; not on a phone. */
  onToggleHelp?: () => void;
  /** The tabs under the bar (not on a phone): ↓ from the buttons goes there, ↑ comes back. */
  tabs?: readonly PlayerChromeTab[];
  /** The open tab, or null. */
  openTab?: string | null;
  onOpenTab?: (id: string | null) => void;
}

/**
 * The controls of the video player (concept 13, 11.2): a head with back, the title or its logo and
 * the episode, picture in picture and full screen; a line with the title and chapter and the time
 * (a click switches to the remaining time and the end time); the timeline; a row of buttons:
 * transport and volume on the left, subtitles, sound, the gear and «?» on the right; and the tabs
 * under it. It shows state and reports actions; the page owns the media element and the key map.
 *
 * Focus model (desktop): the row is one Tab stop with a roving tab index. ←/→ go from button to
 * button, ↑ goes to the timeline, ↓ to the tabs. On the timeline ↓ comes back to the row; its
 * other keys go on to the page (±10 s, volume). The tab strip is one Tab stop as well (←/→ from
 * tab to tab, Enter opens or closes one, ↑ back to the row, ↓ into the open tab); in a row of cards
 * ←/→ go from card to card and ↑ goes back to the strip. When the open tab closes with the focus in
 * it (Escape), the focus goes to its tab.
 *
 * On a phone the options at the top right and the transport in the middle are a toolbar each:
 * ↓ goes from the options to the transport and on to the timeline, ↑ back up.
 */
export function PlayerChrome({
  layout = 'desktop',
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
  helpOpen = false,
  onToggleHelp,
  tabs = [],
  openTab = null,
  onOpenTab,
}: PlayerChromeProps) {
  const { t, i18n } = useTranslation();
  const phone = layout === 'phone';
  const row = useRef<HTMLDivElement>(null);
  const options = useRef<HTMLDivElement>(null);
  const seekZone = useRef<HTMLDivElement>(null);
  const tabsZone = useRef<HTMLDivElement>(null);
  const [now, setNow] = useState(() => Date.now());
  const [activeTab, setActiveTab] = useState<string | null>(null);

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

  const shownTabs = phone ? [] : tabs;
  const open = shownTabs.find((tab) => tab.id === openTab);
  const tabStop = shownTabs.find((tab) => tab.id === activeTab)?.id ?? open?.id ?? shownTabs[0]?.id;
  const tabButton = (id: string) =>
    tabsZone.current?.querySelector<HTMLButtonElement>(`[data-tab-key="${id}"] button`);

  // The open tab closed while the focus was in it (Escape, a card that closed it): to its tab.
  const lastOpen = useRef(openTab);
  useLayoutEffect(() => {
    const closed = lastOpen.current;
    lastOpen.current = openTab;
    if (closed !== null && closed !== openTab && focusLost()) tabButton(closed)?.focus();
  });

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
                // On a phone the buttons at the top right are icons only.
                iconOnly={phone || config.text === undefined}
                aria-label={config.label}
                aria-expanded={config.expanded}
                aria-haspopup="dialog"
                aria-controls={config.expanded ? config.controls : undefined}
                onPress={config.onPress}
                isDisabled={config.isDisabled}
                excludeFromTabOrder={excluded}
              >
                {!phone && config.text !== undefined && (
                  <span className={styles.panelText}>{config.text}</span>
                )}
              </AriaButton>
            ),
          },
        ]
      : [];
  const step = (key: 'previous' | 'next', config: PlayerChromeStep | undefined) =>
    config && !phone
      ? [
          {
            key,
            disabled: transportDisabled || config.isDisabled === true,
            render: (excluded: boolean) => (
              <AriaButton
                variant="ghost"
                icon={key}
                iconOnly
                aria-label={config.label}
                onPress={config.onPress}
                isDisabled={transportDisabled || config.isDisabled}
                excludeFromTabOrder={excluded}
              />
            ),
          },
        ]
      : [];
  const size = phone ? 'lg' : undefined;
  const transport: RowEntry[] = [
    ...step('previous', previous),
    {
      key: 'back',
      disabled: transportDisabled,
      render: (excluded) => (
        <AriaButton
          variant="ghost"
          size={size}
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
          size={size}
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
          size={size}
          icon="forward"
          iconOnly
          aria-label={phone ? t('player.chrome.forward10') : t('player.chrome.forward30')}
          onPress={() => onSkip(phone ? 10 : 30)}
          isDisabled={transportDisabled}
          excludeFromTabOrder={excluded}
        />
      ),
    },
    ...step('next', next),
  ];
  const mute: RowEntry = {
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
  };
  const choices: RowEntry[] = [
    ...panelButton('subtitles', subtitles, 'subtitles'),
    ...panelButton('audio', audio, 'audioTrack'),
    ...panelButton('settings', settings, 'settings'),
  ];
  const help: RowEntry[] =
    onToggleHelp && !phone
      ? [
          {
            key: 'help',
            disabled: false,
            render: (excluded) => (
              <AriaButton
                variant="ghost"
                icon="help"
                iconOnly
                aria-label={t('player.chrome.help')}
                aria-expanded={helpOpen}
                aria-haspopup="dialog"
                onPress={onToggleHelp}
                excludeFromTabOrder={excluded}
              />
            ),
          },
        ]
      : [];

  const focusSeek = () => seekZone.current?.querySelector<HTMLElement>('[role="slider"]')?.focus();
  const focusTabs = () => {
    if (tabStop) tabButton(tabStop)?.focus();
  };
  /** Into the open tab: its row of cards (on the card that is the row's stop) or its content. */
  const focusTabContent = () => {
    const panel = tabsZone.current?.querySelector<HTMLElement>('[data-tab-panel]');
    const target =
      panel?.querySelector<HTMLElement>('li button:not([tabindex="-1"]), li a[href]') ??
      panel?.querySelector<HTMLElement>('[data-tab-content]');
    target?.focus();
  };

  // Bubble phase: a card or a row of cards that handled the key itself marks it defaultPrevented.
  const onTabsKeyDown = (event: KeyboardEvent<HTMLDivElement>) => {
    if (modified(event) || event.defaultPrevented) return;
    const target = event.target;
    if (!(target instanceof HTMLElement)) return;
    const inStrip = target.closest('[data-tab-key]') !== null;
    if (event.key === 'ArrowUp') {
      consume(event);
      if (inStrip) focusToolbar(row.current);
      else focusTabs();
      return;
    }
    if (event.key === 'ArrowDown') {
      if (inStrip) {
        consume(event);
        focusTabContent();
      } else {
        // The content scrolls natively; the arrow does not change the volume.
        event.stopPropagation();
        if (!target.hasAttribute('data-tab-content')) event.preventDefault();
      }
      return;
    }
    if (!inStrip) {
      // ←/→ in the open tab never seek.
      if (event.key === 'ArrowLeft' || event.key === 'ArrowRight') consume(event);
      return;
    }
    const strip = [
      ...(tabsZone.current?.querySelectorAll<HTMLButtonElement>('[data-tab-key] button') ?? []),
    ];
    const to = rovingTarget(event.key, strip.indexOf(target as HTMLButtonElement), strip.length);
    if (to === null) return;
    consume(event);
    strip[to]?.focus();
  };

  const timeline = (
    // ↓ on the timeline goes to the row (on a phone ↑ to the transport above it); the slider
    // passes every other plain arrow on.
    // eslint-disable-next-line jsx-a11y/no-static-element-interactions
    <div
      ref={seekZone}
      onKeyDown={(event) => {
        if (modified(event)) return;
        if (phone ? event.key !== 'ArrowUp' : event.key !== 'ArrowDown') return;
        consume(event);
        focusToolbar(row.current);
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
  );

  const line = (
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
  );

  return (
    <div className={styles.chrome} data-hidden={hidden || undefined} data-layout={layout}>
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
        {phone && (
          <Toolbar
            label={t('player.chrome.options')}
            className={styles.options}
            groupClassName={styles.group}
            groups={[[mute, ...choices]]}
            toolbarRef={options}
            onDown={() => focusToolbar(row.current)}
          />
        )}
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

      {phone && (
        <Toolbar
          label={t('player.chrome.toolbar')}
          className={styles.center}
          groupClassName={styles.group}
          groups={[transport]}
          toolbarRef={row}
          onUp={() => focusToolbar(options.current)}
          onDown={focusSeek}
        />
      )}

      <div className={styles.bottom}>
        {line}
        {timeline}
        {!phone && (
          <Toolbar
            label={t('player.chrome.toolbar')}
            className={styles.bar}
            groupClassName={styles.group}
            groups={[
              [...transport, mute],
              [...choices, ...help],
            ]}
            toolbarRef={row}
            onUp={focusSeek}
            onDown={focusTabs}
          />
        )}
        {shownTabs.length > 0 && (
          // The zone's arrows move between the strip, the open tab and the row above.
          // eslint-disable-next-line jsx-a11y/no-static-element-interactions
          <div ref={tabsZone} className={styles.tabs} onKeyDown={onTabsKeyDown}>
            <div className={styles.tabStrip} role="toolbar" aria-label={t('player.tabs.label')}>
              {shownTabs.map((tab) => (
                <span
                  key={tab.id}
                  className={styles.item}
                  data-tab-key={tab.id}
                  onFocus={() => setActiveTab(tab.id)}
                >
                  <AriaButton
                    variant="ghost"
                    aria-expanded={tab.id === open?.id}
                    aria-controls={tab.id === open?.id ? `player-tab-${tab.id}` : undefined}
                    excludeFromTabOrder={tab.id !== tabStop}
                    onPress={() => onOpenTab?.(tab.id === open?.id ? null : tab.id)}
                  >
                    {tab.label}
                  </AriaButton>
                </span>
              ))}
            </div>
            {open && (
              <section
                key={open.id}
                id={`player-tab-${open.id}`}
                className={styles.tabPanel}
                aria-label={open.label}
                data-tab-panel
              >
                {open.cards && <CardRow label={open.label} cards={open.cards} />}
                {open.content !== undefined && (
                  // Text that scrolls takes the focus (axe «scrollable-region-focusable»); the
                  // arrows scroll it and a remote reaches it.
                  // eslint-disable-next-line jsx-a11y/no-noninteractive-tabindex
                  <div className={styles.tabContent} tabIndex={0} data-tab-content>
                    {open.content}
                  </div>
                )}
              </section>
            )}
          </div>
        )}
      </div>
    </div>
  );
}
