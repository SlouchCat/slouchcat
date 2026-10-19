import type { components, SessionInfo } from '@slouchcat/api-client';
import { accentNames, type AccentName } from '@slouchcat/design-tokens';
import { queryOptions, useMutation, useQueryClient } from '@tanstack/react-query';
import { useNavigate } from '@tanstack/react-router';
import { api, isApiError, request } from '@/api/client';
import i18n from '@/i18n';

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

/** Switches the UI to the language of the active profile (C-3). */
export async function applyProfileLanguage(session: SessionInfo): Promise<void> {
  if (i18n.resolvedLanguage !== session.profile.language) {
    await i18n.changeLanguage(session.profile.language);
  }
}

const isAccentName = (name: string | undefined): name is AccentName =>
  name !== undefined && (accentNames as readonly string[]).includes(name);

/**
 * Applies the accent of the active profile (`profile.accent`, a token name such as `teal`) to the
 * whole app: `--sc-accent` and `--sc-on-accent` on the root element point at that accent's
 * tokens. Without a session or with an unknown name, the defaults from global.css (ember) apply.
 */
export function applyProfileAccent(session: SessionInfo | null): void {
  const { style } = document.documentElement;
  const name = session?.profile.accent;
  if (isAccentName(name)) {
    style.setProperty('--sc-accent', `var(--sc-color-accent-${name}-base)`);
    style.setProperty('--sc-on-accent', `var(--sc-color-accent-${name}-on)`);
  } else {
    style.removeProperty('--sc-accent');
    style.removeProperty('--sc-on-accent');
  }
}

export function useLogout() {
  const queryClient = useQueryClient();
  const navigate = useNavigate();
  return useMutation({
    mutationFn: () => request(api.POST('/auth/logout')),
    onSettled: async () => {
      queryClient.removeQueries({ predicate: (query) => query.queryKey[0] !== 'setup-status' });
      queryClient.setQueryData(sessionQuery.queryKey, null);
      applyProfileAccent(null);
      await navigate({ to: '/login' });
    },
  });
}
