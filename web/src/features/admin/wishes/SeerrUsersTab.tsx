import { useMutation, useQueryClient, type UseQueryResult } from '@tanstack/react-query';
import { useMemo, type ReactNode } from 'react';
import { useTranslation } from 'react-i18next';
import { isApiError } from '@/api/client';
import { problemMessage } from '@/api/errors';
import { paths } from '@/app/paths';
import { intlLocale } from '@/i18n';
import { formatDate, formatDateTime } from '@/lib/format';
import {
  Chip,
  FactList,
  Notice,
  SelectField,
  Skeleton,
  Text,
  TextLink,
  VisuallyHidden,
} from '@/ui';
import {
  AriaButton,
  BandPanel,
  DataTable,
  StatusRegion,
  useStatusMessage,
  type DataTableExpand,
} from '@/ui/aria';
import {
  refreshSeerrUsers,
  setAccountWishMapping,
  wishKeys,
  wishMappingsQuery,
  type SeerrStatus,
  type SeerrUser,
  type WishAccountMapping,
  type WishMappings,
  type WishSettings,
} from '../queries';
import { rightsText, seerrUserColumns, StateChip, userTypeText } from './columns';
import { limitPart } from './SettingsTab';
import { useImmediateSave, type ImmediateSave } from './useImmediateSave';
import styles from './Wishes.module.css';

interface MappingChange {
  seerrUserId: number | null;
}

