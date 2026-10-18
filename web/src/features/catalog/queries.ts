import type { HomePage, ItemDetail } from '@slouchcat/api-client';
import { queryOptions } from '@tanstack/react-query';
import { api, request } from '@/api/client';

export const homeQuery = queryOptions({
  queryKey: ['home'],
  queryFn: ({ signal }): Promise<HomePage> => request(api.GET('/home', { signal })),
  // The router guard prefetches the start page; the page mounting right after must not ask again.
  // The player invalidates it after playback.
  staleTime: 30_000,
});

export const itemQuery = (id: string) =>
  queryOptions({
    queryKey: ['item', id],
    queryFn: ({ signal }): Promise<ItemDetail> =>
      request(api.GET('/items/{id}', { params: { path: { id } }, signal })),
  });
