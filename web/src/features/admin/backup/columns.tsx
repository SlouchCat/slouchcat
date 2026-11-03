import type { Backup } from '@slouchcat/api-client';
import type { TFunction } from 'i18next';
import type { ReactNode } from 'react';
import { formatBytes, formatDateTime } from '@/lib/format';
import { ButtonLink, Chip } from '@/ui';
import type { DataColumn } from '@/ui/aria';
import { downloadUrl } from './api';
import styles from './Backup.module.css';

/**
 * The columns of a kind's table (SIC-3, mockup Sic-C): the file name in full (wrapped, S-5), when
 * it was made, its size, whether it holds the keys and whether it is encrypted; then «Herunterladen»
 * and the row's «Weitere Aktionen» (`actionsOf`). The kind is the table's group; the widths line
 * the groups' tables up (K12-9). Every column has a width in rem, sized from the longest header
 * word and content in German and English with room for a wider system font: the table is at least
 * as wide as their sum (it scrolls in its frame below that) and shares more room out among them.
 */
export function backupColumns(
  t: TFunction,
  language: string,
  fresh: string | null,
  actionsOf: (backup: Backup) => ReactNode,
): DataColumn<Backup>[] {
  const yesNo = (value: boolean) => (value ? t('admin.backup.yes') : t('admin.backup.no'));
  return [
    {
      key: 'name',
      width: '12rem',
      header: t('admin.backup.columns.name'),
      rowHeader: true,
      cell: (backup) => (
        <span className={styles.name}>
          <span className={styles.file}>{backup.name}</span>
          {/* The space keeps name and mark apart in the spoken name. */}
          {backup.name === fresh && ' '}
          {backup.name === fresh && <Chip tone="success">{t('admin.backup.new')}</Chip>}
        </span>
      ),
    },
    {
      key: 'created',
      // The longest date of either language on one line («Sep 30, 2026, 12:59 PM»).
      width: '12rem',
      header: t('admin.backup.columns.created'),
      cell: (backup) => (
        <time className={styles.nowrap} dateTime={backup.createdAt}>
          {formatDateTime(backup.createdAt, language)}
        </time>
      ),
    },
    {
      key: 'size',
      width: '5.5rem',
      header: t('admin.backup.columns.size'),
      align: 'end',
      cell: (backup) => (
        <span className={styles.numbers}>{formatBytes(backup.sizeBytes, language)}</span>
      ),
    },
    {
      key: 'secrets',
      width: '7.5rem',
      header: t('admin.backup.columns.secrets'),
      cell: (backup) => yesNo(backup.includesSecrets),
    },
    {
      key: 'encrypted',
      width: '9.5rem',
      header: t('admin.backup.columns.encrypted'),
      cell: (backup) => yesNo(backup.encrypted),
    },
    {
      key: 'actions',
      width: '16rem',
      header: t('admin.backup.columns.actions'),
      hideHeader: true,
      align: 'end',
      cell: (backup) => (
        <span className={styles.rowActions}>
          <ButtonLink
            variant="ghost"
            icon="download"
            href={downloadUrl(backup)}
            download={backup.name}
            aria-label={t('admin.backup.download.label', { name: backup.name })}
          >
            {t('admin.backup.download.action')}
          </ButtonLink>
          {actionsOf(backup)}
        </span>
      ),
    },
  ];
}
