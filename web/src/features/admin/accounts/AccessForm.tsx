import type { AccountAccessUpdate, AdminAccount } from '@slouchcat/api-client';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { useRef, type FormEvent } from 'react';
import { useTranslation } from 'react-i18next';
import { api, request } from '@/api/client';
import { formErrors } from '@/api/errors';
import { useRatingSystem } from '@/app/session';
import { Notice, Text } from '@/ui';
import { AriaButton, Form, RadioGroup, Switch } from '@/ui/aria';
import { NO_AGE_LIMIT, ratingOf, ratingOptions } from '../../shared/choices';
import { LibraryChoice } from '../../shared/LibraryChoice';
import { LimitFields, limitsOf, limitValues, type LimitValues } from '../../shared/LimitFields';
import { libraryIdsOf, type LibraryOption } from '../../shared/useLibraryOptions';
import {
  wishLimitFieldNames,
  WishLimitFields,
  wishLimitDraftOf,
  wishLimitRequestOf,
  type WishLimitDraft,
} from '../../shared/WishLimitFields';
import {
  accountsQuery,
  dashboardQuery,
  setAccountWishLimit,
  wishKeys,
  wishSettingsQuery,
} from '../queries';
import { RequestRightHint, SeerrUserLine } from './wishes';
import styles from './Accounts.module.css';

/** The access of an account as its form holds it; the draft of an account is one of these. */
export interface AccessDraft {
  rating: string;
  all: boolean;
  /** Library ids as text, sorted, so a ticked and unticked box is no change. */
  ids: string[];
  limits: LimitValues;
  /** «Darf Titel anfordern» (concept 25): the cap of all the account's profiles. */
  canRequest: boolean;
  /** The wish limit; absent while the server names none (Seerr not set up). */
  wishLimit?: WishLimitDraft;
}

export const accessDraftOf = (account: AdminAccount): AccessDraft => ({
  rating: String(account.maxRating),
  all: account.allLibraries,
  ids: account.libraryIds.map(String).sort(),
  limits: limitValues(account.limits),
  canRequest: account.canRequest ?? false,
  ...(account.wishLimit
    ? { wishLimit: wishLimitDraftOf(account.wishLimit, account.wishLimit.effective) }
    : {}),
});

/** The limit as sent: «Standard» and «Unbegrenzt» carry no numbers, so theirs are no change. */
const limitKey = (draft: WishLimitDraft | undefined) =>
  draft && JSON.stringify(wishLimitRequestOf(draft));

/** The access went through, the wish limit after it did not: only the limit is still to save. */
class LimitRefused extends Error {
  constructor(readonly reason: unknown) {
    super('wish limit refused');
  }
}

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
 * libraries and the W-8 caps of every profile of the account, the request right and the wish
 * limit (concept 25, administration 8), saved with its own button: the access first, then the
 * limit when it changed (`setAccountWishLimit`). An
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
  const ratingSystem = useRatingSystem();
  const queryClient = useQueryClient();
  const saveButton = useRef<HTMLButtonElement>(null);
  const wishSettings = useQuery(wishSettingsQuery);
  const stored = accessDraftOf(account);
  const save = useMutation({
    mutationFn: async ({ body, sent }: { body: AccountAccessUpdate; sent: AccessDraft }) => {
      await request(
        api.PATCH('/admin/accounts/{id}', { params: { path: { id: account.id } }, body }),
      );
      if (sent.wishLimit && limitKey(sent.wishLimit) !== limitKey(stored.wishLimit)) {
        try {
          await setAccountWishLimit(account.id, wishLimitRequestOf(sent.wishLimit));
        } catch (error) {
          throw new LimitRefused(error);
        }
      }
    },
    onSuccess: (_data, { sent }) => onSaved(sent),
    // The access is stored: the table and the fold-out show it, the draft keeps only the limit.
    onError: async (error) => {
      if (!(error instanceof LimitRefused)) return;
      await Promise.all([
        queryClient.invalidateQueries({ queryKey: accountsQuery.queryKey }),
        queryClient.invalidateQueries({ queryKey: wishKeys.all }),
        queryClient.invalidateQueries({ queryKey: dashboardQuery.queryKey }),
      ]);
    },
  });
  const limitRefused = save.error instanceof LimitRefused ? save.error.reason : null;
  const errors = limitRefused
    ? formErrors(t, limitRefused, wishLimitFieldNames('', 'wishLimit'))
    : save.error
      ? formErrors(t, save.error)
      : null;
  // The right follows the rating (ADR 0045), a mirror of the server's rule: with an age limit it is
  // off and locked; a rating that crosses 18 sets it on saving, so the switch shows the outcome and
  // the form leaves canRequest out instead of sending the draft's old value.
  const restricted = ratingOf(value.rating) < NO_AGE_LIMIT;
  const crossing = account.maxRating < NO_AGE_LIMIT !== restricted;
  const requestLocked = restricted || crossing;
  const requestShown = !restricted && (crossing || value.canRequest);
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
        // Only a change: an account the server names no right for keeps the one it has.
        ...(!requestLocked && value.canRequest !== stored.canRequest
          ? { canRequest: value.canRequest }
          : {}),
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
          {errors?.form && (
            <Notice tone="danger">
              {limitRefused
                ? t('admin.accounts.access.limitRefused', { reason: errors.form })
                : errors.form}
            </Notice>
          )}
          <div className={styles.columns}>
            <div className={styles.fields}>
              <RadioGroup
                name="maxRating"
                label={t('admin.accounts.access.rating')}
                value={value.rating}
                onChange={(rating) => set({ rating })}
                options={ratingOptions(t, ratingSystem, 18, account.maxRating)}
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
          <fieldset className={styles.fieldset}>
            <legend>{t('admin.accounts.wishes.title')}</legend>
            <SeerrUserLine account={account} />
            <Switch
              name="canRequest"
              isSelected={requestShown}
              isDisabled={requestLocked}
              onChange={(canRequest) => set({ canRequest })}
            >
              {t('admin.accounts.wishes.canRequest')}
            </Switch>
            <Text tone="secondary" variant="caption">
              {t(
                restricted
                  ? 'admin.accounts.wishes.canRequestRestricted'
                  : crossing
                    ? 'admin.accounts.wishes.canRequestReturns'
                    : 'admin.accounts.wishes.canRequestHelp',
              )}
            </Text>
            {requestShown !== stored.canRequest && <RequestRightHint account={account} />}
            {value.wishLimit && account.wishLimit && (
              <WishLimitFields
                value={value.wishLimit}
                onChange={(wishLimit) => set({ wishLimit })}
                defaults={wishSettings.data?.defaultLimit}
                locked={
                  account.wishLimit.seerrAdmin && (
                    <Text tone="secondary" variant="caption">
                      {t('admin.accounts.wishes.adminLimit', {
                        name: account.seerrUser?.displayName ?? '',
                      })}
                    </Text>
                  )
                }
                hint={
                  account.seerrUser === undefined || account.seerrUser.state === 'waiting' ? (
                    <Text tone="secondary" variant="caption">
                      {t('admin.accounts.wishes.limitLater')}
                    </Text>
                  ) : undefined
                }
              />
            )}
          </fieldset>
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
