import type { RemovedItem } from '@slouchcat/api-client';
import { useQuery } from '@tanstack/react-query';
import { useMemo } from 'react';
import { useTranslation } from 'react-i18next';
import { problemMessage } from '@/api/errors';
import { formatDate } from '@/lib/format';
import { Notice, Skeleton, Text, VisuallyHidden } from '@/ui';
import {
  AriaButton,
  DataTable,
  StatusRegion,
  type DataColumn,
  type StatusMessage,
} from '@/ui/aria';
import { removedQuery } from '../queries';
import { fullPath } from './shared';
import styles from './Maintenance.module.css';

interface RemovedTabProps {
  outcome: StatusMessage | undefined;
  hold: boolean;
  /** The title «Wieder aufnehmen» is running for. */
  restoring: string | undefined;
  onRestore: (item: RemovedItem) => void;
}

/**
 * «Entfernte Titel» (concept 20 P-9, section 10.2): the titles taken out of Slouchcat with their
 * kind, library, folder and date, newest first; «Wieder aufnehmen» brings the same title back with
 * all its user data. A season or episode of a series that is out as well comes back with the
 * series and is not listed (the server decides).
 */
export function RemovedTab({ outcome, hold, restoring, onRestore }: RemovedTabProps) {
  const { t, i18n } = useTranslation();
  const number = useMemo(() => new Intl.NumberFormat(i18n.language), [i18n.language]);
  const removed = useQuery({ ...removedQuery, refetchOnWindowFocus: !hold });
  const items = removed.data?.items ?? [];
  const total = removed.data?.total ?? 0;
  const name = (item: RemovedItem) =>
    item.parentTitle ? `${item.parentTitle} › ${item.title}` : item.title;

  const columns: DataColumn<RemovedItem>[] = [
    {
      key: 'title',
      width: '24%',
      header: t('admin.maintenance.removed.columns.title'),
      rowHeader: true,
      cell: (item) => <span className={styles.wrap}>{name(item)}</span>,
    },
    {
      key: 'kind',
      width: '9%',
      header: t('admin.maintenance.columns.kind'),
      cell: (item) => t(`kind.${item.kind}`),
    },
    {
      key: 'library',
      width: '12%',
      header: t('admin.maintenance.removed.columns.library'),
      cell: (item) => item.libraryName,
    },
    {
      key: 'folder',
      header: t('admin.maintenance.removed.columns.folder'),
      cell: (item) =>
        item.folder ? (
          <span className={styles.path}>
            {fullPath(item.folder.rootPath, item.folder.path, true)}
          </span>
        ) : (
          <span className={styles.muted}>{t('admin.maintenance.removed.noFolder')}</span>
        ),
    },
    {
      key: 'removedAt',
      width: '11%',
      header: t('admin.maintenance.removed.columns.removedAt'),
      cell: (item) => formatDate(item.removedAt, i18n.language),
    },
    {
      key: 'action',
      width: '13rem',
      header: t('admin.maintenance.removed.columns.action'),
      hideHeader: true,
      cell: (item) => (
        <AriaButton
          icon="refresh"
          aria-label={t('admin.maintenance.removed.restoreLabel', { title: name(item) })}
          isPending={restoring === item.id}
          isDisabled={restoring !== undefined && restoring !== item.id}
          onPress={() => onRestore(item)}
        >
          {t('admin.maintenance.removed.restore')}
        </AriaButton>
      ),
    },
  ];

  return (
    <div className={styles.tab}>
      <VisuallyHidden as="h2">{t('admin.maintenance.tab.removed')}</VisuallyHidden>
      <Text tone="secondary">{t('admin.maintenance.hint.removed')}</Text>
      <StatusRegion message={outcome} />
      {removed.isPending && <Skeleton shape="block" />}
      {removed.isError && (
        <Notice
          tone="danger"
          title={problemMessage(t, removed.error)}
          actions={
            <AriaButton icon="refresh" onPress={() => void removed.refetch()}>
              {t('common.retry')}
            </AriaButton>
          }
        />
      )}
      {removed.isSuccess && items.length === 0 && (
        <Text tone="secondary">{t('admin.maintenance.removed.empty')}</Text>
      )}
      {items.length > 0 && (
        <>
          <DataTable
            caption={t('admin.maintenance.tab.removed')}
            columns={columns}
            rows={items}
            rowKey={(item) => item.id}
            layout="fixed"
          />
          {total > items.length && (
            <span className={styles.muted}>
              {t('admin.maintenance.shown', {
                shown: number.format(items.length),
                total: number.format(total),
              })}
            </span>
          )}
        </>
      )}
    </div>
  );
}
