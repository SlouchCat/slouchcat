import type { ItemRef } from '@slouchcat/api-client';
import { useIsMutating, useMutation, useQueryClient, type QueryKey } from '@tanstack/react-query';
import { lazy, Suspense, useState, type ReactNode } from 'react';
import { useTranslation } from 'react-i18next';
import { api, request } from '@/api/client';
import { problemMessage } from '@/api/errors';
import { useOnline } from '@/lib/useOnline';
import { WatchedToggle, type WatchedState } from '@/ui';
import styles from './watched.module.css';

// The question comes with react-aria: a chunk of its own, loaded on the first press that asks, so
// the browsing pages stay without it (P-10).
const WatchedChoice = lazy(() =>
  import('@/ui/aria').then((module) => ({ default: module.WatchedChoice })),
);

/** What the check mark of a title reads. */
export type WatchedItem = Pick<
  ItemRef,
  'id' | 'kind' | 'title' | 'progress' | 'watched' | 'watchedCount' | 'childCount'
>;

/**
 * The kinds with a check mark (concept 20 P-5): every title the page «Gesehen» or a series counts;
 * not extras.
 */
export const WATCHED_KINDS: ReadonlySet<string> = new Set([
  'movie',
  'episode',
  'audiobook',
  'podcast_episode',
  'season',
  'series',
  'podcast',
]);

/** A season, series or podcast: its check mark stands for its visible episodes (F-12). */
const isGroup = (kind: ItemRef['kind']) =>
  kind === 'season' || kind === 'series' || kind === 'podcast';

/**
 * The state of a title's check mark (concept 20 F-12, section 6.3), as the server says it: a
 * single title by `progress.watchedAt`; a series or podcast full by `watched`, half by
 * `watchedCount`; a season full when every visible episode is watched.
 */
export function watchedState(item: WatchedItem): WatchedState {
  if (!isGroup(item.kind)) return item.progress?.watchedAt ? 'full' : 'none';
  const count = item.watchedCount ?? 0;
  const full =
    item.kind === 'season'
      ? item.childCount !== undefined && item.childCount > 0 && count >= item.childCount
      : item.watched === true;
  return full ? 'full' : count > 0 ? 'half' : 'none';
}

const isPlainObject = (value: unknown): value is Record<string, unknown> => {
  if (typeof value !== 'object' || value === null) return false;
  const prototype = Object.getPrototypeOf(value) as unknown;
  return prototype === Object.prototype || prototype === null;
};

/** The title as the server will have it after the change (setItemWatched, section 6.4). */
function changed(
  value: Record<string, unknown>,
  on: boolean,
  now: string,
): Record<string, unknown> {
  const item = value as unknown as ItemRef;
  if (isGroup(item.kind)) {
    const next: Record<string, unknown> = { ...value };
    if (item.kind !== 'season') next.watched = on;
    if (on) {
      if (item.kind === 'season' && item.childCount !== undefined)
        next.watchedCount = item.childCount;
    } else {
      next.watchedCount = 0;
    }
    return next;
  }
  const progress = item.progress;
  if (on) {
    return {
      ...value,
      progress: {
        positionMs: progress?.durationMs ?? 0,
        durationMs: progress?.durationMs ?? 0,
        percent: 100,
        ...(progress?.playbackRate !== undefined ? { playbackRate: progress.playbackRate } : {}),
        completed: true,
        updatedAt: now,
        watchedAt: now,
      },
    };
  }
  if (!progress) return value;
  // A finished title starts again at 0; a running position of a second run stays.
  const rest = { ...progress };
  delete rest.watchedAt;
  return {
    ...value,
    progress: progress.completed ? { ...rest, completed: false, positionMs: 0, percent: 0 } : rest,
  };
}

