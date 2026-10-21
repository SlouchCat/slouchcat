import type { ProfileAnnouncement } from '@slouchcat/api-client';
import { queryOptions } from '@tanstack/react-query';
import { api, request } from '@/api/client';

// Announcements of the active profile (M8, A-7). AppLayout asks for them on every start, so this
// module stays apart from the channel queries of the settings page (start bundle, P-10).

/**
 * Published announcements the profile has not acknowledged, newest first (GET /announcements).
 * Asked once per profile: a new one shows at the next start (A-7), not in the middle of a visit.
 */
export const announcementsQuery = (profileId: number) =>
  queryOptions({
    queryKey: ['announcements', profileId],
    queryFn: ({ signal }): Promise<ProfileAnnouncement[]> =>
      request(api.GET('/announcements', { signal })),
    staleTime: Infinity,
    refetchOnWindowFocus: false,
  });

export const ackAnnouncement = (id: number): Promise<void> =>
  request(api.POST('/announcements/{id}/ack', { params: { path: { id } } }));
