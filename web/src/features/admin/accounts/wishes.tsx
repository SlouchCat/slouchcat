import type { AdminAccount } from '@slouchcat/api-client';
import { useMutation, useQueryClient } from '@tanstack/react-query';
import { useTranslation } from 'react-i18next';
import { problemMessage } from '@/api/errors';
import { paths } from '@/app/paths';
import { intlLocale } from '@/i18n';
import { formatDate } from '@/lib/format';
import { Chip, Icon, Notice, Text, TextLink } from '@/ui';
import { StatusRegion, Switch, useStatusMessage } from '@/ui/aria';
import { limitSummary } from '../../shared/WishLimitFields';
import { accountsQuery, dashboardQuery, setAccountCanRequest, wishKeys } from '../queries';
import styles from './Accounts.module.css';

/** The chip of the column «Wünsche»: right and a Seerr user, no right, right without a user. */
const wishTone = {
  allowed: 'success',
  not_allowed: 'outline',
  mapping_missing: 'warning',
} as const satisfies Record<NonNullable<AdminAccount['wishState']>, string>;

export function WishChip({ account }: { account: AdminAccount }) {
  const { t } = useTranslation();
  if (!account.wishState) {
    return <span className={styles.muted}>{t('admin.accounts.wishes.notSetUp')}</span>;
  }
  return (
    <Chip tone={wishTone[account.wishState]}>
      {t(`admin.accounts.wishes.state.${account.wishState}`)}
    </Chip>
  );
}

/** Where an account's Seerr user is changed: its row under Wünsche › Seerr-Benutzer, open. */
export const seerrUserHref = (account: AdminAccount) =>
  `${paths.adminWishes}?tab=seerr-benutzer&item=${account.id}`;

/**
 * «Seerr-Benutzer: grosseltern · eröffnet · Passwort abgeglichen · Ändern» (administration 8): to
 * read only; the user is changed or unlinked on the page of the Seerr users.
 */
export function SeerrUserLine({ account }: { account: AdminAccount }) {
  const { t, i18n } = useTranslation();
  const user = account.seerrUser;
  const separator = t('admin.wishes.separator');
  const parts = user
    ? [
        ...(user.displayName ? [user.displayName] : []),
        user.state === 'linked' && user.linkedBy
          ? t('admin.accounts.wishes.linkedBy', {
              by: t(`admin.wishes.users.by.${user.linkedBy}`),
              date: user.linkedAt ? formatDate(user.linkedAt, intlLocale(i18n.language)) : '',
            })
          : t(`admin.accounts.wishes.userState.${user.state}`),
        ...(user.passwordState ? [t(`admin.accounts.wishes.password.${user.passwordState}`)] : []),
      ]
    : [t('admin.accounts.wishes.noUser')];
  return (
    <p className={styles.line}>
      <Icon name="user" size="sm" />
      <span>
        {t('admin.accounts.wishes.userLine', { user: parts.join(separator) })}{' '}
        <TextLink href={seerrUserHref(account)}>{t('admin.accounts.wishes.change')}</TextLink>
      </span>
    </p>
  );
}

/** What a change of the request right does to the Seerr user (N-9). */
export function RequestRightHint({ account }: { account: AdminAccount }) {
  const { t } = useTranslation();
  const user = account.seerrUser;
  if (!user || user.state === 'waiting') return null;
  return (
    <Text tone="secondary" variant="caption">
      {t(
        user.state === 'created'
          ? 'admin.accounts.wishes.rightsFollow'
          : 'admin.accounts.wishes.rightsStay',
        { name: user.displayName ?? '' },
      )}
    </Text>
  );
}

/**
 * The block «Wünsche» of an administrator (administration 8): only the request right, at once;
 * the limit stays as it is, and a Seerr administrator has none.
 */
export function AdminWishesBlock({ account }: { account: AdminAccount }) {
  const { t } = useTranslation();
  const queryClient = useQueryClient();
  const [message, say] = useStatusMessage();
  const save = useMutation({
    mutationFn: (canRequest: boolean) => setAccountCanRequest(account.id, canRequest),
    onMutate: () => say(null),
    onSuccess: async (_data, canRequest) => {
      await Promise.all([
        queryClient.invalidateQueries({ queryKey: accountsQuery.queryKey }),
        queryClient.invalidateQueries({ queryKey: wishKeys.all }),
        queryClient.invalidateQueries({ queryKey: dashboardQuery.queryKey }),
      ]);
      say(
        t(canRequest ? 'admin.accounts.wishes.savedOn' : 'admin.accounts.wishes.savedOff', {
          email: account.email,
        }),
      );
    },
  });
  const selected = save.isPending ? (save.variables ?? false) : (account.canRequest ?? false);
  const limit = account.wishLimit;
  return (
    <section
      className={styles.block}
      aria-label={t('admin.accounts.wishes.of', { email: account.email })}
    >
      <h3 className={styles.blockTitle}>{t('admin.accounts.wishes.title')}</h3>
      <SeerrUserLine account={account} />
      <Switch name="canRequest" isSelected={selected} onChange={(next) => save.mutate(next)}>
        {t('admin.accounts.wishes.canRequest')}
      </Switch>
      <Text tone="secondary" variant="caption">
        {t('admin.accounts.wishes.canRequestHelp')}
      </Text>
      {limit && (
        <Text tone="secondary" variant="caption">
          {t('admin.accounts.wishes.limitFixed', {
            limit: limit.seerrAdmin
              ? t('admin.wishes.users.noLimitAdmin')
              : limit.mode === 'unlimited'
                ? t('admin.wishes.users.unlimited')
                : limitSummary(t, limit.effective),
          })}
        </Text>
      )}
      {save.error ? (
        <Notice tone="danger">{problemMessage(t, save.error)}</Notice>
      ) : (
        <StatusRegion message={message} />
      )}
    </section>
  );
}
