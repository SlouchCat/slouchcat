import type { components } from '@slouchcat/api-client';

// The state of the app-level radio player (concept 16, 10.3) as a pure reducer, and the priority
// table that keeps two sounds from ever playing at once. The provider (RadioProvider.tsx) turns
// what happens in the app into events and applies the table's answer; the server decides
// everything else (rule 12): the client neither checks a station's visibility nor any limit.

export type RadioStation = components['schemas']['RadioStation'];
export type RadioListen = components['schemas']['RadioListen'];

/** Who holds the radio paused while its station stays chosen. */
export type RadioHold = 'user' | 'trailer' | 'tab';

/** Why the radio stopped with a message for the listener (shown in the mini player). */
export type RadioNotice =
  /** The profile's daily watch time is used up (K-5), with the numbers the server sent. */
  | { kind: 'watchLimit'; limitMinutes?: number; usedMinutes?: number }
  /** A problem code of the server (`radio_capacity`, `radio_unreachable` …) or `network`. */
  | { kind: 'problem'; code: string }
  /** The server ended the listen session (an administrator stopped it, the session was revoked). */
  | { kind: 'ended' }
  /** The browser could not play the stream. */
  | { kind: 'failed' };

export interface RadioState {
  /** `starting` while the server opens the listen session, `live` once it has. */
  phase: 'idle' | 'starting' | 'live';
  /** The chosen station; kept after a stop with a notice, so the mini player can name it. */
  station: RadioStation | null;
  listen: RadioListen | null;
  /** Empty while the radio plays. */
  holds: readonly RadioHold[];
  notice: RadioNotice | null;
  /** Number of the last start, so the late answer of an earlier one is dropped. */
  attempt: number;
}

export type RadioAction =
  | { type: 'start'; station: RadioStation }
  | { type: 'started'; attempt: number; listen: RadioListen }
  | { type: 'hold'; by: RadioHold }
  | { type: 'release'; by: RadioHold }
  /** The listener asks to play: every hold goes (a trailer's cannot be open at the same time). */
  | { type: 'resume' }
  /** `attempt` limits the stop to that start (a failed start must not end a newer one). */
  | { type: 'stop'; notice?: RadioNotice; attempt?: number }
  | { type: 'favorite'; stationId: number; favorite: boolean }
  | { type: 'dismiss' };

export const initialRadioState: RadioState = {
  phase: 'idle',
  station: null,
  listen: null,
  holds: [],
  notice: null,
  attempt: 0,
};

export function radioReducer(state: RadioState, action: RadioAction): RadioState {
  switch (action.type) {
    case 'start':
      return {
        phase: 'starting',
        station: action.station,
        listen: null,
        holds: [],
        notice: null,
        attempt: state.attempt + 1,
      };
    case 'started':
      if (action.attempt !== state.attempt || state.phase !== 'starting') return state;
      return { ...state, phase: 'live', listen: action.listen, station: action.listen.station };
    case 'hold':
      if (state.phase === 'idle' || state.holds.includes(action.by)) return state;
      return { ...state, holds: [...state.holds, action.by] };
    case 'release':
      if (!state.holds.includes(action.by)) return state;
      return { ...state, holds: state.holds.filter((hold) => hold !== action.by) };
    case 'resume':
      return state.holds.length === 0 ? state : { ...state, holds: [] };
    case 'stop':
      if (action.attempt !== undefined && action.attempt !== state.attempt) return state;
      // Nothing to stop: the same state, so the provider sees no change.
      if (state.phase === 'idle' && !state.station && !action.notice) return state;
      return {
        ...initialRadioState,
        attempt: state.attempt,
        station: action.notice ? state.station : null,
        notice: action.notice ?? null,
      };
    case 'favorite': {
      const update = <T extends { id: number; favorite: boolean }>(station: T): T =>
        station.id === action.stationId ? { ...station, favorite: action.favorite } : station;
      return {
        ...state,
        station: state.station && update(state.station),
        listen: state.listen && { ...state.listen, station: update(state.listen.station) },
      };
    }
    case 'dismiss':
      return state.phase === 'idle' ? { ...initialRadioState, attempt: state.attempt } : state;
  }
}

/** The radio should sound: a session is open and nothing holds it. */
export const isPlaying = (state: RadioState) => state.phase === 'live' && state.holds.length === 0;

/** What happens elsewhere in the app that concerns the radio (concept 16, table in 10.3). */
export type RadioEvent =
  /** A film, an episode, an audiobook or a podcast starts (`/play/$itemId`). */
  | 'playStarts'
  /** The offline player opens (`/offline/play/…`, outside the provider: it unmounts). */
  | 'offlinePlayerStarts'
  | 'trailerOpens'
  | 'trailerCloses'
  | 'profileChanges'
  | 'logout'
  /** Navigation inside the app, the administration included. */
  | 'navigates'
  /** The app or the tab goes to the background. */
  | 'background'
  /** The daily watch time of the profile is reached (K-5, 7.10). */
  | 'watchLimit'
  /** Another tab of this browser starts the radio (BroadcastChannel). */
  | 'otherTabStarts';

export type RadioReaction = 'stop' | 'pause' | 'resume' | 'keep';

/** Table 10.3: what the radio does on each event. */
export const priority: Readonly<Record<RadioEvent, RadioReaction>> = {
  playStarts: 'stop',
  offlinePlayerStarts: 'stop',
  trailerOpens: 'pause',
  trailerCloses: 'resume',
  profileChanges: 'stop',
  logout: 'stop',
  navigates: 'keep',
  background: 'keep',
  watchLimit: 'stop',
  otherTabStarts: 'pause',
};

/** The hold a pausing event sets and its resuming counterpart lifts. */
const holdOf: Partial<Record<RadioEvent, RadioHold>> = {
  trailerOpens: 'trailer',
  trailerCloses: 'trailer',
  otherTabStarts: 'tab',
};

/**
 * Applies the reaction of table 10.3 to `state`. A stop for the watch limit carries its notice;
 * `notice` is the notice of that stop.
 */
export function applyEvent(state: RadioState, event: RadioEvent, notice?: RadioNotice): RadioState {
  const hold = holdOf[event];
  switch (priority[event]) {
    case 'stop':
      return radioReducer(state, notice ? { type: 'stop', notice } : { type: 'stop' });
    case 'pause':
      return hold ? radioReducer(state, { type: 'hold', by: hold }) : state;
    case 'resume':
      return hold ? radioReducer(state, { type: 'release', by: hold }) : state;
    case 'keep':
      return state;
  }
}
