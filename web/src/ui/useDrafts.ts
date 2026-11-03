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
 * `originalOf` gives what the server holds for an id; it is read on every call. A draft counts
 * only while it differs from that: when the server comes to hold the same value (a save, another
 * administrator), the draft is spent and dropped, and later changes on the server show through.
 * An id the server no longer knows (`originalOf` gives undefined: the item was deleted) loses its
 * draft the same way.
 * Drafts belong to existing items; a form that creates one keeps its own state.
 */
export function useDrafts<T>(originalOf: (id: string) => T | undefined): Drafts<T> {
  const [drafts, setDrafts] = useState<ReadonlyMap<string, T>>(() => new Map());

  /** The draft of `id` while it is one: the item exists and the draft differs from it. */
  const liveDraft = useCallback(
    (map: ReadonlyMap<string, T>, id: string): { value: T } | undefined => {
      if (!map.has(id)) return undefined;
      const original = originalOf(id);
      const value = map.get(id) as T;
      return original === undefined || same(value, original) ? undefined : { value };
    },
    [originalOf],
  );

  const valueOf = useCallback(
    (id: string) => {
      const live = liveDraft(drafts, id);
      return live ? live.value : originalOf(id);
    },
    [drafts, liveDraft, originalOf],
  );

  const set = useCallback(
    (id: string, next: T | ((current: T) => T)) =>
      setDrafts((current) => {
        const live = liveDraft(current, id);
        const base = live ? live.value : originalOf(id);
        const value = typeof next === 'function' ? (next as (current: T) => T)(base as T) : next;
        const updated = new Map(current);
        if (same(value, originalOf(id))) updated.delete(id);
        else updated.set(id, value);
        return updated;
      }),
    [liveDraft, originalOf],
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

  // Spent drafts (the server holds the same value now, or the item is gone) are dropped during
  // the render that sees them, so a later change on the server is not hidden behind them. React
  // renders again at once with the pruned map; the next pass finds nothing to prune.
  const spent = [...drafts.keys()].filter((id) => liveDraft(drafts, id) === undefined);
  if (spent.length > 0) {
    const pruned = new Map(drafts);
    for (const id of spent) pruned.delete(id);
    setDrafts(pruned);
  }

  return useMemo(() => {
    const dirtyIds = [...drafts.keys()].filter((id) => liveDraft(drafts, id) !== undefined);
    return {
      valueOf,
      set,
      isDirty: (id: string) => dirtyIds.includes(id),
      dirtyIds,
      dirty: dirtyIds.length > 0,
      discard,
      discardAll,
    };
  }, [drafts, liveDraft, valueOf, set, discard, discardAll]);
}
