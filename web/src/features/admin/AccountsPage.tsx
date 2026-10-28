import type { AccountUpdate, AdminAccount, SessionInfo } from '@slouchcat/api-client';
import { useQuery, useQueryClient } from '@tanstack/react-query';
import type { TFunction } from 'i18next';
import { useState, type FormEvent } from 'react';
import { useTranslation } from 'react-i18next';
import { api, isApiError, request } from '@/api/client';
import { formErrors, problemMessage, type FormErrors } from '@/api/errors';
import { paths } from '@/app/paths';
import { sessionQuery, useActiveSession } from '@/app/session';
import { formText, MIN_PASSWORD } from '@/lib/form';
import { formatDateTime } from '@/lib/format';
import { useDocumentTitle } from '@/lib/useDocumentTitle';
import { useTransientMutation } from '@/lib/useTransientMutation';
import { ButtonLink, Chip, Notice, Page, Skeleton, Text } from '@/ui';
import { AriaButton, DataTable, Form, FormDialog, TextField, type DataColumn } from '@/ui/aria';
import { accountsQuery } from './queries';
import data from './AdminData.module.css';
import styles from './AdminTablePage.module.css';

/**
 * A wrong current password belongs to its field; the login text of `invalid_credentials` ("e-mail
 * or password") would point at the wrong thing here.
 */
function accountErrors(t: TFunction, error: unknown): FormErrors {
  if (isApiError(error) && error.code === 'invalid_credentials') {
    return { fields: { currentPassword: t('admin.accounts.edit.wrongPassword') }, form: null };
  }
  return formErrors(t, error);
}

/**
 * "Konto ändern" (A-2): the e-mail and the password of the signed-in main account. Every change
 * needs the current password; the passwords never stay in the query client.
 */
function AccountDialog({ email, onClose }: { email: string; onClose: (saved: boolean) => void }) {
  const { t } = useTranslation();
  const queryClient = useQueryClient();
  const [newPassword, setNewPassword] = useState('');
  const [unchanged, setUnchanged] = useState(false);
  const save = useTransientMutation(
    (body: AccountUpdate): Promise<SessionInfo> => request(api.PUT('/auth/account', { body })),
    async (session) => {
      queryClient.setQueryData(sessionQuery.queryKey, session);
      await queryClient.invalidateQueries({ queryKey: accountsQuery.queryKey });
      onClose(true);
    },
  );
  const errors = save.error ? accountErrors(t, save.error) : null;

  const onSubmit = (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    const data = new FormData(event.currentTarget);
    const nextEmail = formText(data, 'email');
    const changesEmail = nextEmail !== email;
    // The server needs at least one change; say so here instead of sending an empty update.
    setUnchanged(!changesEmail && !newPassword);
    if (!changesEmail && !newPassword) return;
    void save.run({
      ...(changesEmail ? { email: nextEmail } : {}),
      currentPassword: formText(data, 'currentPassword', false),
      ...(newPassword ? { newPassword } : {}),
    });
  };

  return (
    <FormDialog
      isOpen
      onOpenChange={(open) => {
        if (!open) onClose(false);
      }}
      title={t('admin.accounts.edit.title')}
      description={t('admin.accounts.edit.intro')}
    >
      <Form onSubmit={onSubmit} validationErrors={errors?.fields}>
        <div className={styles.form}>
          {errors?.form && <Notice tone="danger">{errors.form}</Notice>}
          {unchanged && <Notice tone="warning">{t('admin.accounts.edit.unchanged')}</Notice>}
          <TextField
            name="email"
            type="email"
            label={t('admin.accounts.edit.email')}
            autoComplete="email"
            defaultValue={email}
            isRequired
          />
          <TextField
            name="newPassword"
            type="password"
            label={t('admin.accounts.edit.newPassword')}
            description={t('admin.accounts.edit.newPasswordHint')}
            autoComplete="new-password"
            value={newPassword}
            onChange={setNewPassword}
            validate={(value) =>
              value === '' || value.length >= MIN_PASSWORD
                ? null
                : t('errors.field.password_too_short')
            }
          />
          {newPassword && (
            <TextField
              name="newPasswordRepeat"
              type="password"
              label={t('admin.accounts.edit.newPasswordRepeat')}
              autoComplete="new-password"
              isRequired
              validate={(value) =>
                value === newPassword ? null : t('errors.field.password_mismatch')
              }
            />
          )}
          <TextField
            name="currentPassword"
            type="password"
            label={t('admin.accounts.edit.currentPassword')}
            description={t('admin.accounts.edit.currentPasswordHint')}
            autoComplete="current-password"
            isRequired
          />
          <div className={styles.dialogActions}>
            <AriaButton variant="ghost" onPress={() => onClose(false)}>
              {t('common.cancel')}
            </AriaButton>
            <AriaButton type="submit" variant="primary" isPending={save.isPending}>
              {save.isPending ? t('common.saving') : t('common.save')}
            </AriaButton>
          </div>
        </div>
      </Form>
    </FormDialog>
  );
}

