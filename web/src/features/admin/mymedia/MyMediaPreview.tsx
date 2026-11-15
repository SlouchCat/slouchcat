import { useMemo } from 'react';
import { useTranslation } from 'react-i18next';
import { Text } from '@/ui';
import { contentOf, entryName, type MyMediaRow } from './entries';
import styles from './MyMedia.module.css';
import { intlLocale } from '@/i18n';

/**
 * The tab «Vorschau» (concept 21 P-11, O-21): both places with the draft, saved or not, each
 * without the entries hidden there: the side navigation's group with names and counts, and the
 * tiles of the start page row «Meine Medien», which fill the width.
 */
export function MyMediaPreview({ rows }: { rows: readonly MyMediaRow[] }) {
  const { t, i18n } = useTranslation();
  const number = useMemo(() => new Intl.NumberFormat(intlLocale(i18n.language)), [i18n.language]);
  const sidebar = rows.filter((row) => !row.hideSidebar);
  const home = rows.filter((row) => !row.hideHome);
  return (
    <div className={styles.preview}>
      <div className={styles.places}>
        <section className={styles.place} aria-label={t('admin.myMedia.preview.sidebarLabel')}>
          <h3 className={styles.placeTitle}>{t('admin.myMedia.preview.sidebar')}</h3>
          {sidebar.length > 0 ? (
            <ul className={styles.navList}>
              {sidebar.map((row) => (
                <li key={row.id} className={styles.navEntry}>
                  <span>{entryName(t, row)}</span>
                  <span className={styles.navCount}>{contentOf(t, number, row)}</span>
                </li>
              ))}
            </ul>
          ) : (
            <p className={styles.empty}>{t('admin.myMedia.preview.empty')}</p>
          )}
        </section>
        <section className={styles.place} aria-label={t('admin.myMedia.preview.homeLabel')}>
          <h3 className={styles.placeTitle}>{t('admin.myMedia.preview.home')}</h3>
          {home.length > 0 ? (
            <ul className={styles.tiles}>
              {home.map((row) => (
                <li key={row.id} className={styles.tile}>
                  <span>{entryName(t, row)}</span>
                  <span className={styles.tileCount}>{contentOf(t, number, row)}</span>
                </li>
              ))}
            </ul>
          ) : (
            <p className={styles.empty}>{t('admin.myMedia.preview.empty')}</p>
          )}
        </section>
      </div>
      <Text tone="secondary" variant="caption">
        {t('admin.myMedia.preview.note')}
      </Text>
    </div>
  );
}
