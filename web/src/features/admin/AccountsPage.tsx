import type { AdminAccount, Invitation } from '@slouchcat/api-client';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { useNavigate, useSearch } from '@tanstack/react-router';
import { useCallback, useEffect, useMemo, useRef, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { api, request } from '@/api/client';
import { problemMessage } from '@/api/errors';
import { useActiveSession } from '@/app/session';
import { formatDateTime } from '@/lib/format';
import { useDocumentTitle } from '@/lib/useDocumentTitle';
import { Notice, Skeleton, Text, VisuallyHidden } from '@/ui';
import {
  ActionMenu,
  AdminPage,
  AriaButton,
  ConfirmDialog,
  DataTable,
  StatusStrip,
  Tabs,
  useDrafts,
  useSearchSelection,
  useUnsavedChanges,
  type ActionMenuItem,
} from '@/ui/aria';
import { useLibraryOptions } from '../shared/useLibraryOptions';
import { accessDraftOf, normaliseAccess, type AccessDraft } from './accounts/AccessForm';
import { OtherAccountDetail, OwnAccountDetail } from './accounts/AccountDetail';
import { accountColumns } from './accounts/columns';
import { invitationNote, InvitationsTable, InviteDialog } from './accounts/Invitations';
import {
  normaliseOwnAccount,
  ownAccountDraftOf,
  type OwnAccountDraft,
} from './accounts/OwnAccountForm';
import { accountsQuery, invitationsQuery } from './queries';
import styles from './accounts/Accounts.module.css';

const TABS = ['accounts', 'invitations'];
/** `?item=me`: the own account (the sidebar's «Profile» needs no account id). */
const ME = 'me';

/** Resolves once React has rendered what the last cache update brought. */
const afterRender = () =>
  new Promise<void>((resolve) => {
    window.setTimeout(() => window.requestAnimationFrame(() => resolve()), 0);
  });

/**
 * Resolves once no dialog is open or closing any more: react-aria hands the focus back when a
 * dialog unmounts, after its exit animation, and only then is it clear where the focus ended.
 */
async function afterDialogs() {
  const nextFrame = () =>
    new Promise<void>((resolve) => window.requestAnimationFrame(() => resolve()));
  for (
    let frame = 0;
    frame < 120 && document.querySelector('[role="dialog"], [role="alertdialog"]');
    frame += 1
  ) {
    await nextFrame();
  }
  // react-aria returns the focus in a frame after the unmount; that comes first.
  await nextFrame();
  await nextFrame();
}

interface Confirmation {
  title: string;
  body: string;
  confirmLabel: string;
  tone: 'primary' | 'danger';
  onConfirm: () => void;
}

/**
 * Accounts with profiles (A-2, concept 12 variant C with KP-1): every account with role,
 * ceiling, libraries, profiles, last contact and the notification channels it switched on, and the
 * invitations that create new ones. The page shows or hides buttons - the server decides.
 */
export function AccountsPage() {
  const { t } = useTranslation();
  useDocumentTitle(t('admin.accounts.title'));
  const session = useActiveSession();
  const allowed = session.account.role === 'admin' && session.profile.isPrimary;
  if (!allowed) {
    return (
      <AdminPage title={t('admin.accounts.title')}>
        <Notice title={t('admin.access.notAllowed.title')}>
          {t('admin.access.notAllowed.body')}
        </Notice>
      </AdminPage>
    );
  }
  return <AccountsContent />;
}

/**
 * The status strip with four numbers, the tabs «Konten» and «Einladungen», and per account a row
 * that folds out: the own account changes its e-mail and password and manages its profiles there;
 * another account shows its access (inline for an account without administration), its profile
 * names and appoint, demote and remove (ADR 0014). What is typed is the page's draft per account
 * (S-17); leaving with it asks first (S-13).
 */
function AccountsContent() {
  const { t, i18n } = useTranslation();
  const session = useActiveSession();
  const ownId = String(session.account.id);
  const isOwner = session.account.isOwner;
  const queryClient = useQueryClient();
  const navigate = useNavigate();
  const number = useMemo(() => new Intl.NumberFormat(i18n.language), [i18n.language]);
  const titleRef = useRef<HTMLHeadingElement>(null);
  const inviteButton = useRef<HTMLButtonElement>(null);

  // The drafts need what the server holds before the queries know whether to hold for them: a
  // passive observer reads the same cache entry without fetching.
  const cached = useQuery({ ...accountsQuery, enabled: false }).data;
  const accessOriginals = useMemo(
    () =>
      new Map(
        (cached ?? []).flatMap((account) =>
          account.role === 'user' && String(account.id) !== ownId
            ? [[String(account.id), accessDraftOf(account)] as const]
            : [],
        ),
      ),
    [cached, ownId],
  );
  const accessOriginalOf = useCallback((id: string) => accessOriginals.get(id), [accessOriginals]);
  const access = useDrafts<AccessDraft>(accessOriginalOf);
  const ownEmail = cached?.find((account) => String(account.id) === ownId)?.email;
  const ownOriginalOf = useCallback(
    (id: string) =>
      id === ownId && ownEmail !== undefined ? ownAccountDraftOf(ownEmail) : undefined,
    [ownId, ownEmail],
  );
  const own = useDrafts<OwnAccountDraft>(ownOriginalOf);
  const unsaved = useUnsavedChanges(access.dirty || own.dirty);

  const [inviting, setInviting] = useState(false);
  const [confirmation, setConfirmation] = useState<Confirmation | null>(null);
  const [confirmOpen, setConfirmOpen] = useState(false);
  // The dialogs of «Meine Profile» hold the page's queries as well.
  const [profilesHeld, setProfilesHeld] = useState(false);
  const hold = confirmOpen || inviting || profilesHeld || unsaved.pausePolling;
  // «Meine Profile» scrolls into view once per arrival at `#meine-profile`, not on every fold.
  const scrolledFor = useRef<string | undefined>(undefined);
  const claimScroll = useCallback((arrival: string) => {
    if (scrolledFor.current === arrival) return false;
    scrolledFor.current = arrival;
    return true;
  }, []);

  const accounts = useQuery({ ...accountsQuery, refetchOnWindowFocus: !hold });
  const invitations = useQuery({ ...invitationsQuery, refetchOnWindowFocus: !hold });
  const libraries = useLibraryOptions();

  const [tab, selectTab] = useSearchSelection('tab', TABS, 'accounts');
  const search: Record<string, unknown> = useSearch({ strict: false });
  const askedTab = search.tab;
  const askedItem = search.item;
  // An unknown `?tab=` shows «Konten»; the address says so too.
  useEffect(() => {
    if (askedTab !== undefined && !(typeof askedTab === 'string' && TABS.includes(askedTab)))
      selectTab('accounts');
  }, [askedTab, selectTab]);

  const [expanded, setExpanded] = useState<ReadonlySet<string>>(() => new Set());
  const { set: setOwnDraft } = own;
  const onExpandedChange = useCallback(
    (key: string, open: boolean) => {
      // Passwords do not outlive the folded own row; the typed e-mail does (S-17).
      if (!open && key === ownId) {
        setOwnDraft(ownId, (draft) =>
          normaliseOwnAccount({
            ...draft,
            newPassword: '',
            newPasswordRepeat: '',
            currentPassword: '',
          }),
        );
      }
      setExpanded((current) => {
        const next = new Set(current);
        if (open) next.add(key);
        else next.delete(key);
        return next;
      });
    },
    [ownId, setOwnDraft],
  );
  // `?item=` opens that account's row once, when it arrives (`me` is the own account); folding it
  // away afterwards is the reader's choice. An account the list does not know leaves the address.
  const loadedAccounts = accounts.data;
  const itemId = askedItem === ME ? ownId : typeof askedItem === 'string' ? askedItem : undefined;
  const itemKnown =
    itemId !== undefined && !!loadedAccounts?.some((account) => String(account.id) === itemId);
  const [openedItem, setOpenedItem] = useState<string | undefined>(undefined);
  // Adjusted while rendering (no effect): the row is open in the same render that shows it.
  if (itemKnown && openedItem !== itemId) {
    setOpenedItem(itemId);
    setExpanded((current) => new Set(current).add(itemId));
  } else if (askedItem === undefined && openedItem !== undefined) {
    setOpenedItem(undefined);
  }
  useEffect(() => {
    if (loadedAccounts && askedItem !== undefined && !itemKnown) {
      void navigate({
        to: '.',
        replace: true,
        hash: true,
        search: ((previous: Record<string, unknown>) => ({
          ...previous,
          item: undefined,
        })) as never,
      });
    }
  }, [loadedAccounts, askedItem, itemKnown, navigate]);

  const [message, setMessage] = useState<string | null>(null);
  const [failure, setFailure] = useState<unknown>(null);
  const refresh = () =>
    Promise.all([
      queryClient.invalidateQueries({ queryKey: accountsQuery.queryKey }),
      queryClient.invalidateQueries({ queryKey: invitationsQuery.queryKey }),
    ]);
  const said = (text: string) => {
    setFailure(null);
    setMessage(text);
  };
  const failed = (error: unknown) => {
    setMessage(null);
    setFailure(error);
  };

  /** The focus fell to the body (its button or row went with a change): the h1 takes it. */
  const focusTitleIfLost = useCallback(() => {
    const active = document.activeElement;
    if (!active || active === document.body) titleRef.current?.focus();
  }, []);

  const setRole = useMutation({
    mutationFn: ({ account, role }: { account: AdminAccount; role: 'admin' | 'user' }) =>
      request(
        api.PUT('/admin/accounts/{id}/role', {
          params: { path: { id: account.id } },
          body: { role },
        }),
      ),
    onSuccess: async (_data, { account, role }) => {
      said(
        t(
          role === 'admin' ? 'admin.accounts.role.promote.done' : 'admin.accounts.role.demote.done',
          {
            email: account.email,
          },
        ),
      );
      await refresh();
      await afterRender();
      focusTitleIfLost();
    },
    onError: failed,
  });
  const remove = useMutation({
    mutationFn: (account: AdminAccount) =>
      request(api.DELETE('/admin/accounts/{id}', { params: { path: { id: account.id } } })),
    onSuccess: async (_data, account) => {
      said(t('admin.accounts.remove.done', { email: account.email }));
      // Its open fold-out and its draft go with it.
      onExpandedChange(String(account.id), false);
      access.discard(String(account.id));
      await refresh();
      // The row and its buttons are gone: once React rendered that, the h1 takes the focus.
      await afterRender();
      titleRef.current?.focus();
    },
    onError: failed,
  });
  const revoke = useMutation({
    mutationFn: (invitation: Invitation) =>
      request(api.DELETE('/admin/invitations/{id}', { params: { path: { id: invitation.id } } })),
    onSuccess: async (_data, invitation) => {
      said(t('admin.accounts.invitations.revoke.done', { note: invitationNote(t, invitation) }));
      await refresh();
      // The button goes with the open state: once React rendered that and the confirmation has
      // returned the focus, the h1 takes it if it fell to the body.
      await afterRender();
      await afterDialogs();
      focusTitleIfLost();
    },
    onError: failed,
  });

  // Every confirmation is the page's own: a row that goes while it is open (another
  // administrator, a refetch) cannot take the dialog with it.
  const ask = useCallback((next: Confirmation) => {
    setConfirmation(next);
    setConfirmOpen(true);
  }, []);
  const onConfirmOpenChange = useCallback(
    (open: boolean) => {
      if (open) return;
      setConfirmOpen(false);
      // The button that opened it may be gone by now.
      void afterRender().then(afterDialogs).then(focusTitleIfLost);
    },
    [focusTitleIfLost],
  );
  const { mutate: setRoleMutate } = setRole;
  const { mutate: removeMutate } = remove;
  const { mutate: revokeMutate } = revoke;
  const askRole = useCallback(
    (account: AdminAccount) => {
      const kind = account.role === 'admin' ? 'demote' : 'promote';
      ask({
        title: t(`admin.accounts.role.${kind}.title`, { email: account.email }),
        body: t(`admin.accounts.role.${kind}.body`),
        confirmLabel: t(`admin.accounts.role.${kind}.confirm`),
        tone: 'primary',
        onConfirm: () =>
          setRoleMutate({ account, role: account.role === 'admin' ? 'user' : 'admin' }),
      });
    },
    [ask, t, setRoleMutate],
  );
  const askRemove = useCallback(
    (account: AdminAccount) =>
      ask({
        title: t('admin.accounts.remove.title', { email: account.email }),
        body: t('admin.accounts.remove.body'),
        confirmLabel: t('admin.accounts.remove.confirm'),
        tone: 'danger',
        onConfirm: () => removeMutate(account),
      }),
    [ask, t, removeMutate],
  );
  const askRevoke = useCallback(
    (invitation: Invitation) =>
      ask({
        title: t('admin.accounts.invitations.revoke.title'),
        body: t('admin.accounts.invitations.revoke.body'),
        confirmLabel: t('admin.accounts.invitations.revoke.confirm'),
        tone: 'danger',
        onConfirm: () => revokeMutate(invitation),
      }),
    [ask, t, revokeMutate],
  );

  const invalidateAccounts = useCallback(
    () => void queryClient.invalidateQueries({ queryKey: accountsQuery.queryKey }),
    [queryClient],
  );
  // «Weitere Aktionen» of another account (S-6): appoint or demote (owner only), and remove
  // under «Gefahr». The own account and the owner have none (ADR 0014).
  const actionsOf = useCallback(
    (account: AdminAccount) => {
      if (String(account.id) === ownId || account.isOwner) return null;
      const items: ActionMenuItem[] = [
        ...(isOwner
          ? [
              {
                id: 'role',
                label: t(
                  account.role === 'admin'
                    ? 'admin.accounts.role.demote.action'
                    : 'admin.accounts.role.promote.action',
                ),
              },
            ]
          : []),
        { id: 'remove', label: t('admin.accounts.remove.action'), icon: 'trash', danger: true },
      ];
      return (
        <ActionMenu
          rowName={account.email}
          items={items}
          onAction={(action) => (action === 'role' ? askRole(account) : askRemove(account))}
        />
      );
    },
    [ownId, isOwner, t, askRole, askRemove],
  );
  const columns = useMemo(
    () => accountColumns(t, i18n.language, actionsOf),
    [t, i18n.language, actionsOf],
  );
  const detail = (account: AdminAccount) => {
    const id = String(account.id);
    if (id === ownId) {
      const value = own.valueOf(id);
      if (!value) return null;
      return (
        <OwnAccountDetail
          account={account}
          draft={{
            value,
            dirty: own.isDirty(id),
            onChange: (next) => own.set(id, normaliseOwnAccount(next)),
            onDiscard: () => own.discard(id),
          }}
          onSaved={() => {
            own.discard(id);
            said(t('admin.accounts.edit.saved'));
          }}
          titleRef={titleRef}
          hold={hold}
          onHoldChange={setProfilesHeld}
          claimScroll={claimScroll}
          onProfilesChanged={invalidateAccounts}
        />
      );
    }
    const value = access.valueOf(id);
    return (
      <OtherAccountDetail
        account={account}
        libraries={libraries}
        access={
          value && {
            value,
            dirty: access.isDirty(id),
            onChange: (next) => access.set(id, normaliseAccess(next, accessOriginalOf(id))),
            onDiscard: () => access.discard(id),
          }
        }
        onAccessSaved={async () => {
          // The list holds the saved access first; then the draft is spent.
          await queryClient.invalidateQueries({ queryKey: accountsQuery.queryKey });
          access.discard(id);
          said(t('admin.accounts.access.saved', { email: account.email }));
        }}
      />
    );
  };

  const list = accounts.data ?? [];
  const invitationList = invitations.data ?? [];
  const open = invitationList.filter((invitation) => invitation.state === 'open');
  const nextExpiry = open.map((invitation) => invitation.expiresAt).sort()[0];
  const admins = list.filter((account) => account.role === 'admin');
  const owners = admins.filter((account) => account.isOwner).length;
  const profileCount = list.reduce((sum, account) => sum + account.profiles.length, 0);
  const ownProfiles = list.find((account) => String(account.id) === ownId)?.profiles.length ?? 0;
  const status = accounts.data && invitations.data && (
    <StatusStrip
      label={t('admin.accounts.status.label')}
      tiles={[
        {
          key: 'accounts',
          label: t('admin.accounts.status.accounts'),
          value: number.format(list.length),
        },
        {
          key: 'admins',
          label: t('admin.accounts.status.admins'),
          value: number.format(admins.length),
          note: t('admin.accounts.status.owners', { count: owners, number: number.format(owners) }),
        },
        {
          key: 'profiles',
          label: t('admin.accounts.status.profiles'),
          value: number.format(profileCount),
          note: t('admin.accounts.status.ownProfiles', {
            count: ownProfiles,
            number: number.format(ownProfiles),
          }),
        },
        {
          key: 'invitations',
          label: t('admin.accounts.status.openInvitations'),
          value: number.format(open.length),
          ...(nextExpiry
            ? {
                note: t('admin.accounts.status.nextExpiry', {
                  date: formatDateTime(nextExpiry, i18n.language),
                }),
              }
            : {}),
        },
      ]}
    />
  );

  return (
    <AdminPage
      title={t('admin.accounts.title')}
      help={t('admin.accounts.intro')}
      titleRef={titleRef}
      status={status}
      actions={
        <AriaButton
          ref={inviteButton}
          variant="primary"
          icon="plus"
          isDisabled={!libraries.isReady}
          onPress={() => setInviting(true)}
        >
          {t('admin.accounts.invitations.invite')}
        </AriaButton>
      }
    >
      {libraries.isError && (
        <Notice
          tone="danger"
          title={t('libraries.loadFailed')}
          actions={
            <AriaButton icon="refresh" onPress={libraries.retry}>
              {t('common.retry')}
            </AriaButton>
          }
        >
          {problemMessage(t, libraries.error)}
        </Notice>
      )}
      {failure !== null && <Notice tone="danger">{problemMessage(t, failure)}</Notice>}
      <div role="status" className={styles.live}>
        {message && failure === null && <Notice live={false}>{message}</Notice>}
      </div>
      {accounts.isPending && <Skeleton shape="block" />}
      {/* Each list fails on its own: its error and retry stand in its tab. */}
      {!accounts.isPending && (
        <Tabs
          label={t('admin.accounts.tabs.label')}
          selectedKey={tab}
          onSelectionChange={selectTab}
          tabs={[
            {
              id: 'accounts',
              label: t('admin.accounts.tabs.accounts'),
              ...(accounts.data
                ? {
                    count: number.format(list.length),
                    countLabel: t('admin.accounts.tabs.accountsCount', {
                      count: list.length,
                      number: number.format(list.length),
                    }),
                  }
                : {}),
              content: (
                <div className={styles.tabBody}>
                  <VisuallyHidden as="h2">{t('admin.accounts.tabs.accounts')}</VisuallyHidden>
                  {accounts.isError && (
                    <Notice
                      tone="danger"
                      title={problemMessage(t, accounts.error)}
                      actions={
                        <AriaButton icon="refresh" onPress={() => void accounts.refetch()}>
                          {t('common.retry')}
                        </AriaButton>
                      }
                    />
                  )}
                  {accounts.data && (
                    <DataTable
                      caption={t('admin.accounts.list')}
                      columns={columns}
                      rows={list}
                      rowKey={(account) => String(account.id)}
                      expand={{ render: detail, expandedKeys: expanded, onExpandedChange }}
                    />
                  )}
                </div>
              ),
            },
            {
              id: 'invitations',
              label: t('admin.accounts.tabs.invitations'),
              ...(invitations.data
                ? { count: t('admin.accounts.tabs.open', { number: number.format(open.length) }) }
                : {}),
              content: (
                <div className={styles.tabBody}>
                  <VisuallyHidden as="h2">{t('admin.accounts.tabs.invitations')}</VisuallyHidden>
                  {invitations.isPending && <Skeleton shape="block" />}
                  {invitations.isError && (
                    <Notice
                      tone="danger"
                      title={problemMessage(t, invitations.error)}
                      actions={
                        <AriaButton icon="refresh" onPress={() => void invitations.refetch()}>
                          {t('common.retry')}
                        </AriaButton>
                      }
                    />
                  )}
                  {invitations.data &&
                    (invitationList.length === 0 ? (
                      <Text tone="secondary">{t('admin.accounts.invitations.empty')}</Text>
                    ) : (
                      <InvitationsTable
                        invitations={invitationList}
                        options={libraries.options}
                        onRevoke={askRevoke}
                      />
                    ))}
                </div>
              ),
            },
          ]}
        />
      )}
      {confirmation && (
        <ConfirmDialog
          isOpen={confirmOpen}
          onOpenChange={onConfirmOpenChange}
          title={confirmation.title}
          body={confirmation.body}
          confirmLabel={confirmation.confirmLabel}
          cancelLabel={t('common.cancel')}
          tone={confirmation.tone}
          onConfirm={confirmation.onConfirm}
        />
      )}
      <ConfirmDialog {...unsaved.dialog} />
      {inviting && (
        <InviteDialog
          options={libraries.options}
          // The new link is a row of «Einladungen»: that tab shows once the dialog closes.
          onCreated={() => selectTab('invitations')}
          onClose={() => {
            setInviting(false);
            inviteButton.current?.focus();
          }}
        />
      )}
    </AdminPage>
  );
}
