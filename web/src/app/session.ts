import type { components, Profile, SessionInfo } from '@slouchcat/api-client';
import {
  queryOptions,
  useMutation,
  useQuery,
  useQueryClient,
  type QueryClient,
} from '@tanstack/react-query';
import { getRouteApi, useNavigate } from '@tanstack/react-router';
import { api, isApiError, request } from '@/api/client';
import { detectLanguage, setLanguage } from '@/i18n';
import { useTransientMutation } from '@/lib/useTransientMutation';
import { isAccentName } from '@/ui/accent';

type SetupStatus = components['schemas']['SetupStatus'];

export const setupStatusQuery = queryOptions({
  queryKey: ['setup-status'],
  queryFn: ({ signal }): Promise<SetupStatus> => request(api.GET('/setup', { signal })),
  staleTime: Infinity,
});

/** The current session, or null when nobody is logged in (401). */
export const sessionQuery = queryOptions({
  queryKey: ['session'],
  queryFn: async ({ signal }): Promise<SessionInfo | null> => {
    try {
      return await request(api.GET('/auth/session', { signal }));
    } catch (error) {
      if (isApiError(error, 401)) return null;
      throw error;
    }
  },
  staleTime: 60_000,
});

/** A session whose profile is chosen: everything behind the profile picker runs with one. */
export type ActiveSession = SessionInfo & { profile: Profile };

/**
 * Switches the UI to the language of the active profile (C-3). A session without a profile (the
 * picker is next) keeps the language the UI has.
 */
export async function applyProfileLanguage(session: SessionInfo | null): Promise<void> {
  const language = session?.profile?.language;
  if (language) await setLanguage(language);
}

/**
 * Applies the accent of the active profile (`profile.accent`, a token name such as `teal`) to the
 * whole app: `--sc-accent` and `--sc-on-accent` on the root element point at that accent's
 * tokens. Without a session or with an unknown name, the defaults from global.css (ember) apply.
 */
export function applyProfileAccent(session: SessionInfo | null): void {
  const { style } = document.documentElement;
  const name = session?.profile?.accent;
  if (isAccentName(name)) {
    style.setProperty('--sc-accent', `var(--sc-color-accent-${name}-base)`);
    style.setProperty('--sc-on-accent', `var(--sc-color-accent-${name}-on)`);
  } else {
    style.removeProperty('--sc-accent');
    style.removeProperty('--sc-on-accent');
  }
}

/** Query keys that outlive every change: nothing in them depends on a session. */
const SURVIVING_KEYS = ['setup-status', 'session'];

/** Cache of poster and backdrop images of the service worker (vite.config.ts, workbox). */
export const IMAGE_CACHE = 'slouchcat-images';

/**
 * Drops the cached images (they are cached per URL, and a profile must not see what another one
 * loaded). Best effort: no Cache API (plain HTTP) or a failing delete does not stop the switch.
 */
async function dropImageCache(): Promise<void> {
  try {
    if (typeof caches !== 'undefined') await caches.delete(IMAGE_CACHE);
  } catch {
    // Nothing to do: the server also sends `Clear-Site-Data: "cache"`.
  }
}

interface ClearOptions {
  /** The account stays the same (profile switch or edit): its profile list survives. */
  sameAccount?: boolean;
}

/**
 * Forgets everything the previous profile (or account) loaded: lists, details, progress, and the
 * cached images of the service worker.
 */
export async function clearProfileData(
  queryClient: QueryClient,
  { sameAccount = false }: ClearOptions = {},
): Promise<void> {
  const keep = sameAccount ? [...SURVIVING_KEYS, 'auth'] : SURVIVING_KEYS;
  queryClient.removeQueries({ predicate: (query) => !keep.includes(String(query.queryKey[0])) });
  await dropImageCache();
}

/**
 * Makes `session` the current one: drops the data of the previous profile, stores the session and
 * applies its language and accent without a reload (C-3, B-3).
 */
export async function adoptSession(
  queryClient: QueryClient,
  session: SessionInfo,
  options: ClearOptions = {},
): Promise<void> {
  await clearProfileData(queryClient, options);
  queryClient.setQueryData(sessionQuery.queryKey, session);
  applyProfileAccent(session);
  await applyProfileLanguage(session);
}

/**
 * Re-reads the session after the active profile was edited. Rating, libraries, language and
 * accent decide what the catalog shows, so it is treated like a switch to that profile.
 */
export async function refreshSession(queryClient: QueryClient): Promise<void> {
  const session = await queryClient.fetchQuery({ ...sessionQuery, staleTime: 0 });
  if (session) await adoptSession(queryClient, session, { sameAccount: true });
}

const activeRoute = getRouteApi('/_authed/_active');

/** The session of a page behind the profile picker, kept current when the profile is edited. */
export function useActiveSession(): ActiveSession {
  const { session: fromRoute } = activeRoute.useRouteContext();
  const live = useQuery(sessionQuery).data;
  return live?.profile ? { ...live, profile: live.profile } : fromRoute;
}

export function useLogout() {
  const queryClient = useQueryClient();
  const navigate = useNavigate();
  return useMutation({
    mutationFn: () => request(api.POST('/auth/logout')),
    onSettled: async () => {
      await clearProfileData(queryClient);
      queryClient.setQueryData(sessionQuery.queryKey, null);
      applyProfileAccent(null);
      await setLanguage(detectLanguage());
      await navigate({ to: '/login' });
    },
  });
}

/**
 * Activates a profile of the account (K-4); the PIN goes along for a protected one. The PIN never
 * stays in the query client (see useTransientMutation).
 */
export function useSelectProfile() {
  const queryClient = useQueryClient();
  const { run, ...state } = useTransientMutation(
    ({ id, pin }: { id: number; pin?: string }): Promise<SessionInfo> =>
      request(
        api.POST('/auth/profiles/{id}/select', {
          params: { path: { id } },
          body: pin === undefined ? undefined : { pin },
        }),
      ),
    (session) => adoptSession(queryClient, session, { sameAccount: true }),
  );
  return { select: run, ...state };
}
