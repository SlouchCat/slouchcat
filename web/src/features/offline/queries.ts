import { queryOptions } from '@tanstack/react-query';
import { api, request } from '@/api/client';
import type { DownloadOptions, DownloadRules } from './types';

/** Download profiles with size estimates of a title (O-1); a season sums its episodes. */
export const downloadOptionsQuery = (id: string, mediaFileId?: string) =>
  queryOptions({
    queryKey: ['item', id, 'download-options', mediaFileId ?? null],
    queryFn: ({ signal }): Promise<DownloadOptions> =>
      request(
        api.GET('/items/{id}/download-options', {
          params: { path: { id }, query: mediaFileId ? { mediaFileId } : {} },
          signal,
        }),
      ),
  });

/** The auto-download rule of the active profile on this device (O-3). */
export const downloadRulesQuery = queryOptions({
  queryKey: ['offline', 'rules'],
  queryFn: ({ signal }): Promise<DownloadRules> => request(api.GET('/offline/rules', { signal })),
});
