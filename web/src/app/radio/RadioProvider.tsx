import { useQuery } from '@tanstack/react-query';
import { useRouterState } from '@tanstack/react-router';
import {
  useCallback,
  useEffect,
  useMemo,
  useReducer,
  useRef,
  useState,
  type ReactNode,
} from 'react';
import { flushSync } from 'react-dom';
import { apiUrl, isApiError } from '@/api/client';
import { problemExtension } from '@/api/errors';
import { sessionQuery } from '../session';
import { openRadioChannel, type RadioChannel } from './channel';
import { createRadioEngine, type RadioEngine, type RadioEngineFactory } from './engine';
import {
  applyEvent,
  initialRadioState,
  isPlaying,
  radioReducer,
  wantsSound,
  type RadioAction,
  type RadioEvent,
  type RadioNotice,
  type RadioState,
  type RadioStation,
} from './priority';
import {
  currentRadioEntry,
  endRadioListen,
  previousRadioEntries,
  radioNowQuery,
  sendRadioHeartbeat,
  startRadioListen,
} from './queries';
import { RadioContext, type RadioPlayer } from './useRadio';

// The radio player of the whole app (concept 16, 10.3; plan W1). It sits in the route `_authed`
// (R-1), so it outlives the profile picker and every page with or without the app header; the
// offline player lies outside and unmounts it, which stops the radio as table 10.3 wants. One
// <audio> element, in the page only while a station is chosen (a page's own player is the only
// one there otherwise), one listen session at a time. The provider ends the session on the server
// (keepalive DELETE) when the station changes, the radio stops, the provider unmounts or the tab
// is closed; the server expires a session it never hears of again (S6).

/** This tab on the BroadcastChannel: one id per page load. */
const TAB_ID =
  typeof crypto !== 'undefined' && 'randomUUID' in crypto
    ? crypto.randomUUID()
    : `${Date.now()}-${Math.random()}`;

type ProviderAction = RadioAction | { type: 'event'; event: RadioEvent; notice?: RadioNotice };

function reducer(state: RadioState, action: ProviderAction): RadioState {
  return action.type === 'event'
    ? applyEvent(state, action.event, action.notice)
    : radioReducer(state, action);
}

/** The notice for a start the server refused. */
function startNotice(error: unknown): RadioNotice {
  if (isApiError(error, 403) && error.code === 'watch_limit_reached') return limitNotice(error);
  return { kind: 'problem', code: isApiError(error) ? error.code : 'network' };
}

function limitNotice(error: unknown): RadioNotice {
  const limitMinutes = problemExtension(error, 'limitMinutes');
  const usedMinutes = problemExtension(error, 'usedMinutes');
  return {
    kind: 'watchLimit',
    ...(limitMinutes === undefined ? {} : { limitMinutes }),
    ...(usedMinutes === undefined ? {} : { usedMinutes }),
  };
}

/**
 * What a failed heartbeat means: the limit is reached (403), the relay ended (410), the session
 * is gone (401, 404). Null for anything else (a lost connection): the next beat tries again.
 */
function heartbeatFailure(error: unknown): { event?: RadioEvent; notice: RadioNotice } | null {
  if (isApiError(error, 403) && error.code === 'watch_limit_reached') {
    return { event: 'watchLimit', notice: limitNotice(error) };
  }
  if (isApiError(error, 410)) return { notice: { kind: 'problem', code: 'radio_unreachable' } };
  if (isApiError(error, 401) || isApiError(error, 404)) return { notice: { kind: 'ended' } };
  return null;
}

interface RadioProviderProps {
  children: ReactNode;
  /** Attaches the stream to the audio element; tests pass a fake engine. */
  engine?: RadioEngineFactory;
}

