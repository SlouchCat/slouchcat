import { useCallback, useEffect, useRef, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { useAudioControls } from '@/features/player/useAudioControls';
import { AriaButton, AudioPlayer, focusOnMount } from '@/ui/aria';
import { offlineLoader } from './loader';
import { OfflineVideo, type StoredSubtitle } from './OfflineVideo';
import { readFile, subtitleFile } from './opfs';
import { recordPlayback, storeRate, type OfflinePlayback, type PlaybackSample } from './playback';
import { buildPlaylist } from './playlist';
import type { DownloadManifest, StoredWatchTime } from './types';
import { remainingMs } from './watchTime';

// The player of a download (design 4): hls.js plays a VOD playlist built from the manifest
// through a loader that reads the byte ranges from OPFS (spike decision AN-20), so seeking needs
// no network and no search through the fragments. Without MediaSource the whole file plays from a
// blob URL. A video download has the controls of the video player (OfflineVideo, F-15); its
// subtitles are the stored WebVTT files, chosen in a local panel. Progress
// goes into the event queue (O-5); the offline watch time stops playback at 0 (AN-18), also
// when the device is online: this player knows no server stop (decision of 2026-10-20, K12).
// An audio download (manifest `audio/*`, M6) plays in an <audio> element behind the AudioPlayer,
// with the chapters of the manifest and its speed kept per profile on the download.

/** Interval of `playing` events, as online (useProgressReporter). */
export const OFFLINE_PROGRESS_MS = 10_000;
const RESUME_TAIL_MS = 5000;
const WATCH_CHECK_MS = 1000;

export type OfflineEngine = 'mse' | 'blob';

interface OfflinePlayerProps {
  title: string;
  manifest: DownloadManifest;
  media: File;
  playback: OfflinePlayback;
  /** Stored subtitle track ids (StoredDownload.subtitlesStored). */
  subtitleIds: readonly number[];
  /** Position to resume from (source timeline). */
  resumeMs: number;
  /** Watch time of the profile; undefined without a daily limit. */
  watch: StoredWatchTime | undefined;
  /** Server time minus device time of the last sync. */
  serverOffsetMs: number;
  onBack: () => void;
  /** Where `onBack` leads: the downloads (opened there) or the offline library; the help says it. */
  backTo: 'downloads' | 'library';
  /** The profile's language: its tracks come first in the panels (11.10). */
  profileLanguage?: string;
  /** Audio downloads: the stored cover, the author or podcast, the title's accent. */
  coverUrl?: string;
  byline?: string;
  accent?: string;
  /** Audio downloads: the profile's speed to start with (resumeRate). */
  initialRate?: number;
}

/**
 * Seconds the file's timeline is ahead of the source timeline in the blob fallback: download
 * fragments keep the HLS timeline (stream A: media time = source time + 10 s). hls.js maps them
 * back itself; a media element playing the file directly starts there.
 */
export function timelineOffset(
  video: Pick<HTMLVideoElement, 'buffered' | 'seekable' | 'duration'>,
  durationMs: number,
): number {
  const buffered = video.buffered.length > 0 ? video.buffered.start(0) : 0;
  const seekable = video.seekable.length > 0 ? video.seekable.start(0) : 0;
  const start = Math.max(buffered, seekable);
  if (start > 0.25) return start;
  const extra = Number.isFinite(video.duration) ? video.duration - durationMs / 1000 : 0;
  return extra > 0.5 ? extra : 0;
}

/** The media session (notification, media keys) offers no play: the limit is reached (W5). */
function blockMediaSession(): void {
  if (typeof navigator === 'undefined' || !('mediaSession' in navigator)) return;
  try {
    navigator.mediaSession.playbackState = 'paused';
    navigator.mediaSession.setActionHandler('play', () => undefined);
  } catch {
    // A browser without the play action: the play listener pauses again.
  }
}

function releaseMediaSession(): void {
  if (typeof navigator === 'undefined' || !('mediaSession' in navigator)) return;
  try {
    navigator.mediaSession.setActionHandler('play', null);
  } catch {
    // Nothing to give back.
  }
}

export function OfflinePlayer({
  title,
  manifest,
  media,
  playback,
  subtitleIds,
  resumeMs,
  watch,
  serverOffsetMs,
  onBack,
  backTo,
  profileLanguage,
  coverUrl,
  byline,
  accent,
  initialRate = 1,
}: OfflinePlayerProps) {
  const { t } = useTranslation();
  const audio = manifest.mimeType.startsWith('audio/');
  // The <video> element, or the <audio> element of an audio download.
  const video = useRef<HTMLMediaElement>(null);
  const [openedAt] = useState(() => performance.now());
  const [engine, setEngine] = useState<OfflineEngine | null>(null);
  const [blobUrl, setBlobUrl] = useState<string | null>(null);
  const [playing, setPlaying] = useState(false);
  const [currentMs, setCurrentMs] = useState(resumeMs);
  const durationMs = manifest.durationMs;
  const [buffered, setBuffered] = useState<[number, number][]>([]);
  const [startMs, setStartMs] = useState<number | null>(null);
  const [seek, setSeek] = useState<{ ms: number; count: number } | null>(null);
  const [failed, setFailed] = useState(false);
  // Used up before the start: the player never plays (AN-18).
  const [usedUp] = useState(
    () => watch !== undefined && remainingMs(watch, Date.now() + serverOffsetMs) <= 0,
  );
  const [limitReached, setLimitReached] = useState(usedUp);
  const [subtitles, setSubtitles] = useState<StoredSubtitle[]>([]);
  // Seconds the element's timeline is ahead of the source (blob fallback only).
  const offset = useRef(0);
  const watchRef = useRef(watch);
  const locked = useRef(usedUp);
  // hls.js of the MSE path, to stop and detach it at the limit.
  const hlsRef = useRef<{ stopLoad: () => void; detachMedia: () => void } | null>(null);
  // Source position (s) kept once the media is detached (limit, unmount): currentTime is 0 then.
  const frozenAt = useRef<number | null>(null);

  // ---------- watch time (AN-18) ----------

  const playedSince = useRef<number | null>(null);
  const unrecordedMs = useRef(0);
  const stopClock = useCallback(() => {
    if (playedSince.current !== null) {
      unrecordedMs.current += performance.now() - playedSince.current;
      playedSince.current = null;
    }
  }, []);
  /** Milliseconds the profile may still play: the stored rest minus what is not booked yet. */
  const remaining = useCallback(() => {
    const current = watchRef.current;
    if (!current) return Infinity;
    const running = playedSince.current === null ? 0 : performance.now() - playedSince.current;
    return remainingMs(current, Date.now() + serverOffsetMs) - unrecordedMs.current - running;
  }, [serverOffsetMs]);

  // ---------- progress events (O-5) ----------

  const seq = useRef(1);
  const sendEvent = useCallback(
    // `target`: the element the caller captured; while the player unmounts the ref is null (W6).
    (state: PlaybackSample['state'], target?: HTMLMediaElement) => {
      const element = target ?? video.current;
      if (!element || startMs === null) return;
      const running = playedSince.current !== null;
      stopClock();
      const played = Math.round(unrecordedMs.current);
      unrecordedMs.current = 0;
      if (running) playedSince.current = performance.now();
      const position = frozenAt.current ?? element.currentTime - offset.current;
      const sample: PlaybackSample = {
        positionMs: Math.max(0, Math.round(position * 1000)),
        durationMs: Math.max(1, durationMs),
        playedMs: played,
        seq: seq.current,
        state,
        ...(audio ? { playbackRate: Math.round(element.playbackRate * 100) / 100 } : {}),
      };
      seq.current += 1;
      void recordPlayback(playback, sample).then(
        (next) => {
          if (next) watchRef.current = next;
        },
        () => undefined,
      );
    },
    [audio, durationMs, playback, startMs, stopClock],
  );
  const sendLater = useRef(sendEvent);
  useEffect(() => {
    sendLater.current = sendEvent;
  });

  /**
   * After the limit nothing may start the video again (media keys, the media notification, the
   * context menu, picture-in-picture, W5): hls.js stops and lets go of the element, the blob
   * fallback loses its source, and the media session offers no play.
   */
  const release = useCallback((element: HTMLMediaElement | null) => {
    if (element && frozenAt.current === null) {
      frozenAt.current = element.currentTime - offset.current;
    }
    if (hlsRef.current) {
      hlsRef.current.stopLoad();
      hlsRef.current.detachMedia();
    } else if (element?.getAttribute('src')) {
      element.removeAttribute('src');
      element.load();
    }
    blockMediaSession();
  }, []);

  /** The day's watch time is used up: stop for good, with the hint. */
  const stopForLimit = useCallback(() => {
    const element = video.current;
    locked.current = true;
    setLimitReached(true);
    // A playing video is released after its pause event booked the position (onPause).
    if (element && !element.paused) element.pause();
    else release(element);
  }, [release]);

  useEffect(() => {
    const timer = window.setInterval(() => {
      if (!locked.current && remaining() <= 0) stopForLimit();
    }, WATCH_CHECK_MS);
    return () => window.clearInterval(timer);
  }, [remaining, stopForLimit]);

  // ---------- engine: hls.js over the OPFS loader, else a blob URL ----------

  useEffect(() => {
    const element = video.current;
    if (!element) return;
    let cancelled = false;
    let destroy = () => {};
    import('hls.js/light')
      .then(({ default: Hls }) => {
        if (cancelled) return;
        if (!Hls.isSupported()) {
          const url = URL.createObjectURL(media);
          destroy = () => URL.revokeObjectURL(url);
          setBlobUrl(url);
          setEngine('blob');
          return;
        }
        const resume = resumeMs > 0 && resumeMs < durationMs - RESUME_TAIL_MS ? resumeMs / 1000 : 0;
        const hls = new Hls({
          loader: offlineLoader({ playlist: buildPlaylist(manifest), media }),
          startPosition: resume,
        });
        let recovered = false;
        hls.on(Hls.Events.ERROR, (_event, data) => {
          if (!data.fatal) return;
          if (data.type === Hls.ErrorTypes.MEDIA_ERROR && !recovered) {
            recovered = true;
            hls.recoverMediaError();
          } else {
            setFailed(true);
          }
        });
        // The URL only names the playlist for hls.js; the loader never fetches it.
        hls.loadSource(`${window.location.origin}/offline/media/${playback.downloadId}/index.m3u8`);
        hls.attachMedia(element);
        hlsRef.current = hls;
        destroy = () => {
          hlsRef.current = null;
          hls.destroy();
        };
        setEngine('mse');
      })
      .catch(() => setFailed(true));
    return () => {
      cancelled = true;
      // The last progress event (unmount) needs the position the element loses with hls.js.
      if (frozenAt.current === null) frozenAt.current = element.currentTime - offset.current;
      destroy();
    };
  }, [durationMs, manifest, media, playback.downloadId, resumeMs]);

  // ---------- subtitles from OPFS ----------

  useEffect(() => {
    let active = true;
    const urls: string[] = [];
    void (async () => {
      const list: StoredSubtitle[] = [];
      for (const entry of manifest.subtitles) {
        if (!subtitleIds.includes(entry.track.id)) continue;
        const file = await readFile(playback.downloadId, subtitleFile(entry.track.id)).catch(
          () => null,
        );
        if (!file) continue;
        const url = URL.createObjectURL(file);
        urls.push(url);
        list.push({ track: entry.track, url });
      }
      if (active) setSubtitles(list);
    })();
    return () => {
      active = false;
      for (const url of urls) URL.revokeObjectURL(url);
    };
  }, [manifest.subtitles, playback.downloadId, subtitleIds]);

  // ---------- controls ----------

  // What the listener wants, not what the element reports: re-attaching the element (hls.js after
  // a media error) runs the load algorithm, which clears `paused` without a `pause` event. The
  // element would then read as paused while the player still shows "Pause", and the reloaded
  // source would play on even after a pause.
  const wantPlaying = useRef(true);

  const togglePlay = useCallback(() => {
    const element = video.current;
    if (!element || locked.current) return;
    wantPlaying.current = !wantPlaying.current;
    if (wantPlaying.current) element.play().catch(() => setPlaying(false));
    else {
      setPlaying(false);
      element.pause();
    }
  }, []);
  const seekTo = useCallback((ms: number) => {
    const element = video.current;
    if (!element || locked.current) return;
    const end = Number.isFinite(element.duration) ? element.duration : Infinity;
    element.currentTime = Math.min(end, Math.max(offset.current, ms / 1000 + offset.current));
    setCurrentMs((element.currentTime - offset.current) * 1000);
  }, []);
  const seekBy = useCallback(
    (seconds: number) => {
      const element = video.current;
      if (element) seekTo((element.currentTime - offset.current + seconds) * 1000);
    },
    [seekTo],
  );
  /** The position now on the source timeline; kept once the media is detached (limit, unmount). */
  const positionOf = useCallback(
    () =>
      Math.max(0, (frozenAt.current ?? (video.current?.currentTime ?? 0) - offset.current) * 1000),
    [],
  );

  // Audio downloads have the audio player's keys; a video download has the video player's
  // (OfflineVideo).
  const keepRate = useCallback(
    (rate: number) => {
      void storeRate(playback.downloadId, playback.profileId, rate).catch(() => undefined);
    },
    [playback.downloadId, playback.profileId],
  );
  const audioControls = useAudioControls({
    media: video,
    chapters: manifest.chapters ?? [],
    durationMs,
    positionMs: currentMs,
    positionOf,
    initialRate,
    active: !limitReached,
    enabled: audio,
    seekTo,
    togglePlay,
    onBack,
    onRateChange: keepRate,
  });
  // ---------- media element events ----------

  useEffect(() => {
    const element = video.current;
    if (!element || engine === null) return;
    let seekStarted: number | null = null;
    let seeked = false;
    const finishSeek = () => {
      if (seekStarted === null) return;
      const ms = Math.round(performance.now() - seekStarted);
      seekStarted = null;
      setSeek((value) => ({ ms, count: (value?.count ?? 0) + 1 }));
    };
    // Blob fallback: the cues are on the source timeline, the file's timeline is ahead. A track
    // loads its cues when it is switched on, so every load shifts what is not shifted yet.
    const shiftCues = () => {
      if (engine !== 'blob') return;
      const shift = offset.current;
      for (const node of element.querySelectorAll('track')) {
        // Every browser has `track`; a DOM without text tracks (tests) has none.
        const cues = (node.track as TextTrack | undefined)?.cues;
        const applied = Number(node.dataset.shift ?? 0);
        if (!cues || cues.length === 0 || applied === shift) continue;
        // The list stays sorted by start time and re-sorts while a cue moves: walking it would
        // shift some cues twice and others not at all (review step 6). A copy keeps the order.
        for (const cue of Array.from(cues)) {
          cue.startTime += shift - applied;
          cue.endTime += shift - applied;
        }
        node.dataset.shift = String(shift);
      }
    };
    const onLoadedMetadata = () => {
      if (engine === 'blob') {
        offset.current = timelineOffset(element, durationMs);
        const resume = resumeMs > 0 && resumeMs < durationMs - RESUME_TAIL_MS ? resumeMs / 1000 : 0;
        element.currentTime = offset.current + resume;
        shiftCues();
      }
      if (!locked.current && wantPlaying.current) element.play().catch(() => setPlaying(false));
    };
    const onPlaying = () => {
      wantPlaying.current = true;
      setPlaying(true);
      setStartMs((value) => value ?? Math.round(performance.now() - openedAt));
      playedSince.current ??= performance.now();
      finishSeek();
    };
    const onWaiting = () => stopClock();
    const onSeeking = () => {
      seekStarted ??= performance.now();
      seeked = false;
    };
    const onSeeked = () => {
      seeked = true;
    };
    const onPlay = () => {
      // Nothing outside the player's own buttons (media keys, notification) plays past the limit.
      if (locked.current) element.pause();
    };
    // A pause the element reports (the listener, the sleep timer, the limit) is a wish; the silent
    // one of the load algorithm is not and fires no event.
    const onPause = () => {
      wantPlaying.current = false;
      setPlaying(false);
      if (element.ended) return;
      stopClock();
      sendLater.current('paused', element);
      if (locked.current) release(element);
    };
    const onEnded = () => {
      wantPlaying.current = false;
      setPlaying(false);
      stopClock();
      sendLater.current('ended');
    };
    const onTime = () => {
      setCurrentMs(Math.max(0, (element.currentTime - offset.current) * 1000));
      if (seeked && !element.paused) finishSeek();
    };
    const onProgress = () => {
      const ranges: [number, number][] = [];
      for (let index = 0; index < element.buffered.length; index += 1) {
        ranges.push([
          (element.buffered.start(index) - offset.current) * 1000,
          (element.buffered.end(index) - offset.current) * 1000,
        ]);
      }
      setBuffered(ranges);
    };
    const onError = () => {
      if (engine === 'blob') setFailed(true);
    };
    element.addEventListener('loadedmetadata', onLoadedMetadata);
    element.addEventListener('play', onPlay);
    element.addEventListener('playing', onPlaying);
    element.addEventListener('waiting', onWaiting);
    element.addEventListener('seeking', onSeeking);
    element.addEventListener('seeked', onSeeked);
    element.addEventListener('pause', onPause);
    element.addEventListener('ended', onEnded);
    element.addEventListener('timeupdate', onTime);
    element.addEventListener('progress', onProgress);
    element.addEventListener('error', onError);
    // `load` of a <track> does not bubble: listen in the capture phase.
    element.addEventListener('load', shiftCues, true);
    return () => {
      element.removeEventListener('load', shiftCues, true);
      element.removeEventListener('loadedmetadata', onLoadedMetadata);
      element.removeEventListener('play', onPlay);
      element.removeEventListener('playing', onPlaying);
      element.removeEventListener('waiting', onWaiting);
      element.removeEventListener('seeking', onSeeking);
      element.removeEventListener('seeked', onSeeked);
      element.removeEventListener('pause', onPause);
      element.removeEventListener('ended', onEnded);
      element.removeEventListener('timeupdate', onTime);
      element.removeEventListener('progress', onProgress);
      element.removeEventListener('error', onError);
    };
  }, [durationMs, engine, openedAt, release, resumeMs, stopClock]);

  // Every 10 s while playing, when the page is hidden or left, and when the player closes.
  useEffect(() => {
    const element = video.current;
    if (!element || startMs === null) return;
    const interval = window.setInterval(() => {
      if (!element.paused && !element.ended) sendLater.current('playing', element);
    }, OFFLINE_PROGRESS_MS);
    const onHidden = () => {
      if (document.visibilityState === 'hidden') {
        sendLater.current(element.paused ? 'paused' : 'playing', element);
      }
    };
    const onPageHide = () => sendLater.current('paused', element);
    document.addEventListener('visibilitychange', onHidden);
    window.addEventListener('pagehide', onPageHide);
    return () => {
      window.clearInterval(interval);
      document.removeEventListener('visibilitychange', onHidden);
      window.removeEventListener('pagehide', onPageHide);
      // Leaving the player books what played since the last event (W6).
      if (!element.ended) sendLater.current('paused', element);
    };
  }, [startMs]);

  // The media session offers play again once the player is gone.
  useEffect(() => () => releaseMediaSession(), []);

  const limit = watch?.limitMinutes;

  if (audio) {
    return (
      <AudioPlayer
        {...audioControls}
        title={title}
        byline={byline}
        coverUrl={coverUrl}
        accent={accent}
        chapters={manifest.chapters ?? []}
        currentMs={currentMs}
        durationMs={durationMs}
        playing={playing}
        onTogglePlay={togglePlay}
        onSeek={seekTo}
        onBack={onBack}
        isDisabled={limitReached}
        alert={
          limitReached ? (
            <>
              <p>{t('player.watchLimit.title')}</p>
              <p>
                {limit !== undefined
                  ? t('player.watchLimit.body', { count: limit })
                  : t('player.watchLimit.bodyUnknown')}
              </p>
              <div>
                {/* focusOnMount, not react-aria's autoFocus, which can wait 500 ms for a
                    transition (11.13). */}
                <AriaButton ref={focusOnMount} variant="secondary" icon="back" onPress={onBack}>
                  {t('common.back')}
                </AriaButton>
              </div>
            </>
          ) : failed ? (
            <>
              <p>{t('player.error.title')}</p>
              <p>{t('offline.player.failed')}</p>
            </>
          ) : null
        }
        data-engine={engine ?? undefined}
        data-download-id={playback.downloadId}
        data-profile-id={playback.profileId}
        data-start-ms={startMs ?? undefined}
        data-seek-ms={seek?.ms}
        data-seek-count={seek?.count}
        data-ended-reason={limitReached ? 'limit' : undefined}
        data-rate={audioControls.rate}
      >
        {/* Spoken word without text tracks: nothing to caption. */}
        {/* eslint-disable-next-line jsx-a11y/media-has-caption */}
        <audio
          ref={video}
          src={engine === 'blob' ? (blobUrl ?? undefined) : undefined}
          preload="auto"
        />
      </AudioPlayer>
    );
  }

  return (
    <OfflineVideo
      media={video}
      src={engine === 'blob' ? (blobUrl ?? undefined) : undefined}
      title={title}
      durationMs={durationMs}
      currentMs={currentMs}
      positionOf={positionOf}
      buffered={buffered}
      playing={playing}
      wantsPlaying={() => wantPlaying.current}
      chapters={manifest.chapters ?? []}
      audioTrack={manifest.audio}
      subtitles={subtitles}
      profileLanguage={profileLanguage}
      failed={failed}
      limitReached={limitReached}
      limitMinutes={limit}
      togglePlay={togglePlay}
      seekTo={seekTo}
      seekBy={seekBy}
      onBack={onBack}
      backTo={backTo}
      data={{
        'data-engine': engine ?? undefined,
        'data-download-id': playback.downloadId,
        'data-profile-id': playback.profileId,
        'data-start-ms': startMs ?? undefined,
        'data-seek-ms': seek?.ms,
        'data-seek-count': seek?.count,
        'data-ended-reason': limitReached ? 'limit' : undefined,
      }}
    />
  );
}
