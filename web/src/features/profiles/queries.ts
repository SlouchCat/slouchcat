import type { Profile, ProfileSummary } from '@slouchcat/api-client';
import { queryOptions } from '@tanstack/react-query';
import { api, request } from '@/api/client';

/** Profiles of the account as the picker shows them (works before a profile is chosen). */
export const myProfilesQuery = queryOptions({
  queryKey: ['auth', 'profiles'],
  queryFn: ({ signal }): Promise<ProfileSummary[]> =>
    request(api.GET('/auth/profiles', { signal })),
  // The list is what the viewer is about to pick from: ask again every time the picker opens.
  staleTime: 0,
});

/** Profiles with their settings, for the primary profile's management page (K-2). */
export const manageProfilesQuery = queryOptions({
  queryKey: ['profiles'],
  queryFn: ({ signal }): Promise<Profile[]> => request(api.GET('/profiles', { signal })),
});
