import type { HomePage, ItemDetail, Person, Trailer } from '@slouchcat/api-client';
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

/** Trailers of a title, the profile language first (W-10). */
export const trailersQuery = (id: string) =>
  queryOptions({
    queryKey: ['item', id, 'trailers'],
    queryFn: ({ signal }): Promise<Trailer[]> =>
      request(api.GET('/items/{id}/trailers', { params: { path: { id } }, signal })),
  });

/** A person with the credits the active profile may see; 404 when none is visible. */
export const personQuery = (id: number) =>
  queryOptions({
    queryKey: ['person', id],
    queryFn: ({ signal }): Promise<Person> =>
      request(api.GET('/people/{id}', { params: { path: { id } }, signal })),
  });
