import type {
  Integration,
  IntegrationCreated,
  IntegrationEvent,
  PathMapping,
} from '@slouchcat/api-client';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { useEffect, useId, useRef, useState, type FormEvent } from 'react';
import { useTranslation } from 'react-i18next';
import { formErrors, problemMessage } from '@/api/errors';
import { formatDateTime } from '@/lib/format';
import { Chip, Notice, Panel, Skeleton, Text } from '@/ui';
import {
  AriaButton,
  ConfirmDialog,
  DataTable,
  Form,
  RadioGroup,
  SettingRow,
  TextField,
  type Drafts,
} from '@/ui/aria';
import {
  createIntegration,
  dashboardQuery,
  deleteIntegration,
  integrationEventsQuery,
  integrationsQuery,
  renewIntegrationToken,
  setIntegrationPathMap,
} from './queries';
import styles from './IntegrationsPage.module.css';
import table from './AdminTablePage.module.css';

const KINDS = ['radarr', 'sonarr'] as const satisfies readonly Integration['kind'][];

const resultTone = {
  scan_queued: 'success',
  acknowledged: 'info',
  ignored: 'neutral',
  outside_roots: 'warning',
  invalid_payload: 'warning',
} as const satisfies Record<IntegrationEvent['result'], string>;

/**
 * The unsaved path mapping of one integration (S-17), keyed by its id in the page's drafts:
 * `base` is the stored mapping the rows started from; comparing it with the stored mapping of the
 * moment says whether somebody else has saved this integration in between.
 */
export interface PathMapDraft {
  base: PathMapping[];
  rows: PathMapping[];
}

/** One mapping as a string, to tell two versions of it apart. */
const stored = (pairs: PathMapping[]) => JSON.stringify(pairs);

/**
 * The address of a just created or renewed integration: shown once, with a copy button. After
 * this panel is gone the address cannot be recovered — only the token's last four characters
 * stay (plan 3.8 point 2).
 */
function CreatedAddress({
  created,
  onDismiss,
}: {
  created: IntegrationCreated;
  onDismiss: () => void;
}) {
  const { t } = useTranslation();
  const input = useRef<HTMLInputElement>(null);
  const [status, setStatus] = useState<'copied' | 'failed' | null>(null);
  const headingId = useId();

  // The address exists once: put the cursor on it (selected), so Ctrl+C is all that is left.
  useEffect(() => {
    input.current?.focus();
  }, []);

  const copy = async () => {
    try {
      await navigator.clipboard.writeText(created.url);
      setStatus('copied');
    } catch {
      // No clipboard on plain-HTTP origins: select the text for Ctrl+C instead.
      input.current?.focus();
      input.current?.select();
      setStatus('failed');
    }
  };

  return (
    <Panel as="section" aria-labelledby={headingId}>
      <h3 id={headingId} className={styles.itemTitle}>
        {t('admin.integrations.webhooks.created.title', { name: created.integration.name })}
      </h3>
      <Notice tone="warning">{t('admin.integrations.webhooks.created.once')}</Notice>
      <TextField
        label={t('admin.integrations.webhooks.created.label')}
        value={created.url}
        isReadOnly
        mono
        selectOnFocus
        inputRef={input}
      />
      <div className={styles.actions}>
        <AriaButton variant="primary" icon="copy" onPress={() => void copy()}>
          {t('admin.integrations.webhooks.created.copy')}
        </AriaButton>
        <AriaButton variant="ghost" onPress={onDismiss}>
          {t('admin.integrations.webhooks.created.dismiss')}
        </AriaButton>
      </div>
      <p role="status" className={styles.live}>
        {status === 'copied' && t('admin.integrations.webhooks.created.copied')}
        {status === 'failed' && t('admin.integrations.webhooks.created.copyFailed')}
      </p>
    </Panel>
  );
}

