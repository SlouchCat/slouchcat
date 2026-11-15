import { useId } from 'react';
import { Button } from './Button';
import type { IconName } from './Icon';
import styles from './PlayerStep.module.css';

/**
 * The card over an episode button (concept 23, 3.2): the episode's picture, «Nächste Folge», its
 * code and title, its length and the key. Shown while the pointer is on the button or it has the
 * keyboard focus; the button's name says the same to a screen reader.
 */
export interface PlayerStepCard {
  /** backdropSmall, else backdrop of the episode; none: an empty frame. */
  imageUrl?: string;
  /** «Nächste Folge», «Vorherige Folge». */
  heading: string;
  /** «S01E05 · Die Brücke», «Staffel 2 · S02E01 · …», a podcast episode's title. */
  name: string;
  /** The length, for a podcast episode the date as well. */
  detail?: string;
  /** The key: «N», «P». */
  keys: string;
}

interface PlayerStepProps {
  icon: IconName;
  /** Spoken name: «Nächstes Kapitel», «Nächste Folge: S01E05 · Die Brücke». */
  label: string;
  onPress: () => void;
  /**
   * Greyed in place (concept 23, 3.1): the button stays focusable and in its row, a press does
   * nothing, and this hint («Keine Kapitel», «Keine weitere Folge») is its description.
   */
  unavailable?: string;
  card?: PlayerStepCard;
  /** The card opens to the right of the button (the leftmost button of a row). */
  cardAlign?: 'start' | 'center';
  /** Off for real (a switch is on its way, the playback ended): not focusable. */
  isDisabled?: boolean;
  excludeFromTabOrder?: boolean;
  size?: 'md' | 'lg';
}

/**
 * A chapter or episode button of a player's transport (concept 23): a plain button, because a
 * greyed one must keep the focus and carry `aria-disabled` (the react-aria button sets
 * `aria-disabled` itself and drops it). The hint and the card are drawn over the button and are
 * hidden from the accessibility tree: the hint is the button's description, the card repeats its
 * name.
 */
export function PlayerStep({
  icon,
  label,
  onPress,
  unavailable,
  card,
  cardAlign = 'center',
  isDisabled = false,
  excludeFromTabOrder = false,
  size,
}: PlayerStepProps) {
  const hintId = useId();
  const greyed = unavailable !== undefined && !isDisabled;
  return (
    <span className={styles.step}>
      <Button
        variant="ghost"
        size={size}
        icon={icon}
        iconOnly
        aria-label={label}
        aria-disabled={greyed || undefined}
        aria-describedby={greyed ? hintId : undefined}
        isDisabled={isDisabled}
        excludeFromTabOrder={excludeFromTabOrder}
        onPress={greyed ? undefined : onPress}
      />
      {greyed && (
        <span id={hintId} className={styles.hint} aria-hidden="true">
          {unavailable}
        </span>
      )}
      {card && !greyed && !isDisabled && (
        <span className={styles.card} data-align={cardAlign} aria-hidden="true">
          {card.imageUrl ? (
            <img
              className={styles.image}
              src={card.imageUrl}
              alt=""
              width={320}
              height={180}
              decoding="async"
              draggable={false}
            />
          ) : (
            <span className={styles.image} />
          )}
          <span className={styles.body}>
            <span className={styles.heading}>{card.heading}</span>
            <span className={styles.name}>{card.name}</span>
            <span className={styles.detail}>
              {card.detail}
              <kbd className={styles.keys}>{card.keys}</kbd>
            </span>
          </span>
        </span>
      )}
    </span>
  );
}
