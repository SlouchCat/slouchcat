import { describe, expect, it } from 'vitest';
import {
  applyEvent,
  initialRadioState,
  isPlaying,
  priority,
  radioReducer,
  type RadioListen,
  type RadioState,
  type RadioStation,
} from './priority';

const station: RadioStation = {
  id: 7,
  name: 'Radio Swiss Pop',
  country: 'CH',
  languages: ['de'],
  genres: ['pop'],
  kinds: ['music'],
  moods: [],
  hls: false,
  showsTitles: true,
  favorite: false,
  recommended: false,
  state: 'ok',
};

const listen: RadioListen = {
  listenId: '01JC0000000000000000000000',
  playlistUrl: '/radio/live/01JC0000000000000000000000/index.m3u8?rt=token',
  heartbeatSeconds: 30,
  startOffsetSeconds: -8,
  timeShiftSeconds: 1800,
  station,
};

function live(): RadioState {
  const starting = radioReducer(initialRadioState, { type: 'start', station });
  return radioReducer(starting, { type: 'started', attempt: starting.attempt, listen });
}

describe('table 10.3', () => {
  it('stops when a film, episode, audiobook or podcast starts (/play/$itemId, also offline)', () => {
    for (const event of ['playStarts', 'offlinePlayerStarts'] as const) {
      const next = applyEvent(live(), event);
      expect(next.phase).toBe('idle');
      expect(next.listen).toBeNull();
      expect(next.station).toBeNull();
    }
  });

  it('pauses for a trailer and plays on after it closed', () => {
    const paused = applyEvent(live(), 'trailerOpens');
    expect(paused.phase).toBe('live');
    expect(isPlaying(paused)).toBe(false);
    const resumed = applyEvent(paused, 'trailerCloses');
    expect(isPlaying(resumed)).toBe(true);
    expect(resumed.listen).toBe(listen);
  });

  it('keeps a pause of the listener after the trailer closed', () => {
    const paused = radioReducer(live(), { type: 'hold', by: 'user' });
    const after = applyEvent(applyEvent(paused, 'trailerOpens'), 'trailerCloses');
    expect(isPlaying(after)).toBe(false);
    expect(after.holds).toEqual(['user']);
  });

  it('stops on a profile change and on logout', () => {
    expect(applyEvent(live(), 'profileChanges').phase).toBe('idle');
    expect(applyEvent(live(), 'logout').phase).toBe('idle');
  });

  it('keeps playing on navigation, the administration included, and in the background', () => {
    const state = live();
    expect(applyEvent(state, 'navigates')).toBe(state);
    expect(applyEvent(state, 'background')).toBe(state);
  });

  it('stops with a notice when the daily watch time is reached', () => {
    const next = applyEvent(live(), 'watchLimit', { kind: 'watchLimit', limitMinutes: 60 });
    expect(next.phase).toBe('idle');
    expect(next.notice).toEqual({ kind: 'watchLimit', limitMinutes: 60 });
    // The mini player still names the station the notice is about.
    expect(next.station?.id).toBe(station.id);
  });

  it('pauses when another tab starts the radio, until the listener plays here again', () => {
    const paused = applyEvent(live(), 'otherTabStarts');
    expect(isPlaying(paused)).toBe(false);
    expect(paused.holds).toEqual(['tab']);
    expect(isPlaying(radioReducer(paused, { type: 'resume' }))).toBe(true);
  });

  it('answers every event of the table', () => {
    expect(priority).toEqual({
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
    });
  });
});

describe('radioReducer', () => {
  it('drops the answer of an earlier start', () => {
    const first = radioReducer(initialRadioState, { type: 'start', station });
    const second = radioReducer(first, { type: 'start', station: { ...station, id: 8 } });
    const late = radioReducer(second, { type: 'started', attempt: first.attempt, listen });
    expect(late).toBe(second);
    expect(radioReducer(second, { type: 'stop', attempt: first.attempt })).toBe(second);
  });

  it('ignores holds while nothing plays and stop when idle', () => {
    expect(radioReducer(initialRadioState, { type: 'hold', by: 'trailer' })).toBe(
      initialRadioState,
    );
    expect(radioReducer(initialRadioState, { type: 'stop' })).toBe(initialRadioState);
  });

  it('dismisses a notice', () => {
    const stopped = radioReducer(live(), { type: 'stop', notice: { kind: 'ended' } });
    const dismissed = radioReducer(stopped, { type: 'dismiss' });
    expect(dismissed.station).toBeNull();
    expect(dismissed.notice).toBeNull();
  });

  it('marks the favourite on the station and the listen session', () => {
    const next = radioReducer(live(), { type: 'favorite', stationId: station.id, favorite: true });
    expect(next.station?.favorite).toBe(true);
    expect(next.listen?.station.favorite).toBe(true);
  });
});
