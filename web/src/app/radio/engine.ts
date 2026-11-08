import { browserPlatform, hlsEngine } from '@/features/player/capabilities';

// The playback engine of the radio player: hls.js (light build, its own chunk) where the
// classic MediaSource exists, the platform's own HLS player on the iPhone (ADR 0004). The relay's
// playlist is a live window of fMP4 audio segments with program date-times (contract 1.17.0);
// since contract 1.19.0 the listener plays up to `RadioListen.timeShiftSeconds` behind live in it
// (concept 16, 7.1 point 7).

/** Length of one segment of the relay: closer to live than this counts as «Live» (critic m-7). */
export const RADIO_SEGMENT_SECONDS = 4;

export interface RadioEngine {
  /** Program date-time of the playback position, null while the stream has none. */
  playingDate(): Date | null;
  /** Jumps to the live edge (the «Live» button). */
  toLive(): void;
  /** Plays `offset` seconds behind live, at most from the oldest position of the window. */
  seekTo(offset: number): void;
  /** Seconds the playback position lies behind live (0 at the live position). */
  offsetSeconds(): number;
  /** Seconds of the window behind the live position the element can seek to now. */
  seekableSeconds(): number;
  /**
   * Called before the element plays on after a pause: when the window slid past the paused
   * position, it jumps one segment after the oldest position and answers true (critic M-5).
   */
  resumeAt(): boolean;
  destroy(): void;
}

export interface RadioEngineEvents {
  /** The stream cannot be played (after one recovery attempt). */
  onFatal(): void;
  /** The server answered 401: the listen session ended. */
  onRevoked(): void;
}

/** Attaches a stream to the audio element; the provider passes a fake one in tests. */
export type RadioEngineFactory = (
  audio: HTMLAudioElement,
  url: string,
  events: RadioEngineEvents,
) => Promise<RadioEngine>;

/** Why no engine was attached: the browser plays no HLS at all. */
export class RadioUnsupported extends Error {
  constructor() {
    super('this browser cannot play HLS');
    this.name = 'RadioUnsupported';
  }
}

/** The platform player stays this far behind the end of its seekable range (`EXT-X-START`). */
const NATIVE_LIVE_DELAY_SECONDS = 8;

type TimeShift = Pick<
  RadioEngine,
  'toLive' | 'seekTo' | 'offsetSeconds' | 'seekableSeconds' | 'resumeAt'
>;

/**
 * The time shift of an element whose live position `live` names (hls.js: `liveSyncPosition`;
 * the platform player: the end of the seekable range less its delay); null while it is unknown.
 */
export function timeShiftOf(
  audio: Pick<HTMLMediaElement, 'currentTime' | 'seekable'>,
  live: () => number | null,
): TimeShift {
  const oldest = () => (audio.seekable.length > 0 ? audio.seekable.start(0) : null);
  const seekTo = (offset: number) => {
    const position = live();
    if (position === null) return;
    const target = position - Math.max(0, offset);
    audio.currentTime = Math.max(oldest() ?? target, target);
  };
  return {
    toLive: () => seekTo(0),
    seekTo,
    offsetSeconds: () => {
      const position = live();
      return position === null ? 0 : Math.max(0, position - audio.currentTime);
    },
    seekableSeconds: () => {
      const position = live();
      const start = oldest();
      return position === null || start === null ? 0 : Math.max(0, position - start);
    },
    resumeAt: () => {
      const start = oldest();
      if (start === null || audio.currentTime >= start) return false;
      audio.currentTime = start + RADIO_SEGMENT_SECONDS;
      return true;
    },
  };
}

/** The live position of the platform player: the end of its seekable range less its delay. */
function nativeLive(audio: HTMLAudioElement): number | null {
  const { seekable } = audio;
  if (seekable.length === 0) return null;
  return Math.max(seekable.start(0), seekable.end(seekable.length - 1) - NATIVE_LIVE_DELAY_SECONDS);
}

function nativeEngine(
  audio: HTMLAudioElement,
  url: string,
  events: RadioEngineEvents,
): RadioEngine {
  const onError = () => events.onFatal();
  audio.addEventListener('error', onError);
  audio.src = url;
  return {
    playingDate: () => {
      // Safari exposes the program date-time of the playlist start (EXT-X-PROGRAM-DATE-TIME).
      const start = (audio as { getStartDate?: () => Date }).getStartDate?.();
      const ms = start?.getTime();
      return ms === undefined || Number.isNaN(ms) ? null : new Date(ms + audio.currentTime * 1000);
    },
    ...timeShiftOf(audio, () => nativeLive(audio)),
    destroy: () => {
      audio.removeEventListener('error', onError);
      audio.removeAttribute('src');
      audio.load();
    },
  };
}

async function hlsJsEngine(
  audio: HTMLAudioElement,
  url: string,
  events: RadioEngineEvents,
): Promise<RadioEngine> {
  const { default: Hls } = await import('hls.js/light');
  // Two segments behind the edge as the playlist's start offset says; a small forward buffer, so
  // a pause does not fetch much that is not heard (listen time counts delivered segments, M-2).
  // The back buffer keeps 90 s: a seek further back fetches its segments again (m-6).
  const hls = new Hls({
    liveSyncDurationCount: 2,
    lowLatencyMode: false,
    maxBufferLength: 20,
    backBufferLength: 90,
  });
  let mediaRecovered = false;
  let networkRecovered = false;
  hls.on(Hls.Events.ERROR, (_event, data) => {
    if (data.response?.code === 401) {
      events.onRevoked();
      return;
    }
    if (!data.fatal) return;
    if (data.type === Hls.ErrorTypes.MEDIA_ERROR && !mediaRecovered) {
      mediaRecovered = true;
      hls.recoverMediaError();
    } else if (data.type === Hls.ErrorTypes.NETWORK_ERROR && !networkRecovered) {
      networkRecovered = true;
      hls.startLoad();
    } else {
      events.onFatal();
    }
  });
  hls.loadSource(url);
  hls.attachMedia(audio);
  return {
    playingDate: () => hls.playingDate,
    ...timeShiftOf(audio, () => hls.liveSyncPosition),
    destroy: () => hls.destroy(),
  };
}

/** The engine this browser plays HLS with; rejects with RadioUnsupported without one. */
export const createRadioEngine: RadioEngineFactory = async (audio, url, events) => {
  switch (hlsEngine(browserPlatform())) {
    case 'native':
      return nativeEngine(audio, url, events);
    case 'hlsjs':
      return hlsJsEngine(audio, url, events);
    default:
      throw new RadioUnsupported();
  }
};
