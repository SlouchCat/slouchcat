import type { AdminAccount } from '@slouchcat/api-client';
import type { TFunction } from 'i18next';
import type { ReactNode } from 'react';
import { formatDateTime } from '@/lib/format';
import { Chip } from '@/ui';
import type { DataColumn } from '@/ui/aria';
import styles from './Accounts.module.css';

/** The profile names of an account, the primary one marked, as one line. */
export const profileNames = (t: TFunction, account: AdminAccount) =>
  account.profiles
    .map((profile) =>
      profile.isPrimary ? t('admin.accounts.primaryProfile', { name: profile.name }) : profile.name,
    )
    .join(t('admin.accounts.listSeparator'));

/**
 * The columns of the account table (KON-2, mockup KP-1): the e-mail folds the row out; role with
 * the owner badge, rating, libraries, profiles (one line, the names in full in the fold-out), last
 * contact and the channels the account switched on; «Weitere Aktionen» of another account last.
 */
export function accountColumns(
  t: TFunction,
  language: string,
  actions: (account: AdminAccount) => ReactNode,
): DataColumn<AdminAccount>[] {
  return [
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
        <span className={styles.chips}>
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
      cell: (account) => <span className={styles.clip}>{profileNames(t, account)}</span>,
    },
    {
      key: 'lastSeen',
      header: t('admin.accounts.columns.lastSeen'),
      cell: (account) =>
        account.lastSeenAt ? (
          <time className={styles.nowrap} dateTime={account.lastSeenAt}>
            {formatDateTime(account.lastSeenAt, language)}
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
          <span className={styles.chips}>
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
      cell: actions,
    },
  ];
}
