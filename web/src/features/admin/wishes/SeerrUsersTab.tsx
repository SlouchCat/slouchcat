import { useMutation, useQueryClient, type UseQueryResult } from '@tanstack/react-query';
import { useMemo, useRef, useState, type ReactNode } from 'react';
import { useTranslation } from 'react-i18next';
import { isApiError } from '@/api/client';
import { problemMessage } from '@/api/errors';
import { paths } from '@/app/paths';
import { Chip, Icon, Notice, Skeleton, Text, TextLink, VisuallyHidden } from '@/ui';
import {
  afterDialogsClosed,
  afterRender,
  AriaButton,
  BandPanel,
  ConfirmDialog,
  DataTable,
  focusLost,
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
  type SeerrStatus,
  type SeerrUser,
  type WishAccountMapping,
  type WishMappings,
  type WishSettings,
} from '../queries';
import { rightsText, seerrUserColumns, userTypeText } from './columns';
import { QuotaLines } from '../../shared/quota';
import { useImmediateSave } from './useImmediateSave';
import styles from './Wishes.module.css';
import {
  AccountDetail,
  linkCandidate,
  TO_SETTINGS,
  withoutSeerrUser,
  type MappingChange,
  type RowAction,
} from './AccountDetail';

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

  // The account's new mapping into the list, before the lists are read again.
  const putMapping = (next: WishAccountMapping) =>
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
      putMapping(next);
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
  // The account whose Seerr user administers Seerr, while the question is open: the user of the
  // same e-mail («In Seerr eröffnen»), or `seerrUserId` chosen by hand.
  const [privileged, setPrivileged] = useState<{
    mapping: WishAccountMapping;
    seerrUserId?: number;
  } | null>(null);
  // The first question stays until it is answered; a later answer does not replace it.
  const ask = (mapping: WishAccountMapping, seerrUserId?: number) =>
    setPrivileged(
      (current) => current ?? { mapping, ...(seerrUserId === undefined ? {} : { seerrUserId }) },
    );
  const body = useRef<HTMLDivElement>(null);
  const actions = useImmediateSave<RowAction, WishAccountMapping>({
    save: (action, key) =>
      action.kind === 'provision'
        ? provisionSeerrAccount(Number(key), action.allowPrivileged)
        : setSeerrPasswordMode(Number(key), action.mode),
    saved: async (next, action) => {
      putMapping(next);
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
      if (next.state === 'waiting' && next.provisionError === 'seerr_user_privileged') {
        // Seerr has a user of this e-mail that administers Seerr: ask before linking it.
        ask(next);
        return t('admin.wishes.users.provisioned.privileged', { email: next.email });
      }
      if (next.state === 'waiting' && next.provisionError === 'seerr_user_taken') {
        return t('admin.wishes.users.provisioned.taken', { email: next.email });
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
  const without = accounts.filter(withoutSeerrUser).length;
  const shared = list?.sharedUser;
  const todo = without;
  const provisionOff = settings !== undefined && !settings.provisionAccounts;
  const toConnection = `${paths.adminIntegrations}?item=seerr`;
  const stale = status !== undefined && status.state !== 'connected';

  const linkPrivileged = (mapping: WishAccountMapping, seerrUserId?: number) => {
    if (seerrUserId === undefined) {
      actions.change(String(mapping.accountId), { kind: 'provision', allowPrivileged: true });
    } else {
      save.change(String(mapping.accountId), { seerrUserId });
    }
    // The buttons of a waiting account go once it is linked: the focus goes to its row's toggle.
    void afterDialogsClosed().then(async () => {
      await afterRender();
      if (focusLost()) {
        body.current
          ?.querySelector<HTMLElement>(`tr[data-row-key="${mapping.accountId}"] [aria-expanded]`)
          ?.focus();
      }
    });
  };

  return (
    <div className={styles.tabBody} ref={body}>
      <VisuallyHidden as="h2">{t('admin.wishes.tabs.users')}</VisuallyHidden>
      <ConfirmDialog
        isOpen={privileged !== null}
        onOpenChange={(open) => {
          if (!open) setPrivileged(null);
        }}
        title={t('admin.wishes.users.privilegedTitle', {
          name:
            (privileged &&
              (privileged.seerrUserId === undefined
                ? linkCandidate(privileged.mapping, users.data ?? [])
                : byId.get(privileged.seerrUserId)
              )?.displayName) ??
            '',
        })}
        body={t('admin.wishes.users.privilegedBody', { email: privileged?.mapping.email ?? '' })}
        confirmLabel={t('admin.wishes.users.privilegedConfirm')}
        cancelLabel={t('common.cancel')}
        tone="danger"
        onConfirm={() => {
          if (privileged) linkPrivileged(privileged.mapping, privileged.seerrUserId);
        }}
      />
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
          actions={<TextLink href={TO_SETTINGS}>{t('admin.wishes.users.toSettings')}</TextLink>}
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
            <TextLink href={TO_SETTINGS}>{t('admin.wishes.users.changeSettings')}</TextLink>
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
            <TextLink href={TO_SETTINGS}>{t('admin.wishes.users.toSettings')}</TextLink>
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
          <div className={styles.tableFrame} data-perf="first-work-row">
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
                    onLinkPrivileged={ask}
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
