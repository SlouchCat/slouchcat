import type { Backup, BackupRequest, components, RestoreResult } from '@slouchcat/api-client';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { useEffect, useId, useRef, useState, type FormEvent } from 'react';
import { Form } from 'react-aria-components';
import { useTranslation } from 'react-i18next';
import { api, apiUrl, request } from '@/api/client';
import { formErrors, problemMessage } from '@/api/errors';
import { useActiveSession } from '@/app/session';
import { formatBytes, formatDateTime } from '@/lib/format';
import { useDocumentTitle } from '@/lib/useDocumentTitle';
import { useEdited } from '@/lib/useEdited';
import { useTransientMutation } from '@/lib/useTransientMutation';
import { ButtonLink, Notice, Page, Panel, Skeleton, Text } from '@/ui';
import {
  AriaButton,
  ConfirmDialog,
  DataTable,
  FormDialog,
  RadioGroup,
  TextField,
  type DataColumn,
} from '@/ui/aria';
import { backupsQuery } from './queries';
import { reloadToLogin, RESTART_POLL_MS, restartedSince } from './restart';
import data from './AdminData.module.css';
import styles from './AdminTablePage.module.css';

/** Shortest passphrase of a backup with secrets (plan 3.8 point 3). */
const MIN_PASSPHRASE = 12;

type RestoreUpload = components['schemas']['RestoreUpload'];

const downloadUrl = (backup: Backup) =>
  apiUrl(`/admin/backups/${encodeURIComponent(backup.name)}/download`);

const restoreBackup = (name: string, passphrase: string): Promise<RestoreResult> =>
  request(
    api.POST('/admin/backups/{name}/restore', {
      params: { path: { name } },
      body: passphrase ? { passphrase } : {},
    }),
  );

function restoreUpload(file: File, passphrase: string): Promise<RestoreResult> {
  // The server reads the passphrase before the archive streams in (RestoreUpload): it must be the
  // first part. openapi-fetch sends a FormData as it is; its type knows the parts only as strings.
  const form = new FormData();
  if (passphrase) form.append('passphrase', passphrase);
  form.append('file', file);
  return request(api.POST('/admin/backups/restore', { body: form as unknown as RestoreUpload }));
}

/** A restore the server accepted: it restarts now. `acceptedAt` is when the 202 arrived. */
interface Restart {
  result: RestoreResult;
  acceptedAt: number;
}

/** «Sicherung erstellen» (A-4): without secrets (default) or with them behind a passphrase. */
function CreateDialog({ onClose }: { onClose: (created: Backup | null) => void }) {
  const { t } = useTranslation();
  const queryClient = useQueryClient();
  const [mode, setMode] = useState<'without' | 'with'>('without');
  const [passphrase, setPassphrase] = useState('');
  const create = useTransientMutation(
    (body: BackupRequest): Promise<Backup> => request(api.POST('/admin/backups', { body })),
    async (backup) => {
      await queryClient.invalidateQueries({ queryKey: backupsQuery.queryKey });
      onClose(backup);
    },
  );
  const { edited, submitted, onInput } = useEdited();
  const errors = create.error && !edited ? formErrors(t, create.error) : null;

  const onSubmit = (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    submitted();
    void create.run(
      mode === 'with' ? { includeSecrets: true, passphrase } : { includeSecrets: false },
    );
  };

  return (
    <FormDialog
      isOpen
      onOpenChange={(open) => {
        if (!open) onClose(null);
      }}
      title={t('admin.backup.create.title')}
      description={t('admin.backup.create.intro')}
    >
      <Form onSubmit={onSubmit} validationErrors={errors?.fields}>
        <div className={styles.form} onInput={onInput}>
          {errors?.form && <Notice tone="danger">{errors.form}</Notice>}
          <RadioGroup
            name="includeSecrets"
            label={t('admin.backup.create.scope')}
            value={mode}
            onChange={(value) => setMode(value === 'with' ? 'with' : 'without')}
            options={[
              { value: 'without', label: t('admin.backup.create.without') },
              { value: 'with', label: t('admin.backup.create.with') },
            ]}
            description={
              mode === 'with'
                ? t('admin.backup.create.withHint', { min: MIN_PASSPHRASE })
                : t('admin.backup.create.withoutHint')
            }
          />
          {mode === 'with' && (
            <>
              <TextField
                name="passphrase"
                type="password"
                label={t('admin.backup.create.passphrase')}
                description={t('admin.backup.create.passphraseHint', { min: MIN_PASSPHRASE })}
                autoComplete="new-password"
                isRequired
                value={passphrase}
                onChange={setPassphrase}
                validate={(value) =>
                  value.length >= MIN_PASSPHRASE
                    ? null
                    : t('admin.backup.create.passphraseTooShort', { min: MIN_PASSPHRASE })
                }
              />
              <TextField
                name="passphraseRepeat"
                type="password"
                label={t('admin.backup.create.passphraseRepeat')}
                autoComplete="new-password"
                isRequired
                validate={(value) =>
                  value === passphrase ? null : t('admin.backup.create.passphraseMismatch')
                }
              />
            </>
          )}
          <div className={styles.dialogActions}>
            <AriaButton variant="ghost" onPress={() => onClose(null)}>
              {t('common.cancel')}
            </AriaButton>
            <AriaButton type="submit" variant="primary" isPending={create.isPending}>
              {create.isPending
                ? t('admin.backup.create.creating')
                : t('admin.backup.create.submit')}
            </AriaButton>
          </div>
        </div>
      </Form>
    </FormDialog>
  );
}

