import type { HomePage, ItemDetail } from '@slouchcat/api-client';
import { queryOptions } from '@tanstack/react-query';
import { api, request } from '@/api/client';

export const homeQuery = queryOptions({
  queryKey: ['home'],
  queryFn: ({ signal }): Promise<HomePage> => request(api.GET('/home', { signal })),
});

export const itemQuery = (id: string) =>
  queryOptions({
    queryKey: ['item', id],
    queryFn: ({ signal }): Promise<ItemDetail> =>
      request(api.GET('/items/{id}', { params: { path: { id } }, signal })),
  });