export function RadioProvider({ children, engine = createRadioEngine }: RadioProviderProps) {
  const [state, dispatch] = useReducer(reducer, initialRadioState);
  const playing = isPlaying(state);
  const audioRef = useRef<HTMLAudioElement>(null);
  const engineRef = useRef<RadioEngine | null>(null);
  const channelRef = useRef<RadioChannel | null>(null);
  // Bumped when an engine is attached, so the element starts playing then.
  const [attached, setAttached] = useState(0);
  // Program date-time of the playback position (ms, whole seconds), null without one.
  const [position, setPosition] = useState<number | null>(null);
  // Seconds behind live and the long pause (7.2), each for the listen session it was seen in, so
  // a new session starts live without an effect that resets them.
  const [shift, setShift] = useState<{ listenId?: string; offset: number }>({ offset: 0 });
  const [longPauseOf, setLongPauseOf] = useState<string | undefined>(undefined);

  // The latest state for callbacks and event handlers.
  const stateRef = useRef(state);
  const playingRef = useRef(playing);
  const mounted = useRef(true);
  useEffect(() => {
    stateRef.current = state;
    playingRef.current = playing;
  });
  useEffect(() => {
    mounted.current = true;
    return () => {
      mounted.current = false;
    };
  }, []);

  const hasAudio = state.phase !== 'idle';
  const listenId = state.listen?.listenId;
  const playlistUrl = state.listen?.playlistUrl;
  const heartbeatSeconds = state.listen?.heartbeatSeconds;
  const timeShiftSeconds = state.listen?.timeShiftSeconds ?? 0;

  /** Reads how far the element plays behind live, for the session that plays now. */
  const readShift = useCallback(() => {
    const engine = engineRef.current;
    const id = stateRef.current.listen?.listenId;
    const offset = engine ? Math.round(engine.offsetSeconds()) : 0;
    setShift((before) =>
      before.listenId === id && before.offset === offset ? before : { listenId: id, offset },
    );
  }, []);

  // Before the element plays on: a pause longer than the window lost its position, the engine
  // jumps to the oldest one and the listener hears why (7.2, critic M-5).
  const resumeInWindow = useCallback(() => {
    if (engineRef.current?.resumeAt()) {
      setLongPauseOf(stateRef.current.listen?.listenId);
      readShift();
    }
  }, [readShift]);

  // The element must be started by the listener's own gesture (iPhone Safari): a play() right in
  // the click, before the stream is there, lets the later play() of the engine through. With a
  // stream attached the click plays it at once.
  const playInGesture = useCallback(
    (resume: boolean) => {
      const audio = audioRef.current;
      if (!audio || (!resume && audio.src)) return;
      if (resume) resumeInWindow();
      audio.play().catch(() => undefined);
    },
    [resumeInWindow],
  );

  const play = useCallback(
    (station: RadioStation) => {
      const current = stateRef.current;
      if (current.phase !== 'idle' && current.station?.id === station.id) {
        playInGesture(current.phase === 'live');
        dispatch({ type: 'resume' });
        return;
      }
      const attempt = current.attempt + 1;
      stateRef.current = radioReducer(current, { type: 'start', station });
      // The element exists only while a station is chosen: committed now, it is there for the
      // play() of the gesture.
      flushSync(() => dispatch({ type: 'start', station }));
      playInGesture(false);
      startRadioListen(station.id).then(
        (listen) => {
          const now = stateRef.current;
          // A newer start, a stop or the end of the provider came first: the session is not used.
          if (!mounted.current || now.phase !== 'starting' || now.attempt !== attempt) {
            void endRadioListen(listen.listenId);
            return;
          }
          dispatch({ type: 'started', attempt, listen });
        },
        (error: unknown) => dispatch({ type: 'stop', attempt, notice: startNotice(error) }),
      );
    },
    [playInGesture],
  );

  const toggle = useCallback(() => {
    const current = stateRef.current;
    if (current.phase === 'idle') {
      if (current.station) play(current.station);
      return;
    }
    // A pause while the station still starts holds it: it opens paused.
    if (wantsSound(current)) {
      dispatch({ type: 'hold', by: 'user' });
    } else {
      playInGesture(current.phase === 'live');
      dispatch({ type: 'resume' });
    }
  }, [play, playInGesture]);

  const stop = useCallback(() => dispatch({ type: 'stop' }), []);
  const toLive = useCallback(() => {
    engineRef.current?.toLive();
    setLongPauseOf(undefined);
    readShift();
    dispatch({ type: 'resume' });
  }, [readShift]);
  const seek = useCallback(
    (offset: number) => {
      engineRef.current?.seekTo(offset);
      setLongPauseOf(undefined);
      readShift();
    },
    [readShift],
  );
  const hold = useCallback((by: 'trailer' | 'tab') => {
    dispatch({ type: 'hold', by });
    return () => dispatch({ type: 'release', by });
  }, []);
  const markFavorite = useCallback(
    (stationId: number, favorite: boolean) => dispatch({ type: 'favorite', stationId, favorite }),
    [],
  );
  const dismiss = useCallback(() => dispatch({ type: 'dismiss' }), []);

  // The stream of the session: attached to the element, detached when the session ends.
  useEffect(() => {
    const audio = audioRef.current;
    if (!playlistUrl || !audio) return;
    let cancelled = false;
    let created: RadioEngine | null = null;
    const fail = (notice: RadioNotice) => {
      if (!cancelled) dispatch({ type: 'stop', notice });
    };
    engine(audio, apiUrl(playlistUrl), {
      onFatal: () => fail({ kind: 'failed' }),
      onRevoked: () => fail({ kind: 'ended' }),
    }).then(
      (result) => {
        if (cancelled) {
          result.destroy();
          return;
        }
        created = result;
        engineRef.current = result;
        setAttached((count) => count + 1);
      },
      () => fail({ kind: 'failed' }),
    );
    return () => {
      cancelled = true;
      created?.destroy();
      engineRef.current = null;
    };
  }, [engine, playlistUrl]);

  // The end of a session on the server. The stop waits one task, so React's development double
  // mount (StrictMode) does not end the session it is about to use. A closed tab ends it at once
  // (`pagehide` without the back-forward cache); a tab in the background plays on (10.3).
  const pendingEnd = useRef<{ id: string; timer: number } | undefined>(undefined);
  useEffect(() => {
    if (!listenId) return;
    if (pendingEnd.current?.id === listenId) {
      window.clearTimeout(pendingEnd.current.timer);
      pendingEnd.current = undefined;
    }
    const onPageHide = (event: PageTransitionEvent) => {
      if (!event.persisted) void endRadioListen(listenId);
    };
    window.addEventListener('pagehide', onPageHide);
    return () => {
      window.removeEventListener('pagehide', onPageHide);
      const timer = window.setTimeout(() => {
        if (pendingEnd.current?.id === listenId) pendingEnd.current = undefined;
        void endRadioListen(listenId);
      }, 0);
      pendingEnd.current = { id: listenId, timer };
    };
  }, [listenId]);

  // The heartbeat, every `heartbeatSeconds` while the session exists (also paused), and once more
  // when the listener pauses or plays on. It reports the pause and learns about the limit; the
  // server keeps the session alive by the segments it delivers (M-2).
  const beat = useCallback((id: string) => {
    sendRadioHeartbeat(id, playingRef.current).catch((error: unknown) => {
      if (stateRef.current.listen?.listenId !== id) return;
      const failure = heartbeatFailure(error);
      if (!failure) return;
      dispatch(
        failure.event
          ? { type: 'event', event: failure.event, notice: failure.notice }
          : { type: 'stop', notice: failure.notice },
      );
    });
  }, []);
  useEffect(() => {
    if (!listenId || !heartbeatSeconds) return;
    const timer = window.setInterval(() => beat(listenId), heartbeatSeconds * 1000);
    return () => window.clearInterval(timer);
  }, [beat, heartbeatSeconds, listenId]);
  const reported = useRef<{ id: string | undefined; playing: boolean }>({
    id: undefined,
    playing: false,
  });
  useEffect(() => {
    const before = reported.current;
    reported.current = { id: listenId, playing };
    if (listenId && before.id === listenId && before.playing !== playing) beat(listenId);
  }, [beat, listenId, playing]);

  // The element follows the state: plays while nothing holds the radio, pauses otherwise.
  useEffect(() => {
    const audio = audioRef.current;
    if (!audio || !engineRef.current) return;
    if (playing) {
      if (audio.paused) resumeInWindow();
      audio.play().catch((error: unknown) => {
        // The browser wants a gesture first: the listener presses play in the mini player.
        if (error instanceof DOMException && error.name === 'NotAllowedError') {
          dispatch({ type: 'hold', by: 'user' });
        }
      });
    } else if (!audio.paused) {
      audio.pause();
    }
  }, [attached, playing, resumeInWindow]);

  // A pause or play from outside the page (the media keys, the system's controls) is the
  // listener's wish. The silent pause of the load algorithm fires no event.
  useEffect(() => {
    const audio = audioRef.current;
    if (!audio) return;
    const onPause = () => {
      if (playingRef.current) dispatch({ type: 'hold', by: 'user' });
    };
    const onPlay = () => {
      if (!playingRef.current && stateRef.current.phase === 'live') dispatch({ type: 'resume' });
    };
    const onTime = () => {
      const date = engineRef.current?.playingDate();
      setPosition(date ? Math.floor(date.getTime() / 1000) * 1000 : null);
      readShift();
    };
    audio.addEventListener('pause', onPause);
    audio.addEventListener('play', onPlay);
    audio.addEventListener('timeupdate', onTime);
    return () => {
      audio.removeEventListener('pause', onPause);
      audio.removeEventListener('play', onPlay);
      audio.removeEventListener('timeupdate', onTime);
    };
  }, [hasAudio, readShift]);

  // While paused the live edge moves on and no `timeupdate` comes: the offset is read every second
  // (the mini player's «−12:30» grows).
  useEffect(() => {
    if (!attached || !playlistUrl) return;
    const timer = window.setInterval(readShift, 1000);
    return () => window.clearInterval(timer);
  }, [attached, playlistUrl, readShift]);

  // Another tab that starts the radio pauses this one; this one says so when it plays.
  useEffect(() => {
    const channel = openRadioChannel(TAB_ID, () =>
      dispatch({ type: 'event', event: 'otherTabStarts' }),
    );
    channelRef.current = channel;
    return () => {
      channel.close();
      channelRef.current = null;
    };
  }, []);
  useEffect(() => {
    if (playing) channelRef.current?.claim();
  }, [playing]);

  // A film, an episode, an audiobook or a podcast starts: the radio stops (10.3).
  const pathname = useRouterState({ select: (router) => router.resolvedLocation?.pathname });
  useEffect(() => {
    if (pathname?.startsWith('/play/')) dispatch({ type: 'event', event: 'playStarts' });
  }, [pathname]);

  // Another profile or a logout stops the radio; the picker alone does not (R-1).
  const session = useQuery(sessionQuery).data;
  const profileId = session === null ? null : session?.profile?.id;
  const lastProfile = useRef(profileId);
  useEffect(() => {
    const before = lastProfile.current;
    lastProfile.current = profileId;
    if (typeof before !== 'number' || before === profileId) return;
    dispatch({ type: 'event', event: profileId === null ? 'logout' : 'profileChanges' });
  }, [profileId]);

  // «Jetzt läuft»: the title timeline of the running station.
  const liveStation = state.phase === 'live' ? state.station?.id : undefined;
  const now = useQuery({
    ...radioNowQuery(liveStation ?? 0, timeShiftSeconds),
    enabled: liveStation !== undefined,
  });
  const entries = liveStation === undefined ? undefined : now.data?.entries;
  const entry = useMemo(
    () =>
      entries ? currentRadioEntry(entries, position === null ? null : new Date(position)) : null,
    [entries, position],
  );
  const history = useMemo(
    () => (entries ? previousRadioEntries(entries, entry) : []),
    [entries, entry],
  );
  const offset = listenId !== undefined && shift.listenId === listenId ? shift.offset : 0;
  const longPause = listenId !== undefined && longPauseOf === listenId;

  const value = useMemo<RadioPlayer>(
    () => ({
      state,
      playing,
      entry,
      history,
      offset,
      timeShiftSeconds,
      longPause,
      play,
      toggle,
      stop,
      toLive,
      seek,
      hold,
      markFavorite,
      dismiss,
    }),
    [
      state,
      playing,
      entry,
      history,
      offset,
      timeShiftSeconds,
      longPause,
      play,
      toggle,
      stop,
      toLive,
      seek,
      hold,
      markFavorite,
      dismiss,
    ],
  );

  return (
    <RadioContext.Provider value={value}>
      {/* The radio's own element; nothing to see, the mini player is the control. Live radio has
          no caption track. */}
      {hasAudio && (
        // eslint-disable-next-line jsx-a11y/media-has-caption
        <audio ref={audioRef} preload="none" data-testid="radio-audio" hidden />
      )}
      {children}
    </RadioContext.Provider>
  );
}
