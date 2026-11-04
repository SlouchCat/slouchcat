import type { Track } from '@slouchcat/api-client';

/** ISO 639-2 codes and names that files and sidecars use, in the two-letter form of profiles. */
const baseCodes: Record<string, string> = {
  ger: 'de',
  deu: 'de',
  german: 'de',
  deutsch: 'de',
  // Swiss German counts as German (11.3).
  gsw: 'de',
  eng: 'en',
  english: 'en',
  fre: 'fr',
  fra: 'fr',
  ita: 'it',
  spa: 'es',
  por: 'pt',
  dut: 'nl',
  nld: 'nl',
  jpn: 'ja',
  kor: 'ko',
  chi: 'zh',
  zho: 'zh',
  rus: 'ru',
  pol: 'pl',
  swe: 'sv',
  dan: 'da',
  nor: 'no',
  fin: 'fi',
  tur: 'tr',
};

/**
 * The language of a track as a profile stores it ("ger" → "de", "pt-BR" → "pt", "gsw" → "de");
 * undefined for none or "und".
 */
export function baseLanguage(code: string | undefined): string | undefined {
  if (!code) return undefined;
  const lower = code.toLowerCase();
  if (lower === 'und' || lower === 'zxx' || lower === 'mul') return undefined;
  const primary = lower.split(/[-_]/)[0] ?? lower;
  return baseCodes[primary] ?? primary;
}

/** A track with its position in the file's list (for «Spur 3» when it has no language). */
export interface PlacedTrack {
  track: Track;
  position: number;
}

export interface OrderedTracks {
  /** The profile's language, then English, each in the file's order. */
  mine: PlacedTrack[];
  /** Every other track, alphabetically by its name: behind «Weitere Sprachen (n)». */
  more: PlacedTrack[];
}

/**
 * The order of the sound and subtitle panels (V-5, 11.10): the profile's language first (in stage 1
 * `profiles.language`), then English, then the rest alphabetically. Only the display: the server
 * picks the track (rule 12), and a choice sends the request as before.
 */
export function orderTracks(
  tracks: readonly Track[],
  profileLanguage: string | undefined,
  nameOf: (placed: PlacedTrack) => string,
  uiLanguage: string,
): OrderedTracks {
  const placed = tracks.map((track, position) => ({ track, position }));
  const preferred = [baseLanguage(profileLanguage), 'en'].filter(
    (language, index, all): language is string =>
      language !== undefined && all.indexOf(language) === index,
  );
  const rank = (entry: PlacedTrack) => {
    const language = baseLanguage(entry.track.language);
    return language === undefined ? -1 : preferred.indexOf(language);
  };
  const mine = preferred.flatMap((_, index) => placed.filter((entry) => rank(entry) === index));
  const collator = new Intl.Collator(uiLanguage);
  const more = placed
    .filter((entry) => rank(entry) < 0)
    .map((entry) => ({ entry, name: nameOf(entry) }))
    .sort((a, b) => collator.compare(a.name, b.name) || a.entry.position - b.entry.position)
    .map(({ entry }) => entry);
  return { mine, more };
}

/** Titles that name a subtitle for the deaf and hard of hearing. */
const SDH = /\b(sdh|cc)\b|hearing|hörgeschädigt|gehörlos/i;

/** Whether a subtitle track is for the deaf and hard of hearing, by its title. */
export function isSdh(track: Track): boolean {
  return track.title !== undefined && SDH.test(track.title);
}
