import type { Issue, IssueKind, Library } from '@slouchcat/api-client';
import { useInfiniteQuery } from '@tanstack/react-query';
import type { TFunction } from 'i18next';
import { useMemo, type ReactNode } from 'react';
import { useTranslation } from 'react-i18next';
import { problemMessage } from '@/api/errors';
import { paths } from '@/app/paths';
import { Notice, Skeleton, Text, TextLink, VisuallyHidden } from '@/ui';
import { AriaButton, DataTable, DisclosureGroup, SelectCheck, type DataColumn } from '@/ui/aria';
import { issuesQuery } from '../queries';
import { issueKey, libraryKey, titleWithYear, type Outcome } from './shared';
import styles from './Maintenance.module.css';

/** What the page does for a tab: its actions, held by the page so a dialog outlives the rows. */
export interface IssueHandlers {
  onSelect: (kind: IssueKind, keys: string[]) => void;
  onExpandedChange: (key: string, expanded: boolean) => void;
  onMatch: (issue: Issue) => void;
  onRefresh: (kind: IssueKind, itemIds: string[]) => void;
  onRating: (itemIds: string[]) => void;
  onCleanup: (kind: IssueKind, itemIds: string[]) => void;
  onRemove: (kind: IssueKind, mediaFileIds: string[]) => void;
}

interface IssueTabProps {
  kind: IssueKind;
  /** The libraries in the order of the libraries page: they name and order the sections. */
  libraries: readonly Library[] | undefined;
  selected: readonly string[];
  outcome: Outcome | undefined;
  /** Open state of the library sections; a section is open until it is folded away. */
  expanded: Readonly<Record<string, boolean>>;
  /** A dialog or a confirmation is open: no refetch on window focus meanwhile. */
  hold: boolean;
  refreshing: boolean;
  cleaning: boolean;
  handlers: IssueHandlers;
}

interface LibrarySection {
  id: number;
  name: string;
  issues: Issue[];
}

/**
 * The loaded issues by library (11.3): the sections follow the libraries page, a library the
 * list does not know (yet) comes last under its id. Only what is loaded is grouped; «Mehr laden»
 * adds to the sections.
 */
function sectionsOf(
  t: TFunction,
  issues: readonly Issue[],
  libraries: readonly Library[] | undefined,
): LibrarySection[] {
  const byLibrary = new Map<number, Issue[]>();
  for (const issue of issues) {
    const list = byLibrary.get(issue.libraryId);
    if (list) list.push(issue);
    else byLibrary.set(issue.libraryId, [issue]);
  }
  const known = (libraries ?? []).flatMap((library) => {
    const list = byLibrary.get(library.id);
    return list ? [{ id: library.id, name: library.name, issues: list }] : [];
  });
  const knownIds = new Set(known.map((section) => section.id));
  const unknown = [...byLibrary.entries()]
    .filter(([id]) => !knownIds.has(id))
    .sort(([a], [b]) => a - b)
    .map(([id, list]) => ({ id, name: t('admin.maintenance.library', { id }), issues: list }));
  return [...known, ...unknown];
}

/** A path or a finding in full, wrapped where it has to (S-5: reachable without a pointer). */
function LongText({ text, code = false }: { text: string | undefined; code?: boolean }) {
  const { t } = useTranslation();
  if (!text) return <>{t('admin.maintenance.none')}</>;
  return code ? (
    <code className={styles.path}>{text}</code>
  ) : (
    <span className={styles.wrap}>{text}</span>
  );
}

/**
 * One tab of the maintenance page (PFL-2): the hint, the outcome of the last action, the toolbar
 * with the actions and the selection, one section per library with a table of the loaded issues,
 * «X von Y» and «Mehr laden» (50 per page).
 */
