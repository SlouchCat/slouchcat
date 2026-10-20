import type { DuplicateGroup, Issue, IssueKind, Rating } from '@slouchcat/api-client';
import {
  useInfiniteQuery,
  useMutation,
  useQueries,
  useQuery,
  useQueryClient,
} from '@tanstack/react-query';
import type { TFunction } from 'i18next';
import { useState, type FormEvent, type ReactNode } from 'react';
import { Form } from 'react-aria-components';
import { useTranslation } from 'react-i18next';
import { api, request } from '@/api/client';
import { problemMessage } from '@/api/errors';
import { paths } from '@/app/paths';
import { useActiveSession } from '@/app/session';
import { formatBytes } from '@/lib/format';
import { useDocumentTitle } from '@/lib/useDocumentTitle';
import { Notice, Page, Panel, Skeleton, Text } from '@/ui';
import {
  AriaButton,
  ConfirmDialog,
  FormDialog,
  RadioGroup,
  SelectableList,
  Tabs,
  type SelectableItem,
} from '@/ui/aria';
import { ratingOptions, titleRatingOf } from '../shared/choices';
import { MatchDialog } from './MatchDialog';
import {
  duplicatesQuery,
  issueCountQuery,
  issueKinds,
  issuesQuery,
  maintenanceKey,
} from './queries';
import styles from './MaintenancePage.module.css';

const DUPLICATES = 'duplicates';

/** Selection key of an issue: unplayable issues are per file, the others per title. */
const issueKey = (issue: Issue) => `${issue.itemId}:${issue.mediaFileId ?? ''}`;

const titleWithYear = (t: TFunction, title: string, year: number | undefined) =>
  year !== undefined ? t('admin.titleYear', { title, year }) : title;

/** Outcome line above the list after an action. */
type Outcome = { tone: 'info' | 'danger'; text: string } | null;

function useMaintenanceRefresh() {
  const queryClient = useQueryClient();
  return () => queryClient.invalidateQueries({ queryKey: maintenanceKey });
}

function RatingDialog({
  itemIds,
  onClose,
  onDone,
}: {
  itemIds: string[];
  onClose: () => void;
  onDone: (outcome: Outcome) => void;
}) {
  const { t } = useTranslation();
  // No preselection: an unrated title counts as FSK 18, and a wrong default (FSK 0) would open it
  // to every profile with two key presses.
  const [rating, setRating] = useState('');
  const chosen = titleRatingOf(rating);
  const assign = useMutation({
    mutationFn: (value: Rating) =>
      request(api.POST('/admin/items/rating', { body: { itemIds, rating: value } })),
    onSuccess: (result) => {
      onDone({ tone: 'info', text: t('admin.maintenance.done.rating', { count: result.updated }) });
      onClose();
    },
  });
  const onSubmit = (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    if (chosen !== undefined && !assign.isPending) assign.mutate(chosen);
  };
  return (
    <FormDialog
      isOpen
      onOpenChange={(open) => {
        if (!open) onClose();
      }}
      title={t('admin.maintenance.rating.title')}
      description={t('admin.maintenance.rating.description', { count: itemIds.length })}
    >
      <Form className={styles.form} onSubmit={onSubmit}>
        {assign.isError && <Notice tone="danger">{problemMessage(t, assign.error)}</Notice>}
        <RadioGroup
          name="rating"
          label={t('admin.maintenance.rating.label')}
          value={rating}
          onChange={setRating}
          options={ratingOptions(t)}
          isRequired
        />
        <p className={styles.muted} aria-live="polite">
          {chosen !== undefined &&
            t('admin.maintenance.rating.summary', {
              count: itemIds.length,
              rating: t('rating.fsk', { age: chosen }),
            })}
        </p>
        <div className={styles.dialogActions}>
          <AriaButton variant="ghost" onPress={onClose}>
            {t('common.cancel')}
          </AriaButton>
          <AriaButton
            type="submit"
            variant="primary"
            isDisabled={chosen === undefined}
            isPending={assign.isPending}
          >
            {assign.isPending
              ? t('admin.maintenance.rating.submitting')
              : t('admin.maintenance.rating.submit')}
          </AriaButton>
        </div>
      </Form>
    </FormDialog>
  );
}

function issueMeta(t: TFunction, issue: Issue): ReactNode {
  return (
    <>
      <span>{t(`kind.${issue.itemKind}`)}</span>
      {issue.path && <code className={styles.path}>{issue.path}</code>}
      {issue.detail && <span>{issue.detail}</span>}
    </>
  );
}

