import type { SetupFinished, SetupStep } from '@slouchcat/api-client';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { useNavigate } from '@tanstack/react-router';
import { useEffect, useRef, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { api, request } from '@/api/client';
import { problemMessage } from '@/api/errors';
import { paths } from '@/app/paths';
import { refreshSession } from '@/app/session';
import { FactList, Notice, StatusDot, TextLink } from '@/ui';
import { AriaButton } from '@/ui/aria';
import {
  librariesQuery,
  metadataSettingsQuery,
  settingsQuery,
  systemQuery,
} from '../admin/queries';
import { RestartPanel } from '../admin/RestartPanel';
import { manageProfilesQuery } from '../profiles/queries';
import { PROVIDERS, providerSource } from '../admin/integrations/ProviderCard';
import { setupPath, setupProgressQuery, stepState } from './setupState';
import { SetupLayout } from './SetupLayout';
import styles from './SetupLayout.module.css';

/** The steps the summary lists, each with «Ändern». */
const LISTED: readonly SetupStep[] = [
  'region',
  'services',
  'playback',
  'libraries',
  'household',
  'access',
];

/** How often the finish page asks for the scans (O-13). */
const SCAN_POLL_MS = 2000;

/** What each step chose, in one line; undefined while its data loads. */
function useValues(): Partial<Record<SetupStep, string>> {
  const { t } = useTranslation();
  const metadata = useQuery(metadataSettingsQuery).data;
  const settings = useQuery(settingsQuery).data;
  const libraries = useQuery(librariesQuery).data;
  const profiles = useQuery(manageProfilesQuery).data;
  const progress = useQuery(setupProgressQuery).data;
  const sep = t('format.separator');
  const values: Partial<Record<SetupStep, string>> = {};
  if (metadata) {
    values.region = t('setup.summary.region', {
      country: metadata.ratingCountry,
      languages: metadata.textLangs.join(sep),
    });
  }
  if (settings) {
    const set = PROVIDERS.filter((provider) => providerSource(provider, settings.keys) !== 'none');
    values.services = t('setup.summary.services', {
      count: set.length + (settings.email.source !== 'none' ? 1 : 0),
    });
    values.playback = t('setup.summary.playback', {
      hw:
        settings.transcodeHw === 'auto'
          ? t('system.gpu.choice.auto')
          : settings.transcodeHw === 'software'
            ? t('system.gpu.choice.software')
            : t(`system.gpu.backend.${settings.transcodeHw}`),
      trickplay: t(progress?.trickplayDefault ? 'setup.summary.on' : 'setup.summary.off'),
    });
    values.access = settings.publicUrl ?? t('setup.summary.noAddress');
  }
  if (libraries) values.libraries = libraries.map((library) => library.name).join(sep);
  if (profiles) values.household = t('setup.summary.profiles', { count: profiles.length });
  return values;
}

/**
 * The finish page (O-13): «Slouchcat startet neu …» first when the transcode choice needs it,
 * then the first scan of every library with its progress, and «Zur Startseite».
 */
function FinishPage({ finished, finishedAt }: { finished: SetupFinished; finishedAt: number }) {
  const { t } = useTranslation();
  const queryClient = useQueryClient();
  const navigate = useNavigate();
  const heading = useRef<HTMLHeadingElement>(null);
  const [restartAt, setRestartAt] = useState<number | null>(
    finished.restart === 'scheduled' ? finishedAt : null,
  );
  const libraries = useQuery({
    ...librariesQuery,
    enabled: restartAt === null,
    refetchInterval: SCAN_POLL_MS,
  });
  useEffect(() => {
    if (restartAt === null) heading.current?.focus();
  }, [restartAt]);

  if (restartAt !== null) {
    return (
      <RestartPanel
        acceptedAt={restartAt}
        title={t('setup.finish.restartTitle')}
        body={t('setup.finish.restartBody')}
        onBack={() => {
          void queryClient.invalidateQueries({ queryKey: systemQuery.queryKey });
          setRestartAt(null);
        }}
      />
    );
  }
  return (
    <section className={styles.section} aria-labelledby="setup-finish">
      <h2 id="setup-finish" ref={heading} tabIndex={-1} className={styles.sectionTitle}>
        {t('setup.finish.title')}
      </h2>
      {finished.restart === 'busy' && (
        <Notice tone="warning" live={false}>
          {t('setup.finish.busy')}
        </Notice>
      )}
      <FactList
        facts={(libraries.data ?? []).map((library) => {
          const { scan } = library;
          const state = t(`admin.libraries.state.${scan.state}`);
          const progress =
            scan.state === 'running' && scan.filesTotal
              ? t('setup.finish.scanProgress', {
                  state,
                  seen: scan.filesSeen ?? 0,
                  total: scan.filesTotal,
                })
              : state;
          return [
            library.name,
            <span key={library.id} className={styles.row}>
              <StatusDot
                state={
                  scan.state === 'done'
                    ? 'success'
                    : scan.state === 'failed' || scan.state === 'aborted'
                      ? 'danger'
                      : 'active'
                }
              />
              {progress}
            </span>,
          ] as const;
        })}
      />
      <div className={styles.row}>
        <AriaButton
          variant="primary"
          onPress={() => void refreshSession(queryClient).then(() => navigate({ to: paths.home }))}
        >
          {t('setup.finish.home')}
        </AriaButton>
      </div>
    </section>
  );
}

/**
 * Step 9 (concept 22, 3.3): every step with what it chose and «Ändern», then «Einrichtung
 * abschliessen» (finishSetup), which needs a library and restarts the server when the transcode
 * choice differs from the running one; the finish page follows in place.
 */
export function SummaryStep() {
  const { t } = useTranslation();
  const queryClient = useQueryClient();
  const progress = useQuery(setupProgressQuery).data;
  const values = useValues();
  // When the server accepted the finish (ms): the restart panel waits from then on.
  const [finishedAt, setFinishedAt] = useState(0);
  const finish = useMutation({
    mutationFn: (): Promise<SetupFinished> => request(api.POST('/admin/setup/finish')),
    onSuccess: () => {
      setFinishedAt(Date.now());
      void queryClient.invalidateQueries({ queryKey: setupProgressQuery.queryKey });
    },
  });

  if (finish.data) {
    return (
      <SetupLayout step="summary">
        <FinishPage finished={finish.data} finishedAt={finishedAt} />
      </SetupLayout>
    );
  }
  return (
    <SetupLayout
      step="summary"
      intro={t('setup.steps.summary.intro')}
      footer={{
        onNext: () => finish.mutate(),
        nextLabel: finish.isPending ? t('setup.actions.finishing') : t('setup.actions.finish'),
        isPending: finish.isPending,
      }}
      error={finish.error ? problemMessage(t, finish.error) : undefined}
    >
      <FactList
        facts={LISTED.map((step) => {
          const state = stepState(progress, step);
          return [
            t(`setup.steps.${step}.nav`),
            <span key={step} className={styles.row}>
              <span>
                {t(state === 'done' ? 'setup.summary.done' : 'setup.summary.recommended')}
                {values[step] ? `${t('format.separator')}${values[step]}` : ''}
              </span>
              <TextLink href={setupPath(step)}>
                {t('setup.summary.change', { step: t(`setup.steps.${step}.nav`) })}
              </TextLink>
            </span>,
          ] as const;
        })}
      />
    </SetupLayout>
  );
}
