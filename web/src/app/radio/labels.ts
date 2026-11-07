import type { TFunction } from 'i18next';

// Names of the radio's codes for the UI. Countries and languages come from the browser
// (Intl.DisplayNames, R-8); cantons and the facet values (genre, kind, mood, quality) from the
// locale files. A value the locale does not know yet (a new key of the server's rules) shows as
// the server sent it.

export type RadioFacet = 'genre' | 'kind' | 'mood' | 'quality';

function displayName(language: string, type: 'region' | 'language', code: string): string {
  try {
    return new Intl.DisplayNames([language], { type }).of(code) ?? code;
  } catch {
    return code;
  }
}

/** A country by its ISO 3166-1 code, in the UI language (`CH` → «Schweiz»). */
export const countryName = (language: string, code: string) =>
  displayName(language, 'region', code.toUpperCase());

/** A language by its ISO 639-1 code, in the UI language (`fr` → «Französisch»). */
export const radioLanguageName = (language: string, code: string) =>
  displayName(language, 'language', code);

/** A region by its ISO 3166-2 code: the Swiss cantons from the locale, others as the code. */
export const regionName = (t: TFunction, code: string) =>
  t(`radio.region.${code}`, { defaultValue: code });

/** The text of a facet value (`genre` `pop` → «Pop»). */
export const facetValueName = (t: TFunction, facet: RadioFacet, value: string) =>
  t(`radio.${facet}.${value}`, { defaultValue: value });

/** «Schweiz · Zürich»: the place of a station, as far as it is known. */
export function stationPlace(
  t: TFunction,
  language: string,
  station: { country?: string; region?: string },
): string | undefined {
  const parts = [
    station.country && countryName(language, station.country),
    station.region && regionName(t, station.region),
  ].filter((part): part is string => Boolean(part));
  return parts.length > 0 ? parts.join(' · ') : undefined;
}
