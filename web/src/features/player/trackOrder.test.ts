import type { Track } from '@slouchcat/api-client';
import { describe, expect, it } from 'vitest';
import { baseLanguage, isSdh, orderTracks, type PlacedTrack } from './trackOrder';

const track = (id: number, language?: string, title?: string): Track => ({
  id,
  index: id,
  kind: 'subtitle',
  codec: 'subrip',
  language,
  title,
  default: false,
  forced: false,
  source: 'embedded',
  text: true,
});

const names: Record<string, string> = {
  de: 'Deutsch',
  en: 'Englisch',
  fr: 'Französisch',
  it: 'Italienisch',
  es: 'Spanisch',
  ja: 'Japanisch',
};
const nameOf = ({ track: entry, position }: PlacedTrack) =>
  entry.title ?? names[baseLanguage(entry.language) ?? ''] ?? `Spur ${position + 1}`;
const ids = (list: PlacedTrack[]) => list.map((entry) => entry.track.id);

describe('baseLanguage (11.3 «Sprachcodes»)', () => {
  it('maps the codes of files to the two letters of profiles', () => {
    expect(baseLanguage('ger')).toBe('de');
    expect(baseLanguage('deu')).toBe('de');
    expect(baseLanguage('gsw')).toBe('de');
    expect(baseLanguage('German')).toBe('de');
    expect(baseLanguage('eng')).toBe('en');
    expect(baseLanguage('pt-BR')).toBe('pt');
    expect(baseLanguage('de')).toBe('de');
    expect(baseLanguage('und')).toBeUndefined();
    expect(baseLanguage(undefined)).toBeUndefined();
  });
});

describe('orderTracks (V-5, 11.10)', () => {
  const tracks = [
    track(1, 'spa'),
    track(2, 'eng'),
    track(3, 'ita'),
    track(4, 'ger'),
    track(5),
    track(6, 'fre'),
    track(7, 'gsw', 'Schwiizerdütsch'),
    track(8, 'en', 'Englisch SDH'),
  ];

  it('puts the profile language first, then English, each in the file’s order', () => {
    const { mine } = orderTracks(tracks, 'de', nameOf, 'de');
    expect(ids(mine)).toEqual([4, 7, 2, 8]);
  });

  it('sorts the other languages alphabetically by their name, tracks without one among them', () => {
    const { more } = orderTracks(tracks, 'de', nameOf, 'de');
    expect(more.map(nameOf)).toEqual(['Französisch', 'Italienisch', 'Spanisch', 'Spur 5']);
  });

  it('keeps the positions of the file for «Spur n»', () => {
    const { more } = orderTracks(tracks, 'de', nameOf, 'de');
    expect(more.find((entry) => entry.track.id === 5)?.position).toBe(4);
  });

  it('lists English once for an English profile', () => {
    const { mine, more } = orderTracks(tracks, 'en', nameOf, 'en');
    expect(ids(mine)).toEqual([2, 8]);
    expect(ids(more)).toContain(4);
    expect(mine.length + more.length).toBe(tracks.length);
  });

  it('without a profile language only English comes first', () => {
    expect(ids(orderTracks(tracks, undefined, nameOf, 'de').mine)).toEqual([2, 8]);
  });
});

describe('isSdh', () => {
  it('reads the title', () => {
    expect(isSdh(track(1, 'en', 'English SDH'))).toBe(true);
    expect(isSdh(track(1, 'de', 'Deutsch für Hörgeschädigte'))).toBe(true);
    expect(isSdh(track(1, 'en', 'English (CC)'))).toBe(true);
    expect(isSdh(track(1, 'en', 'English'))).toBe(false);
    expect(isSdh(track(1, 'en'))).toBe(false);
  });
});
