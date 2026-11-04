import type { Chapter, Track } from '@slouchcat/api-client';
import { useCallback, useEffect, useRef, useState, type RefObject } from 'react';
import { useTranslation } from 'react-i18next';
import { nextChapterStart, previousChapterStart } from '@/features/player/audio';
import { stepPlaybackRate } from '@/features/player/keymap';
import { chapterAt, languageName, trackLabel } from '@/features/player/labels';
import { usePlayerLayout, volumeIsReadOnly } from '@/features/player/layout';
import { SettingsPanel } from '@/features/player/panels/SettingsPanel';
import { PANEL_IDS } from '@/features/player/panels/panelIds';
import { usePlayerPanels } from '@/features/player/panels/usePlayerPanels';
import { AudioPanel, SubtitlePanel } from '@/features/player/panels/TrackPanels';
import { SHORTCUT_ROWS, ShortcutHelp } from '@/features/player/ShortcutHelp';
import { SubtitleLayer, useSubtitleCues } from '@/features/player/SubtitleLayer';
import { usePlayerTabs } from '@/features/player/tabs/playerTabs';
import { useBoostedVolume } from '@/features/player/useBoostedVolume';
import { useChapterChoice } from '@/features/player/useChapterChoice';
import { useControlsVisibility } from '@/features/player/useControlsVisibility';
import { useFullscreen } from '@/features/player/useFullscreen';
import { usePhoneTap } from '@/features/player/usePhoneTap';
import { usePlayerKeys } from '@/features/player/usePlayerKeys';
import { usePlayerPrefs } from '@/features/player/usePlayerPrefs';
import { useSubtitleStyle } from '@/features/player/useSubtitleStyle';
import { AriaButton, focusOnMount, PlayerChrome } from '@/ui/aria';
import styles from '@/features/player/Player.module.css';

/** A subtitle of the download, stored as WebVTT in OPFS and offered through a blob URL. */
export interface StoredSubtitle {
  track: Track;
  url: string;
}

// Module functions: the React compiler does not let a component write to an object it received.
function applyRate(element: HTMLMediaElement, rate: number): void {
  // A new source falls back to the default rate; both keep the speed.
  element.defaultPlaybackRate = rate;
  element.playbackRate = rate;
}

interface OfflineVideoProps {
  /** The video element; OfflinePlayer owns it (engine, watch time, progress events). */
  media: RefObject<HTMLMediaElement | null>;
  /** The blob URL of the fallback without MediaSource; none while hls.js plays. */
  src: string | undefined;
  title: string;
  durationMs: number;
  /** Position on the source timeline. */
  currentMs: number;
  /** The position now (source timeline), also between two `timeupdate` events. */
  positionOf: () => number;
  buffered: readonly (readonly [number, number])[];
  playing: boolean;
  /** Whether the viewer wants the video to play (not what the element reports). */
  wantsPlaying: () => boolean;
  /** The chapters the download carries (manifest). */
  chapters: readonly Chapter[];
  /** The one audio track of the download (manifest). */
  audioTrack: Track | undefined;
  /** The subtitles stored with the download. */
  subtitles: readonly StoredSubtitle[];
  /** The profile's language, first in the panels (11.10). */
  profileLanguage: string | undefined;
  failed: boolean;
  /** The day's watch time is used up (AN-18): nothing plays any more. */
  limitReached: boolean;
  limitMinutes: number | undefined;
  togglePlay: () => void;
  seekTo: (ms: number) => void;
  seekBy: (seconds: number) => void;
  onBack: () => void;
  /** The `data-*` attributes of the player region (engine, download, start and seek times). */
  data: Record<`data-${string}`, string | number | undefined>;
}

/**
 * The controls of a video download (F-15): the same PlayerChrome, key map (usePlayerKeys), panels,
 * subtitle layer and phone layout as the video player, with what a download has. Sound and
 * subtitles show only the tracks in the download and change nothing on a server; the gear has
 * speed, the subtitle style and night mode, no quality, version or stream info; the chapters of
 * the manifest are a tab (on a phone a page of the gear). Picture in picture stays off: after the
 * day's watch time nothing may play on (W5).
 */
