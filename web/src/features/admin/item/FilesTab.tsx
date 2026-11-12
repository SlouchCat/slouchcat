import type { AdminItem } from '@slouchcat/api-client';
import { useTranslation } from 'react-i18next';
import { formatBytes, formatDateTime } from '@/lib/format';
import { Chip, Notice, Text, VisuallyHidden } from '@/ui';
import { DataTable, type DataColumn } from '@/ui/aria';
import { fullPath } from '../maintenance/shared';
import styles from './AdminItem.module.css';

type MediaFile = AdminItem['mediaFiles'][number];

/**
 * The file's name and its folder in its row, so two versions with the same name stay apart; the
 * whole path is in the fold-out.
 */
function splitPath(path: string): { name: string; folder: string } {
  const parts = path.split('/');
  const name = parts.pop() || path;
  return { name, folder: parts.pop() ?? '' };
}

/**
 * The tab «Dateien» (TIT-6): per file a row with name, version, size and state; it folds out
 * into the whole path in the container (library root and path, concept 20 P-10), when it was probed and the whole probe error, so long texts stay
 * reachable by keyboard and touch (S-5).
 */
export function FilesTab({
  item,
  isExpanded,
  onExpandedChange,
}: {
  item: AdminItem;
  isExpanded: (key: string) => boolean;
  onExpandedChange: (key: string, expanded: boolean) => void;
}) {
  const { t, i18n } = useTranslation();
  const none = t('admin.item.facts.none');
  const keyOf = (file: MediaFile) => `file:${file.id}`;
  const columns: DataColumn<MediaFile>[] = [
    {
      key: 'name',
      header: t('admin.item.files.columns.name'),
      rowHeader: true,
      cell: (file) => {
        const { name, folder } = splitPath(file.path);
        return (
          <span className={styles.fileName}>
            {name}
            {folder && ' '}
            {folder && <span className={styles.folder}>{folder}</span>}
          </span>
        );
      },
    },
    {
      key: 'version',
      header: t('admin.item.files.columns.version'),
      cell: (file) =>
        file.versionLabel ? (
          <Chip tone="outline">{file.versionLabel}</Chip>
        ) : (
          <span className={styles.muted}>{none}</span>
        ),
    },
    {
      key: 'size',
      header: t('admin.item.files.columns.size'),
      align: 'end',
      cell: (file) => (
        <span className={styles.numbers}>{formatBytes(file.sizeBytes, i18n.language)}</span>
      ),
    },
    {
      key: 'state',
      header: t('admin.item.files.columns.state'),
      cell: (file) => (
        <span className={styles.chips}>
          {!file.available && (
            <Chip tone="warning" icon="warning">
              {t('item.file.unavailable')}
            </Chip>
          )}
          {file.probeError && <Chip tone="warning">{t('admin.item.files.probeError')}</Chip>}
        </span>
      ),
    },
  ];
  const expandedKeys = new Set(item.mediaFiles.map(keyOf).filter(isExpanded));
  return (
    <section className={styles.tabBody} aria-labelledby="admin-item-files">
      <VisuallyHidden as="h2" id="admin-item-files">
        {t('admin.item.files.title')}
      </VisuallyHidden>
      {item.mediaFiles.length === 0 ? (
        <Text tone="secondary">{t('admin.item.files.none')}</Text>
      ) : (
        <DataTable
          caption={t('admin.item.files.title')}
          columns={columns}
          rows={item.mediaFiles}
          rowKey={keyOf}
          expand={{
            expandedKeys,
            onExpandedChange,
            render: (file) => (
              <div className={styles.stack}>
                <code className={styles.path}>{fullPath(file.rootPath, file.path)}</code>
                {file.probedAt && (
                  <span className={styles.muted}>
                    {t('admin.item.files.probedAt', {
                      date: formatDateTime(file.probedAt, i18n.language),
                    })}
                  </span>
                )}
                {file.probeError && (
                  <Notice tone="warning" live={false} title={t('admin.item.files.probeError')}>
                    {file.probeError}
                  </Notice>
                )}
              </div>
            ),
          }}
        />
      )}
    </section>
  );
}