/**
 * «Wiederherstellen» with input: an uploaded archive (`backup` null) or an encrypted backup of
 * the list. Either way the last step is the confirmation with the warning.
 */
function RestoreDialog({
  backup,
  onClose,
  onAccepted,
}: {
  backup: Backup | null;
  onClose: () => void;
  onAccepted: (result: RestoreResult) => void;
}) {
  const { t } = useTranslation();
  const fileId = useId();
  const [file, setFile] = useState<File | null>(null);
  const [passphrase, setPassphrase] = useState('');
  const restore = useTransientMutation(
    (): Promise<RestoreResult> =>
      backup ? restoreBackup(backup.name, passphrase) : restoreUpload(file!, passphrase),
    onAccepted,
  );
  const { edited, submitted, onInput } = useEdited();
  const errors = restore.error && !edited ? formErrors(t, restore.error) : null;
  const ready = backup ? passphrase !== '' : file !== null;

  return (
    <FormDialog
      isOpen
      onOpenChange={(open) => {
        if (!open) onClose();
      }}
      title={
        backup
          ? t('admin.backup.restore.titleNamed', { name: backup.name })
          : t('admin.backup.restore.titleUpload')
      }
    >
      <Form onSubmit={(event) => event.preventDefault()} validationErrors={errors?.fields}>
        <div className={styles.form} onInput={onInput}>
          {errors?.form && <Notice tone="danger">{errors.form}</Notice>}
          <Notice tone="warning" live={false}>
            {t('admin.backup.restore.warning')}
          </Notice>
          {!backup && (
            <div className={data.field}>
              <label htmlFor={fileId} className={data.label}>
                {t('admin.backup.restore.file')}
              </label>
              <input
                id={fileId}
                name="file"
                type="file"
                accept=".gz,.age"
                aria-describedby={`${fileId}-hint`}
                className={data.file}
                onChange={(event) => setFile(event.currentTarget.files?.[0] ?? null)}
              />
              <p id={`${fileId}-hint`} className={data.hint}>
                {t('admin.backup.restore.fileHint')}
              </p>
            </div>
          )}
          <TextField
            name="passphrase"
            type="password"
            label={t('admin.backup.restore.passphrase')}
            description={
              backup
                ? t('admin.backup.restore.passphraseHint')
                : t('admin.backup.restore.passphraseUploadHint')
            }
            autoComplete="off"
            isRequired={backup !== null}
            value={passphrase}
            onChange={setPassphrase}
          />
          <div className={styles.dialogActions}>
            <AriaButton variant="ghost" onPress={onClose}>
              {t('common.cancel')}
            </AriaButton>
            <ConfirmDialog
              trigger={
                <AriaButton variant="danger" isDisabled={!ready} isPending={restore.isPending}>
                  {restore.isPending
                    ? t('admin.backup.restore.running')
                    : t('admin.backup.restore.action')}
                </AriaButton>
              }
              title={t('admin.backup.restore.confirmTitle')}
              body={t('admin.backup.restore.confirmBody')}
              confirmLabel={t('admin.backup.restore.confirm')}
              cancelLabel={t('common.cancel')}
              tone="danger"
              onConfirm={() => {
                submitted();
                void restore.run(undefined);
              }}
            />
          </div>
        </div>
      </Form>
    </FormDialog>
  );
}

