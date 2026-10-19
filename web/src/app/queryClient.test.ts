import { describe, expect, it, vi } from 'vitest';
import { ApiError } from '@/api/client';
import { createQueryClient } from './queryClient';

const problem = (status: number, code: string) =>
  new ApiError(status, { type: 'about:blank', title: code, status, code });

describe('createQueryClient', () => {
  it('reacts to 401 and profile_required of queries and of mutations alike', async () => {
    const onUnauthenticated = vi.fn();
    const onProfileRequired = vi.fn();
    const client = createQueryClient({ onUnauthenticated, onProfileRequired });

    await client
      .fetchQuery({
        queryKey: ['a'],
        queryFn: () => Promise.reject(problem(401, 'unauthenticated')),
      })
      .catch(() => undefined);
    await client
      .getMutationCache()
      .build(client, { mutationFn: () => Promise.reject(problem(403, 'profile_required')) })
      .execute(undefined)
      .catch(() => undefined);
    expect(onUnauthenticated).toHaveBeenCalledTimes(1);
    expect(onProfileRequired).toHaveBeenCalledTimes(1);

    // A wrong password is a 401 as well, but the session did not end.
    await client
      .getMutationCache()
      .build(client, { mutationFn: () => Promise.reject(problem(401, 'invalid_credentials')) })
      .execute(undefined)
      .catch(() => undefined);
    expect(onUnauthenticated).toHaveBeenCalledTimes(1);
  });
});
