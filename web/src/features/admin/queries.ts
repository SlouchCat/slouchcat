import type { Guest, Invitation, Library } from '@slouchcat/api-client';
import { queryOptions } from '@tanstack/react-query';
import { api, request } from '@/api/client';

/** Poll interval while a scan is queued or running. */
export const SCAN_POLL_MS = 5000;

export const isScanning = (library: Library) =>
  library.scan.state === 'queued' || library.scan.state === 'running';

export const librariesQuery = queryOptions({
  queryKey: ['admin', 'libraries'],
  queryFn: ({ signal }): Promise<Library[]> => request(api.GET('/admin/libraries', { signal })),
  refetchInterval: (query) => (query.state.data?.some(isScanning) ? SCAN_POLL_MS : false),
});

export const invitationsQuery = queryOptions({
  queryKey: ['admin', 'invitations'],
  queryFn: ({ signal }): Promise<Invitation[]> =>
    request(api.GET('/admin/invitations', { signal })),
});

export const guestsQuery = queryOptions({
  queryKey: ['admin', 'guests'],
  queryFn: ({ signal }): Promise<Guest[]> => request(api.GET('/admin/guests', { signal })),
});