/** Name and kind of a new integration. */
function NewIntegrationForm({ onCreated }: { onCreated: (created: IntegrationCreated) => void }) {
  const { t } = useTranslation();
  const queryClient = useQueryClient();
  const [name, setName] = useState('');
  const [kind, setKind] = useState<Integration['kind']>('radarr');

  const create = useMutation({
    mutationFn: createIntegration,
    onSuccess: async (created) => {
      onCreated(created);
      setName('');
      setKind('radarr');
      await queryClient.invalidateQueries({ queryKey: integrationsQuery.queryKey });
      await queryClient.invalidateQueries({ queryKey: dashboardQuery.queryKey });
    },
  });
  const errors = create.error ? formErrors(t, create.error) : null;

  const onSubmit = (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    create.mutate({ name: name.trim(), kind });
  };

  return (
    <SettingRow
      label={t('admin.integrations.webhooks.form.title')}
      help={t('admin.integrations.webhooks.form.nameHint')}
    >
      <Form className={styles.stack} onSubmit={onSubmit} validationErrors={errors?.fields}>
        <TextField
          name="name"
          label={t('admin.integrations.webhooks.form.name')}
          autoComplete="off"
          isRequired
          value={name}
          onChange={setName}
          validate={(text) => (text.trim() ? null : t('errors.field.required'))}
        />
        <RadioGroup
          name="kind"
          label={t('admin.integrations.webhooks.form.kind')}
          value={kind}
          onChange={(value) => setKind(KINDS.find((k) => k === value) ?? 'radarr')}
          options={KINDS.map((k) => ({
            value: k,
            label: t(`admin.integrations.webhooks.kind.${k}`),
          }))}
        />
        <div className={styles.actions}>
          <AriaButton type="submit" variant="primary" icon="plus" isPending={create.isPending}>
            {t('admin.integrations.webhooks.form.submit')}
          </AriaButton>
        </div>
        {errors?.form && <Notice tone="danger">{problemMessage(t, create.error)}</Notice>}
      </Form>
    </SettingRow>
  );
}

/** The prefix pairs of one integration: add, change and remove rows, then save them together. */
function PathMapForm({
  integration,
  drafts,
}: {
  integration: Integration;
  drafts: Drafts<PathMapDraft>;
}) {
  const { t } = useTranslation();
  const queryClient = useQueryClient();
  const [message, setMessage] = useState<string | null>(null);
  const id = String(integration.id);
  const { discard } = drafts;
  const draft = drafts.valueOf(id) ?? { base: integration.pathMap, rows: integration.pathMap };
  const rows = draft.rows;

  const save = useMutation({
    mutationFn: (pairs: PathMapping[]) => setIntegrationPathMap(integration.id, pairs),
    onSuccess: async (updated) => {
      // The stored rows first, so the form never shows the old ones between the two.
      queryClient.setQueryData(integrationsQuery.queryKey, (list) =>
        list?.map((entry) => (entry.id === updated.id ? updated : entry)),
      );
      discard(id);
      setMessage(t('admin.integrations.webhooks.pathMap.saved'));
      await queryClient.invalidateQueries({ queryKey: integrationsQuery.queryKey });
    },
  });
  const errors = save.error ? formErrors(t, save.error) : null;

  // A second admin saved this integration while rows were typed here: they stay, with an offer to
  // take the other version. Rows that match the stored mapping, or the one they started from, are
  // no change of their own, so the form simply follows the stored mapping again.
  const server = stored(integration.pathMap);
  const dirty = drafts.isDirty(id);
  const spent = dirty && (stored(rows) === server || stored(rows) === stored(draft.base));
  const elsewhere = dirty && !spent && stored(draft.base) !== server;
  useEffect(() => {
    if (spent) discard(id);
  }, [spent, discard, id]);
  const takeOver = () => {
    setMessage(null);
    discard(id);
  };

  const setRows = (next: (current: PathMapping[]) => PathMapping[]) =>
    drafts.set(id, (current) => ({ ...current, rows: next(current.rows) }));

  const change = (index: number, field: keyof PathMapping) => (value: string) =>
    setRows((current) => current.map((row, i) => (i === index ? { ...row, [field]: value } : row)));

  const onSubmit = (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    setMessage(null);
    save.mutate(rows.map((row) => ({ from: row.from.trim(), to: row.to.trim() })));
  };

  return (
    <Form className={styles.form} onSubmit={onSubmit} validationErrors={errors?.fields}>
      <Text tone="secondary">{t('admin.integrations.webhooks.pathMap.intro')}</Text>
      {elsewhere && (
        <Notice tone="warning">
          {t('admin.integrations.webhooks.pathMap.elsewhere')}{' '}
          <AriaButton variant="ghost" onPress={takeOver}>
            {t('admin.integrations.webhooks.pathMap.takeOver')}
          </AriaButton>
        </Notice>
      )}
      {rows.map((row, index) => (
        // The index is the identity here: the rows have no id, and reordering is not offered.
        <div key={index} className={styles.fields}>
          <TextField
            name={`pathMap.${index}.from`}
            label={t('admin.integrations.webhooks.pathMap.from')}
            autoComplete="off"
            spellCheck="false"
            mono
            value={row.from}
            onChange={change(index, 'from')}
          />
          <TextField
            name={`pathMap.${index}.to`}
            label={t('admin.integrations.webhooks.pathMap.to')}
            autoComplete="off"
            spellCheck="false"
            mono
            value={row.to}
            onChange={change(index, 'to')}
          />
          <AriaButton
            variant="ghost"
            icon="trash"
            aria-label={t('admin.integrations.webhooks.pathMap.remove', { from: row.from })}
            onPress={() => setRows((current) => current.filter((_, i) => i !== index))}
          >
            {t('admin.integrations.webhooks.pathMap.removeAction')}
          </AriaButton>
        </div>
      ))}
      <div className={styles.actions}>
        <AriaButton
          icon="plus"
          onPress={() => setRows((current) => [...current, { from: '', to: '' }])}
        >
          {t('admin.integrations.webhooks.pathMap.add')}
        </AriaButton>
        <AriaButton
          type="submit"
          variant="primary"
          isPending={save.isPending}
          aria-label={t('admin.integrations.webhooks.pathMap.saveLabel', {
            name: integration.name,
          })}
        >
          {t('admin.integrations.save')}
        </AriaButton>
      </div>
      {errors?.form && <Notice tone="danger">{problemMessage(t, save.error)}</Notice>}
      <div role="status" className={styles.live}>
        {message && !save.error && <Notice live={false}>{message}</Notice>}
      </div>
    </Form>
  );
}

