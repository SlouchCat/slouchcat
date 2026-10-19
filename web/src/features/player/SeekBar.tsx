import { useRef, useState, type PointerEvent } from 'react';
import styles from './Player.module.css';

interface SeekBarProps {
  label: string;
  valueText: string;
  currentMs: number;
  durationMs: number;
  /** Buffered ranges as [startMs, endMs]. */
  buffered: readonly (readonly [number, number])[];
  onSeek: (ms: number) => void;
}

/**
 * Seek bar as an ARIA slider. Arrow keys are handled by the player's key map (±10 s), so the
 * slider itself only handles the pointer.
 */
export function SeekBar({
  label,
  valueText,
  currentMs,
  durationMs,
  buffered,
  onSeek,
}: SeekBarProps) {
  const track = useRef<HTMLDivElement>(null);
  const [dragging, setDragging] = useState(false);
  const fraction = (ms: number) => (durationMs > 0 ? Math.min(1, Math.max(0, ms / durationMs)) : 0);

  const seekTo = (event: PointerEvent<HTMLDivElement>) => {
    const rect = track.current?.getBoundingClientRect();
    if (!rect || rect.width === 0 || durationMs <= 0) return;
    const ratio = Math.min(1, Math.max(0, (event.clientX - rect.left) / rect.width));
    onSeek(ratio * durationMs);
  };

  return (
    <div
      ref={track}
      className={styles.seek}
      role="slider"
      tabIndex={0}
      aria-label={label}
      aria-valuemin={0}
      aria-valuemax={Math.round(durationMs / 1000)}
      aria-valuenow={Math.round(currentMs / 1000)}
      aria-valuetext={valueText}
      data-dragging={dragging || undefined}
      onPointerDown={(event) => {
        event.currentTarget.setPointerCapture(event.pointerId);
        setDragging(true);
        seekTo(event);
      }}
      onPointerMove={(event) => {
        if (dragging) seekTo(event);
      }}
      onPointerUp={(event) => {
        event.currentTarget.releasePointerCapture(event.pointerId);
        setDragging(false);
      }}
      onPointerCancel={() => setDragging(false)}
    >
      <div className={styles.seekTrack}>
        {buffered.map(([start, end]) => (
          <span
            key={start}
            className={styles.seekBuffered}
            style={{
              left: `${fraction(start) * 100}%`,
              width: `${(fraction(end) - fraction(start)) * 100}%`,
            }}
          />
        ))}
        <span className={styles.seekPlayed} style={{ width: `${fraction(currentMs) * 100}%` }} />
        <span className={styles.seekThumb} style={{ left: `${fraction(currentMs) * 100}%` }} />
      </div>
    </div>
  );
}
