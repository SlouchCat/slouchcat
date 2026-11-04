import { useRef, useState, type CSSProperties, type PointerEvent } from 'react';
import { useTranslation } from 'react-i18next';
import { formatPlayerTime } from '@/lib/format';
import styles from './SeekBar.module.css';

/** A chapter on the timeline (contract `Chapter`); one without a title is numbered by position. */
export interface SeekBarChapter {
  startMs: number;
  title?: string;
}

/** A stretch the server detected (contract `Segment`). */
export interface SeekBarSegment {
  startMs: number;
  endMs: number;
}

interface SeekBarProps {
  label: string;
  valueText: string;
  currentMs: number;
  durationMs: number;
  /** Buffered ranges as [startMs, endMs]. */
  buffered: readonly (readonly [number, number])[];
  onSeek: (ms: number) => void;
  /** Chapter marks on the track; the preview names the chapter under the pointer. */
  chapters?: readonly SeekBarChapter[];
  /** Intro and outro, drawn lighter than the rest of the unfilled track. */
  segments?: { intro?: SeekBarSegment; outro?: SeekBarSegment };
  /** Shift+←: the page decides where "previous chapter" goes (the start of this one or the last). */
  onPreviousChapter?: () => void;
  /** Shift+→. */
  onNextChapter?: () => void;
}

/** Index of the chapter that plays at `ms` (the last one starting at or before it), -1 before all. */
export function chapterIndexAt(chapters: readonly SeekBarChapter[], ms: number): number {
  let index = -1;
  chapters.forEach((chapter, at) => {
    if (chapter.startMs <= ms) index = at;
  });
  return index;
}

/**
 * Seek bar as an ARIA slider: chapter marks, a lighter intro and outro, and while the pointer is
 * over it a small field with the time and the chapter there. Plain arrow keys are handled by the
 * player's key map (±10 s); the slider itself takes Shift+←/→ for the chapters, when the page
 * passes them, and the pointer.
 */
export function SeekBar({
  label,
  valueText,
  currentMs,
  durationMs,
  buffered,
  onSeek,
  chapters = [],
  segments,
  onPreviousChapter,
  onNextChapter,
}: SeekBarProps) {
  const { t } = useTranslation();
  const track = useRef<HTMLDivElement>(null);
  const [dragging, setDragging] = useState(false);
  const [pointerMs, setPointerMs] = useState<number | null>(null);
  const fraction = (ms: number) => (durationMs > 0 ? Math.min(1, Math.max(0, ms / durationMs)) : 0);
  const percent = (ms: number) => `${fraction(ms) * 100}%`;
  const stretch = (start: number, end: number): CSSProperties => ({
    left: percent(start),
    width: `${(fraction(end) - fraction(start)) * 100}%`,
  });

  const positionAt = (event: PointerEvent<HTMLDivElement>) => {
    const rect = track.current?.getBoundingClientRect();
    if (!rect || rect.width === 0 || durationMs <= 0) return null;
    return Math.min(1, Math.max(0, (event.clientX - rect.left) / rect.width)) * durationMs;
  };
  const seekTo = (event: PointerEvent<HTMLDivElement>) => {
    const ms = positionAt(event);
    if (ms !== null) onSeek(ms);
  };

  const chapterAtPointer = pointerMs === null ? -1 : chapterIndexAt(chapters, pointerMs);
  const chapterName =
    chapterAtPointer < 0
      ? undefined
      : (chapters[chapterAtPointer]?.title ??
        t('player.chapters.untitled', { n: chapterAtPointer + 1 }));

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
      data-pointer={pointerMs !== null || undefined}
      onKeyDown={(event) => {
        if (!event.shiftKey || event.altKey || event.ctrlKey || event.metaKey) return;
        const action =
          event.key === 'ArrowLeft'
            ? onPreviousChapter
            : event.key === 'ArrowRight'
              ? onNextChapter
              : undefined;
        if (!action) return;
        event.preventDefault();
        event.stopPropagation();
        action();
      }}
      onPointerDown={(event) => {
        event.currentTarget.setPointerCapture(event.pointerId);
        setDragging(true);
        seekTo(event);
      }}
      onPointerMove={(event) => {
        setPointerMs(positionAt(event));
        if (dragging) seekTo(event);
      }}
      onPointerUp={(event) => {
        event.currentTarget.releasePointerCapture(event.pointerId);
        setDragging(false);
      }}
      onPointerCancel={() => {
        setDragging(false);
        setPointerMs(null);
      }}
      onPointerLeave={() => {
        if (!dragging) setPointerMs(null);
      }}
    >
      <div className={styles.track}>
        {buffered.map(([start, end]) => (
          <span key={start} className={styles.buffered} style={stretch(start, end)} />
        ))}
        {segments?.intro && (
          <span
            className={styles.segment}
            data-segment="intro"
            style={stretch(segments.intro.startMs, segments.intro.endMs)}
          />
        )}
        {segments?.outro && (
          <span
            className={styles.segment}
            data-segment="outro"
            style={stretch(segments.outro.startMs, segments.outro.endMs)}
          />
        )}
        <span className={styles.played} style={{ width: percent(currentMs) }} />
        {chapters
          .filter((chapter) => chapter.startMs > 0 && chapter.startMs < durationMs)
          .map((chapter) => (
            <span
              key={chapter.startMs}
              className={styles.mark}
              data-chapter-mark=""
              style={{ left: percent(chapter.startMs) }}
            />
          ))}
        <span className={styles.thumb} style={{ left: percent(currentMs) }} />
      </div>
      {pointerMs !== null && (
        // The slider speaks its value; the field is for the pointer only. Its fixed width (room
        // for a chapter image in stage 3) keeps it inside the bar at both ends.
        <div
          className={styles.preview}
          aria-hidden="true"
          style={{ left: `clamp(5rem, ${percent(pointerMs)}, calc(100% - 5rem))` }}
        >
          <span className={styles.previewTime}>{formatPlayerTime(pointerMs, durationMs)}</span>
          {chapterName && <span className={styles.previewChapter}>{chapterName}</span>}
        </div>
      )}
    </div>
  );
}
