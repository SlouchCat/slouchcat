import type { DuplicateGroup, Issue, IssueKind, RelinkPair } from '@slouchcat/api-client';
import { useMutation, useQueries, useQuery, useQueryClient } from '@tanstack/react-query';
import { useCallback, useMemo, useRef, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { api, isApiError, request } from '@/api/client';
import { problemMessage } from '@/api/errors';
import { useActiveSession } from '@/app/session';
import { useDocumentTitle } from '@/lib/useDocumentTitle';
import { Notice, Page } from '@/ui';
import {
  AdminPage,
  ConfirmDialog,
  Tabs,
  afterDialogsClosed,
  focusTitleIfLost,
  useSearchSelection,
  type StatusMessage,
  type TabItem,
} from '@/ui/aria';
import { DuplicatesTab } from './maintenance/DuplicatesTab';
import { IssueTab, type IssueHandlers } from './maintenance/IssueTab';
import { RatingDialog } from './maintenance/RatingDialog';
import { RelinkTab } from './maintenance/RelinkTab';
import {
  DUPLICATES,
  groupKey,
  pairKey,
  RELINK,
  TABS,
  userDataSummary,
  type Confirmation,
  type Outcome,
} from './maintenance/shared';
import { MatchDialog } from './MatchDialog';
import {
  duplicatesQuery,
  issueCountQuery,
  issueKinds,
  librariesQuery,
  maintenanceKey,
  relinkQuery,
} from './queries';

/** The dialog of the page besides its confirmation: assignment or rating of the selection. */
type ActionDialog = { type: 'match'; issue: Issue } | { type: 'rating'; itemIds: string[] } | null;

/**
 * Library maintenance (B-9; concept 12, variant C): one tab per kind of issue with its count
 * (PFL-1) and the duplicates. A tab shows its hint, the toolbar with the actions and the
 * selection, and the loaded issues in one section per library with «Mehr laden» (PFL-2, PFL-3);
 * the duplicates come as one section per group (PFL-4), the orphaned user data as one table of
 * pairs (ADR 0023). Selection, outcome and the title to keep belong to the page, so they stay
 * when the tab changes (S-17).
 */
export function MaintenancePage() {
  const { t } = useTranslation();
  useDocumentTitle(t('admin.maintenance.title'));
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
  return <MaintenanceContent />;
}

function MaintenanceContent() {
  const { t, i18n } = useTranslation();
  const queryClient = useQueryClient();
  const titleRef = useRef<HTMLHeadingElement>(null);
  const number = useMemo(() => new Intl.NumberFormat(i18n.language), [i18n.language]);

  // An unknown `?tab=` shows the first tab; the address says so too.
  const [tab, selectTab] = useSearchSelection('tab', TABS, issueKinds[0], { normalise: true });

  const [selection, setSelection] = useState<Partial<Record<IssueKind, string[]>>>({});
  // Keyed per message, so the same outcome twice in a row is announced again.
  const [outcomes, setOutcomes] = useState<Partial<Record<string, StatusMessage>>>({});
  const [expanded, setExpanded] = useState<Record<string, boolean>>({});
  const [targets, setTargets] = useState<Record<string, string>>({});
  // The pairs taken out of the orphaned user data; every other pair is selected.
  const [relinkExcluded, setRelinkExcluded] = useState<string[]>([]);
  const [dialog, setDialog] = useState<ActionDialog>(null);
  const [confirmation, setConfirmation] = useState<Confirmation | null>(null);
  const [confirmOpen, setConfirmOpen] = useState(false);
  // While a dialog is open nothing refetches on its own: the rows it was opened for stay.
  const hold = confirmOpen || dialog !== null;

  const counts = useQueries({
    queries: issueKinds.map((kind) => ({
      ...issueCountQuery(kind),
      refetchOnWindowFocus: !hold,
    })),
  });
  const duplicates = useQuery({ ...duplicatesQuery, refetchOnWindowFocus: !hold });
  const relinkPairs = useQuery({ ...relinkQuery, refetchOnWindowFocus: !hold });
  // The names of the sections; the page does not follow a scan.
  const libraries = useQuery({
    ...librariesQuery,
    refetchInterval: false,
    refetchOnWindowFocus: !hold,
  });

  /**
   * Shows the outcome in the tab, drops its selection and fetches again. Once React has rendered
   * the answer, the focus moves to the h1 when the rows that had it are gone (ALL-2), or when the
   * button that had it went off with the selection.
   */
  const finish = useCallback(
    async (tabId: string, outcome: Outcome, removed = false) => {
      setOutcomes((current) => ({
        ...current,
        [tabId]: { ...outcome, key: (current[tabId]?.key ?? 0) + 1 },
      }));
      setSelection((current) => ({ ...current, [tabId]: [] }));
      await queryClient.invalidateQueries({ queryKey: maintenanceKey });
      await afterDialogsClosed();
      if (removed) titleRef.current?.focus();
      else focusTitleIfLost(titleRef);
    },
    [queryClient],
  );
  const failed = (tabId: string, error: unknown) =>
    setOutcomes((current) => ({
      ...current,
      [tabId]: {
        tone: 'danger',
        text: problemMessage(t, error),
        key: (current[tabId]?.key ?? 0) + 1,
      },
    }));

  const refreshItems = useMutation({
    // One request after the other: a selection over several pages must not fire hundreds of
    // requests at once, and one failure must not hide what the others did.
    mutationFn: async ({ ids }: { kind: IssueKind; ids: string[] }) => {
      let failures = 0;
      let firstError: unknown;
      for (const id of ids) {
        try {
          await request(api.POST('/admin/items/{id}/refresh', { params: { path: { id } } }));
        } catch (error) {
          failures += 1;
          firstError ??= error;
        }
      }
      return { failures, firstError };
    },
    onSuccess: ({ failures, firstError }, { kind, ids }) =>
      finish(
        kind,
        failures === 0
          ? { tone: 'info', text: t('admin.maintenance.done.refresh', { count: ids.length }) }
          : {
              tone: 'danger',
              text: t('admin.maintenance.done.refreshFailed', {
                count: ids.length,
                failed: failures,
                error: problemMessage(t, firstError),
              }),
            },
      ),
    onError: (error, { kind }) => failed(kind, error),
  });
  const cleanup = useMutation({
    mutationFn: ({
      body,
    }: {
      kind: IssueKind;
      body: { itemIds?: string[]; mediaFileIds?: string[] };
    }) => request(api.POST('/admin/maintenance/cleanup', { body })),
    onSuccess: (result, { kind }) =>
      finish(
        kind,
        {
          tone: 'info',
          text: t('admin.maintenance.done.cleanup', {
            items: t('admin.maintenance.titleCount', { count: result.items }),
            files: t('admin.maintenance.fileCount', { count: result.files }),
          }),
        },
        true,
      ),
    onError: (error, { kind }) => failed(kind, error),
  });
  const merge = useMutation({
    mutationFn: ({ group, targetId }: { group: DuplicateGroup; targetId: string }) =>
      request(
        api.POST('/admin/maintenance/merge', {
          body: {
            targetId,
            sourceIds: group.items.map((item) => item.itemId).filter((id) => id !== targetId),
          },
        }),
      ),
    onSuccess: () =>
      finish(DUPLICATES, { tone: 'info', text: t('admin.maintenance.done.merge') }, true),
    onError: (error) => failed(DUPLICATES, error),
  });

  const relink = useMutation({
    mutationFn: (pairs: RelinkPair[]) =>
      request(
        api.POST('/admin/maintenance/relink', {
          body: { pairs: pairs.map(({ deletedId, liveId }) => ({ deletedId, liveId })) },
        }),
      ),
    onSuccess: async (result) => {
      const count = result.relinked.length;
      const text = [
        result.stoppedBy === 'scan_running'
          ? t('admin.maintenance.done.relinkStopped', { count })
          : t('admin.maintenance.done.relink', {
              titles: t('admin.maintenance.titleCount', { count }),
            }),
        result.skipped > 0 && t('admin.maintenance.done.relinkSkipped', { count: result.skipped }),
      ]
        .filter(Boolean)
        .join(' ');
      // The h1 takes the focus only when the rows are gone (focusTitleIfLost).
      await finish(RELINK, { tone: 'info', text });
      // The pairs taken out stay out; only those no longer listed are forgotten.
      const listed = new Set(queryClient.getQueryData(relinkQuery.queryKey)?.pairs.map(pairKey));
      setRelinkExcluded((current) => current.filter((key) => listed.has(key)));
    },
    // A refused request means a scan runs (409) or the request was malformed (422; a pair that is
    // gone is skipped, not refused): the list reloads either way, so a second try acts on what is
    // there now.
    onError: (error) => {
      setOutcomes((current) => ({
        ...current,
        [RELINK]: {
          tone: 'danger',
          text: isApiError(error, 422)
            ? t('admin.maintenance.relink.listChanged')
            : problemMessage(t, error),
          key: (current[RELINK]?.key ?? 0) + 1,
        },
      }));
      void queryClient.invalidateQueries({ queryKey: maintenanceKey });
    },
  });

  // Every confirmation is the page's own: the rows that asked for it may go while it is open.
  const ask = useCallback((next: Confirmation) => {
    setConfirmation(next);
    setConfirmOpen(true);
  }, []);
  const onConfirmOpenChange = useCallback((open: boolean) => {
    if (open) return;
    setConfirmOpen(false);
    // The button that opened it may be gone or off by now.
    void afterDialogsClosed().then(() => focusTitleIfLost(titleRef));
  }, []);
  const closeDialog = useCallback(() => {
    setDialog(null);
    void afterDialogsClosed().then(() => focusTitleIfLost(titleRef));
  }, []);

  const { mutate: refreshMutate } = refreshItems;
  const { mutate: cleanupMutate } = cleanup;
  const { mutate: mergeMutate } = merge;
  const { mutate: relinkMutate } = relink;
  const handlers = useMemo<IssueHandlers>(
    () => ({
      onSelect: (kind, keys) => setSelection((current) => ({ ...current, [kind]: keys })),
      onExpandedChange: (key, open) => setExpanded((current) => ({ ...current, [key]: open })),
      onMatch: (issue) => setDialog({ type: 'match', issue }),
      onRefresh: (kind, ids) => refreshMutate({ kind, ids }),
      onRating: (itemIds) => setDialog({ type: 'rating', itemIds }),
      onCleanup: (kind, itemIds) =>
        ask({
          title: t('admin.maintenance.cleanup.title'),
          body: t('admin.maintenance.cleanup.body', { count: itemIds.length }),
          confirmLabel: t('admin.maintenance.cleanup.confirm'),
          tone: 'danger',
          onConfirm: () => cleanupMutate({ kind, body: { itemIds } }),
        }),
      onRemove: (kind, mediaFileIds) =>
        ask({
          title: t('admin.maintenance.remove.title'),
          body: t('admin.maintenance.remove.body', { count: mediaFileIds.length }),
          confirmLabel: t('admin.maintenance.remove.confirm'),
          tone: 'danger',
          onConfirm: () => cleanupMutate({ kind, body: { mediaFileIds } }),
        }),
    }),
    [ask, t, refreshMutate, cleanupMutate],
  );
  const onMerge = useCallback(
    (group: DuplicateGroup, targetId: string) => {
      const target = group.items.find((item) => item.itemId === targetId);
      ask({
        title: t('admin.maintenance.duplicates.confirmTitle'),
        body: t('admin.maintenance.duplicates.confirmBody', {
          count: group.items.length - 1,
          title: target?.title ?? '',
        }),
        confirmLabel: t('admin.maintenance.duplicates.merge'),
        // The other titles are deleted; the button that asks stays primary (mockup Pfl-C).
        tone: 'danger',
        onConfirm: () => mergeMutate({ group, targetId }),
      });
    },
    [ask, t, mergeMutate],
  );

  const onRelink = useCallback(
    (pairs: RelinkPair[]) => {
      // Two copies of one deleted title: one merges, the other is skipped; the user data of the
      // title counts once.
      const titles = [...new Map(pairs.map((pair) => [pair.deletedId, pair])).values()];
      const data = userDataSummary(t, titles);
      const count = titles.length;
      const extra = pairs.length - count;
      ask({
        title: t('admin.maintenance.relink.confirmTitle'),
        body: [
          t('admin.maintenance.relink.confirmMerge', { count }),
          data
            ? t('admin.maintenance.relink.confirmKept', { data })
            : t('admin.maintenance.relink.confirmNoData', { count }),
          t('admin.maintenance.relink.confirmRemoved', { count }),
          extra > 0 && t('admin.maintenance.relink.confirmExtra', { count: extra }),
        ]
          .filter(Boolean)
          .join(' '),
        confirmLabel: t('admin.maintenance.relink.action'),
        onConfirm: () => relinkMutate(pairs),
      });
    },
    [ask, t, relinkMutate],
  );

  const relinkTotal = relinkPairs.data?.total;
  const tabs: TabItem[] = [
    ...issueKinds.map((kind, index) => {
      const count = counts[index]?.data;
      return {
        id: kind,
        label: t(`admin.maintenance.tab.${kind}`),
        count: count === undefined ? undefined : number.format(count),
        countLabel: count === undefined ? undefined : t('admin.maintenance.entries', { count }),
        content: (
          <IssueTab
            kind={kind}
            libraries={libraries.data}
            selected={selection[kind] ?? []}
            outcome={outcomes[kind]}
            expanded={expanded}
            hold={hold}
            refreshing={refreshItems.isPending && refreshItems.variables.kind === kind}
            cleaning={cleanup.isPending}
            handlers={handlers}
          />
        ),
      };
    }),
    {
      id: DUPLICATES,
      label: t('admin.maintenance.tab.duplicates'),
      count: duplicates.data === undefined ? undefined : number.format(duplicates.data.length),
      countLabel:
        duplicates.data === undefined
          ? undefined
          : t('admin.maintenance.duplicates.groups', { count: duplicates.data.length }),
      content: (
        <DuplicatesTab
          outcome={outcomes[DUPLICATES]}
          targets={targets}
          expanded={expanded}
          hold={hold}
          merging={merge.isPending ? groupKey(merge.variables.group) : undefined}
          onTarget={(key, itemId) => setTargets((current) => ({ ...current, [key]: itemId }))}
          onExpandedChange={handlers.onExpandedChange}
          onMerge={onMerge}
        />
      ),
    },
    {
      id: RELINK,
      label: t('admin.maintenance.tab.relink'),
      count: relinkTotal === undefined ? undefined : number.format(relinkTotal),
      countLabel:
        relinkTotal === undefined
          ? undefined
          : t('admin.maintenance.titleCount', { count: relinkTotal }),
      content: (
        <RelinkTab
          outcome={outcomes[RELINK]}
          excluded={relinkExcluded}
          hold={hold}
          relinking={relink.isPending}
          onExcludedChange={setRelinkExcluded}
          onRelink={onRelink}
        />
      ),
    },
  ];

  return (
    <AdminPage
      title={t('admin.maintenance.title')}
      help={t('admin.maintenance.intro')}
      titleRef={titleRef}
    >
      <Tabs
        label={t('admin.maintenance.tabs')}
        selectedKey={tab}
        onSelectionChange={selectTab}
        tabs={tabs}
      />
      {dialog?.type === 'match' && (
        <MatchDialog
          itemId={dialog.issue.itemId}
          title={dialog.issue.title}
          year={dialog.issue.year}
          kind={dialog.issue.itemKind === 'series' ? 'series' : 'movie'}
          onClose={closeDialog}
          onMatched={() =>
            void finish(dialog.issue.kind, {
              tone: 'info',
              text: t('admin.maintenance.done.match'),
            })
          }
        />
      )}
      {dialog?.type === 'rating' && (
        <RatingDialog
          itemIds={dialog.itemIds}
          onClose={closeDialog}
          onDone={(outcome) => void finish('unrated', outcome)}
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
          tone={confirmation.tone}
          onConfirm={confirmation.onConfirm}
        />
      )}
    </AdminPage>
  );
}
