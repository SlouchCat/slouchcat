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
import { pairKey, userDataSummary } from './shared';
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
 * data still on them. Every pair starts selected; «Umhängen» asks first and merges the copies
 * into the deleted titles, which keep their ids.
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
          label={t('admin.maintenance.selectItem', { title: pair.title })}
          isSelected={!out.has(pairKey(pair))}
          onChange={(on) => toggle(pairKey(pair), on)}
        />
      ),
    },
    {
      key: 'title',
      width: '26%',
      header: t('admin.maintenance.columns.title'),
      rowHeader: true,
      // The live copy has an administration page; the deleted title has none until it is back.
      cell: (pair) => <TextLink href={paths.adminItem(pair.liveId)}>{pair.title}</TextLink>,
    },
    {
      key: 'kind',
      width: '10%',
      header: t('admin.maintenance.columns.kind'),
      cell: (pair) => t(`kind.${pair.kind}`),
    },
    {
      key: 'library',
      width: '14%',
      header: t('admin.maintenance.relink.columns.library'),
      cell: (pair) => pair.libraryName || t('admin.maintenance.library', { id: pair.libraryId }),
    },
    {
      key: 'deletedAt',
      width: '14%',
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
                isDisabled={chosen.length === 0 || relinking}
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
