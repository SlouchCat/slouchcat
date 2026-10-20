import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { useState, type FormEvent } from 'react';
import { Form } from 'react-aria-components';
import { useTranslation } from 'react-i18next';
import { api, request } from '@/api/client';
import { formErrors, problemMessage } from '@/api/errors';
import { Notice, Skeleton, Text } from '@/ui';
import { AriaButton, CheckboxGroup, RadioGroup, TextField } from '@/ui/aria';
import { kickOfflineEngine } from './engine';
import { qualityLabel } from './labels';
import { downloadRulesQuery } from './queries';
import { isIos } from './support';
import type { DownloadQuality, DownloadRules } from './types';
import styles from './OfflinePages.module.css';

const QUALITIES: DownloadQuality[] = ['high', 'medium', 'low'];
const MAX_COUNT = { min: 1, max: 20 };

type Flag = 'enabled' | 'nextEpisodes' | 'wifiOnly';

function flagsOf(rules: DownloadRules): Flag[] {
  const flags: Flag[] = [];
  if (rules.enabled) flags.push('enabled');
  if (rules.nextEpisodes) flags.push('nextEpisodes');
  if (rules.wifiOnly) flags.push('wifiOnly');
  return flags;
}

function RulesFields({ initial }: { initial: DownloadRules }) {
  const { t } = useTranslation();
  const queryClient = useQueryClient();
  // iOS cannot tell Wi-Fi from cellular: the option is hidden there (AN-13), its value kept.
  const [ios] = useState(() => isIos());
  const [flags, setFlags] = useState<string[]>(() => flagsOf(initial));
  const [maxCount, setMaxCount] = useState(String(initial.maxCount));
  const [quality, setQuality] = useState<string>(initial.quality);
  const save = useMutation({
    mutationFn: (body: DownloadRules): Promise<DownloadRules> =>
      request(api.PUT('/offline/rules', { body })),
    onSuccess: (rules) => {
      queryClient.setQueryData(downloadRulesQuery.queryKey, rules);
      // The next sync applies the rule (O-3).
      kickOfflineEngine();
    },
  });
  const errors = save.error ? formErrors(t, save.error) : null;

  const onSubmit = (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    const count = Number(maxCount);
    if (!Number.isInteger(count) || count < MAX_COUNT.min || count > MAX_COUNT.max) return;
    save.mutate({
      enabled: flags.includes('enabled'),
      nextEpisodes: flags.includes('nextEpisodes'),
      // Podcasts come with M6; the stored value goes back unchanged.
      newPodcastEpisodes: initial.newPodcastEpisodes,
      wifiOnly: ios ? initial.wifiOnly : flags.includes('wifiOnly'),
      maxCount: count,
      quality: quality as DownloadQuality,
    });
  };

  const options = [
    { value: 'enabled', label: t('offline.rules.enabled') },
    { value: 'nextEpisodes', label: t('offline.rules.nextEpisodes') },
    ...(ios ? [] : [{ value: 'wifiOnly', label: t('offline.rules.wifiOnly') }]),
  ];

  return (
    <Form className={styles.form} onSubmit={onSubmit} validationErrors={errors?.fields}>
      {errors?.form && <Notice tone="danger">{errors.form}</Notice>}
      {save.isSuccess && !save.isPending && <Notice>{t('offline.rules.saved')}</Notice>}
      <CheckboxGroup
        name="flags"
        label={t('offline.rules.options')}
        value={flags}
        onChange={(next) => {
          save.reset();
          setFlags(next);
        }}
        options={options}
        description={ios ? t('offline.rules.wifiIos') : undefined}
      />
      <TextField
        name="maxCount"
        label={t('offline.rules.maxCount')}
        description={t('offline.rules.maxCountHint', MAX_COUNT)}
        inputMode="numeric"
        isRequired
        value={maxCount}
        onChange={(next) => {
          save.reset();
          setMaxCount(next.replace(/\D/g, '').slice(0, 2));
        }}
        validate={(value) => {
          const count = Number(value);
          return Number.isInteger(count) && count >= MAX_COUNT.min && count <= MAX_COUNT.max
            ? null
            : t('offline.rules.maxCountInvalid', MAX_COUNT);
        }}
      />
      <RadioGroup
        name="quality"
        label={t('offline.rules.quality')}
        value={quality}
        onChange={(next) => {
          save.reset();
          setQuality(next);
        }}
        options={QUALITIES.map((value) => ({ value, label: qualityLabel(t, value) }))}
      />
      <div className={styles.formActions}>
        <AriaButton type="submit" variant="primary" isPending={save.isPending}>
          {save.isPending ? t('common.saving') : t('offline.rules.save')}
        </AriaButton>
      </div>
    </Form>
  );
}

/** Auto-downloads of the active profile on this device (O-3, AN-13). */
export function DownloadRulesForm({ profileName }: { profileName: string }) {
  const { t } = useTranslation();
  const rules = useQuery(downloadRulesQuery);
  return (
    <section className={styles.section} aria-labelledby="rules-heading">
      <h2 id="rules-heading" className={styles.sectionTitle}>
        {t('offline.rules.title')}
      </h2>
      <Text tone="secondary">{t('offline.rules.intro', { name: profileName })}</Text>
      {rules.isPending && <Skeleton shape="block" />}
      {rules.isError && (
        <Notice
          tone="danger"
          title={problemMessage(t, rules.error)}
          actions={
            <AriaButton icon="refresh" onPress={() => void rules.refetch()}>
              {t('common.retry')}
            </AriaButton>
          }
        />
      )}
      {/* The form starts from the first answer; a later refetch must not undo what is typed. */}
      {rules.data && <RulesFields initial={rules.data} />}
    </section>
  );
}
