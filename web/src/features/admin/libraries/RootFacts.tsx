import type { LibraryRoot } from '@slouchcat/api-client';
import { useTranslation } from 'react-i18next';
import { Chip } from '@/ui';
import { watchKeys } from './libraryColumns';
import styles from './Libraries.module.css';

/**
 * The technical facts of a stored folder (BIB-2, concept 24 4.2 and 4.4): titles and files, file
 * system, the warning «Netzlaufwerk», «nur lesbar» or «schreibbar» (ADR 0032: whether Slouchcat
 * could delete files there, checked without writing) and how changes are noticed. The same line
 * stands in the fold-out of the page and in the folder card of the form.
 */
export function RootFacts({ root }: { root: LibraryRoot }) {
  const { t } = useTranslation();
  return (
    <span className={styles.rootMeta}>
      <span className={styles.muted}>
        {t('admin.libraries.folders.content', { items: root.itemCount, files: root.fileCount })}
      </span>
      <Chip tone="info">{t('admin.libraries.fsType', { type: root.fsType })}</Chip>
      {root.warning === 'network_share' && (
        <Chip tone="warning" icon="warning">
          {t('admin.libraries.warning.network_share')}
        </Chip>
      )}
      {root.readOnly !== undefined && (
        <Chip tone={root.readOnly ? 'outline' : 'warning'}>
          {root.readOnly
            ? t('admin.libraries.folders.readOnly')
            : t('admin.libraries.folders.writable')}
        </Chip>
      )}
      <span className={styles.muted}>{t(watchKeys[root.watch])}</span>
    </span>
  );
}
