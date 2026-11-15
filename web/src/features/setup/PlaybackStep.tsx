import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { useNavigate } from '@tanstack/react-router';
import { useState } from 'react';
import { useTranslation } from 'react-i18next';
import { problemMessage } from '@/api/errors';
import { Notice, Skeleton, Text } from '@/ui';
import { Switch } from '@/ui/aria';
import { settingsQuery, systemQuery } from '../admin/queries';
import { HardwareChoice } from '../shared/HardwareChoice';
import { SystemState } from '../shared/SystemState';
import { setupPath, setupProgressQuery, updateSetupProgress } from './setupState';
import { SetupLayout } from './SetupLayout';
import styles from './SetupLayout.module.css';

/**
 * Step 5 (concept 22, 3.3, F-10): what the server found (graphics chip, HDR to SDR, inotify
 * limit), the choice of the transcode backend from the probes at start, which applies after the
 * restart at «Einrichtung abschliessen», and the default of «Vorschau beim Spulen» for the new
 * video libraries of the next step, kept by the server (setup_wizard) through a sign-out.
 */
export function PlaybackStep() {
  const { t } = useTranslation();
  const queryClient = useQueryClient();
  const navigate = useNavigate();
  const system = useQuery(systemQuery);
  const settings = useQuery(settingsQuery);
  const progress = useQuery(setupProgressQuery).data;
  const [trickplay, setTrickplay] = useState<boolean | null>(null);
  const trickplayOn = trickplay ?? progress?.trickplayDefault ?? false;
  const advance = useMutation({
    mutationFn: async (state: 'done' | 'skipped') =>
      queryClient.setQueryData(
        setupProgressQuery.queryKey,
        await updateSetupProgress(
          state === 'done'
            ? { step: 'playback', state, trickplayDefault: trickplayOn }
            : { step: 'playback', state },
        ),
      ),
    onSuccess: () => navigate({ to: setupPath('libraries') }),
  });
  const failed = system.error ?? settings.error;

  return (
    <SetupLayout
      step="playback"
      intro={t('setup.steps.playback.intro')}
      footer={{
        onNext: () => advance.mutate('done'),
        isPending: advance.isPending,
        onSkip: () => advance.mutate('skipped'),
      }}
      error={advance.error ? problemMessage(t, advance.error) : undefined}
    >
      {failed ? (
        <Notice tone="danger">{problemMessage(t, failed)}</Notice>
      ) : system.data && settings.data ? (
        <>
          <SystemState system={system.data} />
          <HardwareChoice
            system={system.data}
            settings={settings.data}
            restartHint={t('setup.playback.restart')}
          />
        </>
      ) : (
        <Skeleton shape="block" />
      )}
      <section className={styles.section} aria-labelledby="setup-trickplay">
        <h2 id="setup-trickplay" className={styles.sectionTitle}>
          {t('admin.libraries.trickplay.action')}
        </h2>
        <Switch isSelected={trickplayOn} onChange={setTrickplay}>
          {t('setup.playback.trickplay')}
        </Switch>
        <Text tone="secondary">{t('setup.playback.trickplayHint')}</Text>
      </section>
    </SetupLayout>
  );
}