export function IssueTab({
  kind,
  libraries,
  selected,
  outcome,
  expanded,
  hold,
  refreshing,
  cleaning,
  handlers,
}: IssueTabProps) {
  const { t, i18n } = useTranslation();
  const number = useMemo(() => new Intl.NumberFormat(i18n.language), [i18n.language]);
  const issues = useInfiniteQuery({ ...issuesQuery(kind), refetchOnWindowFocus: !hold });
  const list = useMemo(() => issues.data?.pages.flatMap((page) => page.items) ?? [], [issues.data]);
  const total = issues.data?.pages.at(-1)?.total ?? 0;
  const sections = useMemo(() => sectionsOf(t, list, libraries), [t, list, libraries]);

  const chosen = list.filter((issue) => selected.includes(issueKey(issue)));
  const itemIds = [...new Set(chosen.map((issue) => issue.itemId))];
  const mediaFileIds = chosen.flatMap((issue) => (issue.mediaFileId ? [issue.mediaFileId] : []));
  const selectable = kind !== 'rating_mismatch';
  const none = chosen.length === 0;
  const matchTarget = chosen.length === 1 ? chosen[0] : undefined;
  const canMatch =
    matchTarget !== undefined &&
    (matchTarget.itemKind === 'movie' || matchTarget.itemKind === 'series');
  const allKeys = list.map(issueKey);
  const all = list.length > 0 && chosen.length === list.length;

  const toggle = (key: string, on: boolean) => {
    const keys = chosen.map(issueKey).filter((other) => other !== key);
    handlers.onSelect(kind, on ? [...keys, key] : keys);
  };

  const refreshButton = (label: string) => (
    <AriaButton
      icon="refresh"
      isDisabled={none}
      isPending={refreshing}
      onPress={() => handlers.onRefresh(kind, itemIds)}
    >
      {label}
    </AriaButton>
  );
  const actions: Partial<Record<IssueKind, ReactNode>> = {
    unmatched: (
      <>
        <AriaButton
          variant="primary"
          isDisabled={!canMatch}
          onPress={() => matchTarget && handlers.onMatch(matchTarget)}
        >
          {t('admin.maintenance.action.match')}
        </AriaButton>
        {refreshButton(t('admin.maintenance.action.refresh'))}
      </>
    ),
    no_images: refreshButton(t('admin.maintenance.action.refresh')),
    unrated: (
      <AriaButton variant="primary" isDisabled={none} onPress={() => handlers.onRating(itemIds)}>
        {t('admin.maintenance.action.rating')}
      </AriaButton>
    ),
    orphaned: (
      <AriaButton
        variant="danger"
        isDisabled={none || cleaning}
        onPress={() => handlers.onCleanup(kind, itemIds)}
      >
        {t('admin.maintenance.action.cleanup')}
      </AriaButton>
    ),
    unplayable: (
      <>
        {refreshButton(t('admin.maintenance.action.recheck'))}
        <AriaButton
          variant="danger"
          isDisabled={none || cleaning}
          onPress={() => handlers.onRemove(kind, mediaFileIds)}
        >
          {t('admin.maintenance.action.remove')}
        </AriaButton>
      </>
    ),
  };

  const columns: DataColumn<Issue>[] = [
    ...(selectable
      ? [
          {
            key: 'select',
            header: t('admin.maintenance.selection'),
            hideHeader: true,
            cell: (issue: Issue) => (
              <SelectCheck
                label={t('admin.maintenance.selectItem', { title: issue.title })}
                isSelected={selected.includes(issueKey(issue))}
                onChange={(on) => toggle(issueKey(issue), on)}
              />
            ),
          },
        ]
      : []),
    {
      key: 'title',
      header: t('admin.maintenance.columns.title'),
      rowHeader: true,
      cell: (issue) => (
        <TextLink href={paths.adminItem(issue.itemId)}>
          {titleWithYear(t, issue.title, issue.year)}
        </TextLink>
      ),
    },
    {
      key: 'kind',
      header: t('admin.maintenance.columns.kind'),
      cell: (issue) => t(`kind.${issue.itemKind}`),
    },
    {
      key: 'path',
      header: t('admin.maintenance.columns.path'),
      cell: (issue) => <LongText text={issue.path} code />,
    },
    {
      key: 'detail',
      header: t('admin.maintenance.columns.detail'),
      cell: (issue) => <LongText text={issue.detail} />,
    },
  ];
  const kindName = t(`admin.maintenance.tab.${kind}`);

  return (
    <div className={styles.tab}>
      <VisuallyHidden as="h2">{kindName}</VisuallyHidden>
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
      {issues.isSuccess && list.length === 0 && (
        <Text tone="secondary">{t('admin.maintenance.empty')}</Text>
      )}
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
                  onPress={() => handlers.onSelect(kind, all ? [] : allKeys)}
                >
                  {all ? t('admin.maintenance.selectNone') : t('admin.maintenance.selectAll')}
                </AriaButton>
              </div>
            </div>
          )}
          {sections.map((section) => {
            const key = libraryKey(kind, section.id);
            const picked = section.issues.filter((issue) =>
              selected.includes(issueKey(issue)),
            ).length;
            return (
              <DisclosureGroup
                key={section.id}
                title={section.name}
                count={number.format(section.issues.length)}
                countLabel={t('admin.maintenance.entries', { count: section.issues.length })}
                meta={
                  picked > 0 ? (
                    <span className={styles.muted}>
                      {t('admin.maintenance.selected', { count: picked })}
                    </span>
                  ) : undefined
                }
                isExpanded={expanded[key] ?? true}
                onExpandedChange={(open) => handlers.onExpandedChange(key, open)}
              >
                <DataTable
                  caption={t('admin.maintenance.caption', {
                    kind: kindName,
                    library: section.name,
                  })}
                  columns={columns}
                  rows={section.issues}
                  rowKey={issueKey}
                />
              </DisclosureGroup>
            );
          })}
          <div className={styles.actions}>
            <span className={styles.muted}>
              {t('admin.maintenance.shown', {
                shown: number.format(list.length),
                total: number.format(total),
              })}
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
    </div>
  );
}
