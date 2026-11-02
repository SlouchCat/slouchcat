import type { AdminCollection, AdminItemHit } from '@slouchcat/api-client';
import {
  useMutation,
  useQueries,
  useQuery,
  useQueryClient,
  type UseQueryResult,
} from '@tanstack/react-query';
import { useCallback, useMemo, useRef, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { problemMessage } from '@/api/errors';
import { useActiveSession } from '@/app/session';
import { useDocumentTitle } from '@/lib/useDocumentTitle';
import { Notice, Skeleton, Text } from '@/ui';
import {
  AdminPage,
  AriaButton,
  BandPanel,
  ConfirmDialog,
  DataTable,
  StatusRegion,
  afterDialogsClosed,
  afterRender,
  focusTitleIfLost,
  useDrafts,
  useSearchSelection,
  useStatusMessage,
  useUnsavedChanges,
  type TabItem,
} from '@/ui/aria';
import { adminCollectionQuery, collectionsQuery, deleteCollection } from '../queries';
import { ManualMembers, TmdbMembers, type Member } from './CollectionDetail';
import { collectionColumns } from './columns';
import { NameDialog } from './NameDialog';

/** The filter tabs: every collection (the default, no `?tab=`), the manual ones, TMDB's. */
const ALL = 'all';
const MANUAL = 'manual';
const TMDB = 'tmdb';
const TAB_IDS = [ALL, MANUAL, TMDB] as const;

/** What the stored-members observers give the page; module-level, so the result stays stable. */
const storedOf = (results: UseQueryResult<AdminCollection>[]) =>
  results.map((result) => result.data);

/** The page's confirmation: the one ConfirmDialog of the page asks it. */
interface Confirmation {
  title: string;
  body: string;
  confirmLabel: string;
  onConfirm: () => void;
}

/** «Sammlung anlegen» or «Umbenennen» of a collection. */
type Naming = { kind: 'create' } | { kind: 'rename'; collection: AdminCollection };

/** Two drawings of a member show the same. */
const sameMember = (a: Member | undefined, b: Member) =>
  a !== undefined &&
  a.title === b.title &&
  a.kind === b.kind &&
  a.year === b.year &&
  a.posterUrl === b.posterUrl;

const without = (set: ReadonlySet<string>, key: string) => {
  if (!set.has(key)) return set;
  const next = new Set(set);
  next.delete(key);
  return next;
};

/**
 * Collections (M7, E-6; concept 12, variant C): automatic ones from TMDB (read only) and manual
 * ones, with source and number of titles, filtered by the tabs «Alle», «Manuell» and «TMDB»
 * (SAM-1 to SAM-3). Each collection is a row that folds out into its members: a manual one is
 * filled from the title search, ordered by position and saved there; a TMDB one shows its
 * members only. The members being edited are the page's draft per collection (S-17).
 */
export function CollectionsPage() {
  const { t } = useTranslation();
  useDocumentTitle(t('admin.collections.title'));
  const session = useActiveSession();
  const allowed = session.account.role === 'admin' && session.profile.isPrimary;
  if (!allowed) {
    return (
      <AdminPage title={t('admin.collections.title')}>
        <Notice title={t('admin.access.notAllowed.title')}>
          {t('admin.access.notAllowed.body')}
        </Notice>
      </AdminPage>
    );
  }
  return <CollectionsContent />;
}

function CollectionsContent() {
  const { t, i18n } = useTranslation();
  const queryClient = useQueryClient();
  const number = useMemo(() => new Intl.NumberFormat(i18n.language), [i18n.language]);
  const titleRef = useRef<HTMLHeadingElement>(null);
  const createButton = useRef<HTMLButtonElement>(null);

  const [naming, setNaming] = useState<Naming | null>(null);
  const [confirmation, setConfirmation] = useState<Confirmation | null>(null);
  const [confirmOpen, setConfirmOpen] = useState(false);
  // Open rows, held by the page: the same collection is open in «Alle» and in its own tab.
  const [expanded, setExpanded] = useState<ReadonlySet<string>>(() => new Set());
  // Rows ever opened: their stored members stay observed, so a draft outlives folding.
  const [opened, setOpened] = useState<ReadonlySet<string>>(() => new Set());
  // Every member the page has drawn, stored or picked from the search: a draft keeps showing a
  // title the server dropped meanwhile (another administrator) or one not saved yet.
  const [seen, setSeen] = useState<ReadonlyMap<string, Member>>(() => new Map());
  const remember = useCallback(
    (members: readonly Member[]) =>
      setSeen((current) => {
        const fresh = members.filter((member) => !sameMember(current.get(member.itemId), member));
        if (fresh.length === 0) return current;
        const next = new Map(current);
        for (const member of fresh) next.set(member.itemId, member);
        return next;
      }),
    [],
  );

  // The drafts need what the server holds before the queries know whether to pause for them:
  // passive observers read the same cache entries without fetching.
  const listed = useQuery({ ...collectionsQuery, enabled: false }).data;
  const openedIds = useMemo(() => [...opened], [opened]);
  const stored = useQueries({
    queries: openedIds.map((id) => ({ ...adminCollectionQuery(id), enabled: false })),
    combine: storedOf,
  });
  const storedById = useMemo(
    () =>
      new Map(stored.flatMap((collection) => (collection ? [[collection.id, collection]] : []))),
    [stored],
  );
  // Stored members are remembered during the render that brings them (as useDrafts prunes); a
  // member equal to the remembered one changes nothing, so the second pass finds nothing new.
  const storedMembers = stored.flatMap((collection) => collection?.items ?? []);
  if (storedMembers.some((member) => !sameMember(seen.get(member.itemId), member))) {
    remember(storedMembers);
  }
  // A draft is the order of member ids (S-17): adding a title and removing it again, or moving
  // one back, is what the server holds and no change.
  const originalOf = useCallback(
    (id: string) =>
      listed?.some((collection) => collection.id === id)
        ? storedById.get(id)?.items.map((item) => item.itemId)
        : undefined,
    [listed, storedById],
  );
  const drafts = useDrafts<string[]>(originalOf);
  const { set: setDraft, discard: discardDraft } = drafts;
  const unsaved = useUnsavedChanges(drafts.dirty);
  // A draft of a folded row keeps its place but does not hold the page; an open one does.
  const openDraft = drafts.dirtyIds.some((id) => expanded.has(id));
  const hold = naming !== null || confirmOpen || unsaved.dialog.isOpen || openDraft;
  const collections = useQuery({ ...collectionsQuery, refetchOnWindowFocus: !hold });

  // An unknown `?tab=` shows «Alle», and the address says so too (replaced, not pushed).
  const [tab, selectTab] = useSearchSelection('tab', TAB_IDS, ALL, { normalise: true });

  const onExpandedChange = useCallback((key: string, open: boolean) => {
    setExpanded((current) => {
      const next = new Set(current);
      if (open) next.add(key);
      else next.delete(key);
      return next;
    });
    if (open) setOpened((current) => (current.has(key) ? current : new Set(current).add(key)));
  }, []);

  // A persistent status region; a repeated message is a new node, so it is announced again.
  const [message, setMessage] = useStatusMessage();
  const [failure, setFailure] = useState<unknown>(null);
  const say = useCallback(
    (text: string) => {
      setFailure(null);
      setMessage(text);
    },
    [setMessage],
  );
  const failed = useCallback(
    (error: unknown) => {
      setMessage(null);
      setFailure(error);
    },
    [setMessage],
  );

  // Every viewer place that shows collections changes with an edit: the start page's row
  // «Sammlungen», the collection pages, the detail pages' section and the search results. The
  // prefix ['admin', 'collections'] covers the stored members too.
  const invalidate = useCallback(
    () =>
      Promise.all(
        [collectionsQuery.queryKey, ['home'], ['collection'], ['item'], ['search']].map(
          (queryKey) => queryClient.invalidateQueries({ queryKey }),
        ),
      ),
    [queryClient],
  );

  const remove = useMutation({
    mutationFn: (collection: AdminCollection) => deleteCollection(collection.id),
    onSuccess: async (_data, collection) => {
      say(t('admin.collections.remove.done', { name: collection.name }));
      // Its open row and its draft go with it.
      setExpanded((current) => without(current, collection.id));
      setOpened((current) => without(current, collection.id));
      discardDraft(collection.id);
      await invalidate();
      // The row and its buttons are gone: once React rendered that, the h1 takes the focus.
      await afterRender();
      titleRef.current?.focus();
    },
    onError: failed,
  });

  // The confirmation is the page's own: a row that goes while it is open (a refetch, another
  // tab) cannot take the dialog with it.
  const ask = useCallback((next: Confirmation) => {
    setConfirmation(next);
    setConfirmOpen(true);
  }, []);
  const onConfirmOpenChange = useCallback((open: boolean) => {
    if (open) return;
    setConfirmOpen(false);
    // The button that opened it may be gone by now.
    void afterDialogsClosed().then(() => focusTitleIfLost(titleRef));
  }, []);
  const { mutate: removeMutate } = remove;
  const askRemove = useCallback(
    (collection: AdminCollection) =>
      ask({
        title: t('admin.collections.remove.title', { name: collection.name }),
        body: t('admin.collections.remove.body'),
        confirmLabel: t('admin.collections.remove.confirm'),
        onConfirm: () => removeMutate(collection),
      }),
    [ask, t, removeMutate],
  );

  const changeMembers = useCallback(
    (collection: AdminCollection, itemIds: string[]) => setDraft(collection.id, itemIds),
    [setDraft],
  );
  const pickMember = useCallback(
    (collection: AdminCollection, hit: AdminItemHit) => {
      remember([{ itemId: hit.id, kind: hit.kind, title: hit.title, year: hit.year }]);
      // A second press before the hit left the list adds nothing.
      setDraft(collection.id, (current) =>
        current.includes(hit.id) ? current : [...current, hit.id],
      );
    },
    [remember, setDraft],
  );
  const discardMembers = useCallback(
    (collection: AdminCollection) => {
      discardDraft(collection.id);
      say(t('admin.collections.members.discarded', { name: collection.name }));
    },
    [discardDraft, say, t],
  );
  const savedMembers = useCallback(
    (saved: AdminCollection) => {
      // The answer holds the saved members: a draft equal to them is spent (useDrafts drops it),
      // a change made while the save was under way stays a draft.
      queryClient.setQueryData(adminCollectionQuery(saved.id).queryKey, saved);
      remember(saved.items);
      say(t('admin.collections.members.done', { name: saved.name }));
      void invalidate();
    },
    [queryClient, remember, say, t, invalidate],
  );
  const rename = useCallback(
    (collection: AdminCollection) => setNaming({ kind: 'rename', collection }),
    [],
  );

  const columns = useMemo(
    () => collectionColumns(t, number, drafts.isDirty),
    [t, number, drafts.isDirty],
  );
  const detail = (collection: AdminCollection) =>
    collection.source === 'tmdb' ? (
      <TmdbMembers collection={collection} hold={hold} />
    ) : (
      <ManualMembers
        collection={collection}
        hold={hold}
        memberIds={drafts.valueOf(collection.id)}
        dirty={drafts.isDirty(collection.id)}
        lookup={(itemId) =>
          storedById.get(collection.id)?.items.find((item) => item.itemId === itemId) ??
          seen.get(itemId)
        }
        onChange={changeMembers}
        onPick={pickMember}
        onDiscard={discardMembers}
        onSaved={savedMembers}
        onRename={rename}
        onRemove={askRemove}
      />
    );

  const list = collections.data ?? [];
  const tabOf = (id: string, label: string, caption: string, rows: AdminCollection[]): TabItem => ({
    id,
    label,
    count: number.format(rows.length),
    countLabel: t('admin.collections.tabs.count', {
      count: rows.length,
      number: number.format(rows.length),
    }),
    content:
      rows.length === 0 ? (
        <Text tone="secondary">
          {list.length === 0 ? t('admin.collections.empty') : t('admin.collections.emptyTab')}
        </Text>
      ) : (
        <DataTable
          caption={caption}
          columns={columns}
          rows={rows}
          rowKey={(collection) => collection.id}
          expand={{ render: detail, expandedKeys: expanded, onExpandedChange }}
        />
      ),
  });

  const closeNaming = () => {
    const wasCreate = naming?.kind === 'create';
    setNaming(null);
    if (wasCreate) createButton.current?.focus();
  };

  return (
    <AdminPage
      title={t('admin.collections.title')}
      help={t('admin.collections.intro')}
      titleRef={titleRef}
      actions={
        <AriaButton
          ref={createButton}
          variant="primary"
          icon="plus"
          onPress={() => setNaming({ kind: 'create' })}
        >
          {t('admin.collections.create.action')}
        </AriaButton>
      }
    >
      {failure !== null && <Notice tone="danger">{problemMessage(t, failure)}</Notice>}
      <StatusRegion message={failure === null ? message : null} />
      {collections.isPending && <Skeleton shape="block" />}
      {collections.isError && (
        <Notice
          tone="danger"
          title={problemMessage(t, collections.error)}
          actions={
            <AriaButton icon="refresh" onPress={() => void collections.refetch()}>
              {t('common.retry')}
            </AriaButton>
          }
        />
      )}
      {collections.data && (
        <BandPanel
          title={t('admin.collections.list')}
          icon="stack"
          tabs={{
            label: t('admin.collections.tabs.label'),
            selectedKey: tab,
            onSelectionChange: selectTab,
            tabs: [
              tabOf(ALL, t('admin.collections.tabs.all'), t('admin.collections.list'), list),
              tabOf(
                MANUAL,
                t('admin.collections.source.manual'),
                t('admin.collections.listManual'),
                list.filter((collection) => collection.source === 'manual'),
              ),
              tabOf(
                TMDB,
                t('admin.collections.source.tmdb'),
                t('admin.collections.listTmdb'),
                list.filter((collection) => collection.source === 'tmdb'),
              ),
            ],
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
      {naming && (
        <NameDialog
          collection={naming.kind === 'rename' ? naming.collection : undefined}
          onClose={closeNaming}
          onSaved={(saved) => {
            say(
              naming.kind === 'rename'
                ? t('admin.collections.rename.done', { name: saved.name })
                : t('admin.collections.create.done', { name: saved.name }),
            );
            closeNaming();
            void invalidate();
          }}
        />
      )}
    </AdminPage>
  );
}
