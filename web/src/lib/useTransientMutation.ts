import { useMutation } from '@tanstack/react-query';
import { useCallback, useState } from 'react';

export type Outcome<TData> = { ok: true; data: TData } | { ok: false; error: unknown };

interface TransientMutation<TData, TVariables> {
  /** Runs the request and resolves with its outcome (it never rejects). */
  run: (variables: TVariables) => Promise<Outcome<TData>>;
  isPending: boolean;
  /** The error of the last run, or null. */
  error: unknown;
  /** Counter that changes with every failed run, to restart timers on the same error. */
  failures: number;
}

/**
 * A mutation whose variables never stay in the query client: for secrets such as a PIN. The
 * mutation is dropped at once (`gcTime: 0`) and reset when it settles, so the mutation cache
 * holds nothing afterwards; the outcome lives in this hook's own state. `onSuccess` runs after
 * the reset.
 */
export function useTransientMutation<TData, TVariables>(
  mutationFn: (variables: TVariables) => Promise<TData>,
  onSuccess?: (data: TData, variables: TVariables) => Promise<void> | void,
): TransientMutation<TData, TVariables> {
  const mutation = useMutation({ mutationFn, gcTime: 0 });
  const { mutateAsync, reset } = mutation;
  const [error, setError] = useState<unknown>(null);
  const [failures, setFailures] = useState(0);

  const run = useCallback(
    async (variables: TVariables): Promise<Outcome<TData>> => {
      setError(null);
      let data: TData;
      try {
        data = await mutateAsync(variables);
      } catch (caught) {
        reset();
        setError(caught);
        setFailures((count) => count + 1);
        return { ok: false, error: caught };
      }
      reset();
      await onSuccess?.(data, variables);
      return { ok: true, data };
    },
    [mutateAsync, reset, onSuccess],
  );

  return { run, isPending: mutation.isPending, error, failures };
}
