import type { SessionInfo, SetupRequest } from '@slouchcat/api-client';
import { useMutation, useQueryClient } from '@tanstack/react-query';
import { useNavigate } from '@tanstack/react-router';
import { useId, useState, type FormEvent, type ReactNode } from 'react';
import { Form } from 'react-aria-components';
import { useTranslation } from 'react-i18next';
import { api, isApiError, request } from '@/api/client';
import { formErrors } from '@/api/errors';
import { applyProfileLanguage, sessionQuery, setupStatusQuery } from '@/app/session';
import { ServerStatus } from '@/features/system/ServerStatus';
import { supportedLanguages, type Language } from '@/i18n';
import { formText } from '@/lib/form';
import { useDocumentTitle } from '@/lib/useDocumentTitle';
import { Logo, Notice, Page, Panel, Stack, Text, TextLink } from '@/ui';
import { AriaButton, RadioGroup, TextField } from '@/ui/aria';
import styles from './SetupPage.module.css';

const TMDB_KEY_URL = 'https://www.themoviedb.org/settings/api';
const MIN_PASSWORD = 10;

function Step({
  step,
  title,
  description,
  children,
}: {
  step: number;
  title: string;
  description: ReactNode;
  children: ReactNode;
}) {
  const { t } = useTranslation();
  const headingId = useId();
  return (
    <Panel as="section" aria-labelledby={headingId}>
      <div className={styles.stepHead}>
        <span className={styles.stepNumber} aria-hidden="true">
          {step}
        </span>
        <div className={styles.stepText}>
          <h2 id={headingId} className={styles.stepTitle}>
            <span className={styles.stepLabel}>{t('setup.step', { step })}</span>
            {title}
          </h2>
          <div className={styles.stepDescription}>{description}</div>
        </div>
      </div>
      {children}
    </Panel>
  );
}

/** First-start wizard (A-1 minimal): main account, first library, optional TMDB key. */
export function SetupPage() {
  const { t, i18n } = useTranslation();
  useDocumentTitle(t('setup.title'));
  const queryClient = useQueryClient();
  const navigate = useNavigate();
  const [password, setPassword] = useState('');
  const [kind, setKind] = useState<'movies' | 'series'>('movies');

  const setup = useMutation({
    mutationFn: (body: SetupRequest): Promise<SessionInfo> => request(api.POST('/setup', { body })),
    onSuccess: async (session) => {
      queryClient.setQueryData(setupStatusQuery.queryKey, { setupRequired: false });
      queryClient.setQueryData(sessionQuery.queryKey, session);
      await applyProfileLanguage(session);
      await navigate({ to: '/' });
    },
    onError: async (error) => {
      if (isApiError(error, 409)) {
        queryClient.setQueryData(setupStatusQuery.queryKey, { setupRequired: false });
        await navigate({ to: '/login' });
      }
    },
  });
  const errors = setup.error ? formErrors(t, setup.error) : null;

  const onSubmit = (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    const data = new FormData(event.currentTarget);
    const field = (name: string) => formText(data, name);
    const language = (supportedLanguages as readonly string[]).includes(i18n.resolvedLanguage ?? '')
      ? (i18n.resolvedLanguage as Language)
      : undefined;
    const tmdbApiKey = field('tmdbApiKey');
    setup.mutate({
      email: field('email'),
      password: formText(data, 'password', false),
      library: { name: field('library.name'), kind, path: field('library.path') },
      ...(tmdbApiKey ? { tmdbApiKey } : {}),
      ...(language ? { language } : {}),
    });
  };

  const required = (value: string) => (value.trim() ? null : t('errors.field.required'));

  return (
    <Page layout="form">
      <Stack gap={8}>
        <Logo name={t('app.name')} />
        <Stack gap={2}>
          <Text as="h1" variant="title">
            {t('setup.title')}
          </Text>
          <Text tone="secondary">{t('setup.intro')}</Text>
        </Stack>

        <Form className={styles.form} onSubmit={onSubmit} validationErrors={errors?.fields}>
          <Step
            step={1}
            title={t('setup.account.title')}
            description={t('setup.account.description')}
          >
            <TextField
              name="email"
              type="email"
              label={t('setup.account.email')}
              autoComplete="email"
              isRequired
              validate={required}
              autoFocus
            />
            <TextField
              name="password"
              type="password"
              label={t('setup.account.password')}
              description={t('setup.account.passwordHint')}
              autoComplete="new-password"
              isRequired
              value={password}
              onChange={setPassword}
              validate={(value) =>
                value.length >= MIN_PASSWORD ? null : t('errors.field.password_too_short')
              }
            />
            <TextField
              name="passwordRepeat"
              type="password"
              label={t('setup.account.passwordRepeat')}
              autoComplete="new-password"
              isRequired
              validate={(value) =>
                value === password ? null : t('errors.field.password_mismatch')
              }
            />
          </Step>

          <Step
            step={2}
            title={t('setup.library.title')}
            description={t('setup.library.description')}
          >
            <TextField
              name="library.name"
              label={t('setup.library.name')}
              defaultValue={t('library.kind.movies')}
              isRequired
              validate={required}
            />
            <RadioGroup
              name="library.kind"
              label={t('setup.library.kind')}
              value={kind}
              onChange={(value) => setKind(value === 'series' ? 'series' : 'movies')}
              options={[
                { value: 'movies', label: t('library.kind.movies') },
                { value: 'series', label: t('library.kind.series') },
              ]}
            />
            <TextField
              name="library.path"
              label={t('setup.library.path')}
              description={t('setup.library.pathHint')}
              autoComplete="off"
              spellCheck="false"
              mono
              isRequired
              validate={(value) => {
                if (!value.trim()) return t('errors.field.required');
                return value.trim().startsWith('/') ? null : t('errors.field.not_absolute');
              }}
            />
          </Step>

          <Step
            step={3}
            title={t('setup.tmdb.title')}
            description={
              <>
                <p>{t('setup.tmdb.description')}</p>
                <p className={styles.link}>
                  <TextLink href={TMDB_KEY_URL} external>
                    {t('setup.tmdb.link')}
                  </TextLink>
                </p>
              </>
            }
          >
            <TextField
              name="tmdbApiKey"
              type="password"
              label={t('setup.tmdb.label')}
              description={t('setup.tmdb.hint')}
              autoComplete="off"
              spellCheck="false"
              mono
            />
          </Step>

          {errors?.form && <Notice tone="danger">{errors.form}</Notice>}
          <AriaButton type="submit" variant="primary" size="lg" block isPending={setup.isPending}>
            {setup.isPending ? t('setup.submitting') : t('setup.submit')}
          </AriaButton>
        </Form>
        <ServerStatus />
      </Stack>
    </Page>
  );
}
