import type { Language } from '@slouchcat/api-client';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { useNavigate } from '@tanstack/react-router';
import { useState } from 'react';
import { useTranslation } from 'react-i18next';
import { api, request } from '@/api/client';
import { problemMessage } from '@/api/errors';
import { refreshSession, sessionQuery } from '@/app/session';
import { currentLanguage, intlLocale, languages, setLanguage } from '@/i18n';
import { FactList } from '@/ui';
import { RadioGroup, TextField } from '@/ui/aria';
import { setupPath } from './setupState';
import { SetupLayout } from './SetupLayout';
import styles from './SetupLayout.module.css';

/** A fixed moment and number for the preview, so the step looks the same every time. */
const PREVIEW_DATE = new Date(Date.UTC(2026, 10, 14, 19, 30));
const PREVIEW_NUMBER = 1234567.5;

/**
 * Step 1 (concept 22, 3.3): every shipped language in its own language, with the English name and
 * the code below; the search finds both. The detected language is chosen; a choice switches the
 * whole interface at once, and the preview shows a date and a number in it. Before the account
 * the choice stays in the browser and goes with the account (step 2); after it «Weiter» stores it
 * as the language of the owner's profile.
 */
export function LanguageStep() {
  const { t } = useTranslation();
  const queryClient = useQueryClient();
  const navigate = useNavigate();
  const session = useQuery(sessionQuery).data;
  const [query, setQuery] = useState('');
  const [chosen, setChosen] = useState<Language>(currentLanguage());
  const needle = query.trim().toLocaleLowerCase();
  const shown = languages.filter(
    (language) =>
      !needle ||
      [language.name, language.englishName, language.code].some((text) =>
        text.toLocaleLowerCase().includes(needle),
      ),
  );
  const locale = intlLocale(chosen);

  const save = useMutation({
    mutationFn: async () => {
      const profile = session?.profile;
      if (profile && profile.language !== chosen) {
        await request(
          api.PATCH('/profiles/{id}', {
            params: { path: { id: profile.id } },
            body: { language: chosen },
          }),
        );
        await refreshSession(queryClient);
      }
    },
    onSuccess: () => navigate({ to: setupPath(session ? 'region' : 'account') }),
  });

  const choose = (code: string) => {
    const language = languages.find((entry) => entry.code === code);
    if (!language) return;
    setChosen(language.code);
    void setLanguage(language.code);
  };

  return (
    <SetupLayout
      step="language"
      intro={t('setup.steps.language.intro')}
      footer={{ onNext: () => save.mutate(), isPending: save.isPending }}
      error={save.error ? problemMessage(t, save.error) : undefined}
    >
      <TextField
        type="search"
        label={t('setup.language.search')}
        value={query}
        onChange={setQuery}
        autoComplete="off"
      />
      <RadioGroup
        label={t('setup.language.choice')}
        orientation="vertical"
        value={chosen}
        onChange={choose}
        options={shown.map((language) => ({
          value: language.code,
          label: t('setup.language.option', {
            name: language.name,
            english: language.englishName,
            code: language.code,
          }),
        }))}
      />
      {shown.length === 0 && <p className={styles.intro}>{t('setup.language.none')}</p>}
      <section className={styles.section} aria-label={t('setup.language.preview')}>
        <FactList
          facts={[
            [
              t('setup.language.previewDate'),
              new Intl.DateTimeFormat(locale, { dateStyle: 'full', timeStyle: 'short' }).format(
                PREVIEW_DATE,
              ),
            ],
            [
              t('setup.language.previewNumber'),
              new Intl.NumberFormat(locale).format(PREVIEW_NUMBER),
            ],
          ]}
        />
      </section>
    </SetupLayout>
  );
}
