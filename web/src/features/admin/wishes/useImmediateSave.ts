import { useMutation } from '@tanstack/react-query';
import { useCallback, useRef, useState } from 'react';
import { useStatusMessage, type StatusMessage } from '@/ui/aria';

interface ImmediateSaveOptions<TPatch, TData> {
  /** Stores one change; `key` names the control it came from. */
  save: (patch: TPatch, key: string) => Promise<TData>;
  /**
   * After the server took the change: put its answer into the query client, and say in a short
   * sentence what now applies (the outcome line).
   */
  saved: (data: TData, patch: TPatch, key: string) => Promise<string> | string;
}

export interface ImmediateSave<TPatch> {
  /** Stores `patch` for the control `key` at once; the control shows it until the answer. */
  change: (key: string, patch: TPatch) => void;
  /** The changes on their way, by control: a control shows its own instead of the stored value. */
  pending: ReadonlyMap<string, TPatch>;
  /**
   * The last change the server refused, with its control (that control shows the stored value
   * again) and a number of its own, so a control can start afresh after each refusal.
   */
  failure: { key: string; error: unknown; id: number } | null;
  /** The outcome of the last change that went through. */
  message: StatusMessage | null;
}

/**
 * Controls that take effect at once, without «Speichern» or «Verwerfen» (concept 25, M-24 and
 * M-25, administration 6 and 7): a change goes to the server the moment it is made, the control
 * shows the new value while it travels, and a short message confirms it. A refused change rolls
 * the control back to what the server holds and keeps the server's reason for the page to show.
 * Each change resolves on its own, so two quick changes of different controls do not wait for
 * each other.
 */
export function useImmediateSave<TPatch, TData>({
  save,
  saved,
}: ImmediateSaveOptions<TPatch, TData>): ImmediateSave<TPatch> {
  const [message, say] = useStatusMessage();
  const [pending, setPending] = useState<ReadonlyMap<string, TPatch>>(() => new Map());
  const [failure, setFailure] = useState<{ key: string; error: unknown; id: number } | null>(null);
  // Counts the refusals, also across a cleared failure: an id is never used twice.
  const refusals = useRef(0);
  const { mutateAsync } = useMutation({
    mutationFn: ({ patch, key }: { patch: TPatch; key: string }) => save(patch, key),
  });

  const settle = useCallback((key: string) => {
    setPending((current) => {
      if (!current.has(key)) return current;
      const next = new Map(current);
      next.delete(key);
      return next;
    });
  }, []);

  const change = useCallback(
    (key: string, patch: TPatch) => {
      say(null);
      setFailure(null);
      setPending((current) => new Map(current).set(key, patch));
      void (async () => {
        try {
          const data = await mutateAsync({ patch, key });
          // The query holds the answer before the control lets go of its own value: no flicker.
          say(await saved(data, patch, key));
        } catch (error) {
          refusals.current += 1;
          setFailure({ key, error, id: refusals.current });
        } finally {
          settle(key);
        }
      })();
    },
    [mutateAsync, saved, say, settle],
  );

  return { change, pending, failure, message };
}
