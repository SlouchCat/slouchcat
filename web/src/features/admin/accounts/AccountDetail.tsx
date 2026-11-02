import type { AdminAccount } from '@slouchcat/api-client';
import { useLocation } from '@tanstack/react-router';
import { useEffect, useRef, type ReactNode, type RefObject } from 'react';
import { useTranslation } from 'react-i18next';
import { Chip, Skeleton, Text } from '@/ui';
import { AriaButton } from '@/ui/aria';
import { ProfileList } from '../../profiles/ProfileList';
import type { LibraryOptions } from '../../shared/useLibraryOptions';
import { AccessForm, type AccessDraft } from './AccessForm';
import { OwnAccountForm, type OwnAccountDraft } from './OwnAccountForm';
import styles from './Accounts.module.css';

/** The fragment of the sidebar entry «Profile» (`/admin/accounts?item=me#meine-profile`). */
export const MY_PROFILES = 'meine-profile';

/** A draft of one form in a fold-out, held by the page (S-17). */
export interface DraftProps<T> {
  value: T;
  dirty: boolean;
  onChange: (next: T) => void;
  onDiscard: () => void;
}

/**
 * The fold-out of the own account (KON-3, KP-1): «Konto ändern» inline, then «Meine Profile» with
 * every profile action (PRO-1 to PRO-3). The sidebar's «Profile» opens it here; the page scrolls
 * to the block when the address names it.
 */
export function OwnAccountDetail({
  account,
  draft,
  onSaved,
  titleRef,
  hold,
  onProfilesChanged,
}: {
  account: AdminAccount;
  draft: DraftProps<OwnAccountDraft>;
  onSaved: () => void;
  titleRef: RefObject<HTMLHeadingElement | null>;
  hold: boolean;
  onProfilesChanged: () => void;
}) {
  const { t } = useTranslation();
  const { hash } = useLocation();
  const profiles = useRef<HTMLElement>(null);
  useEffect(() => {
    if (hash === MY_PROFILES) profiles.current?.scrollIntoView({ block: 'start' });
  }, [hash]);
  return (
    <div className={styles.detail}>
      <OwnAccountForm email={account.email} onSaved={onSaved} {...draft} />
      <section
        ref={profiles}
        id={MY_PROFILES}
        className={styles.block}
        aria-labelledby="my-profiles-heading"
      >
        <ProfileList
          titleRef={titleRef}
          nameHeadings={false}
          hold={hold}
          onChanged={onProfilesChanged}
        >
          {({ add, list }) => (
            <>
              <div className={styles.blockHead}>
                <h3 id="my-profiles-heading" className={styles.blockTitle}>
                  {t('admin.accounts.profiles.mine')}
                </h3>
                {add}
              </div>
              {list}
            </>
          )}
        </ProfileList>
      </section>
    </div>
  );
}

/**
 * The fold-out of another account (KON-4): «Zugriff» inline for an account that administers
 * nothing, the hint for an administrator, the profile names to read (their profiles are the
 * account's own business, `x-visibility: account`), then appoint or demote (owner only) and
 * remove. The buttons stand in the same place for both roles, so a role change keeps the focus on
 * the button that asked for it.
 */
export function OtherAccountDetail({
  account,
  isOwner,
  libraries,
  access,
  onAccessSaved,
  onRole,
  onRemove,
}: {
  account: AdminAccount;
  /** The signed-in account is the owner: it alone appoints and demotes (ADR 0014). */
  isOwner: boolean;
  libraries: LibraryOptions;
  access: DraftProps<AccessDraft> | undefined;
  onAccessSaved: () => Promise<void>;
  onRole: (account: AdminAccount) => void;
  onRemove: (account: AdminAccount) => void;
}) {
  const { t } = useTranslation();
  const demote = account.role === 'admin';
  let first: ReactNode;
  if (account.role !== 'user') {
    first = <Text tone="secondary">{t('admin.accounts.access.adminHint')}</Text>;
  } else if (access && libraries.isReady) {
    first = (
      <AccessForm
        account={account}
        options={libraries.options}
        onSaved={onAccessSaved}
        {...access}
      />
    );
  } else {
    // Without the library list the form is not offered: a missing list never widens a right.
    first = libraries.isPending ? <Skeleton shape="block" /> : null;
  }
  return (
    <div className={styles.detail}>
      {first}
      <section
        className={styles.block}
        aria-label={t('admin.accounts.profiles.of', { email: account.email })}
      >
        <h3 className={styles.blockTitle}>{t('admin.accounts.columns.profiles')}</h3>
        <ul className={styles.chips}>
          {account.profiles.map((profile) => (
            <li key={profile.id}>
              <Chip tone="outline">
                {profile.isPrimary
                  ? t('admin.accounts.primaryProfile', { name: profile.name })
                  : profile.name}
              </Chip>
            </li>
          ))}
        </ul>
        <Text tone="secondary" variant="caption">
          {t('admin.accounts.profiles.readOnly')}
        </Text>
      </section>
      {/* The owner is protected: no role change and no removal (ADR 0014). */}
      {!account.isOwner && (
        <div className={styles.actions}>
          {isOwner && (
            <AriaButton
              variant="secondary"
              aria-label={t(
                demote ? 'admin.accounts.role.demote.label' : 'admin.accounts.role.promote.label',
                { email: account.email },
              )}
              onPress={() => onRole(account)}
            >
              {t(
                demote ? 'admin.accounts.role.demote.action' : 'admin.accounts.role.promote.action',
              )}
            </AriaButton>
          )}
          <AriaButton
            variant="danger"
            icon="trash"
            aria-label={t('admin.accounts.remove.label', { email: account.email })}
            onPress={() => onRemove(account)}
          >
            {t('admin.accounts.remove.action')}
          </AriaButton>
        </div>
      )}
    </div>
  );
}
