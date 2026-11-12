import type { ItemRef } from '@slouchcat/api-client';
import { useIsMutating, useMutation, useQueryClient, type QueryKey } from '@tanstack/react-query';
import { lazy, Suspense, useEffect, useState, type ReactNode } from 'react';
import { useTranslation } from 'react-i18next';
import { api, request } from '@/api/client';
import { problemMessage } from '@/api/errors';
import { useOnline } from '@/lib/useOnline';
import { WatchedToggle, type WatchedState } from '@/ui';
import { isAudioKind } from './labels';
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

/** A title in a query's data: an object with the id, a kind and images (an ItemRef or more). */
const isTitle = (value: Record<string, unknown>, id: string) =>
  value.id === id && typeof value.kind === 'string' && 'images' in value;

/**
 * Replaces every title `id` in a query's data by what `replace` makes of it, walking plain objects
 * and arrays only and depth first, so the titles come in the same order on every walk; every
 * untouched subtree stays the same object (structural sharing).
 */
function replaceTitle(
  value: unknown,
  id: string,
  replace: (title: Record<string, unknown>) => Record<string, unknown>,
): unknown {
  if (Array.isArray(value)) {
    let touched = false;
    const next = value.map((entry: unknown) => {
      const patched = replaceTitle(entry, id, replace);
      if (patched !== entry) touched = true;
      return patched;
    });
    return touched ? next : value;
  }
  if (!isPlainObject(value)) return value;
  let next: Record<string, unknown> | undefined;
  for (const [key, entry] of Object.entries(value)) {
    const patched = replaceTitle(entry, id, replace);
    if (patched !== entry) {
      next ??= { ...value };
      next[key] = patched;
    }
  }
  if (isTitle(value, id)) return replace(next ?? value);
  return next ?? value;
}

/** The originals a change replaced in one query's data, in walk order (useWatchedMutation). */
interface Patched {
  data: unknown;
  originals: Record<string, unknown>[];
}

/** Sets the check mark of title `id` in a query's data and keeps what it replaced. */
function patchWithOriginals(
  data: unknown,
  id: string,
  on: boolean,
  now: string,
): Patched | undefined {
  const originals: Record<string, unknown>[] = [];
  const next = replaceTitle(data, id, (title) => {
    originals.push(title);
    if (watchedState(title as unknown as WatchedItem) === (on ? 'full' : 'none')) return title;
    return changed(title, on, now);
  });
  return next === data ? undefined : { data: next, originals };
}

/**
 * Sets the check mark of title `id` wherever it stands in a query's data (as patchWatchlist does
 * for the watchlist): the title itself only; its episodes or its series follow when the queries
 * are read again. Undefined when nothing changed.
 */
export function patchWatched(data: unknown, id: string, on: boolean, now: string): unknown {
  return patchWithOriginals(data, id, on, now)?.data;
}

/**
 * Puts the titles a change replaced back, exactly as they were (a half series stays half, a
 * second run keeps its position); undefined when the query no longer holds as many of them.
 */