/**
 * Accounts (A-2): the main account and every guest with ceiling, libraries, profiles, last contact
 * and the notification channels they switched on. The main account changes its own e-mail and
 * password here; guests are managed on the access page.
 */
export function AccountsPage() {
  const { t, i18n } = useTranslation();
  useDocumentTitle(t('admin.accounts.title'));
  const session = useActiveSession();
  const allowed = session.account.role === 'admin' && session.profile.isPrimary;
  const accounts = useQuery({ ...accountsQuery, enabled: allowed });
  const [editing, setEditing] = useState(false);
  const [saved, setSaved] = useState(false);

  const columns: DataColumn<AdminAccount>[] = [
    {
      key: 'email',
      header: t('admin.accounts.columns.email'),
      rowHeader: true,
      cell: (account) => account.email,
    },
    {
      key: 'role',
      header: t('admin.accounts.columns.role'),
      cell: (account) => t(`admin.accounts.roles.${account.role}`),
    },
    {
      key: 'rating',
      header: t('admin.accounts.columns.rating'),
      cell: (account) => (
        <span className={styles.nowrap}>{t('rating.fsk', { age: account.maxRating })}</span>
      ),
    },
    {
      key: 'libraries',
      header: t('admin.accounts.columns.libraries'),
      cell: (account) =>
        account.allLibraries
          ? t('libraries.all')
          : t('admin.accounts.libraries', { count: account.libraryIds.length }),
    },
    {
      key: 'profiles',
      header: t('admin.accounts.columns.profiles'),
      cell: (account) =>
        account.profiles
          .map((profile) =>
            profile.isPrimary
              ? t('admin.accounts.primaryProfile', { name: profile.name })
              : profile.name,
          )
          .join(t('admin.accounts.listSeparator')),
    },
    {
      key: 'lastSeen',
      header: t('admin.accounts.columns.lastSeen'),
      cell: (account) =>
        account.lastSeenAt ? (
          <time className={styles.nowrap} dateTime={account.lastSeenAt}>
            {formatDateTime(account.lastSeenAt, i18n.language)}
          </time>
        ) : (
          <span className={styles.muted}>{t('admin.accounts.neverSeen')}</span>
        ),
    },
    {
      key: 'channels',
      header: t('admin.accounts.columns.channels'),
      cell: (account) => {
        const enabled = account.channels.filter((channel) => channel.enabled);
        return enabled.length === 0 ? (
          <span className={styles.muted}>{t('admin.accounts.noChannels')}</span>
        ) : (
          <span className={data.chips}>
            {enabled.map(({ channel }) => (
              <Chip key={channel} tone="outline">
                {t(`admin.accounts.channels.${channel}`)}
              </Chip>
            ))}
          </span>
        );
      },
    },
    {
      key: 'actions',
      header: t('admin.accounts.columns.actions'),
      hideHeader: true,
      align: 'end',
      cell: (account) =>
        account.id === session.account.id ? (
          <span className={styles.rowActions}>
            <AriaButton
              onPress={() => {
                setSaved(false);
                setEditing(true);
              }}
            >
              {t('admin.accounts.edit.action')}
            </AriaButton>
            <ButtonLink variant="ghost" href={paths.profileSettings}>
              {t('admin.accounts.profilesLink')}
            </ButtonLink>
          </span>
        ) : (
          <ButtonLink
            variant="ghost"
            href={paths.adminAccess}
            aria-label={t('admin.accounts.accessLabel', { email: account.email })}
          >
            {t('admin.accounts.accessLink')}
          </ButtonLink>
        ),
    },
  ];

  return (
    <Page>
      <div className={styles.header}>
        <Text as="h1" variant="display">
          {t('admin.accounts.title')}
        </Text>
        <Text tone="secondary">{t('admin.accounts.intro')}</Text>
      </div>
      {!allowed ? (
        <Notice title={t('admin.access.notAllowed.title')}>
          {t('admin.access.notAllowed.body')}
        </Notice>
      ) : (
        <div className={styles.body} aria-busy={accounts.isPending}>
          {saved && <Notice>{t('admin.accounts.edit.saved')}</Notice>}
          {accounts.isPending && <Skeleton shape="block" />}
          {accounts.isError && (
            <Notice
              tone="danger"
              title={problemMessage(t, accounts.error)}
              actions={
                <AriaButton icon="refresh" onPress={() => void accounts.refetch()}>
                  {t('common.retry')}
                </AriaButton>
              }
            />
          )}
          {accounts.data && (
            <DataTable
              caption={t('admin.accounts.list')}
              columns={columns}
              rows={accounts.data}
              rowKey={(account) => String(account.id)}
            />
          )}
        </div>
      )}
      {editing && (
        <AccountDialog
          email={session.account.email}
          onClose={(done) => {
            setSaved(done);
            setEditing(false);
          }}
        />
      )}
    </Page>
  );
}
