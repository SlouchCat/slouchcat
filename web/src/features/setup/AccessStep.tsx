import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { useNavigate } from '@tanstack/react-router';
import { useCallback } from 'react';
import { useTranslation } from 'react-i18next';
import { problemMessage } from '@/api/errors';
import { Notice, Skeleton, Text } from '@/ui';
import { AriaButton, useDrafts } from '@/ui/aria';
import { ServerBlock } from '../admin/integrations/ServerBlock';
import { PUBLIC_URL_DRAFT } from '../admin/integrations/shared';
import { settingsQuery } from '../admin/queries';
import { setupPath, setupProgressQuery, updateSetupProgress } from './setupState';
import { SetupLayout } from './SetupLayout';
import styles from './SetupLayout.module.css';

/**
 * Step 8 (concept 22, 3.3): the public address for invitations, e-mails and push, with the
 * address this browser uses now as the suggestion, and the daily update check (on by default; no
 * usage data leaves the server). The same rows as Verwaltung › Integrationen › Server.
 */
export function AccessStep() {
  const { t } = useTranslation();
  const queryClient = useQueryClient();
  const navigate = useNavigate();
  const settings = useQuery(settingsQuery);
  const data = settings.data;
  const fields = useDrafts<string>(
    useCallback(
      (id: string) => (data ? (id === PUBLIC_URL_DRAFT ? (data.publicUrl ?? '') : '') : undefined),
      [data],
    ),
  );
  const here = window.location.origin;
  const advance = useMutation({
    mutationFn: async (state: 'done' | 'skipped') =>
      queryClient.setQueryData(
        setupProgressQuery.queryKey,
        await updateSetupProgress({ step: 'access', state }),
      ),
    onSuccess: () => navigate({ to: setupPath('summary') }),
  });
  return (
    <SetupLayout
      step="access"
      intro={t('setup.steps.access.intro')}
      footer={{
        onNext: () => advance.mutate('done'),
        isPending: advance.isPending,
        onSkip: () => advance.mutate('skipped'),
      }}
      error={advance.error ? problemMessage(t, advance.error) : undefined}
    >
      {settings.isError && <Notice tone="danger">{problemMessage(t, settings.error)}</Notice>}
      {!data && !settings.isError && <Skeleton shape="block" />}
      {data && (
        <>
          {data.publicUrlSource !== 'env' && !data.publicUrl && (
            <div className={styles.row}>
              <Text tone="secondary">{t('setup.access.here', { url: here })}</Text>
              <AriaButton variant="secondary" onPress={() => fields.set(PUBLIC_URL_DRAFT, here)}>
                {t('setup.access.useHere')}
              </AriaButton>
            </div>
          )}
          <ServerBlock settings={data} fields={fields} />
          <Text tone="secondary">{t('setup.access.noUsageData')}</Text>
        </>
      )}
    </SetupLayout>
  );
}
