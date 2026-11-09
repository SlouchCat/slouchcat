import { useEffect, useMemo, useRef, useState, type CSSProperties, type PointerEvent } from 'react';
import { useTranslation } from 'react-i18next';
import { formatPlayerTime } from '@/lib/format';
import styles from './SeekBar.module.css';

/** A chapter on the timeline (contract `Chapter`); one without a title is numbered by position. */
export interface SeekBarChapter {
  startMs: number;
  title?: string;
  /** The chapter image (stage 3, `Chapter.imageUrl` resolved), shown in the field over the pointer. */
  imageUrl?: string;
}

/** A stretch the server detected (contract `Segment`). */
export interface SeekBarSegment {
  startMs: number;
  endMs: number;
}

/**
 * The preview tiles of the file (contract `Trickplay`, concept 18 part A), with the URL of every
 * sheet resolved by the page (online the server's, offline a stored blob). The server decides
 * whether there are any; the bar only looks the tile up (rule 12).
 */
export interface SeekBarTrickplay {
  tileWidth: number;
  tileHeight: number;
  columns: number;
  rows: number;
  /** Time of every point, ascending; tile `k` shows `pointsMs[k]`. */
  pointsMs: readonly number[];
  /** The sheets that exist now, in any order; a missing one shows the chapter image instead. */
  sheets: readonly { index: number; url: string }[];
}

/** Where tile `k` lies: its sheet, column and row, and the rows of that sheet. */
export interface TrickplayTile {
  index: number;
  sheet: number;
  column: number;
  row: number;
  /** Rows of the sheet: `rows`, the last sheet only as many as it needs. */
  sheetRows: number;
}

/** How long the field stays over the new position after a seek by key (concept 18, 9). */
export const KEY_PREVIEW_MS = 1500;

/**
 * The tile for a position: the last point at or before it (contract `Trickplay`); null before the
 * first point and without points.
 */
export function trickplayTileAt(
  trickplay: Pick<SeekBarTrickplay, 'columns' | 'rows' | 'pointsMs'>,
  ms: number,
): TrickplayTile | null {
  const { columns, rows, pointsMs } = trickplay;
  if (columns < 1 || rows < 1) return null;
  // Binary search for the last point <= ms.
  let low = 0;
  let high = pointsMs.length - 1;
  let index = -1;
  while (low <= high) {
    const middle = (low + high) >> 1;
    if ((pointsMs[middle] ?? Infinity) <= ms) {
      index = middle;
      low = middle + 1;
    } else high = middle - 1;
  }
  if (index < 0) return null;
  const perSheet = columns * rows;
  const sheet = Math.floor(index / perSheet);
  const onSheet = index % perSheet;
  const tilesOnSheet = Math.min(perSheet, pointsMs.length - sheet * perSheet);
  return {
    index,
    sheet,
    column: onSheet % columns,
    row: Math.floor(onSheet / columns),
    sheetRows: Math.ceil(tilesOnSheet / columns),
  };
}

/** The CSS of a tile: the sheet as background, scaled so that one tile fills the box. */
function tileStyle(tile: TrickplayTile, columns: number, url: string): CSSProperties {
  const at = (cell: number, cells: number) => (cells > 1 ? `${(cell / (cells - 1)) * 100}%` : '0%');
  return {
    backgroundImage: `url(${JSON.stringify(url)})`,
    backgroundSize: `${columns * 100}% ${tile.sheetRows * 100}%`,
    backgroundPosition: `${at(tile.column, columns)} ${at(tile.row, tile.sheetRows)}`,
  };
}

