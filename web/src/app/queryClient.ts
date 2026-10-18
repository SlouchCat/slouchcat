import { QueryCache, QueryClient } from '@tanstack/react-query';
import { isApiError } from '@/api/client';

interface QueryClientOptions {
  /** Called when any query answers 401 (session expired or revoked). */
  onUnauthenticated?: () => void;
}

export function createQueryClient({ onUnauthenticated }: QueryClientOptions = {}): QueryClient {
  return new QueryClient({
    queryCache: new QueryCache({
      onError: (error) => {
        if (isApiError(error, 401)) onUnauthenticated?.();
      },
    }),
    defaultOptions: {
      queries: {
        // Client errors (404, 401, 422) are answers, not hiccups: never retry them.
        retry: (failureCount, error) =>
          !(isApiError(error) && error.status < 500) && failureCount < 1,
        refetchOnWindowFocus: true,
      },
    },
  });
}
