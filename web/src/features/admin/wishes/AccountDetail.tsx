import type { TFunction } from 'i18next';
import { type ReactNode } from 'react';
import { useTranslation } from 'react-i18next';
import { isApiError } from '@/api/client';
import { problemMessage } from '@/api/errors';
import { paths } from '@/app/paths';
import { intlLocale } from '@/i18n';
import { formatDate, formatDateTime } from '@/lib/format';
import { Chip, FactList, Icon, Notice, SelectField, Text, TextLink } from '@/ui';
import { AriaButton, type Drafts } from '@/ui/aria';
import {
  type SeerrPasswordMode,
  type SeerrUser,
  type WishAccountMapping,
  type WishSettings,
} from '../queries';
import { userTypeText, waitsForChoice } from './columns';
import { QuotaLines } from '../../shared/quota';
import { limitSummary } from '../../shared/WishLimitFields';
import { type ImmediateSave } from './useImmediateSave';
import { ProfileTags, TakeOver } from './ProfileTags';
import styles from './Wishes.module.css';

export interface MappingChange {
  seerrUserId: number | null;
}

/** «In Seerr eröffnen» (or «Erneut versuchen») and the password mode of one account. */
export type RowAction =
  { kind: 'provision'; allowPrivileged?: boolean } | { kind: 'password'; mode: SeerrPasswordMode };

/** The reasons an account waits for Seerr that have words of their own (`provisionError`). */
const WAIT_REASONS = [
  'unreachable',
  'server',
  'timeout',
  'key_invalid',
  'instance_changed',
  'seerr_user_taken',
  'seerr_user_privileged',
] as const;

/** The classes of a refusal by Seerr that need no words of their own: one neutral sentence. */
const REFUSALS: readonly string[] = [
  'redirect',
  'access_denied',
  'csrf',
  'quota',
  'no_permission',
  'forbidden',
  'not_found',
  'conflict',
  'bad_request',
  'failed',
];

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
  if (isWaitReason(code)) return t(`admin.wishes.users.waitReason.${code}`);
  return REFUSALS.includes(code)
    ? t('admin.wishes.users.waitReason.refused')
    : t('admin.wishes.users.waitReason.other', { reason });
}

/** The settings tab, where a locked action is unlocked. */
export const TO_SETTINGS = `${paths.adminWishes}?tab=einstellungen`;

/**
 * A Seerr user that administers Seerr, as far as the list tells: user 1 and `isAdmin` (MANAGE_USERS,
 * which ADMIN implies). The server's rule is wider (every managing right) and asks again.
 */
export const administersSeerr = (user: SeerrUser) => user.id === 1 || user.isAdmin;

/** An account without a Seerr user: the strip, the tab's count and the warning count the same. */
export const withoutSeerrUser = (mapping: WishAccountMapping) => mapping.seerrUserId === undefined;

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
export function AccountDetail({
  mapping,
  user,
  users,
  save,
  actions,
  settings,
  configured,
  stale,
  tagDrafts,
  onLinkPrivileged,
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
  /**
   * Asks before linking a Seerr user that administers Seerr: the one of the same e-mail
   * («In Seerr eröffnen»), or `seerrUserId` chosen by hand.
   */
  onLinkPrivileged: (mapping: WishAccountMapping, seerrUserId?: number) => void;
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
  const toAccount = `${paths.adminAccounts}?item=${mapping.accountId}`;
  const ready = user !== undefined && mapping.seerrUserId !== undefined;
  const password = ready ? mapping.passwordState : undefined;
  // The user of the same e-mail administers Seerr: linking it waits for a confirmation.
  const privileged =
    mapping.state === 'waiting' && mapping.provisionError === 'seerr_user_privileged';
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
        // A user that administers Seerr is marked, and linking it asks first.
        const named = administersSeerr(candidate)
          ? t('admin.wishes.users.optionPrivileged', { option: base })
          : base;
        const label = mine
          ? t('admin.wishes.users.optionCurrent', { option: named })
          : candidate.gone
            ? t('admin.wishes.users.optionGone', { option: named })
            : taken
              ? t('admin.wishes.users.optionTaken', { option: named, email: owner.email })
              : named;
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
        <Notice
          tone="warning"
          live={false}
          title={t(
            waitsForChoice(mapping)
              ? 'admin.wishes.users.waitingChoiceTitle'
              : 'admin.wishes.users.waitingTitle',
          )}
        >
          {waitText(t, mapping.provisionError)}
        </Notice>
      )}
      {ready && mapping.provisionError === 'email_taken' && (
        <Notice tone="warning" live={false} title={t('admin.wishes.users.emailTakenTitle')}>
          {t('admin.wishes.users.emailTakenBody', { name: user.displayName })}
        </Notice>
      )}
      {candidate && !waitsForChoice(mapping) && (
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
              onPress={() =>
                privileged ? onLinkPrivileged(mapping) : actions.change(key, { kind: 'provision' })
              }
            >
              {privileged
                ? t('admin.wishes.users.linkPrivileged')
                : mapping.state === 'waiting'
                  ? t('admin.wishes.users.retry')
                  : t('admin.wishes.users.provision')}
            </AriaButton>
          )}
          <SelectField
            label={t('admin.wishes.users.chooseLabel', { email: mapping.email })}
            hideLabel
            value={chosen === null ? '' : String(chosen)}
            options={options}
            onChange={(value) => {
              const seerrUserId = value === '' ? null : Number(value);
              const next =
                seerrUserId === null ? undefined : users.find((u) => u.id === seerrUserId);
              if (next && next.id !== mapping.seerrUserId && administersSeerr(next)) {
                onLinkPrivileged(mapping, next.id);
                return;
              }
              save.change(key, { seerrUserId });
            }}
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
              <TextLink href={TO_SETTINGS}>{t('admin.wishes.users.toSettings')}</TextLink>
            </span>
          </p>
        )}
        {password === 'off' && syncOff && (
          <p className={styles.lockedLine}>
            <Icon name="lock" size="sm" />
            <span>
              {t('admin.wishes.users.passwordSyncOff')}{' '}
              <TextLink href={TO_SETTINGS}>{t('admin.wishes.users.toSettings')}</TextLink>
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
