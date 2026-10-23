import type { LoginRequest, SessionInfo } from '@slouchcat/api-client';
import { useMutation, useQueryClient } from '@tanstack/react-query';
import { useNavigate } from '@tanstack/react-router';
import type { FormEvent } from 'react';
import { useTranslation } from 'react-i18next';
import { api, request } from '@/api/client';
import { formErrors } from '@/api/errors';
import { adoptSession } from '@/app/session';
import { formText } from '@/lib/form';
import { useDocumentTitle } from '@/lib/useDocumentTitle';
import { Logo, Notice, Page, Panel, Stack, Text } from '@/ui';
import { AriaButton, Form, TextField } from '@/ui/aria';
import { ServerStatus } from '@/features/system/ServerStatus';
import styles from './AuthPage.module.css';

export function LoginPage() {
  const { t } = useTranslation();
  useDocumentTitle(t('login.title'));
  const queryClient = useQueryClient();
  const navigate = useNavigate();

  const login = useMutation({
    mutationFn: (body: LoginRequest): Promise<SessionInfo> =>
      request(api.POST('/auth/login', { body })),
    onSuccess: async (session) => {
      await adoptSession(queryClient, session);
      // An account with several profiles (or a PIN) picks one first (K-4).
      await navigate({ to: session.profile ? '/' : '/profiles' });
    },
  });
  const errors = login.error ? formErrors(t, login.error) : null;

  const onSubmit = (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    const data = new FormData(event.currentTarget);
    login.mutate({ email: formText(data, 'email'), password: formText(data, 'password', false) });
  };

  const required = (value: string) => (value.trim() ? null : t('errors.field.required'));

  return (
    <Page layout="form">
      <Stack gap={8}>
        <Logo name={t('app.name')} />
        <Stack gap={2}>
          <Text as="h1" variant="title">
            {t('login.title')}
          </Text>
          <Text tone="secondary">{t('login.intro')}</Text>
        </Stack>
        <Panel>
          <Form className={styles.form} onSubmit={onSubmit} validationErrors={errors?.fields}>
            {errors?.form && <Notice tone="danger">{errors.form}</Notice>}
            <TextField
              name="email"
              type="email"
              label={t('login.email')}
              autoComplete="username"
              isRequired
              validate={required}
              autoFocus
            />
            <TextField
              name="password"
              type="password"
              label={t('login.password')}
              autoComplete="current-password"
              isRequired
              validate={required}
            />
            <AriaButton type="submit" variant="primary" size="lg" block isPending={login.isPending}>
              {login.isPending ? t('login.submitting') : t('login.submit')}
            </AriaButton>
          </Form>
        </Panel>
        <ServerStatus />
      </Stack>
    </Page>
  );
}
