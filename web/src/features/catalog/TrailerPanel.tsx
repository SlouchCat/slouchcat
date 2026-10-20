import { useQuery } from '@tanstack/react-query';
import { useState } from 'react';
import { useTranslation } from 'react-i18next';
import { problemMessage } from '@/api/errors';
import { TrailerDialog } from '@/ui/aria';
import { languageName } from './labels';
import { trailersQuery } from './queries';

/** Adds autoplay to the embed URL: the dialog opens because the viewer asked for the trailer. */
function autoplay(embedUrl: string): string {
  try {
    const url = new URL(embedUrl);
    url.searchParams.set('autoplay', '1');
    return url.href;
  } catch {
    return embedUrl;
  }
}

interface TrailerPanelProps {
  itemId: string;
  title: string;
  onClose: () => void;
}

/**
 * The trailer dialog of a detail page (W-10), loaded on demand with react-aria: the languages the
 * server returned (profile language first), the trailer of the chosen one in YouTube's player.
 */
export function TrailerPanel({ itemId, title, onClose }: TrailerPanelProps) {
  const { t } = useTranslation();
  const trailers = useQuery(trailersQuery(itemId));
  const list = trailers.data ?? [];
  const languages = [...new Set(list.map((trailer) => trailer.lang))];
  const [chosen, setChosen] = useState<string | null>(null);
  const language = chosen ?? languages[0] ?? '';
  const trailer = list.find((entry) => entry.lang === language);

  let status: string = t('common.loading');
  if (trailers.isError) status = problemMessage(t, trailers.error);
  else if (trailers.isSuccess && !trailer) status = t('trailer.none');

  return (
    <TrailerDialog
      isOpen
      onOpenChange={(open) => {
        if (!open) onClose();
      }}
      title={t('trailer.title', { title })}
      closeLabel={t('common.close')}
      languageLabel={t('trailer.language')}
      languages={languages.map((code) => ({ value: code, label: languageName(t, code) }))}
      language={language}
      onLanguageChange={setChosen}
      embedUrl={trailer ? autoplay(trailer.embedUrl) : undefined}
      frameTitle={t('trailer.frame', { title })}
      note={t('trailer.privacy')}
    >
      {status}
    </TrailerDialog>
  );
}
