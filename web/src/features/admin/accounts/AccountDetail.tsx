import type { AdminAccount } from '@slouchcat/api-client';
import { useQuery } from '@tanstack/react-query';
import { useLocation } from '@tanstack/react-router';
import { useEffect, useRef, type ReactNode, type RefObject } from 'react';
import { useTranslation } from 'react-i18next';
import { Chip, Skeleton, Text } from '@/ui';
import { ProfileList } from '../../profiles/ProfileList';
import { manageProfilesQuery } from '../../profiles/queries';
import type { LibraryOptions } from '../../shared/useLibraryOptions';
import { AccessForm, type AccessDraft } from './AccessForm';
import { OwnAccountForm, type OwnAccountDraft } from './OwnAccountForm';
import styles from './Accounts.module.css';

/** The fragment of «Meine Profile» (`/admin/accounts?item=me#meine-profile`). */
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
 * every profile action (PRO-1 to PRO-3). `?item=me#meine-profile` opens it here; the block scrolls
 * into view once per arrival at `#meine-profile`, when the profiles are there to scroll to.
 */
export function OwnAccountDetail({
  account,
  draft,
  onSaved,
  titleRef,
  hold,
  onHoldChange,
  claimScroll,
  onProfilesChanged,
}: {
  account: AdminAccount;
  draft: DraftProps<OwnAccountDraft>;
  /** `sent` is stored: the page drops the draft if it still says what was sent. */
  onSaved: (sent: OwnAccountDraft) => void;
  titleRef: RefObject<HTMLHeadingElement | null>;
  hold: boolean;
  /** The profile dialogs are open: the page holds its queries too. */
  onHoldChange: (held: boolean) => void;
  /**
   * True once for an arrival (history entry): the page remembers it over folds, so the block
   * scrolls once per arrival.
   */
  claimScroll: (arrival: string) => boolean;
  onProfilesChanged: () => void;
}) {
  const { t } = useTranslation();
  const location = useLocation();
  const arrival = location.state.__TSR_key ?? location.state.key ?? location.href;
  // A passive observer: the list asks, this only learns that the profiles are there.
  const profilesLoaded = useQuery({ ...manageProfilesQuery, enabled: false }).isSuccess;
  const profiles = useRef<HTMLElement>(null);
  useEffect(() => {
    if (location.hash !== MY_PROFILES || !profilesLoaded || !claimScroll(arrival)) return;
    profiles.current?.scrollIntoView({ block: 'start' });
  }, [location.hash, profilesLoaded, arrival, claimScroll]);
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
          onHoldChange={onHoldChange}
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
 * nothing, the hint for an administrator, and the profile names to read (their profiles are the
 * account's own business, `x-visibility: account`). Appoint, demote and remove are in the row's
 * «Weitere Aktionen» (S-6).
 */
export function OtherAccountDetail({
  account,
  libraries,
  access,
  onAccessSaved,
}: {
  account: AdminAccount;
  libraries: LibraryOptions;
  access: DraftProps<AccessDraft> | undefined;
  /** `sent` is stored: the page drops the draft if it still says what was sent. */
  onAccessSaved: (sent: AccessDraft) => Promise<void>;
}) {
  const { t } = useTranslation();
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
    </div>
  );
}
