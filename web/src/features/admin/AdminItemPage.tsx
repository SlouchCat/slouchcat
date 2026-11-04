import type { AdminItem } from '@slouchcat/api-client';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { getRouteApi } from '@tanstack/react-router';
import { useCallback, useRef, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { api, isApiError, request } from '@/api/client';
import { problemMessage } from '@/api/errors';
import { paths } from '@/app/paths';
import { useActiveSession } from '@/app/session';
import { useDocumentTitle } from '@/lib/useDocumentTitle';
import { ButtonLink, Chip, Notice, Page, Skeleton } from '@/ui';
import {
  AdminPage,
  AriaButton,
  ConfirmDialog,
  StatusRegion,
  Tabs,
  afterDialogsClosed,
  focusTitleIfLost,
  useDrafts,
  useSearchSelection,
  useStatusMessage,
  useUnsavedChanges,
  type TabItem,
} from '@/ui/aria';
import { titleRatingOf } from '../shared/choices';
import { FilesTab } from './item/FilesTab';
import { ImagesTab, imageKindsOf } from './item/ImagesTab';
import {
  FROM_METADATA,
  MetadataTab,
  hasOriginalLanguage,
  languageChoiceOf,
  ratingChoiceOf,
} from './item/MetadataTab';
import { TrailersTab } from './item/TrailersTab';
import { MatchDialog } from './MatchDialog';
import { adminItemQuery, maintenanceKey, metadataSettingsQuery } from './queries';
import styles from './item/AdminItem.module.css';

const route = getRouteApi('/_authed/_active/_chrome/admin/items/$itemId');

/** The first tab; every other tab is in `?tab=` (S-14). */
const FIRST_TAB = 'metadata';

/** The draft key of the original language next to the rating's (the title id). */
const languageKey = (id: string) => `${id}:language`;

type ItemTrailer = AdminItem['trailers'][number];

/**
 * Administration view of one title (TIT-1 to TIT-7, concept 12 variant C): the header with
 * «Zuordnen», «Aktualisieren» and the link to the detail page; tabs Metadaten, Bilder, Dateien
 * and Trailer bound to `?tab=`. The rating and the original language choice are drafts of the
 * page (S-17): they survive the tabs, pause the refetch and ask before the page is left (S-13).
 */
function AdminItemView({ itemId }: { itemId: string }) {
  const { t, i18n } = useTranslation();
  const queryClient = useQueryClient();
  const titleRef = useRef<HTMLHeadingElement>(null);
  const number = new Intl.NumberFormat(i18n.language);

  // The draft needs what the server holds before the query knows whether to pause for it: a
  // passive observer reads the same cache entry without fetching.
  const cached = useQuery({ ...adminItemQuery(itemId), enabled: false }).data;
  const originalOf = useCallback(
    (id: string) => {
      if (!cached) return undefined;
      if (id === cached.id) return ratingChoiceOf(cached);
      return id === languageKey(cached.id) ? languageChoiceOf(cached) : undefined;
    },
    [cached],
  );
  const drafts = useDrafts<string>(originalOf);
  const unsaved = useUnsavedChanges(drafts.dirty);
  const [deleting, setDeleting] = useState<ItemTrailer | null>(null);
  const [confirmOpen, setConfirmOpen] = useState(false);
  const item = useQuery({
    ...adminItemQuery(itemId),
    refetchOnWindowFocus: !(unsaved.pausePolling || confirmOpen),
  });
  const data = item.data;
  useDocumentTitle(data?.title ?? t('admin.item.title'));

  const kinds = data ? imageKindsOf(data) : undefined;
  const tabIds = [FIRST_TAB, ...(kinds ? ['images'] : []), 'files', 'trailers'];
  // An unknown `?tab=` (or «Bilder» of an episode) shows the first tab; the address says so too.
  const [tab, selectTab] = useSearchSelection('tab', tabIds, FIRST_TAB, {
    normalise: data !== undefined,
  });

  const [expanded, setExpanded] = useState<ReadonlySet<string>>(() => new Set());
  const isExpanded = useCallback((key: string) => expanded.has(key), [expanded]);
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

  // The language codes of the select, read once «Originalsprache» is open.
  const languages = useQuery({ ...metadataSettingsQuery, enabled: expanded.has('language') });

  const [matching, setMatching] = useState(false);
  const [message, setMessage] = useStatusMessage();

  const reload = async () => {
    await Promise.all([
      queryClient.invalidateQueries({ queryKey: adminItemQuery(itemId).queryKey }),
      queryClient.invalidateQueries({ queryKey: maintenanceKey }),
      queryClient.invalidateQueries({ queryKey: ['item', itemId] }),
    ]);
  };
  /** Shows the outcome and reads the title again; once React rendered it, a lost focus goes up. */
  const settle = async (text: string, removed = false) => {
    setMessage(text);
    await reload();
    await afterDialogsClosed();
    if (removed) titleRef.current?.focus();
    else focusTitleIfLost(titleRef);
  };
  const adopt = async (next: AdminItem, text: string) => {
    queryClient.setQueryData(adminItemQuery(itemId).queryKey, next);
    await settle(text);
  };
  const refresh = useMutation({
    mutationFn: (): Promise<AdminItem> =>
      request(api.POST('/admin/items/{id}/refresh', { params: { path: { id: itemId } } })),
    onSuccess: (next) => adopt(next, t('admin.item.refreshed')),
  });
  const saveRating = useMutation({
    mutationFn: (value: string) =>
      request(
        api.PUT('/admin/items/{id}/rating', {
          params: { path: { id: itemId } },
          body: { rating: value === FROM_METADATA ? null : (titleRatingOf(value) ?? null) },
        }),
      ),
    // The draft is spent once the reread title holds the saved choice (useDrafts).
    onSuccess: () => settle(t('admin.item.rating.saved')),
  });
  const saveLanguage = useMutation({
    mutationFn: (value: string): Promise<AdminItem> =>
      request(
        api.PUT('/admin/items/{id}/original-language', {
          params: { path: { id: itemId } },
          body: { language: value === FROM_METADATA ? null : value },
        }),
      ),
    onSuccess: (next) => adopt(next, t('admin.item.language.saved')),
  });
  const removeTrailer = useMutation({
    mutationFn: (trailer: ItemTrailer) =>
      request(
        api.DELETE('/admin/items/{id}/trailers/{trailerId}', {
          params: { path: { id: itemId, trailerId: trailer.id } },
        }),
      ),
    // The row and its button are gone: the h1 takes the focus (ALL-2).
    onSuccess: () => settle(t('admin.item.trailers.deleted'), true),
  });

  const askDelete = useCallback((trailer: ItemTrailer) => {
    setDeleting(trailer);
    setConfirmOpen(true);
  }, []);
  const onConfirmOpenChange = useCallback((open: boolean) => {
    if (open) return;
    setConfirmOpen(false);
    // The button that opened it may be gone by now.
    void afterDialogsClosed().then(() => focusTitleIfLost(titleRef));
  }, []);

  const loadError = item.isError ? (
    <Notice
      tone="danger"
      title={problemMessage(t, item.error)}
      actions={
        <AriaButton icon="refresh" onPress={() => void item.refetch()}>
          {t('common.retry')}
        </AriaButton>
      }
    />
  ) : null;
  const current = data;
  if (current === undefined) {
    return (
      <AdminPage title={t('admin.item.title')} titleRef={titleRef}>
        {!item.isError && <Skeleton shape="block" />}
        {item.isError &&
          (isApiError(item.error, 404) ? <Notice>{t('admin.item.notFound')}</Notice> : loadError)}
      </AdminPage>
    );
  }

  const matchable = current.kind === 'movie' || current.kind === 'series';
  const tabs: TabItem[] = [
    {
      id: FIRST_TAB,
      label: t('admin.item.facts.metadata'),
      // A dot while the rating or the language has unsaved changes, so they show from every tab.
      ...(drafts.dirty
        ? {
            count: t('admin.item.tabs.unsavedMark'),
            countLabel: t('admin.item.rating.unsaved'),
            tone: 'warning' as const,
          }
        : {}),
      content: (
        <MetadataTab
          item={current}
          isExpanded={isExpanded}
          onExpandedChange={onExpandedChange}
          rating={drafts.valueOf(current.id) ?? ratingChoiceOf(current)}
          ratingDirty={drafts.isDirty(current.id)}
          onRatingChange={(value) => drafts.set(current.id, value)}
          onRatingSave={() => {
            // Nothing changed: nothing to send, the focus stays on «Speichern» (decision 18).
            if (!drafts.isDirty(current.id)) return;
            saveRating.mutate(drafts.valueOf(current.id) ?? ratingChoiceOf(current));
          }}
          ratingSaving={saveRating.isPending}
          ratingError={saveRating.error}
          language={
            hasOriginalLanguage(current)
              ? {
                  value: drafts.valueOf(languageKey(current.id)) ?? languageChoiceOf(current),
                  dirty: drafts.isDirty(languageKey(current.id)),
                  known: languages.data?.knownLanguages ?? [],
                  onChange: (value) => drafts.set(languageKey(current.id), value),
                  onSave: () => {
                    const key = languageKey(current.id);
                    // Nothing changed: nothing to send (decision 18), as with the rating.
                    if (!drafts.isDirty(key)) return;
                    saveLanguage.mutate(drafts.valueOf(key) ?? languageChoiceOf(current));
                  },
                  saving: saveLanguage.isPending,
                  error: saveLanguage.error,
                }
              : undefined
          }
        />
      ),
    },
    ...(kinds
      ? [
          {
            id: 'images',
            label: t('admin.item.images.title'),
            count: number.format(kinds.length),
            countLabel: t('admin.item.tabs.imageKinds', { count: kinds.length }),
            content: (
              <ImagesTab
                item={current}
                kinds={kinds}
                onChanged={(next, text) => void adopt(next, text)}
              />
            ),
          },
        ]
      : []),
    {
      id: 'files',
      label: t('admin.item.files.title'),
      count: number.format(current.mediaFiles.length),
      countLabel: t('admin.item.tabs.files', { count: current.mediaFiles.length }),
      content: (
        <FilesTab item={current} isExpanded={isExpanded} onExpandedChange={onExpandedChange} />
      ),
    },
    {
      id: 'trailers',
      label: t('admin.item.trailers.title'),
      count: number.format(current.trailers.length),
      countLabel: t('admin.item.tabs.trailers', { count: current.trailers.length }),
      content: (
        <TrailersTab
          item={current}
          onAdded={() => settle(t('admin.item.trailers.added'))}
          onDelete={askDelete}
        />
      ),
    },
  ];

  return (
    <AdminPage
      title={current.title}
      titleRef={titleRef}
      actions={
        <>
          {matchable && (
            <AriaButton variant="primary" onPress={() => setMatching(true)}>
              {t('admin.maintenance.action.match')}
            </AriaButton>
          )}
          <AriaButton icon="refresh" isPending={refresh.isPending} onPress={() => refresh.mutate()}>
            {t('admin.maintenance.action.refresh')}
          </AriaButton>
          <ButtonLink href={paths.item(current.id)} variant="ghost" icon="chevronRight">
            {t('admin.item.view')}
          </ButtonLink>
        </>
      }
      status={
        <p className={styles.kindLine}>
          <Chip tone="info">{t(`kind.${current.kind}`)}</Chip>
          {current.year !== undefined && <span className={styles.muted}>{current.year}</span>}
        </p>
      }
    >
      {/* A failed reread keeps the title on the page; the error and «Erneut versuchen» go above. */}
      {loadError}
      {refresh.isError && <Notice tone="danger">{problemMessage(t, refresh.error)}</Notice>}
      {removeTrailer.isError && (
        <Notice tone="danger">{problemMessage(t, removeTrailer.error)}</Notice>
      )}
      <StatusRegion message={message} />
      <Tabs
        label={t('admin.item.tabs.label')}
        selectedKey={tab}
        onSelectionChange={selectTab}
        tabs={tabs}
      />

      {matching && (
        <MatchDialog
          itemId={current.id}
          title={current.title}
          year={current.year}
          kind={current.kind === 'series' ? 'series' : 'movie'}
          onClose={() => setMatching(false)}
          onMatched={(next) => void adopt(next, t('admin.maintenance.done.match'))}
        />
      )}
      {deleting && (
        <ConfirmDialog
          isOpen={confirmOpen}
          onOpenChange={onConfirmOpenChange}
          title={t('admin.item.trailers.delete.title')}
          body={t('admin.item.trailers.delete.body')}
          confirmLabel={t('admin.item.trailers.delete.confirm')}
          cancelLabel={t('common.cancel')}
          tone="danger"
          onConfirm={() => removeTrailer.mutate(deleting)}
        />
      )}
      <ConfirmDialog {...unsaved.dialog} />
    </AdminPage>
  );
}

/** Administration view of one title (B-6, B-8, B-9): state, ids, rating, images, files, trailers. */
export function AdminItemPage() {
  const { t } = useTranslation();
  const { itemId } = route.useParams();
  const session = useActiveSession();
  const allowed = session.account.role === 'admin' && session.profile.isPrimary;
  if (!allowed) {
    return (
      <Page>
        <Notice title={t('admin.item.notAllowed.title')}>{t('admin.item.notAllowed.body')}</Notice>
      </Page>
    );
  }
  return <AdminItemView key={itemId} itemId={itemId} />;
}
