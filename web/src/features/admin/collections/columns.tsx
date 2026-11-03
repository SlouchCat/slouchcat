import type { AdminCollection } from '@slouchcat/api-client';
import type { TFunction } from 'i18next';
import { Chip, Icon } from '@/ui';
import type { DataColumn } from '@/ui/aria';
import styles from './Collections.module.css';

/**
 * The columns of the collections table (SAM-2, mockup Sam-C): the name with the German name of a
 * TMDB collection folds the row out; source, number of titles and the state: «nicht änderbar»
 * for a TMDB collection, «geändert» while its members have unsaved changes.
 */
export function collectionColumns(
  t: TFunction,
  number: Intl.NumberFormat,
  isDirty: (id: string) => boolean,
): DataColumn<AdminCollection>[] {
  return [
    {
      key: 'name',
      header: t('admin.collections.columns.name'),
      rowHeader: true,
      cell: (collection) => (
        <span className={styles.stack}>
          <span>{collection.name}</span>
          {collection.nameDe && collection.nameDe !== collection.name && (
            <span className={styles.muted}>{collection.nameDe}</span>
          )}
        </span>
      ),
    },
    {
      key: 'source',
      header: t('admin.collections.columns.source'),
      cell: (collection) => (
        <Chip tone={collection.source === 'tmdb' ? 'info' : 'neutral'}>
          {collection.source === 'tmdb'
            ? t('admin.collections.source.tmdb')
            : t('admin.collections.source.manual')}
        </Chip>
      ),
    },
    {
      key: 'count',
      header: t('admin.collections.columns.count'),
      align: 'end',
      cell: (collection) => (
        <span className={styles.numbers}>{number.format(collection.itemCount)}</span>
      ),
    },
    {
      key: 'state',
      header: t('admin.collections.columns.state'),
      hideHeader: true,
      align: 'end',
      cell: (collection) => {
        // TMDB collections come from the provider: nothing to change here (`not_editable`).
        if (collection.source === 'tmdb') {
          return (
            <span className={styles.state}>
              <Icon name="lock" size="sm" />
              {t('admin.collections.locked')}
            </span>
          );
        }
        return isDirty(collection.id) ? (
          <Chip tone="warning">{t('admin.collections.changed')}</Chip>
        ) : null;
      },
    },
  ];
}
