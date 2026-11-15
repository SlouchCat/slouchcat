import type { Language, MetadataSettings, MetadataSettingsUpdate } from '@slouchcat/api-client';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { useNavigate } from '@tanstack/react-router';
import { useMemo, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { problemMessage } from '@/api/errors';
import { sessionQuery, useActiveSession } from '@/app/session';
import { intlLocale } from '@/i18n';
import { FactList, Notice, Skeleton, Text } from '@/ui';
import { CodeBlock, Switch } from '@/ui/aria';
import { LanguageRanks } from '../admin/MetadataPage';
import {
  applyRadioOfferNow,
  metadataSettingsQuery,
  radioOfferQuery,
  systemQuery,
  updateMetadataSettings,
} from '../admin/queries';
import { ratingCountryOptions, ratingStepsText } from '../shared/choices';
import { setupPath, setupProgressQuery, stepState, updateSetupProgress } from './setupState';
import { SetupLayout } from './SetupLayout';
import styles from './SetupLayout.module.css';

/**
 * The countries the step offers before the radio catalog knows any: those of the rating systems
 * and of the languages of concept 22 (table 4.1). Codes are data; their names come from Intl.
 */
const COUNTRIES = [
  'CH',
  'DE',
  'AT',
  'LI',
  'LU',
  'BE',
  'FR',
  'IT',
  'ES',
  'PT',
  'NL',
  'GB',
  'IE',
  'US',
  'CA',
  'BR',
  'PL',
  'CZ',
  'HU',
  'SE',
  'DK',
  'NO',
  'FI',
  'TR',
  'UA',
  'JP',
  'KR',
  'TW',
  'RU',
  'CN',
];

/**
 * The country and metadata languages a language suggests (table 4.1, O-15): the language, then
 * English. The rating country is that country when the server's table knows it, else Germany
 * (F-12; China has no system).
 */
const SUGGESTIONS: Record<Language, { country: string; langs: string[] }> = {
  de: { country: 'CH', langs: ['de', 'en'] },
  'de-DE': { country: 'DE', langs: ['de', 'en'] },
  en: { country: 'US', langs: ['en'] },
  fr: { country: 'FR', langs: ['fr', 'en'] },
  es: { country: 'ES', langs: ['es', 'en'] },
  it: { country: 'IT', langs: ['it', 'en'] },
  nl: { country: 'NL', langs: ['nl', 'en'] },
  'pt-BR': { country: 'BR', langs: ['pt-BR', 'en'] },
  'pt-PT': { country: 'PT', langs: ['pt-PT', 'en'] },
  pl: { country: 'PL', langs: ['pl', 'en'] },
  sv: { country: 'SE', langs: ['sv', 'en'] },
  da: { country: 'DK', langs: ['da', 'en'] },
  // TMDB keeps Norwegian under no (ADR 0033), like model.MetadataBase
  nb: { country: 'NO', langs: ['no', 'en'] },
  fi: { country: 'FI', langs: ['fi', 'en'] },
  cs: { country: 'CZ', langs: ['cs', 'en'] },
  hu: { country: 'HU', langs: ['hu', 'en'] },
  tr: { country: 'TR', langs: ['tr', 'en'] },
  ru: { country: 'RU', langs: ['ru', 'en'] },
  uk: { country: 'UA', langs: ['uk', 'en'] },
  ja: { country: 'JP', langs: ['ja', 'en'] },
  ko: { country: 'KR', langs: ['ko', 'en'] },
  'zh-CN': { country: 'CN', langs: ['zh-CN', 'en'] },
  'zh-TW': { country: 'TW', langs: ['zh-TW', 'en'] },
};

/** The update of the metadata settings with the step's choices; image languages follow the text. */
export function regionUpdate(
  settings: MetadataSettings,
  ratingCountry: string,
  textLangs: string[],
): MetadataSettingsUpdate {
  // The languages follow the text list; an image without text keeps its place, first or last.
  const follow = (chain: string[]) =>
    chain[0] === '' ? ['', ...textLangs] : chain.includes('') ? [...textLangs, ''] : textLangs;
  return {
    textLangs,
    imageLangs: {
      poster: follow(settings.imageLangs.poster),
      backdrop: follow(settings.imageLangs.backdrop),
      logo: follow(settings.imageLangs.logo),
    },
    imageKinds: settings.imageKinds,
    imageMinWidth: settings.imageMinWidth,
    imageByVote: settings.imageByVote,
    localImages: settings.localImages,
    ratingCountry,
    tmdbIncludeAdult: settings.tmdbIncludeAdult,
    fanartEnabled: settings.fanartEnabled,
  };
}

function RegionForm({ settings }: { settings: MetadataSettings }) {
  const { t, i18n } = useTranslation();
  const queryClient = useQueryClient();
  const navigate = useNavigate();
  const session = useActiveSession();
  const progress = useQuery(setupProgressQuery).data;
  const system = useQuery(systemQuery).data;
  const offer = useQuery(radioOfferQuery).data;
  const suggestion = SUGGESTIONS[session.profile.language];
  // A step done before shows what was stored; an open one the suggestion of the language.
  const fresh = stepState(progress, 'region') === 'open';
  const [country, setCountry] = useState(suggestion.country);
  const isRatingCountry = (code: string) =>
    settings.ratingSystems.some((rs) => rs.country === code);
  const [ratingCountry, setRatingCountry] = useState<string>(
    fresh
      ? isRatingCountry(suggestion.country)
        ? suggestion.country
        : 'DE'
      : settings.ratingCountry,
  );
  const ratingSystem = settings.ratingSystems.find((rs) => rs.country === ratingCountry);
  const [langs, setLangs] = useState<string[]>(fresh ? suggestion.langs : settings.textLangs);
  const [radio, setRadio] = useState(true);
  const regions = useMemo(
    () => new Intl.DisplayNames([intlLocale(i18n.language)], { type: 'region' }),
    [i18n.language],
  );
  const nameOf = (code: string) => regions.of(code) ?? code;
  const countries = useMemo(() => {
    const codes = new Set([...COUNTRIES, ...(offer?.countries.map((entry) => entry.code) ?? [])]);
    return [...codes]
      .map((code) => ({ value: code, label: regions.of(code) ?? code }))
      .sort((a, b) => a.label.localeCompare(b.label, intlLocale(i18n.language)));
  }, [offer, regions, i18n.language]);
  const browserZone = Intl.DateTimeFormat().resolvedOptions().timeZone;
  const zoneDiffers = system !== undefined && browserZone !== '' && system.timeZone !== browserZone;

  const save = useMutation({
    mutationFn: async () => {
      const stored = await updateMetadataSettings(regionUpdate(settings, ratingCountry, langs));
      queryClient.setQueryData(metadataSettingsQuery.queryKey, stored);
      // The household step offers the steps of the new rating country (SessionInfo.ratingSystem).
      await queryClient.invalidateQueries({ queryKey: sessionQuery.queryKey });
      // F-8: the country without a station («Nur markierte»); one already chosen stays as it is.
      if (radio && !offer?.countries.some((entry) => entry.code === country && entry.mode)) {
        await applyRadioOfferNow([
          { op: 'chooseCountry', country },
          { op: 'demarkAll', country },
        ]);
        await queryClient.invalidateQueries({ queryKey: radioOfferQuery.queryKey });
      }
      queryClient.setQueryData(
        setupProgressQuery.queryKey,
        await updateSetupProgress({ step: 'region', state: 'done' }),
      );
    },
    onSuccess: () => navigate({ to: setupPath('services') }),
  });
  const skip = useMutation({
    mutationFn: async () =>
      queryClient.setQueryData(
        setupProgressQuery.queryKey,
        await updateSetupProgress({ step: 'region', state: 'skipped' }),
      ),
    onSuccess: () => navigate({ to: setupPath('services') }),
  });
  const failed = save.error ?? skip.error;

  return (
    <SetupLayout
      step="region"
      intro={t('setup.steps.region.intro')}
      footer={{
        onNext: () => save.mutate(),
        isPending: save.isPending,
        onSkip: () => skip.mutate(),
      }}
      error={failed ? problemMessage(t, failed) : undefined}
    >
      <label className={styles.stack}>
        <Text as="span" variant="caption" tone="secondary">
          {t('setup.region.country')}
        </Text>
        <select
          className={styles.select}
          value={country}
          onChange={(event) => setCountry(event.target.value)}
        >
          {countries.map((option) => (
            <option key={option.value} value={option.value}>
              {option.label}
            </option>
          ))}
        </select>
      </label>
      <label className={styles.stack}>
        <Text as="span" variant="caption" tone="secondary">
          {t('setup.region.ratingCountry')}
        </Text>
        <select
          className={styles.select}
          value={ratingCountry}
          onChange={(event) => setRatingCountry(event.target.value)}
        >
          {ratingCountryOptions(t, intlLocale(i18n.language), settings.ratingSystems).map(
            (option) => (
              <option key={option.value} value={option.value}>
                {option.label}
              </option>
            ),
          )}
        </select>
      </label>
      {ratingSystem && (
        <Text tone="secondary">
          {t('setup.region.ratingSteps', { steps: ratingStepsText(t, ratingSystem) })}
        </Text>
      )}
      <section className={styles.section} aria-labelledby="setup-metadata-languages">
        <h2 id="setup-metadata-languages" className={styles.sectionTitle}>
          {t('setup.region.languages')}
        </h2>
        <Text tone="secondary">{t('setup.region.languagesIntro')}</Text>
        <LanguageRanks value={langs} onChange={setLangs} known={settings.knownLanguages} />
      </section>
      <section className={styles.section} aria-labelledby="setup-time-zone">
        <h2 id="setup-time-zone" className={styles.sectionTitle}>
          {t('setup.region.timeZone')}
        </h2>
        {system ? (
          <FactList facts={[[t('setup.region.serverZone'), system.timeZone]]} />
        ) : (
          <Skeleton shape="line" />
        )}
        {zoneDiffers && (
          <>
            <Notice tone="warning" live={false} title={t('setup.region.zoneDiffers.title')}>
              {t('setup.region.zoneDiffers.body', { browser: browserZone })}
            </Notice>
            <CodeBlock
              code={`TZ=${browserZone}`}
              label={t('setup.region.zoneDiffers.command')}
              copyLabel={t('system.copy.action')}
              copiedMessage={t('system.copy.done')}
              copyFailedMessage={t('system.copy.failed')}
            />
          </>
        )}
      </section>
      <section className={styles.section} aria-labelledby="setup-radio">
        <h2 id="setup-radio" className={styles.sectionTitle}>
          {t('setup.region.radio')}
        </h2>
        <Switch isSelected={radio} onChange={setRadio}>
          {t('setup.region.radioToggle', { country: nameOf(country) })}
        </Switch>
        <Text tone="secondary">{t('setup.region.radioHint')}</Text>
      </section>
    </SetupLayout>
  );
}

/** Step 3 (concept 22, 3.3): country, rating country, metadata languages, time zone, radio. */
export function RegionStep() {
  const { t } = useTranslation();
  const settings = useQuery(metadataSettingsQuery);
  if (settings.data) return <RegionForm settings={settings.data} />;
  return (
    <SetupLayout step="region" intro={t('setup.steps.region.intro')}>
      {settings.isError ? (
        <Notice tone="danger">{problemMessage(t, settings.error)}</Notice>
      ) : (
        <Skeleton shape="block" />
      )}
    </SetupLayout>
  );
}
