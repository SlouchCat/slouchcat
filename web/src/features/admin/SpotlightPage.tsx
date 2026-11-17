import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { lazy, Suspense, useCallback, useEffect, useMemo, useRef, useState } from 'react';
import type { TFunction } from 'i18next';
import { useTranslation } from 'react-i18next';
import { problemMessage } from '@/api/errors';
import { paths } from '@/app/paths';
import { useActiveSession } from '@/app/session';
import { useDocumentTitle } from '@/lib/useDocumentTitle';
import type { Wish } from '@/features/wishes/queries';
import { Chip, FactList, Notice, Skeleton, Text, TextLink } from '@/ui';
import {
  AdminPage,
  AriaButton,
  BandPanel,
  ConfirmDialog,
  DataTable,
  StatusRegion,
  afterDialogsClosed,
  focusTitleIfLost,
  useDrafts,
  useStatusMessage,
  useUnsavedChanges,
} from '@/ui/aria';
import {
  dashboardQuery,
  overrideEntry,
  spotlightConfigQuery,
  spotlightEntriesQuery,
  traktStatusQuery,
  wishKeys,
  type SpotlightEntries,
  type SpotlightEntry,
  type SpotlightEntryOverride,
} from './queries';
import {
  entryColumns,
  entryKey,
  entryName,
  isMissing,
  missingColumns,
  orphanColumns,
  unmatchedColumns,
} from './spotlight/columns';
import { NoteDialog, PinDialog } from './spotlight/EntryDialogs';
import { RowChip, rowState } from './spotlight/FetchState';
import styles from './spotlight/Spotlight.module.css';
import {
  SOURCE_DRAFT,
  SourceForm,
  sourceChanged,
  sourceDraftOf,
  sourceMedia,
  type SourceDraft,
} from './spotlight/SourceForm';

const RequestDialog = lazy(() =>
  import('@/features/wishes/RequestDialog').then((module) => ({ default: module.RequestDialog })),
);

/** The ways a library title is found (N-61), in the order the band «Abgleich» counts them. */
const WAYS = [
  ['movie', 'tmdb', 'admin.spotlight.matching.way.movie.tmdb'],
  ['movie', 'imdb', 'admin.spotlight.matching.way.movie.imdb'],
  ['show', 'tvdb', 'admin.spotlight.matching.way.show.tvdb'],
  ['show', 'tmdb', 'admin.spotlight.matching.way.show.tmdb'],
  ['show', 'imdb', 'admin.spotlight.matching.way.show.imdb'],
] as const;

/** «2 fehlen · 1 angefordert · 0 nicht zuordenbar», the head of the band «Fehlende Einträge». */
const missingSummary = (t: TFunction, missing: NonNullable<SpotlightEntries['missing']>) =>
  [
    t('admin.spotlight.lacking.missing', { count: missing.entries }),
    t('admin.spotlight.lacking.requested', { count: missing.requested }),
    t('admin.spotlight.lacking.unmatched', { count: missing.unmatched }),
  ].join(t('admin.spotlight.state.separator'));

/** After "Jetzt abrufen" the status and the entries are asked again this often for a while. */
const POLL_MS = 2000;
const POLL_FOR_MS = 30_000;

const sameEntry = (a: SpotlightEntry, b: SpotlightEntry) => entryKey(a) === entryKey(b);

/** A dialog of one entry; `opening` gives every opening a fresh form. */
interface EntryDialog {
  entry: SpotlightEntry;
  mode: 'pin' | 'note';
  opening: number;
}

/**
 * Spotlight (M11, ADR 0011; concept 12, variant A «Kompakte Liste», mockup Spo-A): the public
 * Trakt list shown as a row of the start page, in three bands that fold away. «Quelle» holds the
 * switch, the list and its numbers with one «Speichern», the state of the fetch, why the row is
 * missing and «Jetzt abrufen»; «Einträge der Liste» lists the fetched entries, each to pin to a
 * place, hide or annotate; «Verwaiste Anpassungen» lists overrides whose title left the list.
 * Order and hiding of the whole row stay on the page «Startseite». Concept 25 (M-27, N-70..N-72):
 * «Fehlende Einträge» lists the entries the library lacks, with their wish and «Anfordern» as the
 * active profile; «Abgleich über TMDB, TVDB und IMDb» says how entries are matched and lists those
 * TMDB cannot resolve. Both only show: the start page never gets a missing title, and nothing here
 * switches that.
 */
