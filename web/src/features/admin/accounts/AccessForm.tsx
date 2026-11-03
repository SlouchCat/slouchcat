import type { AccountAccessUpdate, AdminAccount } from '@slouchcat/api-client';
import { useMutation } from '@tanstack/react-query';
import { useRef, type FormEvent } from 'react';
import { useTranslation } from 'react-i18next';
import { api, request } from '@/api/client';
import { formErrors } from '@/api/errors';
import { Notice, Text } from '@/ui';
import { AriaButton, Form, RadioGroup } from '@/ui/aria';
import { ratingOf, ratingOptions } from '../../shared/choices';
import { LibraryChoice } from '../../shared/LibraryChoice';
import { LimitFields, limitsOf, limitValues, type LimitValues } from '../../shared/LimitFields';
import { libraryIdsOf, type LibraryOption } from '../../shared/useLibraryOptions';
import styles from './Accounts.module.css';

/** The access of an account as its form holds it; the draft of an account is one of these. */
export interface AccessDraft {
  rating: string;
  all: boolean;
  /** Library ids as text, sorted, so a ticked and unticked box is no change. */
  ids: string[];
  limits: LimitValues;
}

export const accessDraftOf = (account: AdminAccount): AccessDraft => ({
  rating: String(account.maxRating),
  all: account.allLibraries,
  ids: account.libraryIds.map(String).sort(),
  limits: limitValues(account.limits),
});

/**
 * With «Alle» the checklist does not count: it takes the stored ids, so an access that ends up
 * as the server holds it is no draft (a box ticked and «Alle» chosen again).
 */
export function normaliseAccess(next: AccessDraft, original: AccessDraft | undefined): AccessDraft {
  const ids = [...next.ids].sort();
  return next.all && original ? { ...next, ids: original.ids } : { ...next, ids };
}

/**
 * «Zugriff» (K-7, KON-4) in the fold-out of an account that administers nothing: rating ceiling,
 * libraries and the W-8 caps of every profile of the account, saved with its own button. An
 * administrator is fixed at FSK 18 and all libraries (ADR 0014) and has no such form. What is
 * typed is the page's draft of the account (S-17), so folding the row or switching tabs keeps it.
 */
export function AccessForm({
  account,
  options,
  value,
  dirty,
  onChange,
  onDiscard,
  onSaved,
}: {
  account: AdminAccount;
  options: readonly LibraryOption[];
  value: AccessDraft;
  dirty: boolean;
  onChange: (next: AccessDraft) => void;
  onDiscard: () => void;
  onSaved: (sent: AccessDraft) => Promise<void>;
}) {
  const { t } = useTranslation();
  const saveButton = useRef<HTMLButtonElement>(null);
  const save = useMutation({
    mutationFn: ({ body }: { body: AccountAccessUpdate; sent: AccessDraft }) =>
      request(api.PATCH('/admin/accounts/{id}', { params: { path: { id: account.id } }, body })),
    onSuccess: (_data, { sent }) => onSaved(sent),
  });
  const errors = save.error ? formErrors(t, save.error) : null;
  const set = (patch: Partial<AccessDraft>) => {
    if (save.error) save.reset();
    onChange({ ...value, ...patch });
  };

  const onSubmit = (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    // Nothing changed: nothing to send, the focus stays on «Speichern» (decision 18).
    if (!dirty) return;
    save.mutate({
      body: {
        maxRating: ratingOf(value.rating),
        allLibraries: value.all,
        libraryIds: value.all ? [] : libraryIdsOf(value.ids),
        // W-8: bandwidth, quality and streams cap every profile of the account.
        limits: limitsOf(value.limits, false),
      },
      sent: value,
    });
  };

  return (
    <section
      className={styles.block}
      aria-label={t('admin.accounts.access.title', { email: account.email })}
    >
      <h3 className={styles.blockTitle}>{t('admin.accounts.access.heading')}</h3>
      <Form onSubmit={onSubmit} validationErrors={errors?.fields}>
        <div className={styles.fields}>
          {errors?.form && <Notice tone="danger">{errors.form}</Notice>}
          <div className={styles.columns}>
            <div className={styles.fields}>
              <RadioGroup
                name="maxRating"
                label={t('admin.accounts.access.rating')}
                value={value.rating}
                onChange={(rating) => set({ rating })}
                options={ratingOptions(t)}
              />
              <LibraryChoice
                label={t('admin.accounts.access.libraries')}
                value={{ all: value.all, ids: value.ids }}
                onChange={(selection) => set(selection)}
                options={options}
              />
            </div>
            <fieldset className={styles.fieldset}>
              <legend>{t('limits.title')}</legend>
              <Text tone="secondary" variant="caption">
                {t('admin.accounts.access.limitsHint')}
              </Text>
              <LimitFields
                value={value.limits}
                onChange={(limits) => set({ limits })}
                withWatch={false}
              />
            </fieldset>
          </div>
          <div className={styles.formActions}>
            {dirty && (
              <AriaButton
                variant="ghost"
                isDisabled={save.isPending}
                onPress={() => {
                  save.reset();
                  onDiscard();
                  // The button goes with the draft; the focus stays in the form.
                  saveButton.current?.focus();
                }}
              >
                {t('admin.accounts.discard')}
              </AriaButton>
            )}
            <AriaButton ref={saveButton} type="submit" variant="primary" isPending={save.isPending}>
              {save.isPending ? t('common.saving') : t('admin.accounts.access.save')}
            </AriaButton>
          </div>
        </div>
      </Form>
    </section>
  );
}
