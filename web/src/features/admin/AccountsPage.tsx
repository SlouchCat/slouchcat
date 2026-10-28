import type {
  AccountAccessUpdate,
  AccountUpdate,
  AdminAccount,
  SessionInfo,
} from '@slouchcat/api-client';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
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
import {
  AriaButton,
  ConfirmDialog,
  DataTable,
  Form,
  FormDialog,
  RadioGroup,
  TextField,
  type DataColumn,
} from '@/ui/aria';
import { ratingOf, ratingOptions } from '../shared/choices';
import { LibraryChoice, type LibrarySelection } from '../shared/LibraryChoice';
import { LimitFields, limitsOf, limitValues, type LimitValues } from '../shared/LimitFields';
import {
  libraryIdsOf,
  useLibraryOptions,
  type LibraryOption,
  type LibraryOptions,
} from '../shared/useLibraryOptions';
import { InvitationsSection, InviteDialog } from './Invitations';
import { accountsQuery, invitationsQuery } from './queries';
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
 * "Konto ändern" (A-2): the e-mail and the password of the signed-in account. Every change needs
 * the current password; the passwords never stay in the query client.
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
 * "Zugriff ändern" (K-7): rating ceiling, libraries and the W-8 caps of one account. Only an
 * account that administers nothing has them; an administrator is fixed at FSK 18 and all
 * libraries (ADR 0014), so its row offers no such button.
 */
