import { useMutation, useQueryClient, type UseQueryResult } from '@tanstack/react-query';
import type { TFunction } from 'i18next';
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
  Icon,
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
  type Drafts,
} from '@/ui/aria';
import {
  dashboardQuery,
  provisionSeerrAccount,
  refreshSeerrUsers,
  setAccountWishMapping,
  setSeerrPasswordMode,
  wishKeys,
  wishMappingsQuery,
  type SeerrPasswordMode,
  type SeerrStatus,
  type SeerrUser,
  type WishAccountMapping,
  type WishMappings,
  type WishSettings,
} from '../queries';
import { rightsText, seerrUserColumns, userTypeText } from './columns';
import { QuotaLines } from '../../shared/quota';
import { limitSummary } from '../../shared/WishLimitFields';
import { useImmediateSave, type ImmediateSave } from './useImmediateSave';
import { ProfileTags, TakeOver } from './ProfileTags';
import styles from './Wishes.module.css';

interface MappingChange {
  seerrUserId: number | null;
}

/** «In Seerr eröffnen» (or «Erneut versuchen») and the password mode of one account. */
type RowAction = { kind: 'provision' } | { kind: 'password'; mode: SeerrPasswordMode };

/** The reasons an account waits for Seerr that have words of their own (`provisionError`). */
const WAIT_REASONS = [
  'unreachable',
  'server',
  'timeout',
  'key_invalid',
  'instance_changed',
  'seerr_user_taken',
] as const;

type WaitReason = (typeof WAIT_REASONS)[number];
const isWaitReason = (reason: string): reason is WaitReason =>
  (WAIT_REASONS as readonly string[]).includes(reason);

/**
 * Why the account waits, in words. The server sends the class of the error, at times followed by
 * Seerr's own message (`server: …`); an unknown class is named as it came.
 */
function waitText(t: TFunction, reason: string | undefined): string {
  if (reason === undefined) return t('admin.wishes.users.waitReason.unreachable');
  const code = reason.split(':')[0]?.trim() ?? reason;
  return isWaitReason(code)
    ? t(`admin.wishes.users.waitReason.${code}`)
    : t('admin.wishes.users.waitReason.other', { reason });
}

/** The same e-mail in any spelling. */
const sameEmail = (a: string | undefined, b: string) =>
  a !== undefined && a.toLowerCase() === b.toLowerCase();

/**
 * The Seerr user «In Seerr eröffnen» would link for an account without one: a user with the same
 * e-mail that belongs to nobody (the server decides; this only names it beforehand).
 */
export function linkCandidate(
  mapping: WishAccountMapping,
  users: readonly SeerrUser[],
): SeerrUser | undefined {
  if (mapping.seerrUserId !== undefined) return undefined;
  return users.find(
    (user) =>
      !user.shared &&
      !user.gone &&
      user.mappedTo === undefined &&
      sameEmail(user.email, mapping.email),
  );
}

