import type { Ref } from 'react';
import { Button } from 'react-aria-components';
import styles from './PlayerCard.module.css';
import { VisuallyHidden } from './VisuallyHidden';

interface PlayerCardProps {
  title: string;
  /** Above the title: the episode code («S01E02»), translated by the page. */
  eyebrow?: string;
  /** Under the title: the runtime, a character («als Ellen Ripley»). */
  caption?: string;
  /** A 16:9 still (`still`); a person shows the initial of the name instead. */
  imageUrl?: string;
  /** `still`: an episode with its image; `person`: a member of the cast with its initial. */
  shape?: 'still' | 'person';
  /** How much has played, 0 … 100, with its spoken form («40 % gesehen»). */
  progress?: { percent: number; label: string };
  /** The episode that plays now: outlined in the accent. */
  isCurrent?: boolean;
  onPress: () => void;
  ref?: Ref<HTMLButtonElement>;
  /** For a row with a roving tab index: takes the card out of the Tab order. */
  excludeFromTabOrder?: boolean;
}

/**
 * A card in a row of the player's tabs (concept 13, V-11): an episode of «Als Nächstes» with its
 * still, code, title and progress, or a person of «Besetzung» with the initial, name and role.
 * The browsing pages use EpisodeCard and CastCard, which are links in the spatial navigation; in
 * the player a card is a button in the row's roving tab index (PlayerChrome).
 */
export function PlayerCard({
  title,
  eyebrow,
  caption,
  imageUrl,
  shape = 'still',
  progress,
  isCurrent = false,
  onPress,
  ref,
  excludeFromTabOrder = false,
}: PlayerCardProps) {
  const played = progress && Math.min(100, Math.max(0, progress.percent));
  return (
    <Button
      ref={ref}
      className={styles.card}
      data-shape={shape}
      aria-current={isCurrent || undefined}
      onPress={onPress}
      excludeFromTabOrder={excludeFromTabOrder}
    >
      <span className={styles.frame} aria-hidden="true">
        {shape === 'person' ? (
          <span className={styles.initial}>{Array.from(title)[0]?.toUpperCase()}</span>
        ) : (
          imageUrl && (
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
          )
        )}
        {played !== undefined && (
          <span className={styles.progress}>
            <span className={styles.played} style={{ width: `${played}%` }} />
          </span>
        )}
      </span>
      {/* The spaces keep the parts of the name apart where an engine joins inline text. */}
      {eyebrow && (
        <>
          <span className={styles.eyebrow}>{eyebrow}</span>{' '}
        </>
      )}
      <span className={styles.title}>{title}</span>
      {caption && (
        <>
          {' '}
          <span className={styles.caption}>{caption}</span>
        </>
      )}
      {progress && (
        <>
          {' '}
          <VisuallyHidden>{progress.label}</VisuallyHidden>
        </>
      )}
    </Button>
  );
}