/** The newest 20 reports of one integration. */
function EventLog({ integration }: { integration: Integration }) {
  const { t, i18n } = useTranslation();
  const events = useQuery(integrationEventsQuery(integration.id));

  if (events.isPending) return <Skeleton shape="block" />;
  if (events.isError) return <Notice tone="danger">{problemMessage(t, events.error)}</Notice>;
  if (events.data.length === 0) {
    return <Text tone="secondary">{t('admin.integrations.webhooks.log.empty')}</Text>;
  }
  return (
    <DataTable
      caption={t('admin.integrations.webhooks.log.caption', { name: integration.name })}
      rows={events.data}
      rowKey={(event) => String(event.id)}
      columns={[
        {
          key: 'receivedAt',
          header: t('admin.integrations.webhooks.log.at'),
          rowHeader: true,
          cell: (event) => formatDateTime(event.receivedAt, i18n.language),
        },
        {
          key: 'event',
          header: t('admin.integrations.webhooks.log.event'),
          cell: (event) => event.event,
        },
        {
          key: 'reportedPath',
          header: t('admin.integrations.webhooks.log.reported'),
          cell: (event) => <code className={table.url}>{event.reportedPath ?? '—'}</code>,
        },
        {
          key: 'resolvedPath',
          header: t('admin.integrations.webhooks.log.resolved'),
          cell: (event) => <code className={table.url}>{event.resolvedPath ?? '—'}</code>,
        },
        {
          key: 'result',
          header: t('admin.integrations.webhooks.log.result'),
          cell: (event) => (
            <Chip tone={resultTone[event.result]}>
              {t(`admin.integrations.webhooks.result.${event.result}`)}
            </Chip>
          ),
        },
      ]}
    />
  );
}

/** Renew and delete of one integration, each after a confirmation. */
function RowActions({
  integration,
  onRenewed,
  onRemoved,
  onError,
}: {
  integration: Integration;
  onRenewed: (created: IntegrationCreated) => void;
  onRemoved: () => void;
  onError: (error: unknown) => void;
}) {
  const { t } = useTranslation();
  const queryClient = useQueryClient();

  const invalidate = async () => {
    await queryClient.invalidateQueries({ queryKey: integrationsQuery.queryKey });
    await queryClient.invalidateQueries({ queryKey: dashboardQuery.queryKey });
  };
  const renew = useMutation({
    mutationFn: () => renewIntegrationToken(integration.id),
    onMutate: () => onError(null),
    onSuccess: async (created) => {
      onRenewed(created);
      await invalidate();
    },
    onError,
  });
  const remove = useMutation({
    mutationFn: () => deleteIntegration(integration.id),
    onMutate: () => onError(null),
    onSuccess: async () => {
      await invalidate();
      // The row and its buttons are gone: the focus goes to the page's heading (ALL-2).
      onRemoved();
    },
    onError,
  });

  return (
    <div className={styles.actions}>
      <ConfirmDialog
        trigger={
          <AriaButton
            icon="refresh"
            isPending={renew.isPending}
            aria-label={t('admin.integrations.webhooks.renew.label', { name: integration.name })}
          >
            {t('admin.integrations.webhooks.renew.action')}
          </AriaButton>
        }
        title={t('admin.integrations.webhooks.renew.title', { name: integration.name })}
        body={t('admin.integrations.webhooks.renew.body')}
        confirmLabel={t('admin.integrations.webhooks.renew.confirm')}
        cancelLabel={t('common.cancel')}
        tone="danger"
        onConfirm={() => renew.mutate()}
      />
      <ConfirmDialog
        trigger={
          <AriaButton
            variant="ghost"
            isPending={remove.isPending}
            aria-label={t('admin.integrations.webhooks.delete.label', { name: integration.name })}
          >
            {t('admin.integrations.webhooks.delete.action')}
          </AriaButton>
        }
        title={t('admin.integrations.webhooks.delete.title', { name: integration.name })}
        body={t('admin.integrations.webhooks.delete.body')}
        confirmLabel={t('admin.integrations.webhooks.delete.confirm')}
        cancelLabel={t('common.cancel')}
        tone="danger"
        onConfirm={() => remove.mutate()}
      />
    </div>
  );
}

