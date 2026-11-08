import { useEffect, useRef, useState } from 'react';
import styles from './TimeShiftSlider.module.css';

// The time shift of «Jetzt läuft» (concept 16, 10.3, stage 2): a native range from the oldest
// position of the window (−30:00) to live, in steps of 5 s. Arrow keys move it, the value is
// spoken as «12 Minuten 30 Sekunden zurück». A drag or a run of key presses seeks once it rests,
// so the player does not fetch the segments of every step on the way. Texts from the app (rule 9).

/** A drag or a key press seeks when the slider rested this long. */
const SEEK_DELAY_MS = 250;
export const TIME_SHIFT_STEP_SECONDS = 5;

interface TimeShiftSliderProps {
  /** Accessible name («Zeitversatz, bis 30 Minuten zurück»). */
  label: string;
  /** The window: how far back the slider goes, in seconds. */
  maxSeconds: number;
  /** Seconds behind live now. */
  offset: number;
  onSeek: (offset: number) => void;
  /** The spoken value («12 Minuten 30 Sekunden zurück», «Live»). */
  valueText: (offset: number) => string;
  /** The shown value under the slider («−12:30», «Live»). */
  shownText: (offset: number) => string;
  /** Under the slider's left end («−30:00») and its right end («Live»). */
  startLabel: string;
  endLabel: string;
}

export function TimeShiftSlider({
  label,
  maxSeconds,
  offset,
  onSeek,
  valueText,
  shownText,
  startLabel,
  endLabel,
}: TimeShiftSliderProps) {
  // The value while the slider moves; null once the seek went out and the player's offset leads.
  const [draft, setDraft] = useState<number | null>(null);
  const timer = useRef<number | undefined>(undefined);
  useEffect(() => () => window.clearTimeout(timer.current), []);
  const shown = Math.min(maxSeconds, draft ?? offset);
  const move = (next: number) => {
    setDraft(next);
    window.clearTimeout(timer.current);
    timer.current = window.setTimeout(() => {
      onSeek(next);
      setDraft(null);
    }, SEEK_DELAY_MS);
  };
  return (
    <div className={styles.slider}>
      <input
        type="range"
        className={styles.range}
        min={-maxSeconds}
        max={0}
        step={TIME_SHIFT_STEP_SECONDS}
        value={-shown}
        aria-label={label}
        aria-valuetext={valueText(shown)}
        data-testid="radio-time-shift"
        onChange={(event) => move(-Number(event.currentTarget.value))}
      />
      <div className={styles.scale} aria-hidden="true">
        <span>{startLabel}</span>
        <span className={styles.value} data-live={shown === 0 ? '' : undefined}>
          {shownText(shown)}
        </span>
        <span>{endLabel}</span>
      </div>
    </div>
  );
}
