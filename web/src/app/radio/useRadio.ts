import { createContext, useContext } from 'react';
import type { RadioNowEntry } from './queries';
import { initialRadioState, type RadioHold, type RadioState, type RadioStation } from './priority';

/** The app-level radio player as the pages see it (RadioProvider). */
export interface RadioPlayer {
  state: RadioState;
  /** A session is open and nothing holds it. */
  playing: boolean;
  /** The title that sounds now, from the station's title timeline; null without one. */
  entry: RadioNowEntry | null;
  /** «Zuvor gespielt»: the entries before `entry`, newest first (7.6). */
  history: readonly RadioNowEntry[];
  /** Whole seconds the playback lies behind live (0 at live). */
  offset: number;
  /** How far behind live the session may play (`RadioListen.timeShiftSeconds`); 0 = live only. */
  timeShiftSeconds: number;
  /**
   * The pause lasted longer than the window: the radio plays on from the oldest position (7.2)
   * until the listener goes live or seeks.
   */
  longPause: boolean;
  /** Plays a station; the station that plays already resumes. */
  play: (station: RadioStation) => void;
  /** Pause or play on, as the listener asks. */
  toggle: () => void;
  /** Ends the listen session. */
  stop: () => void;
  /** Jumps to the live edge. */
  toLive: () => void;
  /** Plays `offset` seconds behind live (the time shift slider). */
  seek: (offset: number) => void;
  /** Holds the radio paused (a trailer) until the returned function is called. */
  hold: (by: Exclude<RadioHold, 'user'>) => () => void;
  /** Sets the favourite mark of the chosen station after the server stored it. */
  markFavorite: (stationId: number, favorite: boolean) => void;
  /** Closes the notice of a stopped station. */
  dismiss: () => void;
}

const idle: RadioPlayer = {
  state: initialRadioState,
  playing: false,
  entry: null,
  history: [],
  offset: 0,
  timeShiftSeconds: 0,
  longPause: false,
  play: () => {},
  toggle: () => {},
  stop: () => {},
  toLive: () => {},
  seek: () => {},
  hold: () => () => {},
  markFavorite: () => {},
  dismiss: () => {},
};

export const RadioContext = createContext<RadioPlayer>(idle);

/** The radio player; outside the provider (a component rendered alone) an idle one. */
export const useRadio = (): RadioPlayer => useContext(RadioContext);
