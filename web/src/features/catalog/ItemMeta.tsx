import type { ItemRef } from '@slouchcat/api-client';
import { useTranslation } from 'react-i18next';
import { formatDate, formatDuration, formatRuntime } from '@/lib/format';
import { Chip } from '@/ui';
import styles from './ItemMeta.module.css';

/**
 * Year · runtime · FSK, as shown under titles in the hero and on the detail page. A podcast
 * episode has its publication date instead of the year, audio titles their duration (M6).
 */
export function ItemMeta({ item }: { item: ItemRef }) {
  const { t, i18n } = useTranslation();
  const parts = [
    item.publishedAt
      ? formatDate(item.publishedAt, i18n.language)
      : item.year !== undefined
        ? String(item.year)
        : null,
    item.runtimeMinutes
      ? formatRuntime(t, item.runtimeMinutes)
      : item.durationMs
        ? formatDuration(t, item.durationMs)
        : null,
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