/** The fold-out of an account (M-24): its Seerr user in facts, the warnings, the choice. */
function AccountDetail({
  mapping,
  user,
  users,
  save,
  actions,
  settings,
  configured,
  stale,
  tagDrafts,
}: {
  mapping: WishAccountMapping;
  user: SeerrUser | undefined;
  users: readonly SeerrUser[];
  save: ImmediateSave<MappingChange>;
  actions: ImmediateSave<RowAction>;
  settings: WishSettings | undefined;
  /** Seerr has an address and a key: «In Seerr eröffnen» can reach it. */
  configured: boolean;
  /** The list of the users is the last one read (Seerr does not answer). */
  stale: boolean;
  /** The tag names typed for the profiles (page drafts). */
  tagDrafts: Drafts<string>;
}) {
  const { t, i18n } = useTranslation();
  const language = intlLocale(i18n.language);
  const key = String(mapping.accountId);
  const pending = save.pending.get(key);
  const chosen = pending ? pending.seerrUserId : (mapping.seerrUserId ?? null);
  const failure = save.failure?.key === key ? save.failure.error : null;
  const acting = actions.pending.get(key);
  const actionFailure = actions.failure?.key === key ? actions.failure.error : null;
  const candidate = linkCandidate(mapping, users);
  const provisionOff = settings !== undefined && !settings.provisionAccounts;
  const toSettings = `${paths.adminWishes}?tab=einstellungen`;
  const toAccount = `${paths.adminAccounts}?item=${mapping.accountId}`;
  const ready = user !== undefined && mapping.seerrUserId !== undefined;
  const password = ready ? mapping.passwordState : undefined;
  const syncOff = settings !== undefined && !settings.syncPassword;

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
      <span key="state" className={styles.factStack}>
        {/* Words, not the row's chip: a long state wraps in its narrow column. */}
        <span>
          {user?.gone
            ? t('admin.wishes.users.gone')
            : t(`admin.wishes.users.state.${mapping.state}`)}
        </span>
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
      mapping.limit.seerrAdmin || mapping.quota?.reason === 'admin' ? (
        t('admin.wishes.users.noLimitAdmin')
      ) : !ready ? (
        <span className={styles.factStack}>
          <span>
            {t('admin.wishes.users.limitOnOpen', {
              limit:
                mapping.limit.mode === 'unlimited'
                  ? t('admin.wishes.users.unlimited')
                  : limitSummary(t, mapping.limit.effective),
            })}
          </span>
          <TextLink href={toAccount}>{t('admin.wishes.users.limitChange')}</TextLink>
        </span>
      ) : (
        <span className={styles.factStack}>
          {mapping.limit.mode === 'unlimited' || mapping.quota?.reason === 'no_limit' ? (
            <span>{t('admin.wishes.users.unlimited')}</span>
          ) : (
            <QuotaLines quota={mapping.quota} limits={mapping.limit.effective} />
          )}
          <span className={styles.secondary}>
            {t(`admin.wishes.users.mode.${mapping.limit.mode}`)}
            {mapping.quota?.at &&
              `${t('admin.wishes.separator')}${t('admin.wishes.users.quotaAt', {
                time: formatDateTime(mapping.quota.at, language),
              })}`}
          </span>
          <TextLink href={toAccount}>{t('admin.wishes.users.limitChange')}</TextLink>
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
      {mapping.state === 'waiting' && (
        <Notice tone="warning" live={false} title={t('admin.wishes.users.waitingTitle')}>
          {waitText(t, mapping.provisionError)}
        </Notice>
      )}
      {ready && mapping.provisionError === 'email_taken' && (
        <Notice tone="warning" live={false} title={t('admin.wishes.users.emailTakenTitle')}>
          {t('admin.wishes.users.emailTakenBody', { name: user.displayName })}
        </Notice>
      )}
      {candidate && (
        <Notice
          tone="info"
          live={false}
          title={t('admin.wishes.users.sameEmailTitle', { name: candidate.displayName })}
        >
          {t('admin.wishes.users.sameEmailBody')}
        </Notice>
      )}
      <FactList facts={facts} />
      <section className={styles.block} aria-labelledby={`seerr-user-${key}`}>
        <h3 id={`seerr-user-${key}`} className={styles.blockTitle}>
          {t('admin.wishes.users.change')}
        </h3>
        {/* The actions of the account's Seerr user, at once and without a dialog (design rule
            1.3): open it, choose another, unlink, and the password mode. */}
        <div className={styles.actions}>
          {mapping.seerrUserId === undefined && (
            <AriaButton
              variant="primary"
              icon={provisionOff ? 'lock' : mapping.state === 'waiting' ? 'refresh' : 'plus'}
              isDisabled={provisionOff || !configured || pending !== undefined}
              isPending={acting?.kind === 'provision'}
              onPress={() => actions.change(key, { kind: 'provision' })}
            >
              {mapping.state === 'waiting'
                ? t('admin.wishes.users.retry')
                : t('admin.wishes.users.provision')}
            </AriaButton>
          )}
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
          {(password === 'synced' || password === 'next_login') && (
            <AriaButton
              variant="ghost"
              icon="lock"
              isPending={acting?.kind === 'password'}
              onPress={() => actions.change(key, { kind: 'password', mode: 'off' })}
            >
              {t('admin.wishes.users.passwordOff')}
            </AriaButton>
          )}
          {password === 'off' && (
            <AriaButton
              variant="ghost"
              icon="lock"
              isDisabled={syncOff}
              isPending={acting?.kind === 'password'}
              onPress={() => actions.change(key, { kind: 'password', mode: 'next_login' })}
            >
              {t('admin.wishes.users.passwordNext')}
            </AriaButton>
          )}
        </div>
        {mapping.seerrUserId === undefined && provisionOff && (
          <p className={styles.lockedLine}>
            <Icon name="lock" size="sm" />
            <span>
              {t('admin.wishes.users.provisionOffHint')}{' '}
              <TextLink href={toSettings}>{t('admin.wishes.users.toSettings')}</TextLink>
            </span>
          </p>
        )}
        {password === 'off' && syncOff && (
          <p className={styles.lockedLine}>
            <Icon name="lock" size="sm" />
            <span>
              {t('admin.wishes.users.passwordSyncOff')}{' '}
              <TextLink href={toSettings}>{t('admin.wishes.users.toSettings')}</TextLink>
            </span>
          </p>
        )}
        {(password === 'admin' || password === 'external') && (
          <Text variant="caption" tone="secondary">
            {t(`admin.wishes.users.passwordNote.${password}`, { name: user?.displayName ?? '' })}
          </Text>
        )}
        {actionFailure ? <Notice tone="danger">{problemMessage(t, actionFailure)}</Notice> : null}
        <Text variant="caption" tone="secondary">
          {stale ? t('admin.wishes.users.chooseHelpStale') : t('admin.wishes.users.chooseHelp')}
        </Text>
        {failure ? (
          <Notice tone="danger">
            {isApiError(failure, 409) ? t('admin.wishes.users.taken') : problemMessage(t, failure)}
          </Notice>
        ) : null}
      </section>
      {ready && <TakeOver seerrUserId={user.id} name={user.displayName} />}
      <ProfileTags mapping={mapping} drafts={tagDrafts} tagsOn={settings?.profileTags ?? true} />
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
  onProvisionAll,
  provisionAllPending,
  provisionAllMessage,
  tagDrafts,
}: {
  mappings: UseQueryResult<WishMappings>;
  users: UseQueryResult<SeerrUser[]>;
  settings: WishSettings | undefined;
  status: SeerrStatus | undefined;
  expanded: ReadonlySet<string>;
  onExpandedChange: DataTableExpand<WishAccountMapping>['onExpandedChange'];
  /** The refresh was queued: the page reads the lists again for a while. */
  onReloaded: () => void;
  /** «Alle eröffnen»: the page asks first (its confirmation names the counts). */
  onProvisionAll: () => void;
  provisionAllPending: boolean;
  /** The outcome of «Alle eröffnen», or its refusal. */
  provisionAllMessage: ReactNode;
  /** The tag names typed for the profiles, kept by the page so leaving asks first. */
  tagDrafts: Drafts<string>;
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
  const actions = useImmediateSave<RowAction, WishAccountMapping>({
    save: (action, key) =>
      action.kind === 'provision'
        ? provisionSeerrAccount(Number(key))
        : setSeerrPasswordMode(Number(key), action.mode),
    saved: async (next, action) => {
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
      await Promise.all([
        queryClient.invalidateQueries({ queryKey: wishKeys.all }),
        queryClient.invalidateQueries({ queryKey: dashboardQuery.queryKey }),
      ]);
      if (action.kind === 'password') {
        return t(
          action.mode === 'off'
            ? 'admin.wishes.users.passwordOffDone'
            : 'admin.wishes.users.passwordNextDone',
          { email: next.email },
        );
      }
      if (next.state === 'waiting') {
        return t('admin.wishes.users.provisioned.waiting', { email: next.email });
      }
      const name = (next.seerrUserId === undefined ? undefined : byId.get(next.seerrUserId))
        ?.displayName;
      return next.state === 'linked' && name
        ? t('admin.wishes.users.provisioned.linked', { email: next.email, name })
        : t('admin.wishes.users.provisioned.created', { email: next.email });
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
  const todo = accounts.filter((mapping) => mapping.seerrUserId === undefined).length;
  const provisionOff = settings !== undefined && !settings.provisionAccounts;
  const toSettings = `${paths.adminWishes}?tab=einstellungen`;
  const toConnection = `${paths.adminIntegrations}?item=seerr`;
  const stale = status !== undefined && status.state !== 'connected';

  return (
    <div className={styles.tabBody}>
      <VisuallyHidden as="h2">{t('admin.wishes.tabs.users')}</VisuallyHidden>
      <StatusRegion message={save.failure ? null : save.message} />
      <StatusRegion message={actions.failure ? null : actions.message} />
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
          <span className={styles.sharedLine}>
            <span>
              {shared
                ? t('admin.wishes.users.shared', { name: shared.displayName })
                : t('admin.wishes.users.sharedLater')}
            </span>
            {shared && list?.sharedQuota && (
              <span>
                {list.sharedQuota.unlimited ? (
                  t('admin.wishes.users.unlimited')
                ) : (
                  <QuotaLines quota={list.sharedQuota} limits={settings.defaultLimit} separator />
                )}
              </span>
            )}
          </span>
        )}
        <span className={styles.actions}>
          <AriaButton
            variant="primary"
            icon={provisionOff ? 'lock' : 'plus'}
            isPending={provisionAllPending}
            isDisabled={!configured || provisionOff || todo === 0}
            onPress={onProvisionAll}
          >
            {t('admin.wishes.users.provisionAll')}
          </AriaButton>
          <AriaButton
            icon="refresh"
            isPending={reload.isPending}
            isDisabled={!configured}
            onPress={() => reload.mutate()}
          >
            {t('admin.wishes.users.reload')}
          </AriaButton>
        </span>
      </div>
      {configured && provisionOff && todo > 0 && (
        <p className={styles.lockedLine}>
          <Icon name="lock" size="sm" />
          <span>
            {t('admin.wishes.users.provisionOffHint')}{' '}
            <TextLink href={toSettings}>{t('admin.wishes.users.toSettings')}</TextLink>
          </span>
        </p>
      )}
      {provisionAllMessage}
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
                    actions={actions}
                    settings={settings}
                    configured={configured}
                    stale={stale}
                    tagDrafts={tagDrafts}
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
