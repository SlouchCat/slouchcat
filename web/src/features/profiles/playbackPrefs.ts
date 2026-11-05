import type { PlaybackPrefs, SessionInfo } from '@slouchcat/api-client';
import { queryOptions, type QueryClient } from '@tanstack/react-query';
import { api, request } from '@/api/client';
import { sessionQuery } from '@/app/session';
import { LANGUAGE_CODES } from '@/features/player/labels';

/** The length limit of each language list (contract schema PlaybackLanguages). */
export const MAX_PLAYBACK_LANGUAGES = 10;

/**
 * The languages the lists offer: the ISO 639-1 codes of the shared language table (labels.ts,
 * trackpick on the server), so every choice is one the server compares tracks with.
 */
export const PLAYBACK_LANGUAGES: readonly string[] = [...new Set(Object.values(LANGUAGE_CODES))];

/** The language and subtitle preferences of the active profile (concept 13 11.4, F-13). */
export const myPlaybackPrefsQuery = queryOptions({
  queryKey: ['me', 'playback-prefs'],
  queryFn: ({ signal }): Promise<PlaybackPrefs> =>
    request(api.GET('/me/playback-prefs', { signal })),
});

/** Replaces the active profile's preferences (all three fields at once). */
export const putMyPlaybackPrefs = (body: PlaybackPrefs): Promise<PlaybackPrefs> =>
  request(api.PUT('/me/playback-prefs', { body }));

/**
 * Takes the stored preferences into the caches: the query of the page, the active profile of the
 * session (the sound panel reads it) and the profile list of the primary profile.
 */
export function adoptPlaybackPrefs(queryClient: QueryClient, prefs: PlaybackPrefs): void {
  queryClient.setQueryData(myPlaybackPrefsQuery.queryKey, prefs);
  queryClient.setQueryData<SessionInfo | null>(sessionQuery.queryKey, (session) =>
    session?.profile ? { ...session, profile: { ...session.profile, ...prefs } } : session,
  );
  void queryClient.invalidateQueries({ queryKey: ['profiles'] });
}

/** The list with `language` in the first place («Als Standard für dieses Profil»). */
export const withFirst = (list: readonly string[], language: string): string[] =>
  [language, ...list.filter((code) => code !== language)].slice(0, MAX_PLAYBACK_LANGUAGES);
