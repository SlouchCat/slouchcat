import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { useTranslation } from 'react-i18next';
import { problemMessage } from '@/api/errors';
import { sessionQuery } from '@/app/session';
import {
  adoptPlaybackPrefs,
  myPlaybackPrefsQuery,
  PLAYBACK_LANGUAGES,
  putMyPlaybackPrefs,
  withFirst,
} from '@/features/profiles/playbackPrefs';
import { AriaButton } from '@/ui/aria';
import { languageName } from '../labels';
import styles from './SubtitleStyleOptions.module.css';

/**
 * «Als Standard für dieses Profil» below the sound panel's list (concept 13 11.4): puts the
 * language of the playing audio track first in the active profile's audio languages
 * (putMyPlaybackPrefs, every profile for itself, F-13). The other preferences stay as the server
 * holds them. A status line says which language is the profile's first, and the button is off
 * while it is the playing one (it stays on after a save, so the focus stays on it); a track without
 * a language the server knows offers nothing. The save reads the server's lists first: a language
 * that is the first there already (another device set it) is not sent again.
 */
export function AudioDefault({ language }: { language: string | undefined }) {
  const { t, i18n } = useTranslation();
  const queryClient = useQueryClient();
  const profile = useQuery(sessionQuery).data?.profile;
  const save = useMutation({
    mutationFn: async (code: string) => {
      const current = await queryClient.fetchQuery({ ...myPlaybackPrefsQuery, staleTime: 0 });
      if (current.audioLanguages[0] === code) return { stored: current, changed: false };
      const stored = await putMyPlaybackPrefs({
        ...current,
        audioLanguages: withFirst(current.audioLanguages, code),
      });
      return { stored, changed: true };
    },
    onSuccess: ({ stored }) => adoptPlaybackPrefs(queryClient, stored),
  });
  if (!profile || !language || !PLAYBACK_LANGUAGES.includes(language)) return null;
  const name = languageName(language, i18n.language) ?? language;
  const isFirst = profile.audioLanguages[0] === language;
  return (
    <div className={styles.footer}>
      <AriaButton
        variant="ghost"
        icon="star"
        block
        isPending={save.isPending}
        isDisabled={isFirst && !save.isSuccess}
        onPress={() => {
          if (!isFirst) save.mutate(language);
        }}
      >
        {t('player.tracks.makeDefault')}
      </AriaButton>
      <p role="status" className={styles.hint}>
        {save.isError
          ? problemMessage(t, save.error)
          : isFirst
            ? t(save.data?.changed ? 'player.tracks.madeDefault' : 'player.tracks.isDefault', {
                language: name,
              })
            : ''}
      </p>
    </div>
  );
}
