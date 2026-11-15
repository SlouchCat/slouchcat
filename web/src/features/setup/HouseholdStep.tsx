import { useMutation, useQueryClient } from '@tanstack/react-query';
import { useNavigate } from '@tanstack/react-router';
import { useRef } from 'react';
import { useTranslation } from 'react-i18next';
import { problemMessage } from '@/api/errors';
import { Text } from '@/ui';
import { ProfileList } from '../profiles/ProfileList';
import { setupPath, setupProgressQuery, updateSetupProgress } from './setupState';
import { SetupLayout } from './SetupLayout';
import styles from './SetupLayout.module.css';

/**
 * Step 7 (concept 22, 3.3, F-9): the profiles of the household, the own one first, as under
 * Einstellungen › Profile: «Profil hinzufügen» with name, age step, colour and an optional PIN. New
 * profiles speak the owner's language; guests with an account of their own come later through
 * Verwaltung › Konten.
 */
export function HouseholdStep() {
  const { t } = useTranslation();
  const queryClient = useQueryClient();
  const navigate = useNavigate();
  const titleRef = useRef<HTMLHeadingElement>(null);
  const advance = useMutation({
    mutationFn: async (state: 'done' | 'skipped') =>
      queryClient.setQueryData(
        setupProgressQuery.queryKey,
        await updateSetupProgress({ step: 'household', state }),
      ),
    onSuccess: () => navigate({ to: setupPath('access') }),
  });
  return (
    <SetupLayout
      step="household"
      intro={t('setup.steps.household.intro')}
      footer={{
        onNext: () => advance.mutate('done'),
        isPending: advance.isPending,
        onSkip: () => advance.mutate('skipped'),
      }}
      error={advance.error ? problemMessage(t, advance.error) : undefined}
    >
      <ProfileList titleRef={titleRef} nameHeadings={false}>
        {({ add, list }) => (
          <div className={styles.stack}>
            <div className={styles.row}>{add}</div>
            {list}
          </div>
        )}
      </ProfileList>
      <Text tone="secondary">{t('setup.household.accounts')}</Text>
    </SetupLayout>
  );
}
