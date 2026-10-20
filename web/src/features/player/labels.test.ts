import { describe, expect, it } from 'vitest';
import i18n from '@/i18n';
import { alienTracks, playback } from '@/test/fixtures';
import { formatBitrate } from '@/lib/format';
import { chapterAt, codecName, languageName, qualityLabel, trackLabel } from './labels';

const t = i18n.t.bind(i18n);

describe('player labels', () => {
  it('names languages from the codes of files', () => {
    expect(languageName('ger', 'de')).toBe('Deutsch');
    expect(languageName('eng', 'de')).toBe('Englisch');
    expect(languageName('de', 'en')).toBe('German');
    expect(languageName('und', 'de')).toBeUndefined();
    expect(languageName(undefined, 'de')).toBeUndefined();
  });

  it('labels tracks with language, codec, channels and source', () => {
    const [aac, ac3] = alienTracks.audio;
    const [srt, pgs] = alienTracks.subtitles;
    expect(trackLabel(t, aac!, 0, 'de')).toBe('Deutsch · AAC · 2.0');
    expect(trackLabel(t, ac3!, 1, 'de')).toBe('Englisch · AC3 · 5.1');
    // The title names the codec already.
    expect(trackLabel(t, srt!, 0, 'de')).toBe('Deutsch (SRT)');
    expect(trackLabel(t, pgs!, 1, 'de')).toBe('Englisch · PGS');
    expect(trackLabel(t, { ...pgs!, language: undefined, forced: true }, 1, 'de')).toBe(
      'Spur 2 · PGS · erzwungen',
    );
    expect(codecName('truehd')).toBe('TrueHD');
  });

  it('names the rung of automatic quality when the server transcodes', () => {
    const auto = playback.qualities[0]!;
    expect(qualityLabel(t, auto, playback)).toBe('Automatisch');
    const transcoded = {
      ...playback,
      transcoding: { mode: 'transcode' as const, hardware: true, height: 720, reason: 'hdr' },
    };
    expect(qualityLabel(t, auto, transcoded)).toBe('Automatisch (720p)');
    expect(qualityLabel(t, playback.qualities[2]!, playback)).toBe('720p');
  });

  it('finds the chapter that contains a position', () => {
    const chapters = [
      { startMs: 0, endMs: 3000 },
      { startMs: 3000, endMs: 6500 },
      { startMs: 6500, endMs: 10_000 },
    ];
    expect(chapterAt(chapters, 0)).toBe(0);
    expect(chapterAt(chapters, 3000)).toBe(1);
    expect(chapterAt(chapters, 9000)).toBe(2);
    expect(chapterAt([{ startMs: 5000, endMs: 6000 }], 1000)).toBe(-1);
  });

  it('formats bit rates in megabits per second', () => {
    expect(formatBitrate(4_000_000, 'en')).toMatch(/^4\sMb(it|ps)?(\/s)?$/);
    expect(formatBitrate(2_500_000, 'de')).toMatch(/^2,5\s/);
  });
});
