import type { RelinkPair } from '@slouchcat/api-client';
import { useQuery } from '@tanstack/react-query';
import { useMemo } from 'react';
import { useTranslation } from 'react-i18next';
import { problemMessage } from '@/api/errors';
import { paths } from '@/app/paths';
import { formatDate } from '@/lib/format';
import { Notice, Skeleton, Text, TextLink, VisuallyHidden } from '@/ui';
import {
  AriaButton,
  DataTable,
  SelectCheck,
  StatusRegion,
  type DataColumn,
  type StatusMessage,
} from '@/ui/aria';
import { relinkQuery } from '../queries';
import { pairKey, titleWithYear, userDataSummary } from './shared';
import styles from './Maintenance.module.css';

interface RelinkTabProps {
  outcome: StatusMessage | undefined;
  /** The pairs the administrator took out; every other listed pair is selected. */
  excluded: readonly string[];
  hold: boolean;
  relinking: boolean;
  onExcludedChange: (keys: string[]) => void;
  onRelink: (pairs: RelinkPair[]) => void;
}

/**
 * Orphaned user data (ADR 0023, idea 04 D1): soft-deleted titles with a live copy and the user
 * data still on them, both titles side by side. Every pair starts selected; «Umhängen» asks first
 * and merges the copies into the deleted titles, which keep their ids. A deleted title with
 * several copies says so in its cell, and the checkboxes of its rows count the copies.
 */
export function RelinkTab({
  outcome,
  excluded,
  hold,
  relinking,
  onExcludedChange,
  onRelink,
}: RelinkTabProps) {
  const { t, i18n } = useTranslation();
  const number = useMemo(() => new Intl.NumberFormat(i18n.language), [i18n.language]);
  const relink = useQuery({ ...relinkQuery, refetchOnWindowFocus: !hold });
  const pairs = useMemo(() => relink.data?.pairs ?? [], [relink.data]);
  const total = relink.data?.total ?? 0;

  // The rows of each deleted title, so its copies are marked and their checkboxes told apart.
  const copies = useMemo(() => {
    const rows = new Map<string, string[]>();
    for (const pair of pairs)
      rows.set(pair.deletedId, [...(rows.get(pair.deletedId) ?? []), pair.liveId]);
    return rows;
  }, [pairs]);
  const copiesOf = (pair: RelinkPair) => copies.get(pair.deletedId) ?? [pair.liveId];
  const checkLabel = (pair: RelinkPair) => {
    const values = {
      copy: titleWithYear(t, pair.title, pair.year),
      library: pair.libraryName || t('admin.maintenance.library', { id: pair.libraryId }),
      date: formatDate(pair.deletedAt, i18n.language),
    };
    const rows = copiesOf(pair);
    return rows.length > 1
      ? t('admin.maintenance.relink.selectPairOf', {
          ...values,
          index: rows.indexOf(pair.liveId) + 1,
          copies: rows.length,
        })
      : t('admin.maintenance.relink.selectPair', values);
  };

  const out = useMemo(() => new Set(excluded), [excluded]);
  const chosen = pairs.filter((pair) => !out.has(pairKey(pair)));
  const all = chosen.length === pairs.length;
  const toggle = (key: string, on: boolean) =>
    onExcludedChange(on ? excluded.filter((other) => other !== key) : [...excluded, key]);

  const columns: DataColumn<RelinkPair>[] = [
    {
      key: 'select',
      header: t('admin.maintenance.selection'),
      hideHeader: true,
      width: '3.5rem',
      cell: (pair) => (
        <SelectCheck
          label={checkLabel(pair)}
          isSelected={!out.has(pairKey(pair))}
          onChange={(on) => toggle(pairKey(pair), on)}
        />
      ),
    },
    {
      key: 'deleted',
      width: '20%',
      header: t('admin.maintenance.relink.columns.deleted'),
      rowHeader: true,
      cell: (pair) => {
        const rows = copiesOf(pair).length;
        return (
          <span className={styles.wrap}>
            {titleWithYear(t, pair.deletedTitle, pair.deletedYear)}
            {rows > 1 && (
              <>
                {' '}
                <span className={styles.muted}>
                  {t('admin.maintenance.relink.copies', { count: rows })}
                </span>
              </>
            )}
          </span>
        );
      },
    },
    {
      key: 'copy',
      width: '20%',
      header: t('admin.maintenance.relink.columns.copy'),
      // The live copy has an administration page; the deleted title has none until it is back.
      cell: (pair) => (
        <TextLink href={paths.adminItem(pair.liveId)}>
          {titleWithYear(t, pair.title, pair.year)}
        </TextLink>
      ),
    },
    {
      key: 'kind',
      width: '8%',
      header: t('admin.maintenance.columns.kind'),
      cell: (pair) => t(`kind.${pair.kind}`),
    },
    {
      key: 'library',
      width: '12%',
      header: t('admin.maintenance.relink.columns.library'),
      cell: (pair) => pair.libraryName || t('admin.maintenance.library', { id: pair.libraryId }),
    },
    {
      key: 'deletedAt',
      width: '11%',
      header: t('admin.maintenance.relink.columns.deletedAt'),
      cell: (pair) => formatDate(pair.deletedAt, i18n.language),
    },
    {
      key: 'userData',
      header: t('admin.maintenance.relink.columns.userData'),
      cell: (pair) => (
        <span className={styles.wrap}>
          {userDataSummary(t, [pair]) || t('admin.maintenance.relink.noUserData')}
        </span>
      ),
    },
  ];

  return (
    <div className={styles.tab}>
      <VisuallyHidden as="h2">{t('admin.maintenance.tab.relink')}</VisuallyHidden>
      <Text tone="secondary">{t('admin.maintenance.hint.relink')}</Text>
      <StatusRegion message={outcome} />
      {relink.isPending && <Skeleton shape="block" />}
      {relink.isError && (
        <Notice
          tone="danger"
          title={problemMessage(t, relink.error)}
          actions={
            <AriaButton icon="refresh" onPress={() => void relink.refetch()}>
              {t('common.retry')}
            </AriaButton>
          }
        />
      )}
      {relink.isSuccess && pairs.length === 0 && (
        <Text tone="secondary">{t('admin.maintenance.relink.empty')}</Text>
      )}
      {pairs.length > 0 && (
        <>
          <div className={styles.toolbar}>
            <div className={styles.actions}>
              <AriaButton
                variant="primary"
                // While it runs it stays focusable (isPending), so the focus stays with it.
                isDisabled={chosen.length === 0}
                isPending={relinking}
                onPress={() => onRelink(chosen)}
              >
                {t('admin.maintenance.relink.action')}
              </AriaButton>
            </div>
            <div className={styles.actions}>
              <span className={styles.muted} aria-live="polite">
                {t('admin.maintenance.selected', { count: chosen.length })}
              </span>
              <AriaButton
                variant="ghost"
                onPress={() => onExcludedChange(all ? pairs.map(pairKey) : [])}
              >
                {all ? t('admin.maintenance.selectNone') : t('admin.maintenance.selectAll')}
              </AriaButton>
            </div>
          </div>
          <DataTable
            caption={t('admin.maintenance.tab.relink')}
            columns={columns}
            rows={pairs}
            rowKey={pairKey}
            layout="fixed"
          />
          {total > pairs.length && (
            <span className={styles.muted}>
              {t('admin.maintenance.shown', {
                shown: number.format(pairs.length),
                total: number.format(total),
              })}
            </span>
          )}
        </>
      )}
    </div>
  );
}
