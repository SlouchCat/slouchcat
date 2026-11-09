import styles from './LetterBar.module.css';
import { SpatialGroup, useSpatialItem } from './spatial';

function LetterButton({
  text,
  label,
  current,
  focusKey,
  onPress,
}: {
  text: string;
  label?: string;
  current: boolean;
  focusKey?: string;
  onPress: () => void;
}) {
  const { ref, onFocus } = useSpatialItem<HTMLButtonElement>({ focusKey });
  return (
    <button
      ref={ref}
      type="button"
      className={styles.letter}
      aria-label={label}
      aria-current={current ? 'true' : undefined}
      onFocus={onFocus}
      onClick={onPress}
    >
      {text}
    </button>
  );
}

interface LetterBarProps {
  /** Spoken name of the bar ("Anfangsbuchstabe"). */
  label: string;
  /** Every letter of the bar, in order. */
  letters: readonly string[];
  /** The letters with titles; the others are disabled. */
  available: readonly string[];
  /** The letter of the titles in view (`aria-current`); undefined for none. */
  current: string | undefined;
  /** Called with a letter that has titles: the list jumps to its first title. */
  onJump: (letter: string) => void;
  /** Spoken names of letters that are symbols (`#`: "Ziffer oder Zeichen"). */
  letterLabels?: Partial<Record<string, string>>;
  /** Focus key prefix: the bar is `<prefix>`, each button `<prefix>-<letter>`. */
  focusKey?: string;
  className?: string;
}

/**
 * The initial letters of a list sorted by title (library page): a column at the side from 768 px,
 * a strip that scrolls sideways below. A letter jumps to the first title with it (concept 17
 * section 6); the letter of the titles in view is marked. A letter without titles is disabled and
 * skipped by the arrow keys.
 */
export function LetterBar({
  label,
  letters,
  available,
  current,
  onJump,
  letterLabels = {},
  focusKey,
  className,
}: LetterBarProps) {
  const key = (suffix: string) => (focusKey ? `${focusKey}-${suffix}` : undefined);
  return (
    <SpatialGroup
      as="nav"
      focusKey={focusKey}
      className={className ? `${styles.bar} ${className}` : styles.bar}
      aria-label={label}
    >
      {letters.map((letter) =>
        // Hooks stay per component: an offered letter registers for the arrows, a disabled one not.
        available.includes(letter) ? (
          <LetterButton
            key={letter}
            text={letter}
            label={letterLabels[letter]}
            current={letter === current}
            focusKey={key(letter)}
            onPress={() => onJump(letter)}
          />
        ) : (
          <button
            key={letter}
            type="button"
            className={styles.letter}
            aria-label={letterLabels[letter]}
            disabled
          >
            {letter}
          </button>
        ),
      )}
    </SpatialGroup>
  );
}