interface SeekBarProps {
  label: string;
  valueText: string;
  currentMs: number;
  durationMs: number;
  /** Buffered ranges as [startMs, endMs]. */
  buffered: readonly (readonly [number, number])[];
  onSeek: (ms: number) => void;
  /** Chapter marks on the track; the preview names the chapter under the pointer, with its image. */
  chapters?: readonly SeekBarChapter[];
  /** Intro and outro, drawn lighter than the rest of the unfilled track. */
  segments?: { intro?: SeekBarSegment; outro?: SeekBarSegment };
  /** Shift+←: the page decides where "previous chapter" goes (the start of this one or the last). */
  onPreviousChapter?: () => void;
  /** Shift+→. */
  onNextChapter?: () => void;
  /**
   * Preview tiles (concept 18 part A): the field shows the tile of the position in the box of the
   * tile's aspect ratio, and the chapter image where the sheet is not there (yet).
   */
  trickplay?: SeekBarTrickplay;
  /**
   * Counts the player's seeks by key (←/→): each change shows the field over the new position for
   * KEY_PREVIEW_MS; a held key keeps it, since every repeat seeks again (concept 18, 9).
   */
  keySeeks?: number;
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
 * over it a small field with the preview tile of the position (concept 18 part A) or else the
 * image of the chapter there (when it has one), the time and the chapter's name. Plain arrow keys
 * are handled by the player's key map (±10 s); after each such seek the page counts `keySeeks` up
 * and the field shows the new position for a moment. The slider itself takes Shift+←/→ for the
 * chapters, when the page passes them, and the pointer: a mouse seeks on press and while dragging,
 * a finger only shows the preview while it drags and seeks where it lets go.
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
  trickplay,
  keySeeks = 0,
}: SeekBarProps) {
  const { t } = useTranslation();
  const track = useRef<HTMLDivElement>(null);
  const [dragging, setDragging] = useState(false);
  const [pointerMs, setPointerMs] = useState<number | null>(null);
  // A finger drags the preview and seeks once, where it lets go.
  const touch = useRef(false);
  // A seek by key shows the field for KEY_PREVIEW_MS (state adjusted while rendering, the timer
  // in an effect).
  const [seenKeySeeks, setSeenKeySeeks] = useState(keySeeks);
  const [keyPreview, setKeyPreview] = useState(false);
  if (keySeeks !== seenKeySeeks) {
    setSeenKeySeeks(keySeeks);
    setKeyPreview(true);
  }
  useEffect(() => {
    if (!keyPreview) return;
    const timer = setTimeout(() => setKeyPreview(false), KEY_PREVIEW_MS);
    return () => clearTimeout(timer);
  }, [keyPreview, seenKeySeeks]);
  // Images that failed to load: the same rule as ChapterCard, a URL that failed is not loaded
  // again while the bar is mounted; the field shows the empty box for it.
  const [failedImages, setFailedImages] = useState<ReadonlySet<string>>(() => new Set());
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

  // The pointer wins over the position of a seek by key.
  const previewMs = pointerMs ?? (keyPreview ? currentMs : null);
  const chapterAtPointer = previewMs === null ? -1 : chapterIndexAt(chapters, previewMs);
  const chapterName =
    chapterAtPointer < 0
      ? undefined
      : (chapters[chapterAtPointer]?.title ??
        t('player.chapters.untitled', { n: chapterAtPointer + 1 }));
  const chapterImage = chapterAtPointer < 0 ? undefined : chapters[chapterAtPointer]?.imageUrl;
  const previewImage = chapterImage && !failedImages.has(chapterImage) ? chapterImage : undefined;
  // Once the title has tiles or a chapter has an image, every position keeps the box (16:9, or the
  // tile's aspect ratio), so the field does not change its height between positions with and
  // without a picture.
  const imageBox = trickplay !== undefined || chapters.some((chapter) => chapter.imageUrl);
  const boxRatio: CSSProperties | undefined = trickplay && {
    aspectRatio: `${trickplay.tileWidth} / ${trickplay.tileHeight}`,
  };
  const sheetUrls = useMemo(
    () => new Map(trickplay?.sheets.map((sheet) => [sheet.index, sheet.url])),
    [trickplay?.sheets],
  );
  const tile = trickplay && previewMs !== null ? trickplayTileAt(trickplay, previewMs) : null;
  const tileUrl = tile ? sheetUrls.get(tile.sheet) : undefined;
  // Only the sheet under the pointer (the field's background) and the next one load; the browser
  // cache keeps them (`immutable`).
  const nextSheetUrl = tile ? sheetUrls.get(tile.sheet + 1) : undefined;
  useEffect(() => {
    if (!nextSheetUrl) return;
    const image = new Image();
    image.src = nextSheetUrl;
  }, [nextSheetUrl]);

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
      data-pointer={previewMs !== null || undefined}
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
        touch.current = event.pointerType === 'touch';
        event.currentTarget.setPointerCapture(event.pointerId);
        setDragging(true);
        setPointerMs(positionAt(event));
        if (!touch.current) seekTo(event);
      }}
      onPointerMove={(event) => {
        setPointerMs(positionAt(event));
        if (dragging && !touch.current) seekTo(event);
      }}
      onPointerUp={(event) => {
        event.currentTarget.releasePointerCapture(event.pointerId);
        if (dragging && touch.current) seekTo(event);
        setDragging(false);
      }}
      onPointerCancel={() => {
        setDragging(false);
        setPointerMs(null);
      }}
      onPointerLeave={() => {
        if (!dragging) setPointerMs(null);
      }}
      // A drag let go outside the bar ends here, without a pointerleave (that one came while the
      // capture held the pointer): the field goes unless the pointer is over the bar.
      onLostPointerCapture={(event) => {
        setDragging(false);
        const box = event.currentTarget.getBoundingClientRect();
        const over =
          event.clientX >= box.left &&
          event.clientX <= box.right &&
          event.clientY >= box.top &&
          event.clientY <= box.bottom;
        if (!over) setPointerMs(null);
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
          .map((chapter, index) => (
            <span
              key={`${index}-${chapter.startMs}`}
              className={styles.mark}
              data-chapter-mark=""
              style={{ left: percent(chapter.startMs) }}
            />
          ))}
        <span className={styles.thumb} style={{ left: percent(currentMs) }} />
      </div>
      {previewMs !== null && (
        // The slider speaks its value; the field is for the pointer only. Its fixed width (the
        // room of the picture) keeps it inside the bar at both ends.
        <div
          className={styles.preview}
          aria-hidden="true"
          data-seek-preview=""
          style={{ left: `clamp(5rem, ${percent(previewMs)}, calc(100% - 5rem))` }}
        >
          {tile && tileUrl && trickplay ? (
            // The tile is a cut-out of its sheet; the box has the tile's aspect ratio.
            <span
              className={`${styles.previewImage} ${styles.previewTile}`}
              data-trickplay-tile={tile.index}
              style={{ ...boxRatio, ...tileStyle(tile, trickplay.columns, tileUrl) }}
            />
          ) : previewImage ? (
            // Only the chapter under the pointer loads its image; the browser cache keeps it for
            // the next visit. The key gives each chapter a fresh element, so the field never shows
            // the previous chapter's image under this one's name while it loads; the box keeps its
            // 16:9 size meanwhile. Decoded at once (no `decoding="async"`): the field is on screen
            // only while the pointer rests.
            <img
              key={previewImage}
              className={styles.previewImage}
              src={previewImage}
              alt=""
              width={480}
              height={270}
              style={boxRatio}
              draggable={false}
              onError={() => setFailedImages((failed) => new Set(failed).add(previewImage))}
            />
          ) : (
            imageBox && <span className={styles.previewImage} data-image-box="" style={boxRatio} />
          )}
          <span className={styles.previewTime}>{formatPlayerTime(previewMs, durationMs)}</span>
          {chapterName && <span className={styles.previewChapter}>{chapterName}</span>}
        </div>
      )}
    </div>
  );
}