/**
 * «Server startet neu»: asks `/healthz` every 2 s and opens the sign-in page as soon as the new
 * server process answers.
 */
function RestartPanel({ restart }: { restart: Restart }) {
  const { t } = useTranslation();
  const heading = useRef<HTMLHeadingElement>(null);
  const { result, acceptedAt } = restart;
  const health = useQuery({
    queryKey: ['restart', 'health', acceptedAt],
    queryFn: ({ signal }) => request(api.GET('/healthz', { signal })),
    refetchInterval: RESTART_POLL_MS,
    retry: false,
    gcTime: 0,
  });
  const back =
    health.data !== undefined && restartedSince(health.data, acceptedAt, health.dataUpdatedAt);

  // The page content changed under the confirmation that started this: the heading takes the
  // focus, so keyboard and screen reader users land on the news.
  useEffect(() => {
    heading.current?.focus();
  }, []);
  useEffect(() => {
    if (back) reloadToLogin();
  }, [back]);

  return (
    <Panel as="section" aria-labelledby="restart-heading">
      <h2 id="restart-heading" ref={heading} tabIndex={-1} className={data.panelTitle}>
        {t('admin.backup.restart.title')}
      </h2>
      <Text>{t('admin.backup.restart.body', { version: result.version })}</Text>
      {!result.includesSecrets && (
        <Notice tone="warning" live={false}>
          {t('admin.backup.restart.noSecrets')}
        </Notice>
      )}
      <p role="status" className={data.status}>
        {back ? t('admin.backup.restart.back') : t('admin.backup.restart.waiting')}
      </p>
    </Panel>
  );
}

/**
 * Backup (A-4): the archives in the backup directory with download, delete and restore, a new
 * backup with or without secrets, and the restore of an uploaded archive. A restore replaces every
 * piece of data and restarts the server; the page then waits for it and opens the sign-in.
 */