function walk(value: unknown, id: string, on: boolean, now: string): unknown {
  if (Array.isArray(value)) {
    let touched = false;
    const next = value.map((entry: unknown) => {
      const patched = walk(entry, id, on, now);
      if (patched !== entry) touched = true;
      return patched;
    });
    return touched ? next : value;
  }
  if (!isPlainObject(value)) return value;
  let next: Record<string, unknown> | undefined;
  for (const [key, entry] of Object.entries(value)) {
    const patched = walk(entry, id, on, now);
    if (patched !== entry) {
      next ??= { ...value };
      next[key] = patched;
    }
  }
  if (value.id === id && typeof value.kind === 'string' && 'images' in value) {
    const item = (next ?? value) as unknown as WatchedItem;
    if (watchedState(item) !== (on ? 'full' : 'none')) next = changed(next ?? value, on, now);
  }
  return next ?? value;
}

/**
 * Sets the check mark of title `id` wherever it stands in a query's data (as patchWatchlist does
 * for the watchlist): the title itself only; its episodes or its series follow when the queries
 * are read again. Undefined when nothing changed.
 */
export function patchWatched(data: unknown, id: string, on: boolean, now: string): unknown {
  const next = walk(data, id, on, now);
  return next === data ? undefined : next;
}

/** Every check mark change shares this key, so a second button of the same title sees it. */
const WATCHED_MUTATION_KEY = ['watched', 'set'] as const;

interface WatchedChange {
  id: string;
  on: boolean;
}

/**
 * The lists and pages that show check marks: a change of one episode moves its season, its series
 * and their cards too, so all of them are read again (the ones on screen at once, the others when
 * they are next shown). Trailers never change.
 */
const SHOWS_CHECKS = new Set([
  'item',
  'home',
  'library',
  'watched',
  'watchlist',
  'search',
  'collection',
  'person',
  'picks',
]);

const showsChecks = (queryKey: QueryKey) =>
  SHOWS_CHECKS.has(String(queryKey[0])) && !(queryKey[0] === 'item' && queryKey[2] === 'trailers');

/** Whether a check mark change of title `id` is in flight, from any button. */
export function useWatchedPending(id: string): boolean {
  return (
    useIsMutating({
      mutationKey: WATCHED_MUTATION_KEY,
      predicate: (mutation) => (mutation.state.variables as WatchedChange | undefined)?.id === id,
    }) > 0
  );
}

/**
 * Sets or removes the check mark of a title (setItemWatched, concept 20 P-5). The new state
 * stands at once wherever the title is cached; an error turns it back. Afterwards every query that
 * shows check marks is read again (SHOWS_CHECKS), without waiting for them.
 */
export function useWatchedMutation() {
  const queryClient = useQueryClient();
  const patch = (keys: readonly QueryKey[], id: string, on: boolean, now: string) => {
    for (const queryKey of keys) {
      queryClient.setQueryData(queryKey, (data: unknown) => patchWatched(data, id, on, now));
    }
  };
  return useMutation({
    mutationKey: WATCHED_MUTATION_KEY,
    mutationFn: ({ id, on }: WatchedChange) =>
      request(api.PUT('/items/{id}/watched', { params: { path: { id } }, body: { watched: on } })),
    onMutate: async ({ id, on }) => {
      const now = new Date().toISOString();
      const keys = queryClient
        .getQueryCache()
        .findAll()
        .filter((query) => patchWatched(query.state.data, id, on, now) !== undefined)
        .map((query) => query.queryKey);
      await Promise.all(
        keys.map((queryKey) => queryClient.cancelQueries({ queryKey, exact: true })),
      );
      patch(keys, id, on, now);
      return { keys, now };
    },
    onError: (_error, { id, on }, context) => {
      if (context) patch(context.keys, id, !on, context.now);
    },
    // Not awaited: the button is free again as soon as the server answered, the lists follow.
    onSettled: () => {
      void queryClient.invalidateQueries({ predicate: (query) => showsChecks(query.queryKey) });
    },
  });
}

interface WatchedOptions {
  /**
   * Called once a removal went through, while the card still stands: the page «Gesehen» moves the
   * focus to the neighbour before the card goes.
   */
  onRemoved?: () => void;
}

/** The behaviour of a check mark: what a press does, the question and the texts. */
export interface WatchedToggleModel {
  state: WatchedState;
  /** Offline: a display only (rule 12, the server decides). */
  readOnly: boolean;
  busy: boolean;
  /** The spoken name ("Gesehen: Nordlicht"), the same in every state. */
  label: string;
  /** What a press does, as the pointer's hint. */
  hint: string;
  /** The name of the display while read only ("Gesehen", "Teilweise gesehen"). */
  stateLabel: string;
  press: () => void;
  /** The question of a season, series or podcast; render it once beside the button. */
  dialog: ReactNode;
  error: string | null;
}