export function OfflineVideo({
  media,
  src,
  title,
  durationMs,
  currentMs,
  positionOf,
  buffered,
  playing,
  wantsPlaying,
  chapters,
  audioTrack,
  subtitles,
  profileLanguage,
  failed,
  limitReached,
  limitMinutes,
  togglePlay,
  seekTo,
  seekBy,
  onBack,
  data,
}: OfflineVideoProps) {
  const { t, i18n } = useTranslation();
  const language = i18n.language;
  const video = media as RefObject<HTMLVideoElement | null>;
  const {
    panel,
    openPanel,
    togglePanel,
    closePanel,
    settingsView,
    setSettingsView,
    openSettings,
    panelButton,
  } = usePlayerPanels();
  // The chapter tab is open; C counts `focusChapter` up, so the chapter playing takes the focus.
  const [tab, setTab] = useState<{ focusChapter?: number } | null>(null);
  const layout = usePlayerLayout();
  const phone = layout === 'phone';
  const [prefs, setPrefs] = usePlayerPrefs();
  const [subtitleStyle, setSubtitleStyle] = useSubtitleStyle();
  const subtitleTrackElement = useRef<HTMLTrackElement>(null);
  const [muteOnly] = useState(() => volumeIsReadOnly());
  const available = !limitReached;

  // ---------- controls, full screen, volume and night mode: as online ----------

  const { idle, wake: poke, hide, osd, showOsd } = useControlsVisibility(playing);
  const { fullscreen, toggleFullscreen } = useFullscreen({ exitOnUnmount: true });
  const { volume, webAudio, nightMode, setNightMode, changeVolume, toggleMute } = useBoostedVolume(
    media,
    { showOsd },
  );

  const [rate, setRate] = useState(1);
  useEffect(() => {
    if (media.current) applyRate(media.current, rate);
  }, [media, rate]);
  const changeRate = useCallback(
    (delta: number) => {
      const next = stepPlaybackRate(media.current?.playbackRate ?? 1, delta);
      setRate(next);
      const shown = new Intl.NumberFormat(language, { maximumFractionDigits: 2 }).format(next);
      showOsd(t('player.chrome.rate', { rate: shown }));
    },
    [language, media, setRate, showOsd, t],
  );

  // ---------- chapters of the manifest ----------

  const hasChapters = chapters.length > 0;
  const previousChapter = useCallback(() => {
    if (chapters.length > 0) seekTo(previousChapterStart(chapters, positionOf()));
  }, [chapters, positionOf, seekTo]);
  const nextChapter = useCallback(() => {
    const target = nextChapterStart(chapters, positionOf());
    if (target !== null) seekTo(target);
  }, [chapters, positionOf, seekTo]);
  const chapterIndex = chapterAt(chapters, currentMs);
  const chapter = chapters[chapterIndex];
  const chapterName = chapter
    ? (chapter.title ?? t('player.chapters.untitled', { n: chapterIndex + 1 }))
    : undefined;
  // On a phone the chapters are a page of the gear (11.2).
  const chapterChoice = useChapterChoice({
    chapters,
    chapterIndex,
    durationMs,
    seekTo,
    closePanel,
  });

  // ---------- subtitles: the stored WebVTT files, drawn by the subtitle layer ----------

  // The download's default subtitle until the viewer picks one; null is «Aus».
  const [picked, setPicked] = useState<number | null | undefined>(undefined);
  const subtitleId =
    picked === undefined
      ? (subtitles.find((entry) => entry.track.default)?.track.id ?? null)
      : picked;
  const subtitle = subtitles.find((entry) => entry.track.id === subtitleId);
  const cueTexts = useSubtitleCues({
    video,
    track: subtitleTrackElement,
    src: subtitle?.url,
    offsetMs: subtitleStyle.offsetMs,
    pictureInPicture: false,
  });

  // ---------- keys, taps and tabs ----------

  // Faded out: no input while playing, nothing open that needs the controls.
  const hidden = idle && playing && panel === null && tab === null;
  const hasSubtitles = subtitles.length > 0;
  usePlayerKeys({
    hidden,
    durationMs,
    onKey: poke,
    actions: {
      togglePlay,
      seekBy,
      seekTo,
      changeVolume,
      changeRate,
      toggleMute,
      toggleFullscreen,
      previousChapter: hasChapters ? previousChapter : undefined,
      nextChapter: hasChapters ? nextChapter : undefined,
      // T and U: only the tracks the download carries.
      openTracks:
        available && (audioTrack !== undefined || hasSubtitles)
          ? (kind) => {
              if (kind === 'audio' ? audioTrack !== undefined : hasSubtitles) openPanel(kind);
            }
          : undefined,
      // C: the chapter tab with the focus on the chapter playing; on a phone the gear's page.
      openChapters:
        hasChapters && available
          ? phone
            ? () => openSettings('chapters', true)
            : () => setTab((current) => ({ focusChapter: (current?.focusChapter ?? 0) + 1 }))
          : undefined,
      toggleHelp: () => togglePanel('help'),
      closeOpen: () => {
        if (panel !== null) closePanel();
        else if (tab !== null) setTab(null);
        else return false;
        return true;
      },
      back: onBack,
    },
  });

  // The pointer on the picture: a mouse plays, pauses and goes full screen; a finger in the phone
  // layout shows or hides the controls, and a double tap seeks 10 s (V-13).
  const pictureTaps = usePhoneTap({
    phone,
    hidden,
    togglePlay,
    toggleFullscreen,
    seekBy,
    showOsd,
    wantsPlaying,
    hide,
  });

  // Only the chapter tab: a download carries no season, cast or description.
  const allTabs = usePlayerTabs({
    t,
    chapters,
    positionMs: currentMs,
    durationMs,
    onChapter: seekTo,
    focusChapter: tab?.focusChapter,
    currentId: '',
    onEpisode: () => undefined,
    onClose: () => setTab(null),
    onPerson: () => undefined,
  });
  const tabs = phone ? [] : allTabs;

  // The help lists only what this player does: no quality, stream info, intro or next episode.
  const helpRows = SHORTCUT_ROWS.filter((row) => {
    switch (row) {
      case 'quality':
      case 'info':
      case 'skip':
      case 'next':
        return false;
      case 'chapter':
      case 'chapters':
        return hasChapters;
      case 'tracks':
        return audioTrack !== undefined;
      case 'subtitles':
        return hasSubtitles;
      default:
        return true;
    }
  });

  const audioWord = audioTrack
    ? (languageName(audioTrack.language, language) ?? trackLabel(t, audioTrack, 0, language))
    : '';
  const subtitleWord = subtitle
    ? (languageName(subtitle.track.language, language) ??
      trackLabel(t, subtitle.track, subtitles.indexOf(subtitle), language))
    : t('player.tracks.off');
  const lineLabel = chapterName
    ? t('player.chrome.lineChapter', { title, chapter: chapterName })
    : title;

  return (
    <main
      className={styles.player}
      aria-label={t('player.region')}
      {...data}
      data-controls={hidden ? 'hidden' : 'visible'}
      onPointerMove={poke}
      onPointerDown={poke}
      onFocus={poke}
    >
      {/* The text tracks are the stored WebVTT files; a download without any has none. The track
          stays hidden: the subtitle layer draws its cues (11.10). */}
      {/* eslint-disable-next-line jsx-a11y/media-has-caption */}
      <video
        ref={video}
        className={styles.video}
        src={src}
        preload="auto"
        playsInline
        disableRemotePlayback
        disablePictureInPicture
        {...pictureTaps}
      >
        {subtitle && (
          <track
            key={subtitle.track.id}
            ref={subtitleTrackElement}
            kind="subtitles"
            src={subtitle.url}
            srcLang={subtitle.track.language}
            label={subtitle.track.title ?? languageName(subtitle.track.language, language) ?? ''}
          />
        )}
      </video>

      <SubtitleLayer texts={cueTexts} style={subtitleStyle} raised={!hidden} />

      {failed && !limitReached && (
        <div className={styles.center} role="alert">
          <p className={styles.failTitle}>{t('player.error.title')}</p>
          <p>{t('offline.player.failed')}</p>
        </div>
      )}

      {limitReached && (
        <div className={styles.center} role="alert">
          <p className={styles.failTitle}>{t('player.watchLimit.title')}</p>
          <p>
            {limitMinutes !== undefined
              ? t('player.watchLimit.body', { count: limitMinutes })
              : t('player.watchLimit.bodyUnknown')}
          </p>
          <div>
            {/* focusOnMount, not react-aria's autoFocus, which can wait 500 ms for a transition (11.13). */}
            <AriaButton ref={focusOnMount} variant="secondary" icon="back" onPress={onBack}>
              {t('common.back')}
            </AriaButton>
          </div>
        </div>
      )}

      {osd && (
        <div className={styles.osd} role="status">
          {osd}
        </div>
      )}

      <PlayerChrome
        layout={layout}
        title={title}
        lineLabel={lineLabel}
        onBack={onBack}
        hidden={hidden}
        currentMs={currentMs}
        durationMs={durationMs}
        buffered={buffered}
        rate={rate}
        timeDisplay={prefs.timeDisplay ?? 'elapsed'}
        onTimeDisplayChange={(display) => setPrefs({ timeDisplay: display })}
        chapters={chapters}
        onSeek={seekTo}
        onPreviousChapter={hasChapters ? previousChapter : undefined}
        onNextChapter={hasChapters ? nextChapter : undefined}
        playing={playing}
        onTogglePlay={togglePlay}
        onSkip={seekBy}
        // The first focus goes to play/pause (focusOnMount, not react-aria's autoFocus: 11.13).
        playButtonRef={focusOnMount}
        previous={
          hasChapters
            ? { label: t('player.chrome.previousChapter'), onPress: previousChapter }
            : undefined
        }
        next={
          nextChapterStart(chapters, currentMs) !== null
            ? { label: t('player.chrome.nextChapter'), onPress: nextChapter }
            : undefined
        }
        transportDisabled={limitReached}
        volume={volume.volume}
        muted={volume.muted}
        onVolumeChange={volume.setVolume}
        onToggleMute={volume.toggleMute}
        volumeBoost={webAudio}
        muteOnly={muteOnly}
        fullscreen={fullscreen}
        onToggleFullscreen={toggleFullscreen}
        subtitles={
          hasSubtitles
            ? panelButton(
                'subtitle',
                t('player.panel.subtitleButton', { language: subtitleWord }),
                subtitleWord,
                limitReached,
              )
            : undefined
        }
        audio={
          audioTrack
            ? panelButton(
                'audio',
                t('player.panel.audioButton', { language: audioWord }),
                audioWord,
                limitReached,
              )
            : undefined
        }
        settings={{
          label: t('player.settings.title'),
          expanded: panel === 'settings',
          controls: PANEL_IDS.settings,
          onPress: () => (panel === 'settings' ? closePanel() : openSettings('main', false)),
          isDisabled: limitReached,
        }}
        tabs={tabs}
        openTab={tab ? 'chapters' : null}
        onOpenTab={(id) => setTab(id === null ? null : {})}
        helpOpen={panel === 'help'}
        onToggleHelp={() => togglePanel('help')}
      />
      {panel === 'help' && <ShortcutHelp onClose={closePanel} rows={helpRows} />}
      {panel === 'settings' && (
        <SettingsPanel
          page={settingsView.page}
          direct={settingsView.direct}
          onPage={(page) => setSettingsView({ page, direct: false })}
          onClose={closePanel}
          chapters={phone && hasChapters ? chapterChoice : undefined}
          rate={rate}
          onRate={setRate}
          subtitleStyle={{ value: subtitleStyle, onChange: setSubtitleStyle }}
          nightMode={webAudio ? { on: nightMode, onChange: setNightMode } : undefined}
        />
      )}
      {panel === 'audio' && audioTrack && (
        <AudioPanel
          tracks={[audioTrack]}
          profileLanguage={profileLanguage}
          selectedIndex={audioTrack.index}
          converts={() => false}
          // The download has this one track: choosing it changes nothing.
          onSelect={closePanel}
          onClose={closePanel}
        />
      )}
      {panel === 'subtitle' && (
        <SubtitlePanel
          tracks={subtitles.map((entry) => entry.track)}
          profileLanguage={profileLanguage}
          selectedId={subtitle?.track.id}
          // Local: the stored files, no new session (rule 12: the server chose them at download).
          // The panel closes with the choice, as online, where a choice starts a new session.
          onSelect={(id) => {
            setPicked(id);
            closePanel();
          }}
          onClose={closePanel}
          subtitleStyle={{ value: subtitleStyle, onChange: setSubtitleStyle }}
        />
      )}
    </main>
  );
}
