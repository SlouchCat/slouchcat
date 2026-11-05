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

  it('a remembered rung below 2160p is a cap, original and 2160p a quality; a choice here wins (V-17)', () => {
    expect(playbackRequest(undefined, {}, undefined, undefined, '480p')).toMatchObject({
      qualityCap: '480p',
    });
    expect(playbackRequest(undefined, {}, undefined, undefined, '480p')).not.toHaveProperty(
      'quality',
    );
    expect(playbackRequest(undefined, {}, undefined, undefined, 'original').quality).toBe(
      'original',
    );
    expect(playbackRequest(undefined, {}, undefined, undefined, '2160p').quality).toBe('2160p');
    const auto = playbackRequest(undefined, {}, undefined, undefined, 'auto');
    expect(auto).not.toHaveProperty('quality');
    expect(auto).not.toHaveProperty('qualityCap');
    const chosen = playbackRequest(undefined, {}, '1080p', 0, '480p');
    expect(chosen).toMatchObject({ quality: '1080p' });
    expect(chosen).not.toHaveProperty('qualityCap');
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

  it('within the language: no commentary, text before bitmap, the default one (review step 9, m-3)', () => {
    const audio = alienTracks.audio[0]!;
    const subtitle = alienTracks.subtitles[0]!;
    const many: MediaFileInfo = {
      ...other,
      tracks: {
        mediaFileId: 'mf-2',
        audio: [
          { ...audio, id: 51, index: 1, language: 'ger', title: 'Kommentar', default: true },
          { ...audio, id: 52, index: 2, language: 'ger', default: false },
          { ...audio, id: 53, index: 3, language: 'deu', default: true },
        ],
        subtitles: [
          { ...subtitle, id: 61, language: 'de', text: false, default: false, forced: false },
          { ...subtitle, id: 62, language: 'de', text: true, default: false, forced: false },
          { ...subtitle, id: 63, language: 'de', text: false, default: true, forced: false },
        ],
      },
    };
    const german: PlaybackInfo = {
      ...playback,
      tracks: {
        ...playback.tracks,
        subtitles: [{ ...subtitle, id: 21, language: 'de', forced: false }],
      },
      selection: { audioTrack: 1, subtitleTrack: 21, subtitleDelivery: 'text' },
    };
    expect(versionTracks(german, many)).toEqual({ audioTrack: 3, subtitle: 62 });
    const texts: MediaFileInfo = {
      ...many,
      tracks: {
        ...many.tracks!,
        subtitles: [
          { ...subtitle, id: 64, language: 'de', text: true, default: false, forced: false },
          { ...subtitle, id: 65, language: 'de', text: true, default: true, forced: false },
        ],
      },
    };
    expect(versionTracks(german, texts)).toMatchObject({ subtitle: 65 });
  });

  it('prefers a decodable sound and a subtitle as SDH as the running one (final review stage 2, G1-m1)', () => {
    const audio = alienTracks.audio[0]!;
    const subtitle = alienTracks.subtitles[0]!;
    const mixed: MediaFileInfo = {
      ...other,
      tracks: {
        mediaFileId: 'mf-2',
        audio: [
          { ...audio, id: 71, index: 1, language: 'de', codec: 'dts', default: true },
          { ...audio, id: 72, index: 2, language: 'de', codec: 'ac3', default: false },
        ],
        subtitles: [
          {
            ...subtitle,
            id: 81,
            language: 'de',
            title: 'SDH',
            text: true,
            default: true,
            forced: false,
          },
          {
            ...subtitle,
            id: 82,
            language: 'de',
            title: 'Deutsch',
            text: true,
            default: false,
            forced: false,
          },
        ],
      },
    };
    const running = (title: string): PlaybackInfo => ({
      ...playback,
      tracks: {
        ...playback.tracks,
        audio: [{ ...audio, id: 11, index: 1, language: 'de' }],
        subtitles: [{ ...subtitle, id: 21, language: 'de', title, forced: false }],
      },
      selection: { audioTrack: 1, subtitleTrack: 21, subtitleDelivery: 'text' },
    });
    expect(versionTracks(running('Deutsch'), mixed, ['aac', 'ac3'])).toEqual({
      audioTrack: 2,
      subtitle: 82,
    });
    expect(versionTracks(running('Deutsch (SDH)'), mixed, ['aac', 'ac3'])).toMatchObject({
      subtitle: 81,
    });
    // Without a decodable codec in the language the default one comes first again.
    expect(versionTracks(running('Deutsch'), mixed, ['aac'])).toMatchObject({ audioTrack: 1 });
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
