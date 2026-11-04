import type { Track } from '@slouchcat/api-client';
import { LANGUAGE_CODES, NO_LANGUAGE } from './labels';

/**
 * The language of a track as a profile stores it ("ger" → "de", "pt-BR" → "pt", "gsw" → "de");
 * undefined for none, "und", "und-Latn", "xx" or several languages ("mul"). The same rule as
 * trackpick.Normalize on the server (review step 9, m-5).
 */
export function baseLanguage(code: string | undefined): string | undefined {
  const lower = code?.trim().toLowerCase();
  if (!lower || NO_LANGUAGE.has(lower) || lower === 'mul') return undefined;
  const primary = lower.replaceAll('_', '-').split('-')[0] ?? '';
  // Swiss German counts as German (11.3); on screen it keeps its own name.
  if (primary === 'gsw') return 'de';
  if (Object.hasOwn(LANGUAGE_CODES, primary)) return LANGUAGE_CODES[primary];
  if (NO_LANGUAGE.has(primary)) return undefined;
  return primary || undefined;
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