/** The fold-out of an account (M-24): its Seerr user in facts, the warnings, the choice. */
function AccountDetail({
  mapping,
  user,
  users,
  save,
  stale,
}: {
  mapping: WishAccountMapping;
  user: SeerrUser | undefined;
  users: readonly SeerrUser[];
  save: ImmediateSave<MappingChange>;
  /** The list of the users is the last one read (Seerr does not answer). */
  stale: boolean;
}) {
  const { t, i18n } = useTranslation();
  const language = intlLocale(i18n.language);
  const key = String(mapping.accountId);
  const pending = save.pending.get(key);
  const chosen = pending ? pending.seerrUserId : (mapping.seerrUserId ?? null);
  const failure = save.failure?.key === key ? save.failure.error : null;

  const options = [
    ...(chosen === null
      ? [{ value: '', label: t('admin.wishes.users.choose'), disabled: true }]
      : []),
    ...users
      .filter((candidate) => !candidate.shared)
      .map((candidate) => {
        const mine = candidate.id === chosen;
        const owner = candidate.mappedTo;
        const taken = owner !== undefined && owner.accountId !== mapping.accountId;
        const base = t('admin.wishes.users.option', {
          name: candidate.displayName,
          id: candidate.id,
        });
        const label = mine
          ? t('admin.wishes.users.optionCurrent', { option: base })
          : candidate.gone
            ? t('admin.wishes.users.optionGone', { option: base })
            : taken
              ? t('admin.wishes.users.optionTaken', { option: base, email: owner.email })
              : base;
        return { value: String(candidate.id), label, disabled: !mine && (taken || candidate.gone) };
      }),
  ];

  const facts: [string, ReactNode][] = [
    [
      t('admin.wishes.users.facts.user'),
      user ? (
        <span className={styles.factStack}>
          <span>{t('admin.wishes.users.option', { name: user.displayName, id: user.id })}</span>
          {user.email && (
            <span className={styles.secondary}>
              {/* A long address wraps after the @, not inside a word. */}
              {user.email.split('@')[0]}@<wbr />
              {user.email.split('@').slice(1).join('@')}
            </span>
          )}
          <span className={styles.secondary}>{userTypeText(t, user)}</span>
        </span>
      ) : (
        t('admin.wishes.users.noUser')
      ),
    ],
    [
      t('admin.wishes.users.facts.state'),
      <span key="state" className={styles.chips}>
        <StateChip mapping={mapping} user={user} />
        {mapping.linkedAt && (
          <span className={styles.secondary}>
            {mapping.linkedBy
              ? t('admin.wishes.users.linkedBy', {
                  by: t(`admin.wishes.users.by.${mapping.linkedBy}`),
                  date: formatDate(mapping.linkedAt, language),
                })
              : t('admin.wishes.users.since', { date: formatDate(mapping.linkedAt, language) })}
          </span>
        )}
      </span>,
    ],
    [
      t('admin.wishes.users.facts.password'),
      user && mapping.passwordState ? (
        <span className={styles.factStack}>
          <span>{t(`admin.wishes.users.password.${mapping.passwordState}`)}</span>
          {mapping.passwordState === 'synced' && mapping.passwordSyncedAt && (
            <span className={styles.secondary}>
              {t('admin.wishes.users.syncedAt', {
                time: formatDateTime(mapping.passwordSyncedAt, language),
              })}
            </span>
          )}
        </span>
      ) : (
        t('admin.wishes.users.passwordFollows')
      ),
    ],
    [
      t('admin.wishes.users.facts.rights'),
      user ? (
        <span className={styles.chips}>
          <Chip
            tone={user.canRequestMovies ? 'success' : 'outline'}
            icon={user.canRequestMovies ? 'check' : 'close'}
          >
            {t('admin.wishes.users.movies')}
          </Chip>
          <Chip
            tone={user.canRequestShows ? 'success' : 'outline'}
            icon={user.canRequestShows ? 'check' : 'close'}
          >
            {t('admin.wishes.users.shows')}
          </Chip>
          <Chip tone={user.autoApprove ? 'info' : 'outline'}>
            {t(
              user.autoApprove
                ? 'admin.wishes.users.autoApproveOn'
                : 'admin.wishes.users.autoApproveOff',
            )}
          </Chip>
          {user.isAdmin && <Chip tone="info">{t('admin.wishes.users.rights.admin')}</Chip>}
        </span>
      ) : (
        t(
          mapping.canRequest
            ? 'admin.wishes.users.followRightOn'
            : 'admin.wishes.users.followRightOff',
        )
      ),
    ],
    [
      t('admin.wishes.users.facts.limit'),
      mapping.limit.seerrAdmin ? (
        t('admin.wishes.users.noLimitAdmin')
      ) : mapping.limit.mode === 'unlimited' ? (
        t('admin.wishes.users.unlimited')
      ) : (
        <span className={styles.factStack}>
          <span>{limitPart(t, mapping.limit.effective, 'movie')}</span>
          <span>{limitPart(t, mapping.limit.effective, 'tv')}</span>
          <span className={styles.secondary}>
            {t(`admin.wishes.users.mode.${mapping.limit.mode}`)}
          </span>
        </span>
      ),
    ],
    [
      t('admin.wishes.users.facts.right'),
      t(mapping.canRequest ? 'admin.wishes.users.mayRequest' : 'admin.wishes.users.mayNotRequest'),
    ],
  ];

  // Rights warnings only for users that existed before: an opened user follows the account.
  const warning =
    user && !user.gone && mapping.state !== 'created' && !user.isAdmin
      ? !user.canRequestMovies && !user.canRequestShows
        ? 'nothing'
        : !user.canRequestShows
          ? 'noShows'
          : !user.canRequestMovies
            ? 'noMovies'
            : null
      : null;

  return (
    <div className={styles.detail}>
      {user?.gone && (
        <Notice
          tone="danger"
          live={false}
          title={t('admin.wishes.users.goneTitle', { name: user.displayName })}
        >
          {t('admin.wishes.users.goneBody')}
        </Notice>
      )}
      {mapping.state === 'linked' &&
        user &&
        (mapping.linkedBy === 'invite' || mapping.linkedBy === 'email_change') && (
          <Notice tone="info" live={false}>
            {t(`admin.wishes.users.selfLinked.${mapping.linkedBy}`, { name: user.displayName })}
          </Notice>
        )}
      {warning && (
        <Notice tone="warning" live={false}>
          {t(`admin.wishes.users.warning.${warning}`, { name: user?.displayName ?? '' })}
        </Notice>
      )}
      {mapping.provisionError && (
        <Notice tone="warning" live={false}>
          {t('admin.wishes.users.provisionError', { error: mapping.provisionError })}
        </Notice>
      )}
      <FactList facts={facts} />
      <section className={styles.block} aria-labelledby={`seerr-user-${key}`}>
        <h3 id={`seerr-user-${key}`} className={styles.blockTitle}>
          {t('admin.wishes.users.change')}
        </h3>
        {/* The actions of the account's Seerr user, at once and without a dialog (design rule
            1.3). «In Seerr eröffnen» and the password mode join this row with their operations. */}
        <div className={styles.actions}>
          <SelectField
            label={t('admin.wishes.users.chooseLabel', { email: mapping.email })}
            hideLabel
            value={chosen === null ? '' : String(chosen)}
            options={options}
            onChange={(value) =>
              save.change(key, { seerrUserId: value === '' ? null : Number(value) })
            }
          />
          {mapping.seerrUserId !== undefined && (
            <AriaButton
              variant="dangerQuiet"
              isDisabled={pending !== undefined}
              onPress={() => save.change(key, { seerrUserId: null })}
            >
              {t('admin.wishes.users.unlink')}
            </AriaButton>
          )}
        </div>
        <Text variant="caption" tone="secondary">
          {stale ? t('admin.wishes.users.chooseHelpStale') : t('admin.wishes.users.chooseHelp')}
        </Text>
        {failure ? (
          <Notice tone="danger">
            {isApiError(failure, 409) ? t('admin.wishes.users.taken') : problemMessage(t, failure)}
          </Notice>
        ) : null}
      </section>
    </div>
  );
}

