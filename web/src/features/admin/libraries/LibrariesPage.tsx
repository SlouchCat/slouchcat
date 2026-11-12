import type { Library, LibraryDeletionQueued } from '@slouchcat/api-client';
import { useMutation, useQueries, useQuery, useQueryClient } from '@tanstack/react-query';
import { useCallback, useEffect, useMemo, useRef, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { api, request } from '@/api/client';
import { problemMessage } from '@/api/errors';
import { paths } from '@/app/paths';
import { useActiveSession } from '@/app/session';
import { useDocumentTitle } from '@/lib/useDocumentTitle';
import { Notice, Page, Skeleton, Text, TextLink, VisuallyHidden } from '@/ui';
import {
  AdminPage,
  AriaButton,
  ConfirmDialog,
  ConfirmInputDialog,
  DataTable,
  StatusRegion,
  Tabs,
  afterDialogsClosed,
  afterRender,
  focusLost,
  focusTitleIfLost,
  useSearchSelection,
  useStatusMessage,
  useUnsavedChanges,
} from '@/ui/aria';
import { jobRunsKey, jobRunsQuery, librariesQuery, isScanning, SCAN_POLL_MS } from '../queries';
import { AddLibraryDialog, useLibraryDraft } from './AddLibraryDialog';
import { DefaultRatingDialog } from './DefaultRatingDialog';
import { libraryColumns } from './libraryColumns';
import { LibraryDetail, type LibraryHandlers } from './LibraryDetail';
import { LibraryFoldersDialog } from './LibraryFoldersDialog';
import { RenameLibraryDialog } from './RenameLibraryDialog';
import { TrickplayDialog } from './TrickplayDialog';
import styles from './Libraries.module.css';

/** The filter tabs by kind (Bib-C): all libraries, then one per kind. */
const TABS = ['all', 'video', 'audiobooks', 'podcasts'] as const;
type TabId = (typeof TABS)[number];

/** The job type that deletes a library (concept 20 P-8, decision of 2026-11-12). */
const DELETE_JOB = 'library.delete';
/** How often the page asks whether a delete it queued has ended. */
const DELETE_POLL_MS = 2000;

/** A delete queued from this page: its job, and the library it hides until the job ends. */
interface Removal {
  jobId: number;
  libraryId: number;
  name: string;
  /** When the 202 came; only an answer fetched after it can say that the job ended. */
  since: number;
}

/** A confirmation the page opens itself (from a row's menu; the row may change meanwhile). */
interface Confirmation {
  title: string;
  body: string;
  confirmLabel: string;
  onConfirm: () => void;
}

/**
 * The page "Bibliotheken" (A-1, B-1; concept 12 variant C, Bib-C): filter tabs by kind with
 * counts (`?tab=`), one row per library that folds out into its actions, scan progress, error
 * band and folders; «Bibliothek hinzufügen» opens a dialog whose draft the page keeps. The list
 * is polled every 5 s while a scan is queued or running, and paused only while a dialog of the
 * page is open: a kept draft does not stop the scan progress. Leaving the page with a draft asks
 * first (S-13); while the dialog is closed, a band says that the draft is there and offers to go
 * on or to discard it (concept 20, 13.2).
 */
export function LibrariesPage() {
  const { t } = useTranslation();
  useDocumentTitle(t('admin.libraries.title'));
  const session = useActiveSession();
  const allowed = session.account.role === 'admin' && session.profile.isPrimary;
  if (!allowed) {
    return (
      <Page>
        <Notice title={t('admin.access.notAllowed.title')}>
          {t('admin.access.notAllowed.body')}
        </Notice>
      </Page>
    );
  }
  return <LibrariesContent />;
}

function LibrariesContent() {
  const { t, i18n } = useTranslation();
  const queryClient = useQueryClient();
  const titleRef = useRef<HTMLHeadingElement>(null);
  const number = useMemo(() => new Intl.NumberFormat(i18n.language), [i18n.language]);

  const [confirmation, setConfirmation] = useState<Confirmation | null>(null);
  const [confirmOpen, setConfirmOpen] = useState(false);
  // The library «Bibliothek löschen …» asks about (concept 20 P-8).
  const [deleting, setDeleting] = useState<Library | null>(null);
  const [addOpen, setAddOpen] = useState(false);
  const [foldersId, setFoldersId] = useState<number | null>(null);
  const [foldersOpen, setFoldersOpen] = useState(false);
  const [ratingId, setRatingId] = useState<number | null>(null);
  const [ratingOpen, setRatingOpen] = useState(false);
  // Each opening of the rating dialog mounts it afresh, so it starts at the library's value.
  const [ratingSession, setRatingSession] = useState(0);
  // The same for «Umbenennen …».
  const [renameId, setRenameId] = useState<number | null>(null);
  const [renameOpen, setRenameOpen] = useState(false);
  const [renameSession, setRenameSession] = useState(0);
  // The same for «Vorschau beim Spulen» (concept 18, 9).
  const [trickplayId, setTrickplayId] = useState<number | null>(null);
  const [trickplayOpen, setTrickplayOpen] = useState(false);
  const [trickplaySession, setTrickplaySession] = useState(0);
  const draft = useLibraryDraft();
  const unsaved = useUnsavedChanges(draft.dirty);
  const continueRef = useRef<HTMLButtonElement>(null);
  // The draft lives outside the dialog, so `unsaved.pausePolling` stays out of this.
  const hold =
    confirmOpen ||
    addOpen ||
    foldersOpen ||
    ratingOpen ||
    renameOpen ||
    trickplayOpen ||
    deleting !== null;

  // An unknown `?tab=` shows the first tab; the address says so too.
  const [tab, selectTab] = useSearchSelection('tab', TABS, 'all', { normalise: true });

  const libraries = useQuery({
    ...librariesQuery,
    refetchInterval: (query) =>
      !hold && query.state.data?.some(isScanning) ? SCAN_POLL_MS : false,
    refetchOnWindowFocus: !hold,
  });
  const data = libraries.data;
  const scanning = data?.some(isScanning) ?? false;

  const [expanded, setExpanded] = useState<ReadonlySet<string>>(() => new Set());
  const onExpandedChange = useCallback(
    (key: string, open: boolean) =>
      setExpanded((current) => {
        const next = new Set(current);
        if (open) next.add(key);
        else next.delete(key);
        return next;
      }),
    [],
  );
  const [message, setMessage] = useStatusMessage();
  const [failure, setFailure] = useState<unknown>(null);

  // A scan that ends with an error while the page watches says so in the status region; a scan
  // cancelled here has its own message already. The page follows the list in the query cache.
  useEffect(() => {
    const listKey = JSON.stringify(librariesQuery.queryKey);
    const states = new Map<number, Library['scan']['state']>();
    const remember = (list: readonly Library[] | undefined) => {
      const ended: string[] = [];
      for (const library of list ?? []) {
        const before = states.get(library.id);
        const now = library.scan.state;
        states.set(library.id, now);
        if (before !== 'queued' && before !== 'running') continue;
        if (now === 'failed') ended.push(t('admin.libraries.scanFailed', { name: library.name }));
        else if (now === 'aborted' && library.scan.lastError !== 'cancelled')
          ended.push(t('admin.libraries.scanAborted', { name: library.name }));
      }
      return ended;
    };
    remember(queryClient.getQueryData<Library[]>(librariesQuery.queryKey));
    return queryClient.getQueryCache().subscribe((event) => {
      if (event.type !== 'updated' || JSON.stringify(event.query.queryKey) !== listKey) return;
      const ended = remember(event.query.state.data as Library[] | undefined);
      if (ended.length === 0) return;
      setFailure(null);
      setMessage(ended.join(t('format.separator')));
    });
  }, [queryClient, t, setMessage]);

  /**
   * Puts the library the server answered with into the list and fetches again; once React has
   * rendered the answer, the h1 takes the focus when the control that had it went with it.
   */
  const refresh = useCallback(
    async (library: Library) => {
      queryClient.setQueryData<Library[]>(librariesQuery.queryKey, (list) =>
        list?.map((entry) => (entry.id === library.id ? library : entry)),
      );
      await queryClient.invalidateQueries({ queryKey: librariesQuery.queryKey });
      await afterDialogsClosed();
      focusTitleIfLost(titleRef);
    },
    [queryClient],
  );
  const succeeded = useCallback(
    (library: Library) => {
      setFailure(null);
      return refresh(library);
    },
    [refresh],
  );
  const failed = useCallback(
    (error: unknown) => {
      setMessage(null);
      setFailure(error);
    },
    [setMessage],
  );
  // The deletes queued from this page (concept 20 P-8). The runs of `library.delete` are polled
  // while one is open; once its job is neither queued nor running in an answer fetched after the
  // 202, the list is fetched again. The library being back in it means the job did not delete it
  // (it failed before deleting anything, or it was cancelled); the jobs page says why.
  const [removals, setRemovals] = useState<readonly Removal[]>([]);
  // The jobs whose end the page has taken up already; a render before the list came back must
  // not take it up again.
  const settled = useRef(new Set<number>());
  const [queuedRuns, runningRuns] = useQueries({
    queries: (['queued', 'running'] as const).map((state) => ({
      ...jobRunsQuery(DELETE_JOB, state),
      enabled: removals.length > 0,
      refetchInterval: removals.length > 0 ? DELETE_POLL_MS : false,
    })),
  });
  const queuedData = queuedRuns?.data;
  const runningData = runningRuns?.data;
  const seen = Math.min(queuedRuns?.dataUpdatedAt ?? 0, runningRuns?.dataUpdatedAt ?? 0);
  useEffect(() => {
    if (removals.length === 0 || !queuedData || !runningData) return;
    const open = new Set([...queuedData.runs, ...runningData.runs].map((run) => run.id));
    const ended = removals.filter(
      (removal) =>
        removal.since <= seen && !open.has(removal.jobId) && !settled.current.has(removal.jobId),
    );
    if (ended.length === 0) return;
    for (const removal of ended) settled.current.add(removal.jobId);
    void queryClient
      .fetchQuery({ ...librariesQuery, staleTime: 0 })
      .finally(() =>
        setRemovals((current) => current.filter((removal) => !ended.includes(removal))),
      )
      .then((list) => {
        const back = ended.filter((removal) =>
          list.some((entry) => entry.id === removal.libraryId),
        );
        const gone = ended.filter((removal) => !back.includes(removal));
        const lines = [
          ...gone.map((removal) => t('admin.libraries.delete.done', { name: removal.name })),
          ...back.map((removal) => t('admin.libraries.delete.notDone', { name: removal.name })),
        ];
        setFailure(null);
        setMessage(lines.join(t('format.separator')), back.length > 0 ? 'warning' : undefined);
      })
      .catch(failed);
  }, [removals, queuedData, runningData, seen, queryClient, t, setMessage, failed]);
  const scan = useMutation({
    mutationFn: (id: number): Promise<Library> =>
      request(api.POST('/admin/libraries/{id}/scan', { params: { path: { id } } })),
    onSuccess: (library) => {
      setMessage(t('admin.libraries.scanStarted', { name: library.name }));
      return succeeded(library);
    },
    onError: failed,
  });
  const cancel = useMutation({
    mutationFn: (id: number): Promise<Library> =>
      request(api.DELETE('/admin/libraries/{id}/scan', { params: { path: { id } } })),
    onSuccess: (library) => {
      setMessage(t('admin.libraries.scanCancelled', { name: library.name }));
      return succeeded(library);
    },
    onError: failed,
  });
  // «Bibliothek löschen …» (concept 20 P-8): the typed name goes to the server, which compares it
  // against its own row and queues the job `library.delete` (202). The library is hidden at once,
  // so the dialog closes and the row leaves the list right away; the focus goes to the h1.
  const removeLibrary = useMutation({
    mutationFn: ({
      library,
      confirmName,
    }: {
      library: Library;
      confirmName: string;
    }): Promise<LibraryDeletionQueued> =>
      request(
        api.POST('/admin/libraries/{id}/deletion', {
          params: { path: { id: library.id } },
          body: { confirmName },
        }),
      ),
    onSuccess: async (queued, { library }) => {
      setDeleting(null);
      setFailure(null);
      setRemovals((current) => [
        ...current,
        { jobId: queued.jobId, libraryId: library.id, name: queued.name, since: Date.now() },
      ]);
      // A poll already on its way could not know the job yet: it is cancelled and asked again.
      void queryClient.invalidateQueries({ queryKey: jobRunsKey });
      setMessage(
        <>
          {t('admin.libraries.delete.queued', { name: queued.name })}{' '}
          <TextLink href={`${paths.adminJobs}?item=${DELETE_JOB}`}>
            {t('admin.libraries.delete.jobs')}
          </TextLink>
        </>,
      );
      queryClient.setQueryData<Library[]>(librariesQuery.queryKey, (list) =>
        list?.filter((entry) => entry.id !== library.id),
      );
      await queryClient.invalidateQueries({ queryKey: librariesQuery.queryKey });
      await afterDialogsClosed();
      focusTitleIfLost(titleRef);
    },
    onError: (error) => {
      setDeleting(null);
      failed(error);
      void afterDialogsClosed().then(() => focusTitleIfLost(titleRef));
    },
  });
  const acceptLoss = useMutation({
    mutationFn: (id: number): Promise<Library> =>
      request(api.POST('/admin/libraries/{id}/accept-loss', { params: { path: { id } } })),
    onSuccess: succeeded,
    onError: failed,
  });

  // Every confirmation is the page's own: a row whose state changes while it is open cannot take
  // the dialog, and the polling pause with it.
  const ask = useCallback((next: Confirmation) => {
    setConfirmation(next);
    setConfirmOpen(true);
  }, []);
  const onConfirmOpenChange = useCallback((open: boolean) => {
    if (open) return;
    setConfirmOpen(false);
    // The menu that opened it may be gone by now.
    void afterDialogsClosed().then(() => focusTitleIfLost(titleRef));
  }, []);

  const { mutate: scanMutate } = scan;
  const { mutate: cancelMutate } = cancel;
  const { mutate: acceptLossMutate } = acceptLoss;
  const handlers = useMemo<LibraryHandlers>(
    () => ({
      onScan: (id) => scanMutate(id),
      onCancelScan: (id) => cancelMutate(id),
      onFolders: (id) => {
        setFoldersId(id);
        setFoldersOpen(true);
      },
      onRename: (id) => {
        setRenameId(id);
        setRenameSession((session) => session + 1);
        setRenameOpen(true);
      },
      onRating: (id) => {
        setRatingId(id);
        setRatingSession((session) => session + 1);
        setRatingOpen(true);
      },
      onTrickplay: (id) => {
        setTrickplayId(id);
        setTrickplaySession((session) => session + 1);
        setTrickplayOpen(true);
      },
      onAcceptLoss: (library) =>
        ask({
          title: t('admin.libraries.acceptLoss.title'),
          body: t('admin.libraries.acceptLoss.body'),
          confirmLabel: t('admin.libraries.acceptLoss.confirm'),
          onConfirm: () => acceptLossMutate(library.id),
        }),
      onDelete: (library) => setDeleting(library),
    }),
    [scanMutate, cancelMutate, acceptLossMutate, ask, t],
  );

  const columns = useMemo(() => libraryColumns(t, hold), [t, hold]);
  const detail = (library: Library) => (
    <LibraryDetail
      library={library}
      scanPending={scan.isPending && scan.variables === library.id}
      cancelPending={cancel.isPending && cancel.variables === library.id}
      handlers={handlers}
    />
  );
  const foldersLibrary = data?.find((library) => library.id === foldersId);
  const ratingLibrary = data?.find((library) => library.id === ratingId);
  const renameLibrary = data?.find((library) => library.id === renameId);
  const trickplayLibrary = data?.find((library) => library.id === trickplayId);

  // The band is gone while the dialog is open, so a dialog opened from it hands the focus back to
  // the band's button once it shows again; with the draft discarded, to the h1.
  const onAddOpenChange = useCallback((open: boolean) => {
    setAddOpen(open);
    if (open) return;
    void afterDialogsClosed().then(() => {
      if (focusLost()) (continueRef.current ?? titleRef.current)?.focus();
    });
  }, []);
  const discardDraft = () => {
    draft.reset();
    setFailure(null);
    setMessage(t('admin.libraries.add.draft.discarded'));
    // The band goes with the button that had the focus.
    void afterRender().then(() => focusTitleIfLost(titleRef));
  };

  const tabName = (id: TabId) =>
    id === 'all' ? t('admin.libraries.tabs.all') : t(`library.kind.${id}`);

  return (
    <AdminPage
      title={t('admin.libraries.title')}
      help={t('admin.libraries.intro')}
      // «Live» only while a scan runs, and not while a dialog holds the polling (S-2).
      liveSeconds={scanning && !hold ? SCAN_POLL_MS / 1000 : undefined}
      actions={
        <AriaButton variant="primary" icon="plus" onPress={() => setAddOpen(true)}>
          {t('admin.libraries.add.title')}
        </AriaButton>
      }
      titleRef={titleRef}
    >
      {libraries.isPending && <Skeleton shape="block" />}
      {libraries.isError && (
        <Notice
          tone="danger"
          title={problemMessage(t, libraries.error)}
          actions={
            <AriaButton icon="refresh" onPress={() => void libraries.refetch()}>
              {t('common.retry')}
            </AriaButton>
          }
        />
      )}
      {failure !== null && <Notice tone="danger">{problemMessage(t, failure)}</Notice>}
      {/* A polite live region: it appears when the dialog closes on a kept draft. */}
      {draft.dirty && !addOpen && (
        <Notice
          tone="warning"
          title={t('admin.libraries.add.draft.band')}
          actions={
            <>
              <AriaButton ref={continueRef} icon="pen" onPress={() => onAddOpenChange(true)}>
                {t('admin.libraries.add.draft.continue')}
              </AriaButton>
              <AriaButton variant="ghost" onPress={discardDraft}>
                {t('admin.libraries.add.draft.discard')}
              </AriaButton>
            </>
          }
        />
      )}
      <StatusRegion message={failure === null ? message : null} />
      {data?.length === 0 && <Notice>{t('admin.libraries.empty')}</Notice>}
      {data && data.length > 0 && (
        <Tabs
          label={t('admin.libraries.tabs.label')}
          selectedKey={tab}
          onSelectionChange={selectTab}
          tabs={TABS.map((id) => {
            const rows = id === 'all' ? data : data.filter((library) => library.kind === id);
            const name = tabName(id);
            return {
              id,
              label: name,
              count: number.format(rows.length),
              countLabel: t('admin.libraries.tabs.count', { count: rows.length }),
              content: (
                <div className={styles.tabBody}>
                  <VisuallyHidden as="h2">{name}</VisuallyHidden>
                  {rows.length === 0 ? (
                    <Text tone="secondary">{t('admin.libraries.tabs.empty')}</Text>
                  ) : (
                    <DataTable
                      caption={t('admin.libraries.tabs.caption', { tab: name })}
                      columns={columns}
                      rows={rows}
                      rowKey={(library) => String(library.id)}
                      expand={{ render: detail, expandedKeys: expanded, onExpandedChange }}
                    />
                  )}
                </div>
              ),
            };
          })}
        />
      )}

      <AddLibraryDialog
        draft={draft}
        isOpen={addOpen}
        onOpenChange={onAddOpenChange}
        onCreated={(library) => {
          setFailure(null);
          setMessage(t('admin.libraries.add.created', { name: library.name }));
        }}
      />
      {foldersLibrary && (
        <LibraryFoldersDialog
          key={foldersLibrary.id}
          library={foldersLibrary}
          isOpen={foldersOpen}
          onOpenChange={setFoldersOpen}
          onChanged={(updated) => void refresh(updated)}
        />
      )}
      {renameLibrary && (
        <RenameLibraryDialog
          key={renameSession}
          library={renameLibrary}
          isOpen={renameOpen}
          onOpenChange={setRenameOpen}
          onSaved={(updated) => {
            setFailure(null);
            setMessage(t('admin.libraries.rename.saved', { name: updated.name }));
            // The name shows in the side navigation and the header of the library page
            // (['libraries', …]), in the rows «Neu in …» of the start page and their lists.
            for (const queryKey of [['libraries'], ['home'], ['homeList'], ['library']])
              void queryClient.invalidateQueries({ queryKey });
            void refresh(updated);
          }}
        />
      )}
      {ratingLibrary && (
        <DefaultRatingDialog
          key={ratingSession}
          library={ratingLibrary}
          isOpen={ratingOpen}
          onOpenChange={setRatingOpen}
          onSaved={(updated) => {
            setFailure(null);
            if (updated.defaultRating !== undefined) {
              setMessage(
                t('admin.libraries.changeRating.saved', {
                  name: updated.name,
                  rating: t('rating.fsk', { age: updated.defaultRating }),
                }),
              );
            }
            void refresh(updated);
          }}
        />
      )}
      {trickplayLibrary && (
        <TrickplayDialog
          key={trickplaySession}
          library={trickplayLibrary}
          isOpen={trickplayOpen}
          onOpenChange={setTrickplayOpen}
          onSaved={(updated) => {
            setFailure(null);
            setMessage(t('admin.libraries.trickplay.saved', { name: updated.name }));
            void refresh(updated);
          }}
        />
      )}
      {confirmation && (
        <ConfirmDialog
          isOpen={confirmOpen}
          onOpenChange={onConfirmOpenChange}
          title={confirmation.title}
          body={confirmation.body}
          confirmLabel={confirmation.confirmLabel}
          cancelLabel={t('common.cancel')}
          tone="danger"
          onConfirm={confirmation.onConfirm}
        />
      )}
      {deleting && (
        <ConfirmInputDialog
          isOpen
          onOpenChange={(open) => {
            if (open) return;
            setDeleting(null);
            void afterDialogsClosed().then(() => focusTitleIfLost(titleRef));
          }}
          title={t('admin.libraries.delete.title', { name: deleting.name })}
          expected={deleting.name}
          inputLabel={t('admin.libraries.delete.inputLabel')}
          mismatchText={t('admin.libraries.delete.mismatch', { name: deleting.name })}
          confirmLabel={t('admin.libraries.delete.confirm')}
          cancelLabel={t('common.cancel')}
          isPending={removeLibrary.isPending}
          onConfirm={(confirmName) => removeLibrary.mutate({ library: deleting, confirmName })}
        >
          <p>
            {t('admin.libraries.delete.lead', {
              name: deleting.name,
              roots: t('admin.libraries.rootCount', { count: deleting.roots.length }),
              count: deleting.scan.itemCount,
              formatted: number.format(deleting.scan.itemCount),
            })}
          </p>
          <p>{t('admin.libraries.delete.files')}</p>
          <p>{t('admin.libraries.delete.backup')}</p>
          <p>{t('admin.libraries.delete.final')}</p>
        </ConfirmInputDialog>
      )}
      <ConfirmDialog {...unsaved.dialog} />
    </AdminPage>
  );
}
