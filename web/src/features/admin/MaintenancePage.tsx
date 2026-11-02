import type { DuplicateGroup, Issue, IssueKind } from '@slouchcat/api-client';
import { useMutation, useQueries, useQuery, useQueryClient } from '@tanstack/react-query';
import { useSearch } from '@tanstack/react-router';
import { useCallback, useEffect, useMemo, useRef, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { api, request } from '@/api/client';
import { problemMessage } from '@/api/errors';
import { useActiveSession } from '@/app/session';
import { useDocumentTitle } from '@/lib/useDocumentTitle';
import { Notice, Page } from '@/ui';
import { AdminPage, ConfirmDialog, Tabs, useSearchSelection, type TabItem } from '@/ui/aria';
import { DuplicatesTab } from './maintenance/DuplicatesTab';
import { IssueTab, type IssueHandlers } from './maintenance/IssueTab';
import { RatingDialog } from './maintenance/RatingDialog';
import {
  afterRender,
  DUPLICATES,
  groupKey,
  TABS,
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
} from './queries';

/** The dialog of the page besides its confirmation: assignment or rating of the selection. */
type ActionDialog = { type: 'match'; issue: Issue } | { type: 'rating'; itemIds: string[] } | null;

/**
 * Library maintenance (B-9; concept 12, variant C): one tab per kind of issue with its count
 * (PFL-1) and the duplicates. A tab shows its hint, the toolbar with the actions and the
 * selection, and the loaded issues in one section per library with «Mehr laden» (PFL-2, PFL-3);
 * the duplicates come as one section per group (PFL-4). Selection, outcome and the title to keep
 * belong to the page, so they stay when the tab changes (S-17).
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

  const [tab, selectTab] = useSearchSelection('tab', TABS, issueKinds[0]);
  // An unknown `?tab=` shows the first tab; the address says so too.
  const search: Record<string, unknown> = useSearch({ strict: false });
  const askedTab = search.tab;
  useEffect(() => {
    if (askedTab !== undefined && !(typeof askedTab === 'string' && TABS.includes(askedTab)))
      selectTab(issueKinds[0]);
  }, [askedTab, selectTab]);

  const [selection, setSelection] = useState<Partial<Record<IssueKind, string[]>>>({});
  const [outcomes, setOutcomes] = useState<Partial<Record<string, Outcome>>>({});
  const [expanded, setExpanded] = useState<Record<string, boolean>>({});
  const [targets, setTargets] = useState<Record<string, string>>({});
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
  // The names of the sections; the page does not follow a scan.
  const libraries = useQuery({
    ...librariesQuery,
    refetchInterval: false,
    refetchOnWindowFocus: !hold,
  });

  /** The focus fell to the body or onto a button the change switched off: the h1 takes it. */
  const focusTitleIfLost = useCallback(() => {
    const active = document.activeElement;
    if (!active || active === document.body || active.matches(':disabled'))
      titleRef.current?.focus();
  }, []);
  /**
   * Shows the outcome in the tab, drops its selection and fetches again. Once React has rendered
   * the answer, the focus moves to the h1 when the rows that had it are gone (ALL-2), or when the
   * button that had it went off with the selection.
   */
  const finish = useCallback(
    async (tabId: string, outcome: Outcome, removed = false) => {
      setOutcomes((current) => ({ ...current, [tabId]: outcome }));
      setSelection((current) => ({ ...current, [tabId]: [] }));
      await queryClient.invalidateQueries({ queryKey: maintenanceKey });
      await afterRender();
      if (removed) titleRef.current?.focus();
      else focusTitleIfLost();
    },
    [queryClient, focusTitleIfLost],
  );
  const failed = (tabId: string, error: unknown) =>
    setOutcomes((current) => ({
      ...current,
      [tabId]: { tone: 'danger', text: problemMessage(t, error) },
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

  // Every confirmation is the page's own: the rows that asked for it may go while it is open.
  const ask = useCallback((next: Confirmation) => {
    setConfirmation(next);
    setConfirmOpen(true);
  }, []);
  const onConfirmOpenChange = useCallback(
    (open: boolean) => {
      if (open) return;
      setConfirmOpen(false);
      // The button that opened it may be gone or off by now.
      void afterRender().then(focusTitleIfLost);
    },
    [focusTitleIfLost],
  );
  const closeDialog = useCallback(() => {
    setDialog(null);
    void afterRender().then(focusTitleIfLost);
  }, [focusTitleIfLost]);

  const { mutate: refreshMutate } = refreshItems;
  const { mutate: cleanupMutate } = cleanup;
  const { mutate: mergeMutate } = merge;
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
