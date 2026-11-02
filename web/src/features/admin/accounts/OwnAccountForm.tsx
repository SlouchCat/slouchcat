import type { AccountUpdate, SessionInfo } from '@slouchcat/api-client';
import { useQueryClient } from '@tanstack/react-query';
import type { TFunction } from 'i18next';
import { useRef, useState, type FormEvent } from 'react';
import { useTranslation } from 'react-i18next';
import { api, isApiError, request } from '@/api/client';
import { formErrors, type FormErrors } from '@/api/errors';
import { sessionQuery } from '@/app/session';
import { MIN_PASSWORD } from '@/lib/form';
import { useTransientMutation } from '@/lib/useTransientMutation';
import { Notice, Text } from '@/ui';
import { AriaButton, Form, TextField } from '@/ui/aria';
import { accountsQuery } from '../queries';
import styles from './Accounts.module.css';

/** What «Konto ändern» holds while it is typed: the page's draft of the own account (S-17). */
export interface OwnAccountDraft {
  email: string;
  newPassword: string;
  newPasswordRepeat: string;
  currentPassword: string;
}

export const ownAccountDraftOf = (email: string): OwnAccountDraft => ({
  email,
  newPassword: '',
  newPasswordRepeat: '',
  currentPassword: '',
});

/** Without a new password its repeat does not count (the field is not even shown). */
export const normaliseOwnAccount = (draft: OwnAccountDraft): OwnAccountDraft =>
  draft.newPassword === '' ? { ...draft, newPasswordRepeat: '' } : draft;

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
 * «Konto ändern» (A-2, KON-3) in the own account's fold-out: the e-mail and the password of the
 * signed-in account. Every change needs the current password; the passwords live in the page's
 * draft while typed and never in the query client.
 */
export function OwnAccountForm({
  email,
  value,
  dirty,
  onChange,
  onDiscard,
  onSaved,
}: {
  /** The e-mail the server holds. */
  email: string;
  value: OwnAccountDraft;
  dirty: boolean;
  onChange: (next: OwnAccountDraft) => void;
  onDiscard: () => void;
  /** `sent` is stored: the page drops the draft if it still says what was sent. */
  onSaved: (sent: OwnAccountDraft) => void;
}) {
  const { t } = useTranslation();
  const queryClient = useQueryClient();
  const saveButton = useRef<HTMLButtonElement>(null);
  const [unchanged, setUnchanged] = useState(false);
  const save = useTransientMutation(
    (body: AccountUpdate): Promise<SessionInfo> => request(api.PUT('/auth/account', { body })),
    async (session) => {
      queryClient.setQueryData(sessionQuery.queryKey, session);
      await queryClient.invalidateQueries({ queryKey: accountsQuery.queryKey });
    },
  );
  // «Verwerfen» hides the answer to the discarded input; the next failed save shows its own.
  const [hiddenFailure, setHiddenFailure] = useState(-1);
  const errors =
    save.error && hiddenFailure !== save.failures ? accountErrors(t, save.error) : null;
  const set = (patch: Partial<OwnAccountDraft>) => {
    setUnchanged(false);
    onChange({ ...value, ...patch });
  };

  const onSubmit = (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    const nextEmail = value.email.trim();
    const changesEmail = nextEmail !== email;
    // The server needs at least one change; say so here instead of sending an empty update.
    setUnchanged(!changesEmail && !value.newPassword);
    if (!changesEmail && !value.newPassword) return;
    const sent = value;
    void save
      .run({
        ...(changesEmail ? { email: nextEmail } : {}),
        currentPassword: value.currentPassword,
        ...(value.newPassword ? { newPassword: value.newPassword } : {}),
      })
      .then((outcome) => {
        if (outcome.ok) onSaved(sent);
      });
  };

  return (
    <section className={styles.block} aria-labelledby="own-account-heading">
      <h3 id="own-account-heading" className={styles.blockTitle}>
        {t('admin.accounts.edit.title')}
      </h3>
      <Text tone="secondary" variant="caption">
        {t('admin.accounts.edit.intro')}
      </Text>
      <Form onSubmit={onSubmit} validationErrors={errors?.fields}>
        <div className={styles.fields}>
          {errors?.form && <Notice tone="danger">{errors.form}</Notice>}
          {unchanged && <Notice tone="warning">{t('admin.accounts.edit.unchanged')}</Notice>}
          <TextField
            name="email"
            type="email"
            label={t('admin.accounts.edit.email')}
            autoComplete="email"
            value={value.email}
            onChange={(next) => set({ email: next })}
            isRequired
          />
          <TextField
            name="newPassword"
            type="password"
            label={t('admin.accounts.edit.newPassword')}
            description={t('admin.accounts.edit.newPasswordHint')}
            autoComplete="new-password"
            value={value.newPassword}
            onChange={(next) => set({ newPassword: next })}
            validate={(next) =>
              next === '' || next.length >= MIN_PASSWORD
                ? null
                : t('errors.field.password_too_short')
            }
          />
          {value.newPassword && (
            <TextField
              name="newPasswordRepeat"
              type="password"
              label={t('admin.accounts.edit.newPasswordRepeat')}
              autoComplete="new-password"
              value={value.newPasswordRepeat}
              onChange={(next) => set({ newPasswordRepeat: next })}
              isRequired
              validate={(next) =>
                next === value.newPassword ? null : t('errors.field.password_mismatch')
              }
            />
          )}
          <TextField
            name="currentPassword"
            type="password"
            label={t('admin.accounts.edit.currentPassword')}
            description={t('admin.accounts.edit.currentPasswordHint')}
            autoComplete="current-password"
            value={value.currentPassword}
            onChange={(next) => set({ currentPassword: next })}
            isRequired
          />
          <div className={styles.formActions}>
            {dirty && (
              <AriaButton
                variant="ghost"
                isDisabled={save.isPending}
                onPress={() => {
                  setHiddenFailure(save.failures);
                  setUnchanged(false);
                  onDiscard();
                  // The button goes with the draft; the focus stays in the form.
                  saveButton.current?.focus();
                }}
              >
                {t('admin.accounts.discard')}
              </AriaButton>
            )}
            <AriaButton ref={saveButton} type="submit" variant="primary" isPending={save.isPending}>
              {save.isPending ? t('common.saving') : t('common.save')}
            </AriaButton>
          </div>
        </div>
      </Form>
    </section>
  );
}