function AccessDialog({
  account,
  options,
  onClose,
}: {
  account: AdminAccount;
  options: readonly LibraryOption[];
  onClose: () => void;
}) {
  const { t } = useTranslation();
  const queryClient = useQueryClient();
  const [rating, setRating] = useState(String(account.maxRating));
  const [selection, setSelection] = useState<LibrarySelection>({
    all: account.allLibraries,
    ids: account.libraryIds.map(String),
  });
  const [limits, setLimits] = useState<LimitValues>(() => limitValues(account.limits));
  const save = useMutation({
    mutationFn: (body: AccountAccessUpdate) =>
      request(api.PATCH('/admin/accounts/{id}', { params: { path: { id: account.id } }, body })),
    onSuccess: async () => {
      await queryClient.invalidateQueries({ queryKey: accountsQuery.queryKey });
      onClose();
    },
  });
  const errors = save.error ? formErrors(t, save.error) : null;

  const onSubmit = (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    save.mutate({
      maxRating: ratingOf(rating),
      allLibraries: selection.all,
      libraryIds: selection.all ? [] : libraryIdsOf(selection.ids),
      // W-8: bandwidth, quality and streams cap every profile of the account.
      limits: limitsOf(limits, false),
    });
  };

  return (
    <FormDialog
      isOpen
      onOpenChange={(open) => {
        if (!open) onClose();
      }}
      title={t('admin.accounts.access.title', { email: account.email })}
    >
      <Form onSubmit={onSubmit} validationErrors={errors?.fields}>
        <div className={styles.form}>
          {errors?.form && <Notice tone="danger">{errors.form}</Notice>}
          <RadioGroup
            name="maxRating"
            label={t('admin.accounts.access.rating')}
            value={rating}
            onChange={setRating}
            options={ratingOptions(t)}
          />
          <LibraryChoice
            label={t('admin.accounts.access.libraries')}
            value={selection}
            onChange={setSelection}
            options={options}
          />
          <fieldset>
            <legend>{t('limits.title')}</legend>
            <Text tone="secondary" variant="caption">
              {t('admin.accounts.access.limitsHint')}
            </Text>
            <LimitFields value={limits} onChange={setLimits} withWatch={false} />
          </fieldset>
          <div className={styles.dialogActions}>
            <AriaButton variant="ghost" onPress={onClose}>
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
 * Accounts (A-2): every account with role, ceiling, libraries, profiles, last contact and the
 * notification channels it switched on. The signed-in account changes its own e-mail and password
 * here; the administration changes the access of the others, and the owner alone appoints and
 * demotes administrators (ADR 0014). The page shows or hides buttons - the server decides.
 */
export function AccountsPage() {
  const { t, i18n } = useTranslation();
  useDocumentTitle(t('admin.accounts.title'));
  const session = useActiveSession();
  const allowed = session.account.role === 'admin' && session.profile.isPrimary;
  const isOwner = session.account.isOwner;
  const accounts = useQuery({ ...accountsQuery, enabled: allowed });
  const libraries: LibraryOptions = useLibraryOptions(allowed);
  const [editing, setEditing] = useState(false);
  const [saved, setSaved] = useState(false);
  const [access, setAccess] = useState<AdminAccount | null>(null);
  const [inviting, setInviting] = useState(false);
  const queryClient = useQueryClient();
  const refresh = () =>
    Promise.all([
      queryClient.invalidateQueries({ queryKey: accountsQuery.queryKey }),
      queryClient.invalidateQueries({ queryKey: invitationsQuery.queryKey }),
    ]);
  const setRole = useMutation({
    mutationFn: ({ id, role }: { id: number; role: 'admin' | 'user' }) =>
      request(api.PUT('/admin/accounts/{id}/role', { params: { path: { id } }, body: { role } })),
    onSuccess: refresh,
  });
  const remove = useMutation({
    mutationFn: (id: number) =>
      request(api.DELETE('/admin/accounts/{id}', { params: { path: { id } } })),
    onSuccess: refresh,
  });
  const actionError = setRole.error ?? remove.error;

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
      cell: (account) => (
        <span className={data.chips}>
          <span>{t(`admin.accounts.roles.${account.role}`)}</span>
          {account.isOwner && <Chip tone="info">{t('admin.accounts.ownerBadge')}</Chip>}
        </span>
      ),
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
          <span className={styles.rowActions}>
            {account.role === 'user' && (
              <AriaButton
                isDisabled={!libraries.isReady}
                aria-label={t('admin.accounts.access.label', { email: account.email })}
                onPress={() => setAccess(account)}
              >
                {t('admin.accounts.access.action')}
              </AriaButton>
            )}
            {isOwner && !account.isOwner && (
              <ConfirmDialog
                trigger={
                  <AriaButton
                    variant="ghost"
                    aria-label={t(
                      account.role === 'admin'
                        ? 'admin.accounts.role.demote.label'
                        : 'admin.accounts.role.promote.label',
                      { email: account.email },
                    )}
                  >
                    {t(
                      account.role === 'admin'
                        ? 'admin.accounts.role.demote.action'
                        : 'admin.accounts.role.promote.action',
                    )}
                  </AriaButton>
                }
                title={t(
                  account.role === 'admin'
                    ? 'admin.accounts.role.demote.title'
                    : 'admin.accounts.role.promote.title',
                  { email: account.email },
                )}
                body={t(
                  account.role === 'admin'
                    ? 'admin.accounts.role.demote.body'
                    : 'admin.accounts.role.promote.body',
                )}
                confirmLabel={t(
                  account.role === 'admin'
                    ? 'admin.accounts.role.demote.confirm'
                    : 'admin.accounts.role.promote.confirm',
                )}
                cancelLabel={t('common.cancel')}
                onConfirm={() =>
                  setRole.mutate({
                    id: account.id,
                    role: account.role === 'admin' ? 'user' : 'admin',
                  })
                }
              />
            )}
            {!account.isOwner && (
              <ConfirmDialog
                trigger={
                  <AriaButton
                    variant="ghost"
                    aria-label={t('admin.accounts.remove.label', { email: account.email })}
                  >
                    {t('admin.accounts.remove.action')}
                  </AriaButton>
                }
                title={t('admin.accounts.remove.title', { email: account.email })}
                body={t('admin.accounts.remove.body')}
                confirmLabel={t('admin.accounts.remove.confirm')}
                cancelLabel={t('common.cancel')}
                tone="danger"
                onConfirm={() => remove.mutate(account.id)}
              />
            )}
          </span>
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
          <div className={data.toolbar}>
            <AriaButton
              variant="primary"
              icon="plus"
              isDisabled={!libraries.isReady}
              onPress={() => setInviting(true)}
            >
              {t('admin.accounts.invitations.invite')}
            </AriaButton>
          </div>
          {saved && <Notice>{t('admin.accounts.edit.saved')}</Notice>}
          {libraries.isError && (
            <Notice
              tone="danger"
              title={t('libraries.loadFailed')}
              actions={
                <AriaButton icon="refresh" onPress={libraries.retry}>
                  {t('common.retry')}
                </AriaButton>
              }
            >
              {problemMessage(t, libraries.error)}
            </Notice>
          )}
          {actionError && <Notice tone="danger">{problemMessage(t, actionError)}</Notice>}
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
          <InvitationsSection options={libraries.options} />
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
      {access && (
        <AccessDialog
          account={access}
          options={libraries.options}
          onClose={() => setAccess(null)}
        />
      )}
      {inviting && <InviteDialog options={libraries.options} onClose={() => setInviting(false)} />}
    </Page>
  );
}