export function BackupPage() {
  const { t, i18n } = useTranslation();
  useDocumentTitle(t('admin.backup.title'));
  const session = useActiveSession();
  const allowed = session.account.role === 'owner' && session.profile.isPrimary;
  const queryClient = useQueryClient();
  const [restart, setRestart] = useState<Restart | null>(null);
  const backups = useQuery({ ...backupsQuery, enabled: allowed && restart === null });
  const [creating, setCreating] = useState(false);
  // The dialog of a restore with input: `null` uploads a file, a backup restores that one.
  const [restoring, setRestoring] = useState<{ backup: Backup | null } | null>(null);
  const [done, setDone] = useState<string | null>(null);
  // A deleted row goes away with its button: the focus moves to the page heading.
  const header = useRef<HTMLDivElement>(null);

  const accepted = (result: RestoreResult) => {
    setRestoring(null);
    setRestart({ result, acceptedAt: Date.now() });
  };
  // A backup without passphrase restores straight from its row's confirmation.
  const restore = useMutation({
    mutationFn: (backup: Backup) => restoreBackup(backup.name, ''),
    onSuccess: accepted,
  });
  const remove = useMutation({
    mutationFn: (backup: Backup) =>
      request(api.DELETE('/admin/backups/{name}', { params: { path: { name: backup.name } } })),
    onSuccess: async (_data, backup) => {
      setDone(t('admin.backup.deleted', { name: backup.name }));
      await queryClient.invalidateQueries({ queryKey: backupsQuery.queryKey });
      header.current?.focus();
    },
  });
  const failure = remove.error ?? restore.error;

  const yesNo = (value: boolean) => (value ? t('admin.backup.yes') : t('admin.backup.no'));
  const columns: DataColumn<Backup>[] = [
    {
      key: 'name',
      header: t('admin.backup.columns.name'),
      rowHeader: true,
      cell: (backup) => <span className={styles.url}>{backup.name}</span>,
    },
    {
      key: 'created',
      header: t('admin.backup.columns.created'),
      cell: (backup) => (
        <time className={styles.nowrap} dateTime={backup.createdAt}>
          {formatDateTime(backup.createdAt, i18n.language)}
        </time>
      ),
    },
    {
      key: 'size',
      header: t('admin.backup.columns.size'),
      align: 'end',
      cell: (backup) => (
        <span className={styles.numbers}>{formatBytes(backup.sizeBytes, i18n.language)}</span>
      ),
    },
    {
      key: 'kind',
      header: t('admin.backup.columns.kind'),
      cell: (backup) => t(`admin.backup.kinds.${backup.kind}`),
    },
    {
      key: 'secrets',
      header: t('admin.backup.columns.secrets'),
      cell: (backup) => yesNo(backup.includesSecrets),
    },
    {
      key: 'encrypted',
      header: t('admin.backup.columns.encrypted'),
      cell: (backup) => yesNo(backup.encrypted),
    },
    {
      key: 'actions',
      header: t('admin.backup.columns.actions'),
      hideHeader: true,
      align: 'end',
      cell: (backup) => (
        <span className={styles.rowActions}>
          <ButtonLink
            variant="ghost"
            icon="download"
            href={downloadUrl(backup)}
            download={backup.name}
            aria-label={t('admin.backup.download.label', { name: backup.name })}
          >
            {t('admin.backup.download.action')}
          </ButtonLink>
          {backup.encrypted ? (
            <AriaButton
              variant="ghost"
              aria-label={t('admin.backup.restore.label', { name: backup.name })}
              onPress={() => setRestoring({ backup })}
            >
              {t('admin.backup.restore.action')}
            </AriaButton>
          ) : (
            <ConfirmDialog
              trigger={
                <AriaButton
                  variant="ghost"
                  aria-label={t('admin.backup.restore.label', { name: backup.name })}
                >
                  {t('admin.backup.restore.action')}
                </AriaButton>
              }
              title={t('admin.backup.restore.confirmTitle')}
              body={t('admin.backup.restore.confirmBodyNamed', { name: backup.name })}
              confirmLabel={t('admin.backup.restore.confirm')}
              cancelLabel={t('common.cancel')}
              tone="danger"
              onConfirm={() => restore.mutate(backup)}
            />
          )}
          <ConfirmDialog
            trigger={
              <AriaButton
                variant="ghost"
                aria-label={t('admin.backup.delete.label', { name: backup.name })}
              >
                {t('admin.backup.delete.action')}
              </AriaButton>
            }
            title={t('admin.backup.delete.title', { name: backup.name })}
            body={t('admin.backup.delete.body')}
            confirmLabel={t('admin.backup.delete.confirm')}
            cancelLabel={t('common.cancel')}
            tone="danger"
            onConfirm={() => remove.mutate(backup)}
          />
        </span>
      ),
    },
  ];

  return (
    <Page>
      <div ref={header} tabIndex={-1} className={styles.header}>
        <Text as="h1" variant="display">
          {t('admin.backup.title')}
        </Text>
        <Text tone="secondary">{t('admin.backup.intro')}</Text>
      </div>
      {!allowed ? (
        <Notice title={t('admin.access.notAllowed.title')}>
          {t('admin.access.notAllowed.body')}
        </Notice>
      ) : restart ? (
        <RestartPanel restart={restart} />
      ) : (
        <div className={styles.body} aria-busy={backups.isPending}>
          <div className={data.toolbar}>
            <AriaButton
              variant="primary"
              icon="plus"
              onPress={() => {
                setDone(null);
                setCreating(true);
              }}
            >
              {t('admin.backup.create.action')}
            </AriaButton>
            <AriaButton onPress={() => setRestoring({ backup: null })}>
              {t('admin.backup.restore.upload')}
            </AriaButton>
          </div>
          {failure && <Notice tone="danger">{problemMessage(t, failure)}</Notice>}
          {done && !failure && <Notice>{done}</Notice>}
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
          {backups.data?.length === 0 && <Notice>{t('admin.backup.empty')}</Notice>}
          {backups.data && backups.data.length > 0 && (
            <DataTable
              caption={t('admin.backup.list')}
              columns={columns}
              rows={backups.data}
              rowKey={(backup) => backup.name}
            />
          )}
        </div>
      )}
      {creating && !restart && (
        <CreateDialog
          onClose={(created) => {
            if (created) setDone(t('admin.backup.created', { name: created.name }));
            setCreating(false);
          }}
        />
      )}
      {restoring && !restart && (
        <RestoreDialog
          backup={restoring.backup}
          onClose={() => setRestoring(null)}
          onAccepted={accepted}
        />
      )}
    </Page>
  );
}
