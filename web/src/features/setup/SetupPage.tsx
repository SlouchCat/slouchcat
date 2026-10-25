import type { PathCheck, SessionInfo, SetupRequest, TMDBCheck } from '@slouchcat/api-client';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { useNavigate } from '@tanstack/react-router';
import { useId, useState, type FormEvent, type ReactNode } from 'react';
import { useTranslation } from 'react-i18next';
import { api, isApiError, request } from '@/api/client';
import { formErrors, problemMessage } from '@/api/errors';
import { adoptSession, setupStatusQuery } from '@/app/session';
import { ServerStatus } from '@/features/system/ServerStatus';
import { currentLanguage } from '@/i18n';
import { formText, MIN_PASSWORD } from '@/lib/form';
import { useDocumentTitle } from '@/lib/useDocumentTitle';
import { useTransientMutation } from '@/lib/useTransientMutation';
import { Logo, Notice, Page, Panel, Stack, Text, TextLink } from '@/ui';
import { AriaButton, Form, TextField } from '@/ui/aria';
import { LibraryFields, libraryRequest, type LibraryKind } from '../shared/LibraryFields';
import { SystemState } from '../shared/SystemState';
import styles from './SetupPage.module.css';

const TMDB_KEY_URL = 'https://www.themoviedb.org/settings/api';

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

/**
 * The folder as the wizard shows it (plan 3.4): file system, how changes are followed, and the
 * hint for network shares and FUSE. It stands in a live region, so the notice is not one itself.
 */
function PathResult({ check }: { check: PathCheck }) {
  const { t } = useTranslation();
  if (!check.ok) return null;
  const parts = [
    check.fsType && t('admin.libraries.fsType', { type: check.fsType }),
    check.watchMode && t(`admin.libraries.watch.${check.watchMode}`),
  ].filter(Boolean);
  return (
    <>
      {parts.length > 0 && (
        <Text tone="secondary" variant="caption">
          {parts.join(t('format.separator'))}
        </Text>
      )}
      {check.warning && (
        <Notice
          tone="warning"
          live={false}
          title={t(`setup.library.warning.${check.warning}.title`)}
        >
          {t(`setup.library.warning.${check.warning}.body`)}
        </Notice>
      )}
    </>
  );
}

/** The field message of a path the server described as unusable (the texts of the submit). */
const pathProblemKey = (problem: NonNullable<PathCheck['problem']>) => {
  if (problem === 'not_absolute') return 'errors.field.not_absolute';
  // The server does not describe a folder below no browsable root, so neither does the message.
  if (problem === 'not_browsable') return 'errors.code.path_not_browsable';
  return 'errors.code.library_path_invalid';
};

/**
 * First-start wizard (A-1): main account, first library with a check of its folder, optional TMDB
 * key with a check against TMDB, and what the server found (graphics chip, inotify limit).
 */
