import { useEffect, useRef, useState } from 'react';
import { ListBox, ListBoxItem } from 'react-aria-components';
import styles from './ImageCandidateGrid.module.css';

export interface ImageCandidateTile {
  /** Stable key: the source's address of the image. */
  id: string;
  /** Preview address; a provider's is loaded without a referrer. */
  previewUrl?: string;
  /** First line under the image: the language ("Deutsch", "Ohne Text"), a book's file name. */
  label: string;
  /** Second line: size, source and rating ("2000 × 3000 · TMDB · 5,4"). */
  detail?: string;
  /** Marked as the current image of the kind. */
  current?: boolean;
  /** Spoken name of the tile ("Deutsch, 2000 × 3000, TMDB wählen"). */
  actionLabel: string;
}

interface ImageCandidateGridProps {
  /** Spoken name of the grid. */
  label: string;
  items: readonly ImageCandidateTile[];
  /** Enter or a click on a tile. */
  onAction: (id: string) => void;
  /** Shown instead of the tiles when there are none. */
  emptyText: string;
  /** The mark of the current tile ("aktuell"). */
  currentText: string;
  /** Shape of the previews: posters stand, backdrops and logos lie, audiobook covers are square. */
  shape?: 'portrait' | 'landscape' | 'square';
  /** Focus the first tile once there is one (a dialog that opens on the grid). */
  autoFocus?: boolean;
}

/**
 * Image candidates as a grid with one Tab stop (react-aria ListBox, layout grid, concept 10 7.3):
 * the arrow keys move in both directions, Home and End jump, Enter or a click chooses. Like
 * PathList the tiles carry no selection, so one click acts. The spoken name is `actionLabel`; the
 * visible lines repeat it and stay out of the accessibility tree.
 */
export function ImageCandidateGrid({
  label,
  items,
  onAction,
  emptyText,
  currentText,
  shape = 'portrait',
  autoFocus,
}: ImageCandidateGridProps) {
  const [node, setNode] = useState<HTMLDivElement | null>(null);
  const pending = useRef(autoFocus === true);
  useEffect(() => {
    if (!pending.current || !node) return;
    const first = node.querySelector<HTMLElement>('[role="option"]');
    if (!first) return;
    pending.current = false;
    first.focus();
  });
  return (
    <ListBox
      ref={setNode}
      aria-label={label}
      className={styles.grid}
      data-shape={shape}
      layout="grid"
      items={items}
      selectionMode="none"
      onAction={(key) => onAction(String(key))}
      renderEmptyState={() => <div className={styles.empty}>{emptyText}</div>}
    >
      {(item: ImageCandidateTile) => (
        <ListBoxItem
          id={item.id}
          textValue={item.actionLabel}
          aria-label={item.actionLabel}
          className={styles.tile}
        >
          <span className={styles.frame}>
            {item.previewUrl ? (
              <img
                className={styles.image}
                src={item.previewUrl}
                alt=""
                loading="lazy"
                referrerPolicy="no-referrer"
              />
            ) : null}
          </span>
          <span className={styles.label} aria-hidden="true">
            {item.label}
            {item.current && <span className={styles.current}>{currentText}</span>}
          </span>
          {item.detail && (
            <span className={styles.detail} aria-hidden="true">
              {item.detail}
            </span>
          )}
        </ListBoxItem>
      )}
    </ListBox>
  );
}
