import type { Library } from '@slouchcat/api-client';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { useCallback, useEffect, useMemo, useRef, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { api, request } from '@/api/client';
import { problemMessage } from '@/api/errors';
import { useActiveSession } from '@/app/session';
import { useDocumentTitle } from '@/lib/useDocumentTitle';
import { Notice, Page, Skeleton, Text, VisuallyHidden } from '@/ui';
import {
  AdminPage,
  AriaButton,
  ConfirmDialog,
  DataTable,
  StatusRegion,
  Tabs,
  afterRender,
  focusTitleIfLost,
  useSearchSelection,
  useStatusMessage,
  useUnsavedChanges,
} from '@/ui/aria';
import { librariesQuery, isScanning, SCAN_POLL_MS } from '../queries';
import { AddLibraryDialog, useLibraryDraft } from './AddLibraryDialog';
import { DefaultRatingDialog } from './DefaultRatingDialog';
import { libraryColumns } from './libraryColumns';
import { LibraryDetail, type LibraryHandlers } from './LibraryDetail';
import { LibraryFoldersDialog } from './LibraryFoldersDialog';
import styles from './Libraries.module.css';

/** The filter tabs by kind (Bib-C): all libraries, then one per kind. */
const TABS = ['all', 'video', 'audiobooks', 'podcasts'] as const;
type TabId = (typeof TABS)[number];

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
 * first (S-13).
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
  const [addOpen, setAddOpen] = useState(false);
  const [foldersId, setFoldersId] = useState<number | null>(null);
  const [foldersOpen, setFoldersOpen] = useState(false);
  const [ratingId, setRatingId] = useState<number | null>(null);
  const [ratingOpen, setRatingOpen] = useState(false);
  // Each opening of the rating dialog mounts it afresh, so it starts at the library's value.
  const [ratingSession, setRatingSession] = useState(0);
  const draft = useLibraryDraft();
  const unsaved = useUnsavedChanges(draft.dirty);
  // The draft lives outside the dialog, so `unsaved.pausePolling` stays out of this.
  const hold = confirmOpen || addOpen || foldersOpen || ratingOpen;

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
      await afterRender();
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
    void afterRender().then(() => focusTitleIfLost(titleRef));
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
      onRating: (id) => {
        setRatingId(id);
        setRatingSession((session) => session + 1);
        setRatingOpen(true);
      },
      onAcceptLoss: (library) =>
        ask({
          title: t('admin.libraries.acceptLoss.title'),
          body: t('admin.libraries.acceptLoss.body'),
          confirmLabel: t('admin.libraries.acceptLoss.confirm'),
          onConfirm: () => acceptLossMutate(library.id),
        }),
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
        onOpenChange={setAddOpen}
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
      <ConfirmDialog {...unsaved.dialog} />
    </AdminPage>
  );
}
