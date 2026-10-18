import type { Health } from '@slouchcat/api-client';
import { useQuery } from '@tanstack/react-query';
import { api } from '@/api/client';

export const healthQueryKey = ['health'] as const;

/** Polls getHealth; the query fails when the server is unreachable or not ready. */
export function useHealth() {
  return useQuery({
    queryKey: healthQueryKey,
    queryFn: async ({ signal }): Promise<Health> => {
      const { data, error } = await api.GET('/healthz', { signal });
      if (!data) throw new Error('Health check failed', { cause: error });
      return data;
    },
    refetchInterval: 30_000,
  });
}
