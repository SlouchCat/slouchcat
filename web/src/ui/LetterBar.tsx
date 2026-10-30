import styles from './LetterBar.module.css';
import { SpatialGroup, useSpatialItem } from './spatial';

function LetterButton({
  text,
  label,
  pressed,
  focusKey,
  onPress,
}: {
  text: string;
  label?: string;
  pressed: boolean;
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
      aria-pressed={pressed}
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
  /** The chosen letter; undefined is "all". */
  value: string | undefined;
  onChange: (letter: string | undefined) => void;
  /** The first button, which lifts the choice: its mark ("•") and its spoken name. */
  allMark: string;
  allLabel: string;
  /** Spoken names of letters that are symbols (`#`: "Ziffer oder Zeichen"). */
  letterLabels?: Partial<Record<string, string>>;
  /** Focus key prefix: the bar is `<prefix>`, each button `<prefix>-<letter>` (`-all` first). */
  focusKey?: string;
  className?: string;
}

/**
 * The initial letters of a list sorted by title (library page): a column at the side from 768 px,
 * a strip that scrolls sideways below. A letter filters the list; a letter without titles is
 * disabled and skipped by the arrow keys.
 */
export function LetterBar({
  label,
  letters,
  available,
  value,
  onChange,
  allMark,
  allLabel,
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
      <LetterButton
        text={allMark}
        label={allLabel}
        pressed={value === undefined}
        focusKey={key('all')}
        onPress={() => onChange(undefined)}
      />
      {letters.map((letter) =>
        // Hooks stay per component: an offered letter registers for the arrows, a disabled one not.
        available.includes(letter) || letter === value ? (
          <LetterButton
            key={letter}
            text={letter}
            label={letterLabels[letter]}
            pressed={letter === value}
            focusKey={key(letter)}
            onPress={() => onChange(letter === value ? undefined : letter)}
          />
        ) : (
          <button
            key={letter}
            type="button"
            className={styles.letter}
            aria-label={letterLabels[letter]}
            aria-pressed={false}
            disabled
          >
            {letter}
          </button>
        ),
      )}
    </SpatialGroup>
  );
}
