import { useEffect, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { useRadio } from '@/app/radio/useRadio';
import { languageName } from '@/features/catalog/labels';
import { autoplay } from '@/features/catalog/TrailerPanel';
import type { ExternalTitle } from '@/features/wishes/queries';
import { TrailerDialog } from '@/ui/aria';

interface TitleTrailerProps {
  title: string;
  trailers: ExternalTitle['trailers'];
  onClose: () => void;
}

/**
 * The trailer dialog of a preview page (concept 25, 5.1): the trailers TMDB named, by language,
 * in YouTube's player at the server's address, started like the trailer dialog of a library title
 * (TrailerPanel, `autoplay`). Loaded on demand with react-aria; the radio pauses while it is open.
 */
export function TitleTrailer({ title, trailers, onClose }: TitleTrailerProps) {
  const { t } = useTranslation();
  const { hold } = useRadio();
  useEffect(() => hold('trailer'), [hold]);
  const languages = [...new Set(trailers.map((trailer) => trailer.lang ?? ''))];
  const [chosen, setChosen] = useState<string | null>(null);
  const language = chosen ?? languages[0] ?? '';
  const trailer = trailers.find((entry) => (entry.lang ?? '') === language);
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
      {trailer ? null : t('trailer.none')}
    </TrailerDialog>
  );
}
