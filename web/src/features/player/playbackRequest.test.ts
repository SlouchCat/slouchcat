import type { MediaFileInfo, PlaybackInfo } from '@slouchcat/api-client';
import { describe, expect, it } from 'vitest';
import { alienDetail, alienTracks, playback } from '@/test/fixtures';
import { playbackRequest, switchTracks, versionTracks } from './PlayerPage';

// Concept 13 11.3, contract 1.11.0: a missing track lets the server choose, so only the first start
// leaves them out; every switch names audio and subtitles, «Aus» as subtitleOff (review m-1).

const withSubtitle: PlaybackInfo = {
  ...playback,
  selection: { audioTrack: 1, subtitleTrack: 21, subtitleDelivery: 'text' },
};
const withoutSubtitle: PlaybackInfo = { ...playback, selection: { audioTrack: 1 } };

describe('playbackRequest', () => {
  it('first start: no track, the server chooses', () => {
    const body = playbackRequest(undefined, {}, undefined, undefined);
    expect(body).not.toHaveProperty('audioTrack');
    expect(body).not.toHaveProperty('subtitleTrack');
    expect(body).not.toHaveProperty('subtitleOff');
    expect(body).not.toHaveProperty('rememberForSeries');
    expect(body.subtitleDelivery).toBe('auto');
  });

  it('«Aus» is subtitleOff, never subtitleTrack null', () => {
    const body = playbackRequest(undefined, { audioTrack: 1, subtitle: 'off' }, undefined, 1000);
    expect(body).toMatchObject({ audioTrack: 1, subtitleOff: true, positionMs: 1000 });
    expect(body).not.toHaveProperty('subtitleTrack');
  });

  it('a track id and the series switch', () => {
    const body = playbackRequest(
      'mf-2',
      { audioTrack: 2, subtitle: 21, rememberForSeries: true },
      '720p',
      undefined,
    );
    expect(body).toMatchObject({
      mediaFileId: 'mf-2',
      audioTrack: 2,
      subtitleTrack: 21,
      rememberForSeries: true,
      quality: '720p',
    });
    expect(body).not.toHaveProperty('subtitleOff');
  });
});

describe('switchTracks', () => {
  const body = (info: PlaybackInfo, change: Parameters<typeof switchTracks>[1]) =>
    playbackRequest(undefined, switchTracks(info, change), change.quality, 0);

  it('subtitles off: the running audio track goes along', () => {
    const b = body(withSubtitle, { subtitleTrack: 'off' });
    expect(b).toMatchObject({ audioTrack: 1, subtitleOff: true });
    expect(b).not.toHaveProperty('subtitleTrack');
  });

  it('another audio track keeps the running subtitles', () => {
    expect(body(withSubtitle, { audioTrack: 2 })).toMatchObject({
      audioTrack: 2,
      subtitleTrack: 21,
    });
  });

  it('another audio track after «Aus» stays without subtitles', () => {
    const b = body(withoutSubtitle, { audioTrack: 2 });
    expect(b).toMatchObject({ audioTrack: 2, subtitleOff: true });
    expect(b).not.toHaveProperty('subtitleTrack');
  });

  it('a quality switch names both tracks and remembers nothing', () => {
    const b = body(withSubtitle, { quality: '720p', rememberForSeries: true });
    expect(b).toMatchObject({ audioTrack: 1, subtitleTrack: 21, quality: '720p' });
    expect(b).not.toHaveProperty('rememberForSeries');
  });

  it('a track chosen with the series switch on is remembered', () => {
    expect(body(withSubtitle, { subtitleTrack: 'off', rememberForSeries: true })).toMatchObject({
      audioTrack: 1,
      subtitleOff: true,
      rememberForSeries: true,
    });
    expect(body(withSubtitle, { audioTrack: 2, rememberForSeries: false })).not.toHaveProperty(
      'rememberForSeries',
    );
  });

  it('a file without audio names no audio track', () => {
    const silent: PlaybackInfo = { ...playback, selection: {} };
    expect(body(silent, { subtitleTrack: 21 })).not.toHaveProperty('audioTrack');
  });
});

describe('versionTracks', () => {
  const other: MediaFileInfo = {
    ...alienDetail.mediaFiles[0]!,
    id: 'mf-2',
    tracks: {
      mediaFileId: 'mf-2',
      audio: [
        { ...alienTracks.audio[1]!, id: 31, index: 1, language: 'en' },
        { ...alienTracks.audio[0]!, id: 32, index: 3, language: 'de' },
      ],
      subtitles: [{ ...alienTracks.subtitles[0]!, id: 41, language: 'de' }],
    },
  };

  it('finds the running tracks in the other version by language', () => {
    expect(versionTracks(withSubtitle, other)).toEqual({ audioTrack: 3, subtitle: 41 });
  });

  it('keeps «Aus»', () => {
    expect(versionTracks(withoutSubtitle, other)).toEqual({ audioTrack: 3, subtitle: 'off' });
  });

  it('leaves to the server what the other version lacks', () => {
    const bare: MediaFileInfo = {
      ...other,
      tracks: { mediaFileId: 'mf-2', audio: [], subtitles: [] },
    };
    expect(versionTracks(withSubtitle, bare)).toEqual({});
    expect(versionTracks(withSubtitle, undefined)).toEqual({});
  });
});
