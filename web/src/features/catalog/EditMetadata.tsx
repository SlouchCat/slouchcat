import type { ItemRef } from '@slouchcat/api-client';
import { useQueryClient } from '@tanstack/react-query';
import { lazy, Suspense } from 'react';
import { useTranslation } from 'react-i18next';
import { Button } from '@/ui';

// The pen's dialog loads with react-aria on demand, like the trailer dialog: the browsing pages
// stay without it (P-10).
const MetadataEditor = lazy(() =>
  import('@/features/admin/editor/MetadataEditor').then((module) => ({
    default: module.MetadataEditor,
  })),
);

/** The kinds the metadata editor edits (updateItemMetadata): the pen shows only on these. */
export const EDITABLE_KINDS: ReadonlySet<string> = new Set([
  'movie',
  'series',
  'season',
  'episode',
]);

/**
 * The metadata editor of one title over a browsing page (concept 15 7.4). Closing it reads the
 * title and the library pages again, so new images and texts stand at once.
 */
export function EditMetadata({ itemId, onClose }: { itemId: string; onClose: () => void }) {
  const queryClient = useQueryClient();
  return (
    <Suspense fallback={null}>
      <MetadataEditor
        itemId={itemId}
        onClose={() => {
          onClose();
          void queryClient.invalidateQueries({ queryKey: ['item', itemId] });
          void queryClient.invalidateQueries({ queryKey: ['library'] });
        }}
      />
    </Suspense>
  );
}

/**
 * The pen on a library cover (concept 15 7.4): a plain button beside the card's link, not a
 * spatial item, so Tab reaches it after the link and the arrow keys pass it by. Small, always
 * shown, on touch screens too.
 */
export function CoverPen({ item, onEdit }: { item: ItemRef; onEdit: (id: string) => void }) {
  const { t } = useTranslation();
  if (!EDITABLE_KINDS.has(item.kind)) return null;
  return (
    <Button
      variant="secondary"
      size="sm"
      icon="pen"
      iconOnly
      aria-label={t('library.editMetadata', { title: item.title })}
      title={t('item.editMetadata')}
      onPress={() => onEdit(item.id)}
    />
  );
}
