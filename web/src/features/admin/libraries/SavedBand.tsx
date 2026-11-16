import type { LibrarySaved } from '@slouchcat/api-client';
import { useMutation, useQueryClient } from '@tanstack/react-query';
import { useTranslation } from 'react-i18next';
import { problemMessage } from '@/api/errors';
import { useRatingSystem } from '@/app/session';
import { Notice } from '@/ui';
import { AriaButton } from '@/ui/aria';
import { ratingLabel } from '../../shared/choices';
import { applyMetadata, jobTypesQuery } from '../queries';
import type { SaveSummary } from './libraryForm';
import { trickplayModeText } from './TrickplaySection';

/** One save of the form «Bibliothek bearbeiten»: the server's answer and what the form changed. */
export interface SavedLibrary {
  saved: LibrarySaved;
  summary: SaveSummary;
}

/**
 * The one band after «Speichern» (concept 24 4.3, «Eine Meldung statt fünf»): what changed in
 * one line each. When languages or images of a folder changed, the band names how many titles
 * have it as their home folder and offers «Jetzt anwenden» for exactly those folders; Slouchcat
 * never applies by itself, because every language costs requests (3.3).
 */
export function SavedBand({ value, onDismiss }: { value: SavedLibrary; onDismiss: () => void }) {
  const { t } = useTranslation();
  const queryClient = useQueryClient();
  const ratingSystem = useRatingSystem();
  const { saved, summary } = value;
  const { library, metadataChanged } = saved;
  const apply = useMutation({
    mutationFn: () =>
      applyMetadata({
        // Languages change the texts, which brings the images along; images alone fetch images.
        what: summary.langs > 0 ? ['texts'] : ['images'],
        rootIds: metadataChanged.map((entry) => entry.rootId),
      }),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: jobTypesQuery.queryKey }),
  });
  const titles = metadataChanged.reduce((sum, entry) => sum + entry.homeTitleCount, 0);
  const lines = [
    summary.name !== undefined && t('admin.libraries.saved.name', { name: summary.name }),
    summary.rating &&
      library.defaultRating !== undefined &&
      t('admin.libraries.saved.rating', {
        rating: ratingLabel(t, library.defaultRating, ratingSystem),
      }),
    summary.trickplay &&
      library.trickplay &&
      t('admin.libraries.trickplay.summary', {
        movies: trickplayModeText(t, library.trickplay.movies),
        series: trickplayModeText(t, library.trickplay.series),
      }),
    summary.added > 0 && t('admin.libraries.saved.added', { count: summary.added }),
    summary.removed > 0 && t('admin.libraries.saved.removed', { count: summary.removed }),
    summary.content > 0 && t('admin.libraries.saved.content', { count: summary.content }),
    summary.moved > 0 && t('admin.libraries.saved.moved', { count: summary.moved }),
    summary.providers > 0 && t('admin.libraries.saved.providers', { count: summary.providers }),
    metadataChanged.length > 0 &&
      t('admin.libraries.saved.metadata', { count: metadataChanged.length, titles }),
  ].filter((line): line is string => typeof line === 'string');

  return (
    <Notice
      title={t('admin.libraries.saved.title', { name: library.name })}
      actions={
        <>
          {metadataChanged.length > 0 && !apply.isSuccess && (
            <AriaButton
              variant="primary"
              isPending={apply.isPending}
              onPress={() => apply.mutate()}
            >
              {t('admin.metadata.apply.run')}
            </AriaButton>
          )}
          <AriaButton variant="ghost" onPress={onDismiss}>
            {t('admin.libraries.saved.dismiss')}
          </AriaButton>
        </>
      }
    >
      {lines.length > 0 && (
        <ul>
          {lines.map((line) => (
            <li key={line}>{line}</li>
          ))}
        </ul>
      )}
      {apply.isSuccess && (
        <p>
          {t('admin.metadata.apply.queued', { id: apply.data.jobId, count: apply.data.titles })}
        </p>
      )}
      {apply.isError && <p>{problemMessage(t, apply.error)}</p>}
    </Notice>
  );
}
