import { MutationCache, QueryCache, QueryClient } from '@tanstack/react-query';
import { isApiError } from '@/api/client';

interface QueryClientOptions {
  /** Called when any query answers 401 (session expired or revoked). */
  onUnauthenticated?: () => void;
  /** Called when a request answers 403 `profile_required` (the profile was deleted meanwhile). */
  onProfileRequired?: () => void;
}

export function createQueryClient({
  onUnauthenticated,
  onProfileRequired,
}: QueryClientOptions = {}): QueryClient {
  // Queries and mutations alike: a 401 means the session ended (a wrong password at the login is
  // a 401 too, but not that), `profile_required` that the active profile is gone.
  const onError = (error: Error) => {
    if (isApiError(error, 401) && error.code !== 'invalid_credentials') onUnauthenticated?.();
    if (isApiError(error, 403) && error.code === 'profile_required') onProfileRequired?.();
  };
  return new QueryClient({
    queryCache: new QueryCache({ onError }),
    mutationCache: new MutationCache({ onError }),
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