export function restoreWatched(
  data: unknown,
  id: string,
  originals: readonly Record<string, unknown>[],
): unknown {
  let index = 0;
  const next = replaceTitle(data, id, (title) => originals[index++] ?? title);
  return index === originals.length ? next : undefined;
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
 * stands at once wherever the title is cached; an error puts back the titles exactly as they were.
 * After the last change in flight every query that shows check marks is read again
 * (SHOWS_CHECKS), without waiting for them.
 */
export function useWatchedMutation() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationKey: WATCHED_MUTATION_KEY,
    mutationFn: ({ id, on }: WatchedChange) =>
      request(api.PUT('/items/{id}/watched', { params: { path: { id } }, body: { watched: on } })),
    onMutate: async ({ id, on }) => {
      const now = new Date().toISOString();
      // Each query patched once; what it replaced is kept for an error (restoreWatched).
      const changes = queryClient
        .getQueryCache()
        .findAll()
        .flatMap((query) => {
          const patched = patchWithOriginals(query.state.data, id, on, now);
          return patched ? [{ queryKey: query.queryKey, ...patched }] : [];
        });
      await Promise.all(
        changes.map(({ queryKey }) => queryClient.cancelQueries({ queryKey, exact: true })),
      );
      for (const { queryKey, data } of changes) queryClient.setQueryData(queryKey, data);
      return { changes: changes.map(({ queryKey, originals }) => ({ queryKey, originals })) };
    },
    onError: (_error, { id }, context) => {
      for (const { queryKey, originals } of context?.changes ?? []) {
        const back = restoreWatched(queryClient.getQueryData(queryKey), id, originals);
        if (back === undefined) void queryClient.invalidateQueries({ queryKey, exact: true });
        else queryClient.setQueryData(queryKey, back);
      }
    },
    // Not awaited: the button is free again as soon as the server answered, the lists follow.
    // Only after the last change in flight, so a quick second press does not see its own state
    // read back over by the answer to the first.
    onSettled: () => {
      if (queryClient.isMutating({ mutationKey: WATCHED_MUTATION_KEY }) > 1) return;
      void queryClient.invalidateQueries({ predicate: (query) => showsChecks(query.queryKey) });
    },
  });
}

/** The texts of a check mark that means «gesehen». */
const SEEN = {
  label: 'watched.label',
  button: 'watched.button',
  set: 'watched.hint.set',
  half: 'watched.hint.half',
  remove: 'watched.hint.remove',
  full: 'watched.state.full',
  partly: 'watched.state.half',
  removeTitle: 'watched.ask.removeTitle',
  halfTitle: 'watched.ask.halfTitle',
  markAll: 'watched.ask.markAll',
} as const;

/** …and the same for audio, which is heard (`watched.heard.*`). */
const HEARD = {
  ...SEEN,
  label: 'watched.heard.label',
  button: 'watched.heard.button',
  set: 'watched.heard.hint.set',
  half: 'watched.heard.hint.half',
  full: 'watched.heard.state.full',
  partly: 'watched.heard.state.half',
  removeTitle: 'watched.heard.ask.removeTitle',
  halfTitle: 'watched.heard.ask.halfTitle',
  markAll: 'watched.heard.ask.markAll',
} as const;

/** How long the note on why a change failed stays beside the check mark. */
const ERROR_SHOWN_MS = 5000;

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
  /** The spoken name ("Gesehen: Nordlicht", "Gehört: …" for audio), the same in every state. */
  label: string;
  /** The text of the detail page's round button: «Gesehen», for audio «Gehört». */
  buttonLabel: string;
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
  // Audiobooks, podcasts and their episodes are heard, not watched (coordinator's decision of
  // 12 November 2026); the page keeps its name «Gesehen».
  const texts = isAudioKind(item.kind) ? HEARD : SEEN;

  // Why a change failed stands for a few seconds, and goes with the next press.
  const { isError, reset } = mutation;
  useEffect(() => {
    if (!isError) return;
    const timer = window.setTimeout(reset, ERROR_SHOWN_MS);
    return () => window.clearTimeout(timer);
  }, [isError, reset]);

  const change = (on: boolean) =>
    mutation.mutate({ id: item.id, on }, { onSuccess: on ? undefined : options.onRemoved });
  const ask = (what: 'remove' | 'half') => {
    setAsking(what);
    setOpen(true);
  };
  const press = () => {
    if (busy || !online) return;
    reset();
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
        title={t(asking === 'half' ? texts.halfTitle : texts.removeTitle, {
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
        markAllLabel={asking === 'half' ? t(texts.markAll) : undefined}
        onMarkAll={asking === 'half' ? () => change(true) : undefined}
      />
    </Suspense>
  );

  return {
    state,
    readOnly: !online,
    busy,
    label: t(texts.label, { title: item.title }),
    buttonLabel: t(texts.button),
    hint: t(state === 'none' ? texts.set : state === 'half' ? texts.half : texts.remove),
    stateLabel: t(state === 'half' ? texts.partly : texts.full),
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