/**
 * Medien › Wünsche › Seerr-Benutzer (concept 25, M-24, administration 6): one row per account
 * with its Seerr user, the state chip, the password state and the rights in Seerr; the row folds
 * out the facts and the choice of the user by hand, which takes effect at once
 * (useImmediateSave; a user taken meanwhile answers 409 at the row and the choice springs back).
 * Above: the hints (Seerr not set up, another instance, deleted users, accounts without a user),
 * what applies to new accounts, «Benutzerliste neu laden»; below: the Seerr users without an
 * account.
 */
export function SeerrUsersTab({
  mappings,
  users,
  settings,
  status,
  expanded,
  onExpandedChange,
  onReloaded,
}: {
  mappings: UseQueryResult<WishMappings>;
  users: UseQueryResult<SeerrUser[]>;
  settings: WishSettings | undefined;
  status: SeerrStatus | undefined;
  expanded: ReadonlySet<string>;
  onExpandedChange: DataTableExpand<WishAccountMapping>['onExpandedChange'];
  /** The refresh was queued: the page reads the lists again for a while. */
  onReloaded: () => void;
}) {
  const { t } = useTranslation();
  const queryClient = useQueryClient();
  const [reloadMessage, sayReload] = useStatusMessage();
  const byId = useMemo(
    () => new Map((users.data ?? []).map((user) => [user.id, user])),
    [users.data],
  );
  const userOf = (mapping: WishAccountMapping) =>
    mapping.seerrUserId === undefined ? undefined : byId.get(mapping.seerrUserId);

  const save = useImmediateSave<MappingChange, WishAccountMapping>({
    save: async ({ seerrUserId }, key) => {
      try {
        return await setAccountWishMapping(Number(key), seerrUserId);
      } catch (error) {
        // Taken meanwhile by another account, or gone: the lists are read again.
        if (isApiError(error, 409)) void queryClient.invalidateQueries({ queryKey: wishKeys.all });
        throw error;
      }
    },
    saved: async (next, { seerrUserId }) => {
      const before = mappings.data?.accounts.find((entry) => entry.accountId === next.accountId);
      queryClient.setQueryData(
        wishMappingsQuery.queryKey,
        (current: WishMappings | undefined) =>
          current && {
            ...current,
            accounts: current.accounts.map((entry) =>
              entry.accountId === next.accountId ? next : entry,
            ),
          },
      );
      await queryClient.invalidateQueries({ queryKey: wishKeys.all });
      const name = (id: number | undefined) =>
        (id === undefined ? undefined : byId.get(id)?.displayName) ?? '';
      return seerrUserId === null
        ? t('admin.wishes.users.unlinked', {
            email: next.email,
            name: name(before?.seerrUserId),
          })
        : t('admin.wishes.users.linked', { email: next.email, name: name(seerrUserId) });
    },
  });
  const reload = useMutation({
    mutationFn: refreshSeerrUsers,
    onMutate: () => sayReload(null),
    onSuccess: () => {
      sayReload(t('admin.wishes.users.reloading'));
      onReloaded();
    },
  });

  const columns = seerrUserColumns(t, userOf);
  const configured = status !== undefined && status.state !== 'not_configured';
  const list = mappings.data;
  const accounts = list?.accounts ?? [];
  const gone = accounts.filter((mapping) => userOf(mapping)?.gone).length;
  const without = accounts.filter(
    (mapping) => mapping.canRequest && mapping.seerrUserId === undefined,
  ).length;
  const shared = list?.sharedUser;
  const toSettings = `${paths.adminWishes}?tab=einstellungen`;
  const toConnection = `${paths.adminIntegrations}?item=seerr`;
  const stale = status !== undefined && status.state !== 'connected';

  return (
    <div className={styles.tabBody}>
      <VisuallyHidden as="h2">{t('admin.wishes.tabs.users')}</VisuallyHidden>
      <StatusRegion message={save.failure ? null : save.message} />
      {status && !configured && (
        <Notice
          tone="info"
          title={t('admin.wishes.users.notConfigured')}
          actions={<TextLink href={toConnection}>{t('admin.wishes.users.toConnection')}</TextLink>}
        >
          {t('admin.wishes.users.notConfiguredBody')}
        </Notice>
      )}
      {status?.instanceChanged && (
        <Notice
          tone="danger"
          title={t('admin.integrations.seerr.instance.title')}
          actions={<TextLink href={toConnection}>{t('admin.wishes.users.toDecision')}</TextLink>}
        >
          {t('admin.wishes.users.instanceBody')}
        </Notice>
      )}
      {gone > 0 && (
        <Notice tone="danger" title={t('admin.wishes.users.goneCount', { count: gone })}>
          {t('admin.wishes.users.goneCountBody')}
        </Notice>
      )}
      {configured && without > 0 && settings && (
        <Notice
          tone="warning"
          title={t('admin.wishes.users.withoutCount', { count: without })}
          actions={<TextLink href={toSettings}>{t('admin.wishes.users.toSettings')}</TextLink>}
        >
          {t(
            settings.unmapped === 'fallback'
              ? 'admin.wishes.users.withoutFallback'
              : 'admin.wishes.users.withoutDeny',
          )}
        </Notice>
      )}
      <div className={styles.bar}>
        {settings && (
          <Text tone="secondary" variant="caption" as="span">
            {t('admin.wishes.users.newAccounts', {
              provision: t(
                settings.provisionAccounts
                  ? 'admin.wishes.users.provisionOn'
                  : 'admin.wishes.users.provisionOff',
              ),
              password: t(
                settings.syncPassword ? 'admin.wishes.users.syncOn' : 'admin.wishes.users.syncOff',
              ),
            })}{' '}
            <TextLink href={toSettings}>{t('admin.wishes.users.changeSettings')}</TextLink>
          </Text>
        )}
        {settings?.unmapped === 'fallback' && (
          <Text tone="secondary" variant="caption" as="span">
            {shared
              ? t('admin.wishes.users.shared', { name: shared.displayName })
              : t('admin.wishes.users.sharedLater')}
          </Text>
        )}
        <AriaButton
          icon="refresh"
          isPending={reload.isPending}
          isDisabled={!configured}
          onPress={() => reload.mutate()}
        >
          {t('admin.wishes.users.reload')}
        </AriaButton>
      </div>
      {reload.error ? (
        <Notice tone="danger">{problemMessage(t, reload.error)}</Notice>
      ) : (
        <StatusRegion message={reloadMessage} />
      )}
      {(mappings.isPending || users.isPending) && <Skeleton shape="block" />}
      {mappings.isError && (
        <Notice
          tone="danger"
          title={problemMessage(t, mappings.error)}
          actions={
            <AriaButton icon="refresh" onPress={() => void mappings.refetch()}>
              {t('common.retry')}
            </AriaButton>
          }
        />
      )}
      {users.isError && <Notice tone="danger" title={problemMessage(t, users.error)} />}
      {list && users.data && (
        <>
          {/* The first work row of the page (docs/perf/verwaltungsseiten.md). */}
          <div data-perf="first-work-row">
            <DataTable
              caption={t('admin.wishes.users.caption')}
              columns={columns}
              rows={accounts}
              rowKey={(mapping) => String(mapping.accountId)}
              expand={{
                render: (mapping) => (
                  <AccountDetail
                    mapping={mapping}
                    user={userOf(mapping)}
                    users={users.data}
                    save={save}
                    stale={stale}
                  />
                ),
                expandedKeys: expanded,
                onExpandedChange,
              }}
            />
          </div>
          <BandPanel
            title={t('admin.wishes.users.unlinkedTitle')}
            icon="user"
            collapsible
            meta={
              <span>
                {t('admin.wishes.users.unlinkedCount', { count: list.unlinkedUsers.length })}
              </span>
            }
          >
            {list.unlinkedUsers.length === 0 ? (
              <Text tone="secondary">{t('admin.wishes.users.unlinkedEmpty')}</Text>
            ) : (
              <DataTable
                caption={t('admin.wishes.users.unlinkedTitle')}
                rows={list.unlinkedUsers}
                rowKey={(user) => String(user.id)}
                columns={[
                  {
                    key: 'name',
                    header: t('admin.wishes.users.columns.user'),
                    rowHeader: true,
                    cell: (user) =>
                      t('admin.wishes.users.option', { name: user.displayName, id: user.id }),
                  },
                  {
                    key: 'email',
                    header: t('admin.wishes.users.columns.email'),
                    cell: (user) =>
                      user.email ?? (
                        <span className={styles.muted}>{t('admin.wishes.users.noEmail')}</span>
                      ),
                  },
                  {
                    key: 'type',
                    header: t('admin.wishes.users.columns.type'),
                    cell: (user) => userTypeText(t, user),
                  },
                  {
                    key: 'rights',
                    header: t('admin.wishes.users.columns.rights'),
                    cell: (user) =>
                      user.gone ? (
                        <Chip tone="danger">{t('admin.wishes.users.gone')}</Chip>
                      ) : (
                        rightsText(t, user)
                      ),
                  },
                ]}
              />
            )}
          </BandPanel>
        </>
      )}
    </div>
  );
}
