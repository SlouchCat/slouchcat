import type { Ref } from 'react';
import { Button } from 'react-aria-components';
import { useTranslation } from 'react-i18next';
import { formatPlayerTime } from '@/lib/format';
import styles from './ChapterCard.module.css';
import { Chip } from './Chip';
import { VisuallyHidden } from './VisuallyHidden';

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
  ref?: Ref<HTMLButtonElement>;
  /** For a row with a roving tab index: a negative one takes the card out of the Tab order. */
  tabIndex?: number;
  excludeFromTabOrder?: boolean;
}

/**
 * A chapter in the player's chapter row (V-8): a 16:9 frame with the chapter image or its number,
 * the progress along its lower edge, «automatisch» on replacement chapters, then the title and the
 * start time. Pressing it jumps there. Its name reads title, «automatisch», time and progress: the
 * mark follows the title in the DOM and only sits on the frame by its position.
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
  ref,
  tabIndex,
  excludeFromTabOrder = false,
}: ChapterCardProps) {
  const { t } = useTranslation();
  const name = title ?? t('player.chapters.untitled', { n: index + 1 });
  const played = progress === undefined ? undefined : Math.min(100, Math.max(0, progress));
  return (
    <Button
      ref={ref}
      className={styles.card}
      aria-current={isCurrent || undefined}
      onPress={onPress}
      // react-aria's Button has no tabIndex; a negative one takes it out of the Tab order.
      excludeFromTabOrder={excludeFromTabOrder || (tabIndex !== undefined && tabIndex < 0)}
    >
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
        {played !== undefined && (
          <span className={styles.progress} aria-hidden="true">
            <span className={styles.played} style={{ width: `${played}%` }} />
          </span>
        )}
      </span>
      {/* The spaces between the parts keep the name apart where an engine joins inline text
          without one ("Die Farmautomatisch"); a flex column drops them from the layout. */}
      <span className={styles.title}>{name}</span>{' '}
      {auto && (
        <>
          <span className={styles.badge}>
            <Chip tone="info">{t('player.chapters.auto')}</Chip>
          </span>{' '}
        </>
      )}
      <span className={styles.time}>{formatPlayerTime(startMs, durationMs)}</span>
      {played !== undefined && (
        <>
          {' '}
          <VisuallyHidden>
            {t('player.chapters.progress', { percent: Math.round(played) })}
          </VisuallyHidden>
        </>
      )}
    </Button>
  );
}
