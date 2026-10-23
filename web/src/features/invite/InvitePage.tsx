import type {
  AcceptInvitationRequest,
  InvitationPreview,
  SessionInfo,
} from '@slouchcat/api-client';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { getRouteApi, useRouter } from '@tanstack/react-router';
import { useState, type FormEvent } from 'react';
import { useTranslation } from 'react-i18next';
import { api, isApiError, request } from '@/api/client';
import { formErrors, problemMessage } from '@/api/errors';
import { paths } from '@/app/paths';
import { adoptSession } from '@/app/session';
import { currentLanguage, isLanguage, type Language } from '@/i18n';
import { formatDateTime } from '@/lib/format';
import { formText, MIN_PASSWORD } from '@/lib/form';
import { useDocumentTitle } from '@/lib/useDocumentTitle';
import { ButtonLink, Chip, Logo, Notice, Page, Panel, Skeleton, Stack, Text } from '@/ui';
import { AriaButton, Form, RadioGroup, TextField } from '@/ui/aria';
import { languageOptions } from '../shared/choices';
import styles from './InvitePage.module.css';

const route = getRouteApi('/invite/$token');

function Gone() {
  const { t } = useTranslation();
  return (
    <Notice
      tone="warning"
      actions={
        <ButtonLink href={paths.login} variant="secondary">
          {t('invite.gone.action')}
        </ButtonLink>
      }
    >
      {t('invite.gone.body')}
    </Notice>
  );
}

function Summary({ invitation }: { invitation: InvitationPreview }) {
  const { t, i18n } = useTranslation();
  return (
    <Panel as="section" aria-labelledby={invitation.note ? 'invite-summary' : undefined}>
      {invitation.note && (
        <h2 id="invite-summary" className={styles.note}>
          {t('invite.for', { note: invitation.note })}
        </h2>
      )}
      <dl className={styles.facts}>
        <div className={styles.fact}>
          <dt>{t('invite.rating')}</dt>
          <dd>
            <Chip tone="outline">{t('rating.fsk', { age: invitation.maxRating })}</Chip>
          </dd>
        </div>
        <div className={styles.fact}>
          <dt>{t('invite.libraries')}</dt>
          <dd className={styles.chips}>
            {invitation.allLibraries ? (
              <Chip tone="info">{t('libraries.all')}</Chip>
            ) : (
              invitation.libraries.map((library) => (
                <Chip key={library.id} tone="info">
                  {library.name}
                </Chip>
              ))
            )}
          </dd>
        </div>
      </dl>
      <Text tone="muted" variant="caption">
        {t('invite.expires', { date: formatDateTime(invitation.expiresAt, i18n.language) })}
      </Text>
    </Panel>
  );
}

function AcceptForm({ token, onGone }: { token: string; onGone: () => void }) {
  const { t } = useTranslation();
  const queryClient = useQueryClient();
  const router = useRouter();
  const [language, setLanguage] = useState<Language>(currentLanguage);

  const accept = useMutation({
    mutationFn: (body: AcceptInvitationRequest): Promise<SessionInfo> =>
      request(api.POST('/invitations/{token}/accept', { params: { path: { token } }, body })),
    onSuccess: async (session) => {
      await adoptSession(queryClient, session);
      await router.navigate({ to: '/' });
    },
    onError: (error) => {
      if (isApiError(error, 410)) onGone();
    },
  });
  const errors = accept.error ? formErrors(t, accept.error) : null;

  const onSubmit = (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    const data = new FormData(event.currentTarget);
    accept.mutate({
      email: formText(data, 'email'),
      password: formText(data, 'password', false),
      profileName: formText(data, 'profileName'),
      language,
    });
  };

  const required = (value: string) => (value.trim() ? null : t('errors.field.required'));

  return (
    <Panel>
      <Form className={styles.form} onSubmit={onSubmit} validationErrors={errors?.fields}>
        {errors?.form && <Notice tone="danger">{errors.form}</Notice>}
        <TextField
          name="email"
          type="email"
          label={t('invite.form.email')}
          autoComplete="email"
          isRequired
          validate={required}
          autoFocus
        />
        <TextField
          name="password"
          type="password"
          label={t('invite.form.password')}
          description={t('invite.form.passwordHint')}
          autoComplete="new-password"
          isRequired
          validate={(value) =>
            value.length >= MIN_PASSWORD ? null : t('errors.field.password_too_short')
          }
        />
        <TextField
          name="profileName"
          label={t('invite.form.profileName')}
          description={t('invite.form.profileNameHint')}
          autoComplete="given-name"
          maxLength={60}
          isRequired
          validate={required}
        />
        <RadioGroup
          name="language"
          label={t('invite.form.language')}
          value={language}
          onChange={(value) => isLanguage(value) && setLanguage(value)}
          options={languageOptions(t)}
        />
        <AriaButton type="submit" variant="primary" size="lg" block isPending={accept.isPending}>
          {accept.isPending ? t('invite.form.submitting') : t('invite.form.submit')}
        </AriaButton>
      </Form>
    </Panel>
  );
}

/** Public page of an invitation link (K-6): preview, or why the link no longer works. */
export function InvitePage() {
  const { t } = useTranslation();
  useDocumentTitle(t('invite.title'));
  const { token } = route.useParams();
  const [gone, setGone] = useState(false);
  const invitation = useQuery({
    queryKey: ['invitation', token],
    queryFn: ({ signal }): Promise<InvitationPreview> =>
      request(api.GET('/invitations/{token}', { params: { path: { token } }, signal })),
    // Every look counts against the client's attempt limit: ask once, not on every focus.
    staleTime: Infinity,
    refetchOnWindowFocus: false,
    retry: false,
  });

  const error = invitation.error;
  const isGone = gone || isApiError(error, 410);
  const notFound = isApiError(error, 404);

  return (
    <Page layout="form">
      <Stack gap={8}>
        <Logo name={t('app.name')} />
        <Stack gap={2}>
          <Text as="h1" variant="title">
            {isGone
              ? t('invite.gone.title')
              : notFound
                ? t('invite.notFound.title')
                : t('invite.title')}
          </Text>
          {!isGone && !notFound && <Text tone="secondary">{t('invite.intro')}</Text>}
        </Stack>

        {invitation.isPending && (
          <div aria-busy="true">
            <Skeleton shape="block" />
          </div>
        )}
        {isGone && <Gone />}
        {notFound && <Notice tone="warning">{t('invite.notFound.body')}</Notice>}
        {invitation.isError && !isGone && !notFound && (
          <Notice
            tone="danger"
            title={problemMessage(t, error)}
            actions={
              <AriaButton icon="refresh" onPress={() => void invitation.refetch()}>
                {t('common.retry')}
              </AriaButton>
            }
          />
        )}
        {invitation.isSuccess && !isGone && (
          <>
            <Summary invitation={invitation.data} />
            <AcceptForm token={token} onGone={() => setGone(true)} />
          </>
        )}
      </Stack>
    </Page>
  );
}
