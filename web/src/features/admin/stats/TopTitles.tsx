import type { StatTitle } from '@slouchcat/api-client';
import { useState } from 'react';
import { useTranslation } from 'react-i18next';
import { paths } from '@/app/paths';
import { formatDuration } from '@/lib/format';
import { PosterFallback, Text, TextLink } from '@/ui';
import { DataTable, ImageTile, type DataColumn } from '@/ui/aria';
import styles from './Stats.module.css';

/** Places 1 to 3 stand as tiles, the rest in the table (mockup Sta-B). */
const PODIUM = 3;

interface Ranked {
  rank: number;
  title: StatTitle;
}

/** A small poster before the title in the table, the fallback when there is none or it fails. */
function Thumb({ src }: { src?: string }) {
  const [failed, setFailed] = useState<string | null>(null);
  return (
    <span className={styles.thumb}>
      {src !== undefined && failed !== src ? (
        <img src={src} alt="" loading="lazy" decoding="async" onError={() => setFailed(src)} />
      ) : (
        <PosterFallback />
      )}
    </span>
  );
}

/**
 * «Meistgesehen» (STA-2): the three most watched titles as tiles with the backdrop (for episodes
 * the series', contract 1.8.0), the rest from place 4 as a table with the poster. Every title
 * links to its administration page; the images are decoration next to the written title.
 */
export function TopTitles({ titles }: { titles: StatTitle[] }) {
  const { t, i18n } = useTranslation();
  const number = new Intl.NumberFormat(i18n.language);
  if (titles.length === 0) return <Text tone="secondary">{t('admin.stats.top.empty')}</Text>;

  const name = (title: StatTitle) =>
    title.year ? t('admin.titleYear', { title: title.title, year: title.year }) : title.title;
  const ranked = titles.map((title, index) => ({ rank: index + 1, title }));
  const rest = ranked.slice(PODIUM);
  const columns: DataColumn<Ranked>[] = [
    {
      key: 'rank',
      header: t('admin.stats.top.columns.rank'),
      align: 'end',
      cell: ({ rank }) => (
        <span className={styles.numbers}>{t('admin.stats.top.rank', { rank })}</span>
      ),
    },
    {
      key: 'title',
      header: t('admin.stats.top.columns.title'),
      rowHeader: true,
      cell: ({ title }) => (
        <span className={styles.titleCell}>
          <Thumb src={title.posterUrl} />
          <TextLink href={paths.adminItem(title.itemId)}>{name(title)}</TextLink>
        </span>
      ),
    },
    {
      key: 'kind',
      header: t('admin.stats.top.columns.kind'),
      cell: ({ title }) => t(`kind.${title.kind}`),
    },
    {
      key: 'plays',
      header: t('admin.stats.columns.plays'),
      align: 'end',
      cell: ({ title }) => <span className={styles.numbers}>{number.format(title.plays)}</span>,
    },
    {
      key: 'watched',
      header: t('admin.stats.columns.watched'),
      align: 'end',
      cell: ({ title }) => (
        <span className={styles.numbers}>{formatDuration(t, title.watchedMs)}</span>
      ),
    },
  ];

  return (
    <div className={styles.top}>
      <ol className={styles.podium} aria-label={t('admin.stats.top.podium')}>
        {ranked.slice(0, PODIUM).map(({ rank, title }) => (
          <li key={title.itemId} className={styles.tile}>
            <ImageTile src={title.backdropUrl} title={title.title} />
            <span className={styles.rank} aria-hidden="true">
              {number.format(rank)}
            </span>
            <span className={styles.tileText}>
              <span className={styles.tileTitle}>
                <TextLink href={paths.adminItem(title.itemId)}>{name(title)}</TextLink>
              </span>
              <span className={styles.tileMeta}>
                {t('admin.stats.top.meta', {
                  kind: t(`kind.${title.kind}`),
                  plays: t('admin.stats.top.plays', { count: title.plays }),
                  watched: formatDuration(t, title.watchedMs),
                })}
              </span>
            </span>
          </li>
        ))}
      </ol>
      {rest.length > 0 && (
        <DataTable
          caption={t('admin.stats.top.rest')}
          columns={columns}
          rows={rest}
          rowKey={({ title }) => title.itemId}
        />
      )}
    </div>
  );
}
