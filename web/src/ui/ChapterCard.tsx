import { Button } from 'react-aria-components';
import { useTranslation } from 'react-i18next';
import { formatPlayerTime } from '@/lib/format';
import styles from './ChapterCard.module.css';
import { Chip } from './Chip';

interface ChapterCardProps {
  /** Position in the chapter list, from 0: the number on the card and the name of an untitled one. */
  index: number;
  /** Without one the chapter is «Kapitel 3», never its start time (V-8, 11.6). */
  title?: string;
  startMs: number;
  /** Length of the title: from an hour on the time reads h:mm:ss. */
  durationMs: number;
  /** The chapter image (stage 3); until there is one the card shows the number. */
  imageUrl?: string;
  /** How much of the chapter has played, 0 … 100; left out for a chapter not reached yet. */
  progress?: number;
  /** A replacement mark every 10 minutes, not a chapter of the file (`Chapter.auto`, F-6). */
  auto?: boolean;
  /** The chapter that plays now: outlined in the accent. */
  isCurrent?: boolean;
  onPress: () => void;
}

/**
 * A chapter in the player's chapter row (V-8): a 16:9 frame with the chapter image or its number,
 * the progress along its lower edge, «automatisch» on replacement chapters, then the title and the
 * start time. Pressing it jumps there.
 */
export function ChapterCard({
  index,
  title,
  startMs,
  durationMs,
  imageUrl,
  progress,
  auto = false,
  isCurrent = false,
  onPress,
}: ChapterCardProps) {
  const { t } = useTranslation();
  const name = title ?? t('player.chapters.untitled', { n: index + 1 });
  const played = progress === undefined ? undefined : Math.min(100, Math.max(0, progress));
  return (
    <Button className={styles.card} aria-current={isCurrent || undefined} onPress={onPress}>
      <span className={styles.frame}>
        {imageUrl ? (
          <img
            className={styles.image}
            src={imageUrl}
            alt=""
            width={480}
            height={270}
            loading="lazy"
            decoding="async"
            draggable={false}
          />
        ) : (
          <span className={styles.number} aria-hidden="true">
            {index + 1}
          </span>
        )}
        {auto && (
          <span className={styles.badge}>
            <Chip tone="info">{t('player.chapters.auto')}</Chip>
          </span>
        )}
        {played !== undefined && (
          <span className={styles.progress} aria-hidden="true">
            <span className={styles.played} style={{ width: `${played}%` }} />
          </span>
        )}
      </span>
      <span className={styles.title}>{name}</span>
      <span className={styles.time}>{formatPlayerTime(startMs, durationMs)}</span>
    </Button>
  );
}
