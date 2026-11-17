import type { TFunction } from 'i18next';
import { useTranslation } from 'react-i18next';
import { Chip } from '@/ui';
import type { DataColumn } from '@/ui/aria';
import type { SeerrUser, WishAccountMapping } from '../queries';
import { quotaShort } from '../../shared/quota';
import styles from './Wishes.module.css';

/** The chip of how an account came to its Seerr user (or that it has none). */
export const stateTone = {
  created: 'success',
  linked: 'success',
  manual: 'success',
  waiting: 'warning',
  none: 'outline',
} as const satisfies Record<WishAccountMapping['state'], string>;

/** The type of a Seerr user (1 Plex, 2 local, 3 Jellyfin, 4 Emby). */
const USER_TYPES = { 1: 'plex', 2: 'local', 3: 'jellyfin', 4: 'emby' } as const satisfies Record<
  SeerrUser['userType'],
  string
>;

export const userTypeText = (t: TFunction, user: SeerrUser) =>
  t(`admin.wishes.users.type.${USER_TYPES[user.userType]}`);

/** What a Seerr user may do, in a few words: the table's cell. */
export function rightsText(t: TFunction, user: SeerrUser): string {
  if (user.isAdmin) return t('admin.wishes.users.rights.admin');
  if (user.canRequestMovies && user.canRequestShows) return t('admin.wishes.users.rights.both');
  if (user.canRequestMovies) return t('admin.wishes.users.rights.movies');
  if (user.canRequestShows) return t('admin.wishes.users.rights.shows');
  return t('admin.wishes.users.rights.none');
}

/** The state chip of an account: «Benutzer gelöscht» when Seerr no longer knows its user. */
export function StateChip({ mapping, user }: { mapping: WishAccountMapping; user?: SeerrUser }) {
  const { t } = useTranslation();
  if (user?.gone) return <Chip tone="danger">{t('admin.wishes.users.gone')}</Chip>;
  return (
    <Chip tone={stateTone[mapping.state]}>{t(`admin.wishes.users.state.${mapping.state}`)}</Chip>
  );
}

/**
 * The columns of the table «Seerr-Benutzer» (M-24): the account's e-mail folds the row out; its
 * Seerr user with the state chip, the password state, the rights in Seerr and what the limit has
 * used, one line each.
 * Without a user the cells say what follows from the account.
 */
export function seerrUserColumns(
  t: TFunction,
  userOf: (mapping: WishAccountMapping) => SeerrUser | undefined,
): DataColumn<WishAccountMapping>[] {
  return [
    {
      key: 'account',
      header: t('admin.wishes.users.columns.account'),
      rowHeader: true,
      cell: (mapping) => mapping.email,
    },
    {
      key: 'user',
      header: t('admin.wishes.users.columns.user'),
      cell: (mapping) => {
        const user = userOf(mapping);
        return (
          <span className={styles.chips}>
            {user ? (
              <span className={styles.clip}>{user.displayName}</span>
            ) : (
              <span className={styles.muted}>{t('admin.wishes.users.noUser')}</span>
            )}
            <StateChip mapping={mapping} user={user} />
          </span>
        );
      },
    },
    {
      key: 'password',
      header: t('admin.wishes.users.columns.password'),
      cell: (mapping) =>
        mapping.passwordState && userOf(mapping) ? (
          <span className={`${styles.clip} ${styles.narrow}`}>
            {t(`admin.wishes.users.password.${mapping.passwordState}`)}
          </span>
        ) : (
          <span className={styles.muted}>{t('admin.wishes.users.passwordFollows')}</span>
        ),
    },
    {
      key: 'rights',
      header: t('admin.wishes.users.columns.rights'),
      cell: (mapping) => {
        const user = userOf(mapping);
        return user ? (
          <span className={styles.nowrap}>{rightsText(t, user)}</span>
        ) : (
          <span className={styles.muted}>{t('admin.wishes.users.rightsFollow')}</span>
        );
      },
    },
    {
      key: 'limit',
      header: t('admin.wishes.users.columns.limit'),
      cell: (mapping) => {
        if (mapping.limit.seerrAdmin || mapping.quota?.reason === 'admin') {
          return <span className={styles.nowrap}>{t('admin.wishes.users.noLimit')}</span>;
        }
        if (mapping.limit.mode === 'unlimited' || mapping.quota?.reason === 'no_limit') {
          return <span className={styles.nowrap}>{t('admin.wishes.users.unlimited')}</span>;
        }
        const used = quotaShort(t, mapping.quota);
        return used.length > 0 ? (
          <span className={styles.factStack}>
            {used.map((line) => (
              <span key={line} className={styles.nowrap}>
                {line}
              </span>
            ))}
          </span>
        ) : (
          <span className={styles.muted}>{t(`admin.wishes.users.mode.${mapping.limit.mode}`)}</span>
        );
      },
    },
  ];
}
