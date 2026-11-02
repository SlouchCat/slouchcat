import type { HomeRowKey } from '@slouchcat/api-client';
import { useTranslation } from 'react-i18next';
import { Text } from '@/ui';
import { rowTitleKey, type HomeRow } from './rows';
import styles from './HomeRows.module.css';

/** How a row looks on the start page, drawn as placeholders: the stage, posters, wide tiles, pills. */
type Shape = 'stage' | 'poster' | 'wide' | 'genre';

const SHAPE = {
  spotlight: 'stage',
  picks: 'poster',
  continue: 'wide',
  recent: 'poster',
  released: 'poster',
  watchlist: 'poster',
  collections: 'wide',
  genres: 'genre',
} as const satisfies Record<HomeRowKey, Shape>;

/** Number of placeholders per shape. */
const COUNT: Record<Shape, number> = { stage: 4, poster: 7, wide: 4, genre: 5 };

function Placeholders({ shape }: { shape: Shape }) {
  const marks = Array.from({ length: COUNT[shape] }, (_, index) => index);
  return (
    <span className={styles.sketch} aria-hidden="true">
      {shape === 'stage' && <span className={styles.stage} />}
      <span className={styles.strip}>
        {marks.map((index) => (
          <span
            key={index}
            className={
              shape === 'poster' ? styles.poster : shape === 'genre' ? styles.pill : styles.tile
            }
          />
        ))}
      </span>
    </span>
  );
}

/**
 * The tab «Vorschau» (concept 12, 11.3): a sketch of the start page in the order on the page,
 * saved or not, without any data. Hidden rows stay in their place, dimmed and marked, so their
 * position can be read; the note says that the real start page leaves them out.
 */
export function HomeRowsPreview({ rows }: { rows: readonly HomeRow[] }) {
  const { t } = useTranslation();
  return (
    <div className={styles.preview}>
      <ol className={styles.frame} aria-label={t('admin.homeRows.preview.label')}>
        {rows.map((row, index) => (
          <li key={row.key} className={`${styles.previewRow} ${row.hidden ? styles.off : ''}`}>
            <span className={styles.previewName}>
              {t('admin.homeRows.preview.row', {
                position: index + 1,
                row: t(rowTitleKey[row.key]),
              })}
              {/* The space keeps name and state apart in the spoken text. */}
              {row.hidden && ' '}
              {row.hidden && (
                <span className={styles.previewState}>{t('admin.homeRows.state.hidden')}</span>
              )}
            </span>
            <Placeholders shape={SHAPE[row.key]} />
          </li>
        ))}
      </ol>
      <Text tone="secondary" variant="caption">
        {t('admin.homeRows.preview.note')}
      </Text>
    </div>
  );
}
