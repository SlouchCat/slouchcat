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
  /** Plays a station; the station that plays already resumes. */
  play: (station: RadioStation) => void;
  /** Pause or play on, as the listener asks. */
  toggle: () => void;
  /** Ends the listen session. */
  stop: () => void;
  /** Jumps to the live edge. */
  toLive: () => void;
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
  play: () => {},
  toggle: () => {},
  stop: () => {},
  toLive: () => {},
  hold: () => () => {},
  markFavorite: () => {},
  dismiss: () => {},
};

export const RadioContext = createContext<RadioPlayer>(idle);

/** The radio player; outside the provider (a component rendered alone) an idle one. */
export const useRadio = (): RadioPlayer => useContext(RadioContext);
