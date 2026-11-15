import type { SessionInfo, SetupRequest } from '@slouchcat/api-client';
import { useMutation, useQueryClient } from '@tanstack/react-query';
import { useNavigate } from '@tanstack/react-router';
import { useRef, useState, type FormEvent } from 'react';
import { useTranslation } from 'react-i18next';
import { api, isApiError, request } from '@/api/client';
import { formErrors } from '@/api/errors';
import { paths } from '@/app/paths';
import { adoptSession, setupStatusQuery } from '@/app/session';
import { currentLanguage } from '@/i18n';
import { formText, MIN_PASSWORD } from '@/lib/form';
import { ButtonLink, isAccentName, Notice } from '@/ui';
import { AccentPicker, Form, TextField } from '@/ui/aria';
import { accentOptions } from '../shared/choices';
import { setupPath, setupProgressQuery } from './setupState';
import { SetupLayout } from './SetupLayout';
import styles from './SetupLayout.module.css';

/**
 * Step 2 (concept 22, 3.3): the owner's e-mail and password (at least 10 characters, twice) and
 * the name and colour of the first profile, which speaks the language of step 1. «Weiter» creates
 * the account through runSetup without a library: from here on the owner is signed in and the
 * server keeps the wizard. Somebody who was faster gets «Slouchcat ist schon eingerichtet» with the
 * way to the login (3.1).
 */
export function AccountStep() {
  const { t } = useTranslation();
  const queryClient = useQueryClient();
  const navigate = useNavigate();
  const form = useRef<HTMLFormElement>(null);
  const [password, setPassword] = useState('');
  const [accent, setAccent] = useState<string>('ember');
  const setup = useMutation({
    mutationFn: (body: SetupRequest): Promise<SessionInfo> => request(api.POST('/setup', { body })),
    onSuccess: async (session) => {
      queryClient.setQueryData(setupStatusQuery.queryKey, { setupRequired: false });
      await adoptSession(queryClient, session);
      await queryClient.invalidateQueries({ queryKey: setupProgressQuery.queryKey });
      await navigate({ to: setupPath('region') });
    },
    onError: (error) => {
      if (isApiError(error, 409)) {
        queryClient.setQueryData(setupStatusQuery.queryKey, { setupRequired: false });
      }
    },
  });
  const taken = isApiError(setup.error, 409);
  const errors = setup.error && !taken ? formErrors(t, setup.error) : null;
  const required = (value: string) => (value.trim() ? null : t('errors.field.required'));

  const onSubmit = (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    const data = new FormData(event.currentTarget);
    setup.mutate({
      email: formText(data, 'email'),
      password: formText(data, 'password', false),
      profileName: formText(data, 'profileName'),
      ...(isAccentName(accent) ? { accent } : {}),
      language: currentLanguage(),
    });
  };

  return (
    <SetupLayout
      step="account"
      intro={t('setup.steps.account.intro')}
      footer={{
        onNext: () => form.current?.requestSubmit(),
        isPending: setup.isPending,
        isDisabled: taken,
      }}
      error={errors?.form}
    >
      {taken ? (
        <Notice
          tone="warning"
          title={t('setup.account.taken.title')}
          actions={
            <ButtonLink href={paths.login} variant="secondary">
              {t('setup.account.taken.action')}
            </ButtonLink>
          }
        >
          {t('setup.account.taken.body')}
        </Notice>
      ) : (
        <Form
          ref={form}
          className={styles.stack}
          onSubmit={onSubmit}
          validationErrors={errors?.fields}
        >
          <TextField
            name="email"
            type="email"
            label={t('setup.account.email')}
            autoComplete="email"
            isRequired
            validate={required}
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
            validate={(value) => (value === password ? null : t('errors.field.password_mismatch'))}
          />
          <TextField
            name="profileName"
            label={t('setup.account.profileName')}
            description={t('setup.account.profileHint')}
            autoComplete="nickname"
            maxLength={60}
            isRequired
            validate={required}
          />
          <AccentPicker
            name="accent"
            label={t('accent.label')}
            value={accent}
            onChange={setAccent}
            options={accentOptions(t)}
          />
          {/* Enter in a field submits like «Weiter». */}
          <button type="submit" hidden />
        </Form>
      )}
    </SetupLayout>
  );
}
