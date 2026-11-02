import type { Backup, RestoreResult } from '@slouchcat/api-client';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { useSearch } from '@tanstack/react-router';
import { useCallback, useEffect, useMemo, useRef, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { api, request } from '@/api/client';
import { problemMessage } from '@/api/errors';
import { useActiveSession } from '@/app/session';
import { formatDateTime } from '@/lib/format';
import { useDocumentTitle } from '@/lib/useDocumentTitle';
import { useTransientMutation } from '@/lib/useTransientMutation';
import { Notice, Skeleton, Text } from '@/ui';
import {
  ActionMenu,
  AdminPage,
  AriaButton,
  BandPanel,
  ConfirmDialog,
  DataTable,
  DisclosureGroup,
  afterRender,
  focusTitleIfLost,
  useSearchSelection,
  type TabItem,
} from '@/ui/aria';
import { restoreBackup, restoreUpload, type Confirmation } from './backup/api';
import { backupColumns } from './backup/columns';
import { CreateTab, type CreateMode } from './backup/CreateTab';
import { PassphraseDialog } from './backup/PassphraseDialog';
import { UploadTab } from './backup/UploadTab';
import { backupsQuery } from './queries';
import { reloadToLogin } from './restart';
import { RestartPanel } from './RestartPanel';
import styles from './backup/Backup.module.css';

const TABS = ['list', 'create', 'upload'];
const KINDS = ['manual', 'auto'] as const;

/** A restore the server accepted: it restarts now. `acceptedAt` is when the 202 arrived. */
interface Restart {
  result: RestoreResult;
  acceptedAt: number;
}

/**
 * Backup (A-4; concept 12, variant C): the archives in the backup directory, one section per kind,
 * with download, restore and delete; a new backup with or without secrets and the restore of an
 * uploaded archive in tabs of their own (SIC-1 to SIC-4). A restore replaces every piece of data
 * and restarts the server; the page then waits for it and opens the sign-in.
 */
export function BackupPage() {
  const { t } = useTranslation();
  useDocumentTitle(t('admin.backup.title'));
  const session = useActiveSession();
  const allowed = session.account.role === 'admin' && session.profile.isPrimary;
  if (!allowed) {
    return (
      <AdminPage title={t('admin.backup.title')}>
        <Notice title={t('admin.access.notAllowed.title')}>
          {t('admin.access.notAllowed.body')}
        </Notice>
      </AdminPage>
    );
  }
  return <BackupContent />;
}

function BackupContent() {
  const { t, i18n } = useTranslation();
  const queryClient = useQueryClient();
  const number = useMemo(() => new Intl.NumberFormat(i18n.language), [i18n.language]);
  const titleRef = useRef<HTMLHeadingElement>(null);

  const [restart, setRestart] = useState<Restart | null>(null);
  const [confirmation, setConfirmation] = useState<Confirmation | null>(null);
  const [confirmOpen, setConfirmOpen] = useState(false);
  // The passphrase dialog of an encrypted backup of the list.
  const [unlocking, setUnlocking] = useState<Backup | null>(null);
  const hold = confirmOpen || unlocking !== null;
  const backups = useQuery({
    ...backupsQuery,
    enabled: restart === null,
    refetchOnWindowFocus: !hold,
  });

  const [tab, selectTab] = useSearchSelection('tab', TABS, 'list');
  const search: Record<string, unknown> = useSearch({ strict: false });
  const askedTab = search.tab;
  // An unknown `?tab=` shows «Sicherungen»; the address says so too.
  useEffect(() => {
    if (askedTab !== undefined && !(typeof askedTab === 'string' && TABS.includes(askedTab)))
      selectTab('list');
  }, [askedTab, selectTab]);

  // The scope of «Neue Sicherung» is the page's: it stays when the tab changes (S-17). The
  // passphrases are not; they live in the tab only.
  const [mode, setMode] = useState<CreateMode>('without');
  const [folded, setFolded] = useState<ReadonlySet<string>>(() => new Set());
  // The backup this page just made, marked «neu» in its row.
  const [fresh, setFresh] = useState<string | null>(null);
  // Each message gets its own key, so the same text twice in a row is announced again.
  const [message, setMessage] = useState<{ text: string; key: number } | null>(null);
  const [failure, setFailure] = useState<unknown>(null);
  const said = (text: string) => {
    setFailure(null);
    setMessage((current) => ({ text, key: (current?.key ?? 0) + 1 }));
  };
  const failed = (error: unknown) => {
    setMessage(null);
    setFailure(error);
  };
  /** A new action starts: the outcome of the last one (message or error) goes away. */
  const start = useCallback(() => {
    setMessage(null);
    setFailure(null);
  }, []);
  const invalidate = () => queryClient.invalidateQueries({ queryKey: backupsQuery.queryKey });

  const accepted = useCallback((result: RestoreResult) => {
    setUnlocking(null);
    setRestart({ result, acceptedAt: Date.now() });
  }, []);
  // The archive of «Aus Datei wiederherstellen» is the page's (not a secret): it survives a tab
  // change, and so does the answer to its upload. The passphrase is in the request only while it
  // runs (useTransientMutation).
  const [uploadFile, setUploadFile] = useState<File | null>(null);
  const upload = useTransientMutation(
    ({ file, passphrase }: { file: File; passphrase: string }) => restoreUpload(file, passphrase),
    accepted,
  );
  // A backup without passphrase restores straight from the confirmation.
  const restore = useMutation({
    mutationFn: (backup: Backup) => restoreBackup(backup.name, ''),
    onSuccess: accepted,
    onError: failed,
  });
  const remove = useMutation({
    mutationFn: (backup: Backup) =>
      request(api.DELETE('/admin/backups/{name}', { params: { path: { name: backup.name } } })),
    onSuccess: async (_data, backup) => {
      said(t('admin.backup.deleted', { name: backup.name }));
      await invalidate();
      // The row and its buttons are gone: once React rendered that, the h1 takes the focus.
      await afterRender();
      titleRef.current?.focus();
    },
    onError: failed,
  });

  // Every confirmation is the page's own: a row that goes while it is open cannot take the
  // dialog with it.
  const ask = useCallback(
    (next: Confirmation) => {
      start();
      setConfirmation(next);
      setConfirmOpen(true);
    },
    [start],
  );
  const onConfirmOpenChange = useCallback((open: boolean) => {
    if (open) return;
    setConfirmOpen(false);
    // The texts stay for the closing animation; the action goes (it may hold a passphrase).
    setConfirmation((current) => current && { ...current, onConfirm: () => undefined });
    // The button that opened it may be gone by now.
    void afterRender().then(() => focusTitleIfLost(titleRef));
  }, []);

  const { mutate: restoreMutate } = restore;
  const { mutate: removeMutate } = remove;
  const askRestore = useCallback(
    (backup: Backup) => {
      if (backup.encrypted) {
        start();
        setUnlocking(backup);
        return;
      }
      ask({
        title: t('admin.backup.restore.confirmTitle'),
        body: t('admin.backup.restore.confirmBodyNamed', { name: backup.name }),
        confirmLabel: t('admin.backup.restore.confirm'),
        defaultFocus: 'cancel',
        onConfirm: () => restoreMutate(backup),
      });
    },
    [ask, start, t, restoreMutate],
  );
  const { run: uploadRun } = upload;
  const askUpload = useCallback(
    (file: File, passphrase: string) =>
      ask({
        title: t('admin.backup.restore.confirmTitle'),
        body: t('admin.backup.restore.confirmBodyNamed', { name: file.name }),
        confirmLabel: t('admin.backup.restore.confirm'),
        defaultFocus: 'cancel',
        onConfirm: () => void uploadRun({ file, passphrase }),
      }),
    [ask, t, uploadRun],
  );
  // While a restore runs nothing else starts: the tabs and the rows' menus wait (and while a
  // backup is deleted, the menus).
  const restoring = restore.isPending || upload.isPending;
  const busy = restoring || remove.isPending;
  const askRemove = useCallback(
    (backup: Backup) =>
      ask({
        title: t('admin.backup.delete.title', { name: backup.name }),
        body: t('admin.backup.delete.body'),
        confirmLabel: t('admin.backup.delete.confirm'),
        onConfirm: () => removeMutate(backup),
      }),
    [ask, t, removeMutate],
  );
  // «Weitere Aktionen» of a row (S-6): both actions destroy data (a restore replaces all of it),
  // so both stand under «Gefahr», restore first.
  const actionsOf = useCallback(
    (backup: Backup) => (
      <ActionMenu
        rowName={backup.name}
        isDisabled={busy}
        items={[
          {
            id: 'restore',
            label: t('admin.backup.restore.menu'),
            icon: 'refresh',
            danger: true,
          },
          { id: 'remove', label: t('admin.backup.delete.action'), icon: 'trash', danger: true },
        ]}
        onAction={(action) => (action === 'remove' ? askRemove(backup) : askRestore(backup))}
      />
    ),
    [t, busy, askRestore, askRemove],
  );
  const columns = useMemo(
    () => backupColumns(t, i18n.language, fresh, actionsOf),
    [t, i18n.language, fresh, actionsOf],
  );

  const created = async (backup: Backup) => {
    said(t('admin.backup.created', { name: backup.name }));
    setFresh(backup.name);
    setFolded((current) => {
      const next = new Set(current);
      next.delete(backup.kind);
      return next;
    });
    await invalidate();
    // The new backup is a row of «Sicherungen»: that tab shows, and the h1 takes the focus the
    // form's button had.
    selectTab('list');
    await afterRender();
    focusTitleIfLost(titleRef);
  };

  const list = backups.data ?? [];
  const groups = KINDS.map((kind) => ({
    kind,
    rows: list.filter((backup) => backup.kind === kind),
  })).filter((group) => group.rows.length > 0);
  const countOf = (count: number) =>
    t('admin.backup.count', { count, number: number.format(count) });

  const listContent = (
    <div className={styles.list}>
      {backups.isPending && <Skeleton shape="block" />}
      {backups.isError && (
        <Notice
          tone="danger"
          title={problemMessage(t, backups.error)}
          actions={
            <AriaButton icon="refresh" onPress={() => void backups.refetch()}>
              {t('common.retry')}
            </AriaButton>
          }
        />
      )}
      {backups.data?.length === 0 && <Text tone="secondary">{t('admin.backup.empty')}</Text>}
      {groups.map(({ kind, rows }) => {
        const name = t(`admin.backup.kinds.${kind}`);
        const newest = rows.map((backup) => backup.createdAt).sort((a, b) => b.localeCompare(a))[0];
        const encrypted = rows.filter((backup) => backup.encrypted).length;
        const meta = [
          ...(kind === 'manual'
            ? [t('admin.backup.groups.encrypted', { number: number.format(encrypted) })]
            : []),
          ...(newest
            ? [t('admin.backup.groups.newest', { date: formatDateTime(newest, i18n.language) })]
            : []),
        ].join(t('format.separator'));
        return (
          <DisclosureGroup
            key={kind}
            title={name}
            count={number.format(rows.length)}
            countLabel={countOf(rows.length)}
            meta={<span className={styles.muted}>{meta}</span>}
            isExpanded={!folded.has(kind)}
            onExpandedChange={(open) =>
              setFolded((current) => {
                const next = new Set(current);
                if (open) next.delete(kind);
                else next.add(kind);
                return next;
              })
            }
          >
            <div className={styles.groupBody}>
              {kind === 'auto' && <Text tone="secondary">{t('admin.backup.groups.autoHint')}</Text>}
              <DataTable
                caption={t('admin.backup.caption', { kind: name })}
                columns={columns}
                rows={rows}
                rowKey={(backup) => backup.name}
              />
            </div>
          </DisclosureGroup>
        );
      })}
    </div>
  );

  const tabs: TabItem[] = [
    {
      id: 'list',
      label: t('admin.backup.tabs.list'),
      ...(backups.data
        ? { count: number.format(list.length), countLabel: countOf(list.length) }
        : {}),
      content: listContent,
    },
    {
      id: 'create',
      label: t('admin.backup.tabs.create'),
      content: <CreateTab mode={mode} onModeChange={setMode} onCreated={created} />,
    },
    {
      id: 'upload',
      label: t('admin.backup.tabs.upload'),
      content: (
        <UploadTab
          file={uploadFile}
          onFileChange={setUploadFile}
          error={upload.error}
          isPending={upload.isPending}
          onRestore={askUpload}
        />
      ),
    },
  ];

  return (
    <AdminPage title={t('admin.backup.title')} help={t('admin.backup.intro')} titleRef={titleRef}>
      {failure !== null && <Notice tone="danger">{problemMessage(t, failure)}</Notice>}
      <div role="status" className={styles.live}>
        {message && failure === null && (
          <Notice key={message.key} live={false}>
            {message.text}
          </Notice>
        )}
      </div>
      {restart ? (
        <RestartPanel
          acceptedAt={restart.acceptedAt}
          title={t('admin.backup.restart.title')}
          body={t('admin.backup.restart.body', { version: restart.result.version })}
          notice={
            restart.result.includesSecrets ? undefined : (
              <Notice tone="warning" live={false}>
                {t('admin.backup.restart.noSecrets')}
              </Notice>
            )
          }
          onBack={reloadToLogin}
        />
      ) : (
        <BandPanel
          title={t('admin.backup.list')}
          icon="archive"
          tabs={{
            label: t('admin.backup.tabs.label'),
            selectedKey: tab,
            onSelectionChange: (key) => {
              start();
              selectTab(key);
            },
            isDisabled: restoring,
            tabs,
          }}
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
          tone="danger"
          defaultFocus={confirmation.defaultFocus}
          onConfirm={confirmation.onConfirm}
        />
      )}
      {unlocking && !restart && (
        <PassphraseDialog
          backup={unlocking}
          ask={ask}
          onClose={() => setUnlocking(null)}
          onAccepted={accepted}
        />
      )}
    </AdminPage>
  );
}