export function SetupPage() {
  const { t } = useTranslation();
  useDocumentTitle(t('setup.title'));
  const queryClient = useQueryClient();
  const navigate = useNavigate();
  const [password, setPassword] = useState('');
  const [kind, setKind] = useState<LibraryKind>('movies');
  const [rating, setRating] = useState('0');
  const [path, setPath] = useState('');
  const [tmdbKey, setTmdbKey] = useState('');
  // A check button pressed with an empty field (the field then asks for a value).
  const [blank, setBlank] = useState<{ path?: boolean; tmdb?: boolean }>({});
  // The values the last submit sent: its field errors belong to them only.
  const [submitted, setSubmitted] = useState({ path: '', tmdb: '' });
  // The guard asked for the setup state already; `system` comes with it.
  const system = useQuery(setupStatusQuery).data?.system;

  const pathCheck = useMutation({
    mutationFn: (value: string): Promise<PathCheck> =>
      request(api.POST('/setup/path-check', { body: { path: value } })),
  });
  // The key is a secret: it never stays in the query client, the answer keeps it in this state.
  const [keyCheck, setKeyCheck] = useState<{ key: string; result: TMDBCheck } | null>(null);
  const tmdbCheck = useTransientMutation(
    (key: string): Promise<TMDBCheck> =>
      request(api.POST('/setup/tmdb-check', { body: { tmdbApiKey: key } })),
    (result, key) => setKeyCheck({ key, result }),
  );

  const setup = useMutation({
    mutationFn: (body: SetupRequest): Promise<SessionInfo> => request(api.POST('/setup', { body })),
    onSuccess: async (session) => {
      queryClient.setQueryData(setupStatusQuery.queryKey, { setupRequired: false });
      await adoptSession(queryClient, session);
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

  // A check answers for the value it was asked about; a changed value hides its answer.
  const checkedPath = pathCheck.variables === path.trim() ? pathCheck.data : undefined;
  const checkedKey = keyCheck?.key === tmdbKey.trim() ? keyCheck.result : undefined;
  const fields: Record<string, string> = {};
  if (blank.path) fields['library.path'] = t('errors.field.required');
  if (checkedPath?.problem) fields['library.path'] = t(pathProblemKey(checkedPath.problem));
  if (blank.tmdb) fields.tmdbApiKey = t('errors.field.required');
  if (checkedKey?.problem) fields.tmdbApiKey = t(`setup.tmdb.result.${checkedKey.problem}`);
  for (const [name, message] of Object.entries(errors?.fields ?? {})) {
    if (name === 'library.path' && path.trim() !== submitted.path) continue;
    if (name === 'tmdbApiKey' && tmdbKey.trim() !== submitted.tmdb) continue;
    fields[name] = message;
  }

  /** Asks the server about the folder once per value (blur and the button share the answer). */
  const checkPath = (fromButton: boolean) => {
    const value = path.trim();
    if (!value) {
      if (fromButton) setBlank((state) => ({ ...state, path: true }));
      return;
    }
    if (pathCheck.variables === value && !pathCheck.isError) return;
    pathCheck.mutate(value);
  };

  const checkKey = () => {
    const value = tmdbKey.trim();
    if (!value) {
      setBlank((state) => ({ ...state, tmdb: true }));
      return;
    }
    setKeyCheck(null);
    void tmdbCheck.run(value);
  };

  const onSubmit = (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    const data = new FormData(event.currentTarget);
    const field = (name: string) => formText(data, name);
    const tmdbApiKey = field('tmdbApiKey');
    setSubmitted({ path: field('library.path'), tmdb: tmdbApiKey });
    setup.mutate({
      email: field('email'),
      password: formText(data, 'password', false),
      library: libraryRequest(field('library.name'), kind, field('library.path'), rating),
      ...(tmdbApiKey ? { tmdbApiKey } : {}),
      language: currentLanguage(),
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

        <Form className={styles.form} onSubmit={onSubmit} validationErrors={fields}>
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
            <LibraryFields
              prefix="library."
              kind={kind}
              onKindChange={setKind}
              rating={rating}
              onRatingChange={setRating}
              path={path}
              onPathChange={(value) => {
                setPath(value);
                setBlank((state) => ({ ...state, path: false }));
              }}
              onPathBlur={() => checkPath(false)}
              pathAddon={
                <div className={styles.check}>
                  <div>
                    <AriaButton
                      icon="folder"
                      isPending={pathCheck.isPending}
                      onPress={() => checkPath(true)}
                    >
                      {t('setup.library.check')}
                    </AriaButton>
                  </div>
                  <div role="status" className={styles.result}>
                    {pathCheck.isPending && (
                      <Text tone="secondary" variant="caption">
                        {t('setup.library.checking')}
                      </Text>
                    )}
                    {checkedPath && <PathResult check={checkedPath} />}
                  </div>
                  {pathCheck.isError && (
                    <Notice tone="danger">{problemMessage(t, pathCheck.error)}</Notice>
                  )}
                </div>
              }
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
              value={tmdbKey}
              onChange={(value) => {
                setTmdbKey(value);
                setBlank((state) => ({ ...state, tmdb: false }));
              }}
            />
            <div className={styles.check}>
              <div>
                <AriaButton icon="check" isPending={tmdbCheck.isPending} onPress={checkKey}>
                  {t('setup.tmdb.check')}
                </AriaButton>
              </div>
              <div role="status" className={styles.result}>
                {tmdbCheck.isPending && (
                  <Text tone="secondary" variant="caption">
                    {t('setup.tmdb.checking')}
                  </Text>
                )}
                {checkedKey?.ok && (
                  <Text tone="secondary" variant="caption">
                    {t('setup.tmdb.result.valid')}
                  </Text>
                )}
              </div>
              {tmdbCheck.error ? (
                <Notice tone="danger">{problemMessage(t, tmdbCheck.error)}</Notice>
              ) : null}
            </div>
          </Step>

          <Step
            step={4}
            title={t('setup.system.title')}
            description={t('setup.system.description')}
          >
            {system ? (
              <SystemState system={system} />
            ) : (
              <Text tone="secondary">{t('setup.system.missing')}</Text>
            )}
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