/** One tab of the maintenance page: the issues of one kind, selectable, with its actions. */
function IssueTab({ kind }: { kind: IssueKind }) {
  const { t } = useTranslation();
  const refresh = useMaintenanceRefresh();
  const issues = useInfiniteQuery(issuesQuery(kind));
  const [selected, setSelected] = useState<string[]>([]);
  const [outcome, setOutcome] = useState<Outcome>(null);
  const [dialog, setDialog] = useState<'match' | 'rating' | null>(null);

  const list = issues.data?.pages.flatMap((page) => page.items) ?? [];
  const total = issues.data?.pages.at(-1)?.total ?? 0;
  const chosen = list.filter((issue) => selected.includes(issueKey(issue)));
  const itemIds = [...new Set(chosen.map((issue) => issue.itemId))];
  const mediaFileIds = chosen.flatMap((issue) => (issue.mediaFileId ? [issue.mediaFileId] : []));
  const selectable = kind !== 'rating_mismatch';

  const finish = async (next: Outcome) => {
    setOutcome(next);
    setSelected([]);
    await refresh();
  };
  const failed = (error: unknown) => setOutcome({ tone: 'danger', text: problemMessage(t, error) });

  const refreshItems = useMutation({
    // One request after the other: a selection over several pages must not fire hundreds of
    // requests at once, and one failure must not hide what the others did.
    mutationFn: async (ids: string[]) => {
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
    onSuccess: ({ failures, firstError }, ids) =>
      finish(
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
    onError: failed,
  });
  const cleanup = useMutation({
    mutationFn: (body: { itemIds?: string[]; mediaFileIds?: string[] }) =>
      request(api.POST('/admin/maintenance/cleanup', { body })),
    onSuccess: (result) =>
      finish({
        tone: 'info',
        text: t('admin.maintenance.done.cleanup', {
          items: t('admin.maintenance.titleCount', { count: result.items }),
          files: t('admin.maintenance.fileCount', { count: result.files }),
        }),
      }),
    onError: failed,
  });

  const none = chosen.length === 0;
  const matchTarget = chosen.length === 1 ? chosen[0] : undefined;
  const canMatch =
    matchTarget !== undefined &&
    (matchTarget.itemKind === 'movie' || matchTarget.itemKind === 'series');

  const refreshButton = (label: string) => (
    <AriaButton
      icon="refresh"
      isDisabled={none}
      isPending={refreshItems.isPending}
      onPress={() => refreshItems.mutate(itemIds)}
    >
      {label}
    </AriaButton>
  );

  const actions: Partial<Record<IssueKind, ReactNode>> = {
    unmatched: (
      <>
        <AriaButton variant="primary" isDisabled={!canMatch} onPress={() => setDialog('match')}>
          {t('admin.maintenance.action.match')}
        </AriaButton>
        {refreshButton(t('admin.maintenance.action.refresh'))}
      </>
    ),
    no_images: refreshButton(t('admin.maintenance.action.refresh')),
    unrated: (
      <AriaButton variant="primary" isDisabled={none} onPress={() => setDialog('rating')}>
        {t('admin.maintenance.action.rating')}
      </AriaButton>
    ),
    orphaned: (
      <ConfirmDialog
        trigger={
          <AriaButton variant="danger" isDisabled={none || cleanup.isPending}>
            {t('admin.maintenance.action.cleanup')}
          </AriaButton>
        }
        title={t('admin.maintenance.cleanup.title')}
        body={t('admin.maintenance.cleanup.body', { count: itemIds.length })}
        confirmLabel={t('admin.maintenance.cleanup.confirm')}
        cancelLabel={t('common.cancel')}
        tone="danger"
        onConfirm={() => cleanup.mutate({ itemIds })}
      />
    ),
    unplayable: (
      <>
        {refreshButton(t('admin.maintenance.action.recheck'))}
        <ConfirmDialog
          trigger={
            <AriaButton variant="danger" isDisabled={none || cleanup.isPending}>
              {t('admin.maintenance.action.remove')}
            </AriaButton>
          }
          title={t('admin.maintenance.remove.title')}
          body={t('admin.maintenance.remove.body', { count: mediaFileIds.length })}
          confirmLabel={t('admin.maintenance.remove.confirm')}
          cancelLabel={t('common.cancel')}
          tone="danger"
          onConfirm={() => cleanup.mutate({ mediaFileIds })}
        />
      </>
    ),
  };

  const items: SelectableItem[] = list.map((issue) => ({
    id: issueKey(issue),
    title: titleWithYear(t, issue.title, issue.year),
    href: paths.adminItem(issue.itemId),
    meta: issueMeta(t, issue),
    selectLabel: t('admin.maintenance.selectItem', { title: issue.title }),
  }));
  const allKeys = items.map((item) => item.id);

  return (
    <div className={styles.tab}>
      <Text tone="secondary">{t(`admin.maintenance.hint.${kind}`)}</Text>
      {outcome && <Notice tone={outcome.tone}>{outcome.text}</Notice>}
      {issues.isPending && <Skeleton shape="block" />}
      {issues.isError && (
        <Notice
          tone="danger"
          title={problemMessage(t, issues.error)}
          actions={
            <AriaButton icon="refresh" onPress={() => void issues.refetch()}>
              {t('common.retry')}
            </AriaButton>
          }
        />
      )}
      {issues.isSuccess && list.length === 0 && <Notice>{t('admin.maintenance.empty')}</Notice>}
      {list.length > 0 && (
        <>
          {selectable && (
            <div className={styles.toolbar}>
              <div className={styles.actions}>{actions[kind]}</div>
              <div className={styles.actions}>
                <span className={styles.muted} aria-live="polite">
                  {t('admin.maintenance.selected', { count: chosen.length })}
                </span>
                <AriaButton
                  variant="ghost"
                  onPress={() => setSelected(chosen.length === items.length ? [] : allKeys)}
                >
                  {chosen.length === items.length
                    ? t('admin.maintenance.selectNone')
                    : t('admin.maintenance.selectAll')}
                </AriaButton>
              </div>
            </div>
          )}
          <SelectableList
            label={t('admin.maintenance.selection')}
            items={items}
            value={selectable ? selected : undefined}
            onChange={selectable ? setSelected : undefined}
          />
          <div className={styles.actions}>
            <span className={styles.muted}>
              {t('admin.maintenance.shown', { shown: list.length, total })}
            </span>
            {issues.hasNextPage && (
              <AriaButton
                variant="secondary"
                isPending={issues.isFetchingNextPage}
                onPress={() => void issues.fetchNextPage()}
              >
                {t('admin.maintenance.more')}
              </AriaButton>
            )}
          </div>
        </>
      )}

      {dialog === 'match' && matchTarget && (
        <MatchDialog
          itemId={matchTarget.itemId}
          title={matchTarget.title}
          year={matchTarget.year}
          kind={matchTarget.itemKind === 'series' ? 'series' : 'movie'}
          onClose={() => setDialog(null)}
          onMatched={() => void finish({ tone: 'info', text: t('admin.maintenance.done.match') })}
        />
      )}
      {dialog === 'rating' && (
        <RatingDialog
          itemIds={itemIds}
          onClose={() => setDialog(null)}
          onDone={(next) => void finish(next)}
        />
      )}
    </div>
  );
}

function DuplicateCard({
  group,
  index,
  onMerged,
}: {
  group: DuplicateGroup;
  index: number;
  onMerged: (outcome: Outcome) => void;
}) {
  const { t, i18n } = useTranslation();
  const [target, setTarget] = useState(group.items[0]?.itemId ?? '');
  const targetItem = group.items.find((item) => item.itemId === target);
  const merge = useMutation({
    mutationFn: () =>
      request(
        api.POST('/admin/maintenance/merge', {
          body: {
            targetId: target,
            sourceIds: group.items.map((item) => item.itemId).filter((id) => id !== target),
          },
        }),
      ),
    onSuccess: () => onMerged({ tone: 'info', text: t('admin.maintenance.done.merge') }),
    onError: (error) => onMerged({ tone: 'danger', text: problemMessage(t, error) }),
  });
  const headingId = `duplicate-${index}`;

  return (
    <Panel as="article" aria-labelledby={headingId}>
      <div className={styles.groupHead}>
        <h3 id={headingId} className={styles.groupTitle}>
          {t('admin.maintenance.duplicates.group', { index: index + 1 })}
        </h3>
        <span className={styles.muted}>
          {t(`admin.maintenance.duplicates.reason.${group.reason}`)}
        </span>
      </div>
      <SelectableList
        label={t('admin.maintenance.duplicates.group', { index: index + 1 })}
        items={group.items.map((item) => ({
          id: item.itemId,
          title: titleWithYear(t, item.title, item.year),
          href: paths.adminItem(item.itemId),
          selectLabel: item.title,
          meta: (
            <>
              <span>{t(`kind.${item.kind}`)}</span>
              {item.files.map((file) => (
                <span key={file.id} className={styles.fileLine}>
                  <code className={styles.path}>{file.path}</code>
                  <span>{formatBytes(file.sizeBytes, i18n.language)}</span>
                </span>
              ))}
            </>
          ),
        }))}
      />
      <div className={styles.mergeRow}>
        <RadioGroup
          label={t('admin.maintenance.duplicates.target')}
          value={target}
          onChange={setTarget}
          options={group.items.map((item, position) => ({
            value: item.itemId,
            label: t('admin.maintenance.duplicates.option', {
              index: position + 1,
              title: item.title,
            }),
          }))}
        />
        <ConfirmDialog
          trigger={
            <AriaButton variant="primary" isPending={merge.isPending}>
              {t('admin.maintenance.duplicates.merge')}
            </AriaButton>
          }
          title={t('admin.maintenance.duplicates.confirmTitle')}
          body={t('admin.maintenance.duplicates.confirmBody', {
            count: group.items.length - 1,
            title: targetItem?.title ?? '',
          })}
          confirmLabel={t('admin.maintenance.duplicates.merge')}
          cancelLabel={t('common.cancel')}
          onConfirm={() => merge.mutate()}
        />
      </div>
    </Panel>
  );
}

function DuplicatesTab() {
  const { t } = useTranslation();
  const refresh = useMaintenanceRefresh();
  const duplicates = useQuery(duplicatesQuery);
  const [outcome, setOutcome] = useState<Outcome>(null);

  return (
    <div className={styles.tab}>
      <Text tone="secondary">{t('admin.maintenance.hint.duplicates')}</Text>
      {outcome && <Notice tone={outcome.tone}>{outcome.text}</Notice>}
      {duplicates.isPending && <Skeleton shape="block" />}
      {duplicates.isError && (
        <Notice
          tone="danger"
          title={problemMessage(t, duplicates.error)}
          actions={
            <AriaButton icon="refresh" onPress={() => void duplicates.refetch()}>
              {t('common.retry')}
            </AriaButton>
          }
        />
      )}
      {duplicates.data?.length === 0 && <Notice>{t('admin.maintenance.duplicates.empty')}</Notice>}
      {duplicates.data?.map((group, index) => (
        <DuplicateCard
          key={group.items.map((item) => item.itemId).join('-')}
          group={group}
          index={index}
          onMerged={(next) => {
            setOutcome(next);
            void refresh();
          }}
        />
      ))}
    </div>
  );
}

/** Library maintenance (B-9): what needs attention, per kind of issue, with the fitting actions. */
export function MaintenancePage() {
  const { t } = useTranslation();
  useDocumentTitle(t('admin.maintenance.title'));
  const session = useActiveSession();
  const allowed = session.account.role === 'owner' && session.profile.isPrimary;
  const [tab, setTab] = useState<string>(issueKinds[0]);
  const counts = useQueries({
    queries: issueKinds.map((kind) => ({ ...issueCountQuery(kind), enabled: allowed })),
  });
  const duplicates = useQuery({ ...duplicatesQuery, enabled: allowed });

  const label = (text: string, count: number | undefined) =>
    count === undefined ? text : t('admin.maintenance.tabCount', { label: text, count });

  return (
    <Page>
      <div className={styles.header}>
        <Text as="h1" variant="display">
          {t('admin.maintenance.title')}
        </Text>
        <Text tone="secondary">{t('admin.maintenance.intro')}</Text>
      </div>
      {allowed ? (
        <Tabs
          label={t('admin.maintenance.tabs')}
          selectedKey={tab}
          onSelectionChange={setTab}
          tabs={[
            ...issueKinds.map((kind, index) => ({
              id: kind,
              label: label(t(`admin.maintenance.tab.${kind}`), counts[index]?.data),
              content: <IssueTab kind={kind} />,
            })),
            {
              id: DUPLICATES,
              label: label(t('admin.maintenance.tab.duplicates'), duplicates.data?.length),
              content: <DuplicatesTab />,
            },
          ]}
        />
      ) : (
        <Notice title={t('admin.maintenance.notAllowed.title')}>
          {t('admin.maintenance.notAllowed.body')}
        </Notice>
      )}
    </Page>
  );
}
