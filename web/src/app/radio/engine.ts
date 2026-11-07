import { browserPlatform, hlsEngine } from '@/features/player/capabilities';

// The playback engine of the radio player: hls.js (light build, its own chunk) where the
// classic MediaSource exists, the platform's own HLS player on the iPhone (ADR 0004). The relay's
// playlist is a live window of fMP4 audio segments with program date-times (contract 1.17.0).

export interface RadioEngine {
  /** Program date-time of the playback position, null while the stream has none. */
  playingDate(): Date | null;
  /** Jumps to the live edge (the «Live» button). */
  toLive(): void;
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

/** The live edge of the element's seekable range, for the native player. */
function liveEdge(audio: HTMLAudioElement): number | null {
  const { seekable } = audio;
  return seekable.length > 0 ? seekable.end(seekable.length - 1) : null;
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
    toLive: () => {
      const edge = liveEdge(audio);
      if (edge !== null) audio.currentTime = edge;
    },
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
  const hls = new Hls({ liveSyncDurationCount: 2, lowLatencyMode: false, maxBufferLength: 20 });
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
    toLive: () => {
      const position = hls.liveSyncPosition;
      if (position !== null) audio.currentTime = position;
    },
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
