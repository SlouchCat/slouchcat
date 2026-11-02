import { useCallback, useMemo, useState } from 'react';

const same = (a: unknown, b: unknown) => JSON.stringify(a) === JSON.stringify(b);

export interface Drafts<T> {
  /** The draft of `id`, else what the server holds (`originalOf`). */
  valueOf: (id: string) => T | undefined;
  /**
   * Sets the draft of `id` (a value, or a function of the current one). A value equal to the
   * original drops the draft: changing a field back is no change.
   */
  set: (id: string, next: T | ((current: T) => T)) => void;
  /** `id` has a draft that differs from the original. */
  isDirty: (id: string) => boolean;
  /** Ids with a draft, in the order they were first changed. */
  dirtyIds: readonly string[];
  /** Any draft at all: the page pauses its polling and guards leaving (useUnsavedChanges). */
  dirty: boolean;
  /** Drops the draft of `id` (Verwerfen of one item, or after it was saved). */
  discard: (id: string) => void;
  /** Drops every draft. */
  discardAll: () => void;
}

/**
 * Unsaved input of a page, keyed by the id of the thing it belongs to (concept 12, S-17): a job
 * type, an account, a pick, a collection, a scope. It lives in the page's state, not in a tab or
 * an expandable row, so switching tabs, folding a row away or selecting another item keeps it,
 * and the same item shows the same draft in two tabs (S-13).
 *
 * `originalOf` gives what the server holds for an id; it is read on every call, so a refetch that
 * brings new data shows through wherever there is no draft.
 */
export function useDrafts<T>(originalOf: (id: string) => T | undefined): Drafts<T> {
  const [drafts, setDrafts] = useState<ReadonlyMap<string, T>>(() => new Map());

  const valueOf = useCallback(
    (id: string) => (drafts.has(id) ? drafts.get(id) : originalOf(id)),
    [drafts, originalOf],
  );

  const set = useCallback(
    (id: string, next: T | ((current: T) => T)) =>
      setDrafts((current) => {
        const base = current.has(id) ? current.get(id) : originalOf(id);
        const value = typeof next === 'function' ? (next as (current: T) => T)(base as T) : next;
        const updated = new Map(current);
        if (same(value, originalOf(id))) updated.delete(id);
        else updated.set(id, value);
        return updated;
      }),
    [originalOf],
  );

  const discard = useCallback(
    (id: string) =>
      setDrafts((current) => {
        if (!current.has(id)) return current;
        const updated = new Map(current);
        updated.delete(id);
        return updated;
      }),
    [],
  );

  const discardAll = useCallback(() => setDrafts(new Map()), []);

  return useMemo(() => {
    const dirtyIds = [...drafts.keys()].filter((id) => !same(drafts.get(id), originalOf(id)));
    return {
      valueOf,
      set,
      isDirty: (id: string) => dirtyIds.includes(id),
      dirtyIds,
      dirty: dirtyIds.length > 0,
      discard,
      discardAll,
    };
  }, [drafts, originalOf, valueOf, set, discard, discardAll]);
}
