import type { ItemRef } from '@slouchcat/api-client';
import type { ReactNode } from 'react';
import { LibraryGrid } from '@/features/catalog/LibraryGrid';
import { PAGED_LIST_VIRTUAL_FROM, type PlaceRange } from '@/lib/pagedList';
import { ItemGrid, type CardFacts } from './ItemGrid';

interface PagedItemGridProps {
  /** The places of the list (usePagedList). */
  slots: readonly (ItemRef | undefined)[];
  /** Focus key prefix: the grid is `<prefix>`, each card `<prefix>-<id>`. */
  focusKey: string;
  label: string;
  showWatched?: boolean;
  /** A control over the cover beside the card's link (the pen, concept 20 P-7; ItemGrid `tool`). */
  tool?: (item: ItemRef) => ReactNode;
  /** Extra controls under a card (the watchlist's "Entfernen"). */
  below?: (item: ItemRef) => ReactNode;
  onUnmarked?: (item: ItemRef) => void;
  /** Called once a title's check mark was removed from its cover (the page «Gesehen»). */
  onUnwatched?: (item: ItemRef) => void;
  /** Caption and bar of a card from the page (the page «Gesehen»). */
  describe?: (item: ItemRef) => CardFacts;
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
  tool,
  below,
  onUnmarked,
  onUnwatched,
  describe,
  onRange,
}: PagedItemGridProps) {
  if (slots.length > PAGED_LIST_VIRTUAL_FROM) {
    return (
      <LibraryGrid
        slots={slots}
        focusKey={focusKey}
        label={label}
        showWatched={showWatched}
        tool={tool}
        below={below}
        onUnmarked={onUnmarked}
        onUnwatched={onUnwatched}
        describe={describe}
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
      tool={tool}
      below={below}
      onUnmarked={onUnmarked}
      onUnwatched={onUnwatched}
      describe={describe}
    />
  );
}
