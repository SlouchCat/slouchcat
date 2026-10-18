import type { components, SessionInfo } from '@slouchcat/api-client';
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

export function useLogout() {
  const queryClient = useQueryClient();
  const navigate = useNavigate();
  return useMutation({
    mutationFn: () => request(api.POST('/auth/logout')),
    onSettled: async () => {
      queryClient.removeQueries({ predicate: (query) => query.queryKey[0] !== 'setup-status' });
      queryClient.setQueryData(sessionQuery.queryKey, null);
      await navigate({ to: '/login' });
    },
  });
}