export function SpotlightPage() {
  const { t } = useTranslation();
  useDocumentTitle(t('admin.spotlight.title'));
  const session = useActiveSession();
  const allowed = session.account.role === 'admin' && session.profile.isPrimary;
  if (!allowed) {
    return (
      <AdminPage title={t('admin.spotlight.title')}>
        <Notice title={t('admin.access.notAllowed.title')}>
          {t('admin.access.notAllowed.body')}
        </Notice>
      </AdminPage>
    );
  }
  return <SpotlightContent />;
}

function SpotlightContent() {
  const { t } = useTranslation();
  const queryClient = useQueryClient();
  const session = useActiveSession();
  const titleRef = useRef<HTMLHeadingElement>(null);

  const [dialog, setDialog] = useState<EntryDialog | null>(null);
  const [dialogOpen, setDialogOpen] = useState(false);
  // The missing entry «Anfordern» opened the request dialog for.
  const [requesting, setRequesting] = useState<SpotlightEntry | null>(null);

  // The config is asked again on focus only while nothing is typed or asked: a draft compares
  // against it. `hold` comes from the draft, which needs the config, so a ref carries it back.
  const held = useRef(false);
  const config = useQuery({ ...spotlightConfigQuery, refetchOnWindowFocus: () => !held.current });
  const configData = config.data;
  const drafts = useDrafts<SourceDraft>(
    useCallback(
      (id: string) => (configData && id === SOURCE_DRAFT ? sourceDraftOf(configData) : undefined),
      [configData],
    ),
  );
  // Blanks around a value are no change: the comparison trims, the draft keeps what was typed.
  const changed = sourceChanged(drafts, configData);
  const unsaved = useUnsavedChanges(changed);
  // Nothing is asked again while a dialog is open or the source holds unsaved input (S-6, S-13).
  const hold = dialogOpen || requesting !== null || unsaved.dialog.isOpen || unsaved.pausePolling;
  useEffect(() => {
    held.current = hold;
  }, [hold]);

  // «Jetzt abrufen» polls the state and the entries every 2 s for 30 s.
  const [pollUntil, setPollUntil] = useState(0);
  useEffect(() => {
    if (pollUntil === 0) return;
    const timer = window.setTimeout(() => setPollUntil(0), Math.max(0, pollUntil - Date.now()));
    return () => window.clearTimeout(timer);
  }, [pollUntil]);
  const polling = pollUntil > 0 && !hold;

  const status = useQuery({
    ...traktStatusQuery,
    refetchInterval: polling ? POLL_MS : false,
    refetchOnWindowFocus: !hold,
  });
  const entries = useQuery({
    ...spotlightEntriesQuery,
    refetchInterval: polling ? POLL_MS : false,
    refetchOnWindowFocus: !hold,
  });
  // The outcome of the last action. Each message is a new node in the status region, so the
  // same sentence twice is announced twice.
  const [message, setMessage] = useStatusMessage();
  const [failure, setFailure] = useState<unknown>(null);
  const clear = useCallback(() => {
    setMessage(null);
    setFailure(null);
  }, [setMessage]);
  const say = useCallback(
    (text: string) => {
      setFailure(null);
      setMessage(text);
    },
    [setMessage],
  );
  const fail = useCallback(
    (error: unknown) => {
      setMessage(null);
      setFailure(error);
    },
    [setMessage],
  );

  // The answer replaces its entry at once, then the list and the start page are asked again.
  const settle = async (saved: SpotlightEntry) => {
    queryClient.setQueryData(spotlightEntriesQuery.queryKey, (data) =>
      data
        ? {
            ...data,
            entries: data.entries.map((entry) => (sameEntry(entry, saved) ? saved : entry)),
            orphans: data.orphans.map((entry) => (sameEntry(entry, saved) ? saved : entry)),
          }
        : data,
    );
    await Promise.all([
      queryClient.invalidateQueries({ queryKey: spotlightEntriesQuery.queryKey }),
      queryClient.invalidateQueries({ queryKey: ['home'] }),
    ]);
  };
  const outcome = (entry: SpotlightEntry, body: SpotlightEntryOverride) => {
    const title = entryName(t, entry);
    if (Object.keys(body).length === 0) return t('admin.spotlight.orphans.removed', { title });
    if (body.hidden !== undefined) {
      return t(body.hidden ? 'admin.spotlight.entries.hidden' : 'admin.spotlight.entries.shown', {
        title,
      });
    }
    if (body.pinnedRank === null) return t('admin.spotlight.unpin.done', { title });
    if (body.pinnedRank !== undefined) {
      return t('admin.spotlight.pin.done', { title, rank: body.pinnedRank });
    }
    return body.note === null
      ? t('admin.spotlight.note.removed', { title })
      : t('admin.spotlight.note.saved', { title });
  };
  const override = useMutation({
    mutationFn: ({ entry, body }: { entry: SpotlightEntry; body: SpotlightEntryOverride }) =>
      overrideEntry(entry, body),
    onSuccess: async (saved, { entry, body }) => {
      say(outcome(entry, body));
      await settle(saved);
      await afterDialogsClosed();
      // An orphan's row and its button are gone: the h1 takes the focus.
      if (Object.keys(body).length === 0) titleRef.current?.focus();
      else focusTitleIfLost(titleRef);
    },
    onError: fail,
  });
  const { mutate: mutateOverride } = override;
  const change = useCallback(
    (entry: SpotlightEntry, body: SpotlightEntryOverride) => {
      clear();
      mutateOverride({ entry, body });
    },
    [clear, mutateOverride],
  );

  const openDialog = useCallback((entry: SpotlightEntry, mode: EntryDialog['mode']) => {
    setDialog((current) => ({ entry, mode, opening: (current?.opening ?? 0) + 1 }));
    setDialogOpen(true);
  }, []);
  const closeDialog = useCallback(() => {
    setDialogOpen(false);
    // The button that opened it may be gone by now.
    void afterDialogsClosed().then(() => focusTitleIfLost(titleRef));
  }, []);

  const queued = async (text: string) => {
    say(text);
    setPollUntil(Date.now() + POLL_FOR_MS);
    await queryClient.invalidateQueries({ queryKey: traktStatusQuery.queryKey });
  };

  const request = useCallback(
    (entry: SpotlightEntry) => {
      clear();
      setRequesting(entry);
    },
    [clear],
  );
  // After a request: the entry's wish, the wishes administration and the dashboard are asked
  // again; the button that opened the dialog is gone then, so the h1 takes a lost focus.
  const requested = async (text: string) => {
    setRequesting(null);
    say(text);
    await Promise.all([
      queryClient.invalidateQueries({ queryKey: spotlightEntriesQuery.queryKey }),
      queryClient.invalidateQueries({ queryKey: wishKeys.all }),
      queryClient.invalidateQueries({ queryKey: dashboardQuery.queryKey }),
    ]);
    await afterDialogsClosed();
    focusTitleIfLost(titleRef);
  };

  const columns = useMemo(
    () =>
      entryColumns(t, {
        onPin: (entry) => openDialog(entry, 'pin'),
        onNote: (entry) => openDialog(entry, 'note'),
        onChange: change,
        onRequest: request,
      }),
    [t, openDialog, change, request],
  );
  const orphans = useMemo(() => orphanColumns(t, (entry) => change(entry, {})), [t, change]);
  const missingCols = useMemo(() => missingColumns(t, request), [t, request]);
  const unmatchedCols = useMemo(() => unmatchedColumns(t), [t]);

  const loadError = (error: unknown, retry: () => void) => (
    <Notice
      tone="danger"
      title={problemMessage(t, error)}
      actions={
        <AriaButton icon="refresh" onPress={retry}>
          {t('common.retry')}
        </AriaButton>
      }
    />
  );

  const list = entries.data;
  // The source starts closed (concept 24, P-3); a field the server rejects opens it again.
  const [sourceOpen, setSourceOpen] = useState(false);
  const openSource = useCallback(() => setSourceOpen(true), []);
  // The closed band says which list it shows, as typed while that is unsaved.
  const shown = configData && (drafts.valueOf(SOURCE_DRAFT) ?? sourceDraftOf(configData));
  const sourceSummary =
    shown && !sourceOpen
      ? [
          t(`admin.spotlight.lists.${shown.listKey}`, { defaultValue: shown.listKey }),
          t(`admin.spotlight.media.${sourceMedia(configData, shown)}`),
        ].join(t('format.separator'))
      : undefined;
  // The band's chip waits for the entries: the hits decide whether the row is shown.
  const state = config.data && status.data && list && rowState(t, status.data, config.data, list);
  const missing = list?.missing;
  const lacking = list?.entries.filter(isMissing) ?? [];
  const unmatched = list?.entries.filter((entry) => entry.unmatched) ?? [];
  const hits = list?.entries.filter((entry) => entry.itemId !== undefined) ?? [];
  const ways = WAYS.flatMap(([media, way, key]) => {
    const count = hits.filter((entry) => entry.media === media && entry.match === way).length;
    return count > 0 ? [[t(key), String(count)] as const] : [];
  });

  return (
    <AdminPage
      title={t('admin.spotlight.title')}
      help={t('admin.spotlight.intro')}
      liveSeconds={polling ? POLL_MS / 1000 : undefined}
      titleRef={titleRef}
    >
      {failure !== null && <Notice tone="danger">{problemMessage(t, failure)}</Notice>}
      <StatusRegion message={failure === null ? message : null} />
      {status.isError && loadError(status.error, () => void status.refetch())}
      {config.isPending && <Skeleton shape="block" />}
      {config.isError && loadError(config.error, () => void config.refetch())}
      {config.data && (
        <BandPanel
          collapsible
          isExpanded={sourceOpen}
          onExpandedChange={setSourceOpen}
          icon="spotlight"
          title={t('admin.spotlight.source.title')}
          meta={
            <>
              {changed && <Chip tone="warning">{t('admin.spotlight.unsaved')}</Chip>}
              {sourceSummary}
              {state && <RowChip state={state} />}
            </>
          }
        >
          <SourceForm
            config={config.data}
            status={status.data}
            statusFailed={status.isError}
            entries={list}
            drafts={drafts}
            onStart={clear}
            onSaved={say}
            onQueued={queued}
            onFailed={fail}
            onInvalid={openSource}
          />
        </BandPanel>
      )}

      {missing && (
        <BandPanel
          collapsible
          icon="heart"
          title={t('admin.spotlight.lacking.title')}
          meta={
            <>
              {missingSummary(t, missing)}
              {!missing.canRequest && (
                <Chip tone="warning">{t('admin.spotlight.lacking.noRequestChip')}</Chip>
              )}
            </>
          }
        >
          <Text tone="secondary">{t('admin.spotlight.lacking.intro')}</Text>
          {missing.canRequest ? (
            <Text tone="secondary">
              {t('admin.spotlight.lacking.asProfile', { name: session.profile.name })}
            </Text>
          ) : (
            <Notice tone="warning" title={t('admin.spotlight.lacking.noRequest.title')}>
              <span className={styles.stack}>
                {t('admin.spotlight.lacking.noRequest.body')}
                <span className={styles.links}>
                  <TextLink href={`${paths.adminWishes}?tab=einstellungen`}>
                    {t('admin.spotlight.lacking.noRequest.settings')}
                  </TextLink>
                  <TextLink href={`${paths.adminIntegrations}?item=seerr`}>
                    {t('admin.spotlight.lacking.noRequest.connection')}
                  </TextLink>
                </span>
              </span>
            </Notice>
          )}
          {lacking.length === 0 ? (
            <Text tone="secondary">{t('admin.spotlight.lacking.empty')}</Text>
          ) : (
            <DataTable
              caption={t('admin.spotlight.lacking.title')}
              columns={missingCols}
              rows={lacking}
              rowKey={entryKey}
            />
          )}
        </BandPanel>
      )}

      <BandPanel
        collapsible
        icon="list"
        title={t('admin.spotlight.entries.title')}
        meta={list ? t('admin.spotlight.entries.count', { count: list.entries.length }) : undefined}
      >
        <Text tone="secondary">{t('admin.spotlight.entries.intro')}</Text>
        {entries.isPending && <Skeleton shape="block" />}
        {entries.isError && loadError(entries.error, () => void entries.refetch())}
        {list?.entries.length === 0 && (
          <Text tone="secondary">{t('admin.spotlight.entries.empty')}</Text>
        )}
        {list && list.entries.length > 0 && (
          <DataTable
            caption={t('admin.spotlight.entries.title')}
            columns={columns}
            rows={list.entries}
            rowKey={entryKey}
            layout="fixed"
          />
        )}
      </BandPanel>

      {missing && (
        <BandPanel
          collapsible
          icon="search"
          title={t('admin.spotlight.matching.title')}
          meta={[
            t('admin.spotlight.matching.hits', { count: hits.length }),
            t('admin.spotlight.lacking.missing', { count: missing.entries }),
            t('admin.spotlight.lacking.unmatched', { count: missing.unmatched }),
          ].join(t('admin.spotlight.state.separator'))}
        >
          <Text tone="secondary">{t('admin.spotlight.matching.intro')}</Text>
          <section className={styles.part} aria-labelledby="spotlight-ways">
            <h3 id="spotlight-ways" className={styles.partTitle}>
              {t('admin.spotlight.matching.waysTitle')}
            </h3>
            {ways.length === 0 ? (
              <Text tone="secondary">{t('admin.spotlight.matching.noHits')}</Text>
            ) : (
              <FactList facts={ways} layout="rows" />
            )}
          </section>
          <section className={styles.part} aria-labelledby="spotlight-unmatched">
            <h3 id="spotlight-unmatched" className={styles.partTitle}>
              {t('admin.spotlight.matching.unmatchedTitle')}
            </h3>
            {unmatched.length === 0 ? (
              <Text tone="secondary">{t('admin.spotlight.matching.allMatched')}</Text>
            ) : (
              <>
                <DataTable
                  caption={t('admin.spotlight.matching.unmatchedTitle')}
                  columns={unmatchedCols}
                  rows={unmatched}
                  rowKey={entryKey}
                />
                <Text tone="secondary">{t('admin.spotlight.matching.unmatchedNote')}</Text>
              </>
            )}
          </section>
        </BandPanel>
      )}

      {list && list.orphans.length > 0 && (
        <BandPanel
          collapsible
          icon="archive"
          title={t('admin.spotlight.orphans.title')}
          meta={t('admin.spotlight.orphans.count', { count: list.orphans.length })}
        >
          <Text tone="secondary">{t('admin.spotlight.orphans.intro')}</Text>
          <DataTable
            caption={t('admin.spotlight.orphans.title')}
            columns={orphans}
            rows={list.orphans}
            rowKey={entryKey}
          />
        </BandPanel>
      )}

      <Text tone="secondary">
        {t('admin.spotlight.rowSettings')}{' '}
        <TextLink href={paths.adminHomeRows}>{t('admin.spotlight.rowSettingsLink')}</TextLink>
      </Text>

      {dialog?.mode === 'pin' && config.data && (
        <PinDialog
          key={dialog.opening}
          entry={dialog.entry}
          isOpen={dialogOpen}
          places={config.data.itemCount}
          others={[...(list?.entries ?? []), ...(list?.orphans ?? [])]}
          onClose={closeDialog}
          onSaved={(saved, body) => {
            say(outcome(dialog.entry, body));
            setDialogOpen(false);
            void settle(saved);
          }}
        />
      )}
      {dialog?.mode === 'note' && (
        <NoteDialog
          key={dialog.opening}
          entry={dialog.entry}
          isOpen={dialogOpen}
          onClose={closeDialog}
          onSaved={(saved, body) => {
            say(outcome(dialog.entry, body));
            setDialogOpen(false);
            void settle(saved);
          }}
        />
      )}
      {requesting?.tmdbId !== undefined && (
        <Suspense fallback={null}>
          <RequestDialog
            title={{
              media: requesting.media,
              tmdbId: requesting.tmdbId,
              title: entryName(t, requesting),
              year: requesting.year,
            }}
            source="admin_spotlight"
            onClose={() => setRequesting(null)}
            onDone={(wish: Wish) =>
              void requested(
                t(
                  wish.role === 'joined'
                    ? 'admin.spotlight.request.joined'
                    : 'admin.spotlight.request.done',
                  { title: entryName(t, requesting) },
                ),
              )
            }
            onAvailable={() =>
              void requested(
                t('admin.spotlight.request.available', { title: entryName(t, requesting) }),
              )
            }
          />
        </Suspense>
      )}
      <ConfirmDialog {...unsaved.dialog} />
    </AdminPage>
  );
}