/**
 * Incoming webhooks of Radarr and Sonarr (I-8, decision 5.6): one address per integration, shown
 * once; a table of the integrations whose rows fold out into the path mapping and the newest 20
 * reports. The mapping's unsaved rows live in the page (`drafts`), so folding a row away keeps
 * them. Slouchcat never calls those services.
 */
export function WebhooksSection({
  drafts,
  onRemoved,
}: {
  drafts: Drafts<PathMapDraft>;
  /** An integration was deleted: the page moves the focus to its heading. */
  onRemoved: () => void;
}) {
  const { t, i18n } = useTranslation();
  const [created, setCreated] = useState<IntegrationCreated | null>(null);
  const [error, setError] = useState<unknown>(null);
  const integrations = useQuery(integrationsQuery);

  return (
    <>
      <Text tone="secondary">{t('admin.integrations.webhooks.intro')}</Text>
      {created && <CreatedAddress created={created} onDismiss={() => setCreated(null)} />}
      <NewIntegrationForm onCreated={setCreated} />
      {error ? <Notice tone="danger">{problemMessage(t, error)}</Notice> : null}
      <div aria-busy={integrations.isPending}>
        {integrations.isPending && <Skeleton shape="block" />}
        {integrations.isError && (
          <Notice tone="danger">{problemMessage(t, integrations.error)}</Notice>
        )}
        {integrations.data?.length === 0 && (
          <Text tone="secondary">{t('admin.integrations.webhooks.empty')}</Text>
        )}
        {integrations.data && integrations.data.length > 0 && (
          <DataTable
            caption={t('admin.integrations.webhooks.caption')}
            rows={integrations.data}
            rowKey={(integration) => String(integration.id)}
            expand={{
              render: (integration) => (
                <div className={styles.detail}>
                  <PathMapForm integration={integration} drafts={drafts} />
                  <EventLog integration={integration} />
                </div>
              ),
            }}
            columns={[
              {
                key: 'name',
                header: t('admin.integrations.webhooks.columns.name'),
                rowHeader: true,
                cell: (integration) => (
                  <>
                    {integration.name}
                    {drafts.isDirty(String(integration.id)) && (
                      <Chip tone="warning">{t('admin.integrations.unsaved')}</Chip>
                    )}
                  </>
                ),
              },
              {
                key: 'kind',
                header: t('admin.integrations.webhooks.columns.kind'),
                cell: (integration) => (
                  <Chip tone="info">
                    {t(`admin.integrations.webhooks.kind.${integration.kind}`)}
                  </Chip>
                ),
              },
              {
                key: 'address',
                header: t('admin.integrations.webhooks.columns.address'),
                cell: (integration) => (
                  <code className={table.url}>
                    {t('admin.integrations.webhooks.tail', { tail: integration.tokenTail })}
                  </code>
                ),
              },
              {
                key: 'lastSeen',
                header: t('admin.integrations.webhooks.columns.lastSeen'),
                cell: (integration) =>
                  integration.lastSeenAt
                    ? formatDateTime(integration.lastSeenAt, i18n.language)
                    : t('admin.integrations.webhooks.neverSeen'),
              },
              {
                key: 'actions',
                header: t('admin.integrations.webhooks.columns.actions'),
                align: 'end',
                cell: (integration) => (
                  <RowActions
                    integration={integration}
                    onRenewed={setCreated}
                    onRemoved={() => {
                      setCreated((current) =>
                        current?.integration.id === integration.id ? null : current,
                      );
                      onRemoved();
                    }}
                    onError={setError}
                  />
                ),
              },
            ]}
          />
        )}
      </div>
    </>
  );
}
