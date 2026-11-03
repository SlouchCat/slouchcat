import { useId, useLayoutEffect, useRef } from 'react';
import { useTranslation } from 'react-i18next';
import styles from './RankSelect.module.css';
import { VisuallyHidden } from './VisuallyHidden';

/**
 * How the other entries make room (concept 12, S-16): `move` takes the entry out and puts it in
 * at the chosen position, the ones in between move along by one; `swap` exchanges it with the
 * entry that held the chosen position.
 */
export type RankMode = 'move' | 'swap';

/** The list with the entry at `from` put at `to` (0-based), by `mode`. */
export function rankTo<T>(list: readonly T[], from: number, to: number, mode: RankMode): T[] {
  const next = [...list];
  if (from === to || from < 0 || to < 0 || from >= list.length || to >= list.length) return next;
  if (mode === 'swap') {
    [next[from], next[to]] = [next[to]!, next[from]!];
    return next;
  }
  const [entry] = next.splice(from, 1);
  next.splice(to, 0, entry!);
  return next;
}

interface RankHintProps {
  id: string;
  mode?: RankMode;
}

/** The sentence that says what a pick does; one per list when the selects share it. */
export function RankHint({ id, mode = 'move' }: RankHintProps) {
  const { t } = useTranslation();
  return (
    <span id={id} className={styles.hint}>
      {mode === 'move' ? t('ui.rank.hintMove') : t('ui.rank.hintSwap')}
    </span>
  );
}

interface RankSelectProps {
  /** Name of the entry ("Deutsch", "TMDB"): the select is «Platz für Deutsch». */
  name: string;
  /** Current position, 0-based. */
  position: number;
  /** Number of positions to choose from. */
  count: number;
  mode?: RankMode;
  /** Gets the chosen position, 0-based; the page reorders with `rankTo`. */
  onChange: (position: number) => void;
  /**
   * Id of a RankHint the selects of a list share; without it the select shows its own hint
   * below it.
   */
  describedBy?: string;
  isDisabled?: boolean;
}

/**
 * The position of an entry as a choice (concept 12, S-16): a native select «Platz 1» … «Platz n»
 * instead of «Nach oben»/«Nach unten» or dragging. ↑ and ↓ pick a position like in any select;
 * the hint, linked with `aria-describedby`, says whether the others move along or swap.
 */
export function RankSelect({
  name,
  position,
  count,
  mode = 'move',
  onChange,
  describedBy,
  isDisabled,
}: RankSelectProps) {
  const { t } = useTranslation();
  const id = useId();
  const ownHint = useId();
  // The page reorders its list by key: React moves this select's node, and a moved node loses
  // the focus. A select that changed its own position takes the focus back once it stands at
  // the new one (↓ on a closed select changes it at once, so this happens on every arrow key).
  const select = useRef<HTMLSelectElement>(null);
  const changedByMe = useRef(false);
  useLayoutEffect(() => {
    if (!changedByMe.current) return;
    changedByMe.current = false;
    select.current?.focus();
  }, [position]);
  return (
    <span className={styles.field}>
      {/* Hidden around the label, not inside it: an empty label would still take a line of the
          column (and its gap) and pull the select below the text of its row. */}
      <VisuallyHidden>
        <label htmlFor={id}>{t('ui.rank.label', { name })}</label>
      </VisuallyHidden>
      <select
        ref={select}
        id={id}
        className={styles.select}
        value={position}
        disabled={isDisabled}
        aria-describedby={describedBy ?? ownHint}
        onChange={(event) => {
          changedByMe.current = true;
          onChange(Number(event.target.value));
          // A page that keeps the order renders nothing new; the flag must not wait for the
          // next change by someone else.
          requestAnimationFrame(() => {
            changedByMe.current = false;
          });
        }}
      >
        {Array.from({ length: count }, (_, index) => (
          <option key={index} value={index}>
            {t('ui.rank.option', { position: index + 1 })}
          </option>
        ))}
      </select>
      {!describedBy && <RankHint id={ownHint} mode={mode} />}
    </span>
  );
}
