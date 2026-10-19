import type { ItemRef } from '@slouchcat/api-client';
import { useTranslation } from 'react-i18next';
import { formatRuntime } from '@/lib/format';
import { Chip } from '@/ui';
import styles from './ItemMeta.module.css';

/** Year · runtime · FSK, as shown under titles in the hero and on the detail page. */
export function ItemMeta({ item }: { item: ItemRef }) {
  const { t } = useTranslation();
  const parts = [
    item.year !== undefined ? String(item.year) : null,
    item.runtimeMinutes ? formatRuntime(t, item.runtimeMinutes) : null,
  ].filter((part): part is string => Boolean(part));
  return (
    <span className={styles.meta}>
      {item.rating !== undefined && (
        <Chip tone="outline">{t('rating.fsk', { age: item.rating })}</Chip>
      )}
      {parts.map((part) => (
        <span key={part} className={styles.part}>
          {part}
        </span>
      ))}
    </span>
  );
}