/**
 * The check mark of a title (concept 20 P-5, 6.2): setting it never asks, even for a whole series;
 * removing it asks only for a season, series or podcast («Bei 19 Folgen fällt der Haken …»); a
 * half check asks «Alle Folgen als gesehen markieren» or «Alle Haken entfernen». Offline it is a
 * display only.
 */
export function useWatchedToggle(
  item: WatchedItem,
  options: WatchedOptions = {},
): WatchedToggleModel {
  const { t } = useTranslation();
  const online = useOnline();
  const mutation = useWatchedMutation();
  const busy = useWatchedPending(item.id);
  const [asking, setAsking] = useState<'remove' | 'half' | null>(null);
  const [open, setOpen] = useState(false);
  const state = watchedState(item);
  const count = item.watchedCount ?? 0;

  const change = (on: boolean) =>
    mutation.mutate({ id: item.id, on }, { onSuccess: on ? undefined : options.onRemoved });
  const ask = (what: 'remove' | 'half') => {
    setAsking(what);
    setOpen(true);
  };
  const press = () => {
    if (busy || !online) return;
    if (state === 'none') change(true);
    else if (state === 'half') ask('half');
    else if (isGroup(item.kind)) ask('remove');
    else change(false);
  };

  const dialog = asking && (
    <Suspense fallback={null}>
      <WatchedChoice
        isOpen={open}
        onOpenChange={setOpen}
        title={t(asking === 'half' ? 'watched.ask.halfTitle' : 'watched.ask.removeTitle', {
          title: item.title,
        })}
        body={
          asking === 'half'
            ? t('watched.ask.halfBody', { count })
            : count > 0
              ? t('watched.ask.removeBody', { count })
              : t('watched.ask.removeBodyAll')
        }
        cancelLabel={t('common.cancel')}
        removeLabel={t('watched.ask.removeAll')}
        onRemove={() => change(false)}
        markAllLabel={asking === 'half' ? t('watched.ask.markAll') : undefined}
        onMarkAll={asking === 'half' ? () => change(true) : undefined}
      />
    </Suspense>
  );

  return {
    state,
    readOnly: !online,
    busy,
    label: t('watched.label', { title: item.title }),
    hint: t(
      state === 'none'
        ? 'watched.hint.set'
        : state === 'half'
          ? 'watched.hint.half'
          : 'watched.hint.remove',
    ),
    stateLabel: t(state === 'half' ? 'watched.state.half' : 'watched.state.full'),
    press,
    dialog,
    error: mutation.isError ? problemMessage(t, mutation.error) : null,
  };
}

interface CoverWatchedProps extends WatchedOptions {
  item: WatchedItem;
  /** `cover` over a picture (cards, season covers), `row` at the end of an episode row. */
  appearance?: 'cover' | 'row';
  /** For a row with one Tab stop (the season shelf's and the spotlight row's roving tabindex). */
  excludeFromTabOrder?: boolean;
}

/**
 * The check mark of a title on its cover or at the end of its row (concept 20 P-5): the toggle,
 * its question and why a change failed. Nothing for an extra.
 */
export function CoverWatched({
  item,
  appearance = 'cover',
  excludeFromTabOrder,
  onRemoved,
}: CoverWatchedProps) {
  const model = useWatchedToggle(item, { onRemoved });
  if (!WATCHED_KINDS.has(item.kind)) return null;
  return (
    <span className={styles.root}>
      <WatchedToggle
        state={model.state}
        label={model.label}
        title={model.hint}
        appearance={appearance}
        readOnly={model.readOnly}
        stateLabel={model.stateLabel}
        busy={model.busy}
        onPress={model.press}
        excludeFromTabOrder={excludeFromTabOrder}
      />
      {model.error && (
        <span className={styles.error} role="alert">
          {model.error}
        </span>
      )}
      {model.dialog}
    </span>
  );
}
