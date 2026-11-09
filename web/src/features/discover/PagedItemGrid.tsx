import type { ItemRef } from '@slouchcat/api-client';
import type { ReactNode } from 'react';
import { LibraryGrid } from '@/features/catalog/LibraryGrid';
import { PAGED_LIST_VIRTUAL_FROM, type PlaceRange } from '@/lib/pagedList';
import { ItemGrid } from './ItemGrid';

interface PagedItemGridProps {
  /** The places of the list (usePagedList). */
  slots: readonly (ItemRef | undefined)[];
  /** Focus key prefix: the grid is `<prefix>`, each card `<prefix>-<id>`. */
  focusKey: string;
  label: string;
  showWatched?: boolean;
  /** Extra controls under a card (the watchlist's "Entfernen"). */
  below?: (item: ItemRef) => ReactNode;
  onUnmarked?: (item: ItemRef) => void;
  /** The places the drawn rows ask for (a virtual grid). */
  onRange: (range: PlaceRange) => void;
}

/**
 * The titles of a list operation without «Mehr laden» (concept 18 C-3): up to
 * PAGED_LIST_VIRTUAL_FROM titles the whole list as an ItemGrid (its pages are read at once),
 * above that the virtual grid of the library (LibraryGrid), which reads the pages of the rows in
 * view and keeps the way back to a card.
 */
export function PagedItemGrid({
  slots,
  focusKey,
  label,
  showWatched = false,
  below,
  onUnmarked,
  onRange,
}: PagedItemGridProps) {
  if (slots.length > PAGED_LIST_VIRTUAL_FROM) {
    return (
      <LibraryGrid
        slots={slots}
        focusKey={focusKey}
        label={label}
        showWatched={showWatched}
        below={below}
        onUnmarked={onUnmarked}
        onRange={onRange}
      />
    );
  }
  return (
    <ItemGrid
      items={slots.filter((item): item is ItemRef => item !== undefined)}
      focusKey={focusKey}
      label={label}
      showWatched={showWatched}
      below={below}
      onUnmarked={onUnmarked}
    />
  );
}
