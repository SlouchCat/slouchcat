import type { TraktStatus } from '@slouchcat/api-client';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { useId, useRef, useState, type FormEvent } from 'react';
import { useTranslation } from 'react-i18next';
import { codeMessage, formErrors, problemMessage } from '@/api/errors';
import { paths } from '@/app/paths';
import { useActiveSession } from '@/app/session';
import { formText } from '@/lib/form';
import { formatDateTime } from '@/lib/format';
import { useDocumentTitle } from '@/lib/useDocumentTitle';
import {
  Notice,
  Page,
  Panel,
  Skeleton,
  StatusDot,
  Text,
  TextLink,
  type StatusDotState,
} from '@/ui';
import {
  AriaButton,
  DataTable,
  Form,
  FormDialog,
  RadioGroup,
  Switch,
  TextField,
  type DataColumn,
} from '@/ui/aria';
import {
  refreshSpotlight,
  setSpotlightConfig,
  setSpotlightEntryOverride,
  spotlightConfigQuery,
  spotlightEntriesQuery,
  traktStatusQuery,
  type SpotlightConfig,
  type SpotlightEntries,
  type SpotlightEntry,
  type SpotlightEntryOverride,
} from './queries';
import { PosterTitle } from './PosterTitle';
import styles from './AdminTablePage.module.css';
import curation from './Curation.module.css';
import layout from './SpotlightPage.module.css';

/** After "Jetzt abrufen" the status and the entries are asked again this often for a while. */
const POLL_MS = 2000;
const POLL_FOR_MS = 30_000;

/** Longest note the server takes (`SpotlightEntryOverride.note`). */
const MAX_NOTE = 280;

const sameEntry = (a: SpotlightEntry, b: SpotlightEntry) =>
  a.media === b.media && a.tmdbId === b.tmdbId;

/**
 * The card's last line (mockup H, plan 7.1): when the list was fetched or why the last fetch
 * failed, how many of its titles are in the library, whether the server has a Trakt key (a link to
 * the integrations page, never the key), and why the row is missing from the start page now.
 */
function FetchState({
  status,
  config,
  entries,
}: {
  status?: TraktStatus;
  config: SpotlightConfig;
  entries?: SpotlightEntries;
}) {
  const { t, i18n } = useTranslation();
  if (!status) return <Skeleton shape="line" />;
  // `trakt_unavailable: HTTP 503`: the code in words when the UI knows it, else the server's text.
  const reason = (text: string) => {
    const [code = ''] = text.split(':');
    return codeMessage(t, code.trim()) ?? text;
  };
  // Hits are system-wide (plan 5.5): a profile may see fewer of them and still lose the row.
  const hits = entries?.entries.filter((entry) => entry.itemId !== undefined).length;
  const missing = (() => {
    if (!status.configured) return t('admin.spotlight.missing.noKey');
    if (!config.enabled) return t('admin.spotlight.missing.disabled');
    if (status.lastError) return t('admin.spotlight.missing.failed');
    if (status.lastSuccess && status.entries === 0) return t('admin.spotlight.missing.empty');
    if (hits !== undefined && hits < config.minHits) {
      return t('admin.spotlight.missing.belowMinHits', { hits, min: config.minHits });
    }
    return null;
  })();
  const dot: StatusDotState =
    !status.configured || status.lastError ? 'danger' : missing ? 'warning' : 'success';
  const fetched = status.lastError
    ? t('admin.spotlight.state.failed', { reason: reason(status.lastError) })
    : status.lastSuccess
      ? t('admin.spotlight.state.fetched', {
          date: formatDateTime(status.lastSuccess, i18n.language),
        })
      : t('admin.spotlight.state.never');
  const separator = <span aria-hidden="true">{t('admin.spotlight.state.separator')}</span>;

  return (
    <div className={layout.state}>
      <p className={layout.stateLine}>
        <StatusDot state={dot} />
        <span>{fetched}</span>
        {hits !== undefined && (
          <>
            {separator}
            <span>{t('admin.spotlight.state.hits', { hits, entries: status.entries })}</span>
          </>
        )}
        {separator}
        <TextLink href={paths.adminIntegrations}>
          {status.configured ? t('admin.spotlight.state.keySet') : t('admin.spotlight.state.noKey')}
        </TextLink>
      </p>
      {missing && <p>{missing}</p>}
    </div>
  );
}

/**
 * "Quelle": the list and its numbers, with the state of the fetch and "Jetzt abrufen" at its foot.
 * Which media a list kind allows, whether it needs an address and whether the media filters after
 * the fetch all come from `listKinds` (plan 5.6): the page keeps no table of its own, so a kind
 * with a single medium shows that medium, locked. `enabled` is the switch in the page head; it is
 * saved with the rest.
 */
function SourceCard({
  config,
  status,
  entries,
  enabled,
  onSaved,
  onQueued,
}: {
  config: SpotlightConfig;
  status?: TraktStatus;
  entries?: SpotlightEntries;
  enabled: boolean;
  onSaved: () => void;
  onQueued: () => Promise<void>;
}) {
  const { t } = useTranslation();
  const queryClient = useQueryClient();
  const headingId = useId();
  const [listKey, setListKey] = useState(config.listKey);
  const [listUrl, setListUrl] = useState(config.listUrl ?? '');
  const [chosenMedia, setChosenMedia] = useState(config.media);
  const [itemCount, setItemCount] = useState(String(config.itemCount));
  const [minHits, setMinHits] = useState(String(config.minHits));
  const [intervalHours, setIntervalHours] = useState(String(config.intervalHours));
  const [message, setMessage] = useState<string | null>(null);

  const kind = config.listKinds.find((entry) => entry.key === listKey);
  const allowedMedia = kind?.media ?? [chosenMedia];
  // A choice the new kind does not offer falls back to the kind's first medium.
  const media = allowedMedia.includes(chosenMedia) ? chosenMedia : (allowedMedia[0] ?? chosenMedia);

  const save = useMutation({
    mutationFn: setSpotlightConfig,
    onSuccess: async (saved) => {
      queryClient.setQueryData(spotlightConfigQuery.queryKey, saved);
      onSaved();
      setMessage(t('admin.spotlight.source.saved'));
      await Promise.all([
        queryClient.invalidateQueries({ queryKey: traktStatusQuery.queryKey }),
        queryClient.invalidateQueries({ queryKey: spotlightEntriesQuery.queryKey }),
        queryClient.invalidateQueries({ queryKey: ['home'] }),
      ]);
    },
  });
  const refresh = useMutation({
    mutationFn: refreshSpotlight,
    onSuccess: async () => {
      setMessage(t('admin.spotlight.source.queued'));
      await onQueued();
    },
  });
  const errors = save.error ? formErrors(t, save.error) : null;
  const failed = refresh.error ?? (errors?.form ? save.error : null);

  const onSubmit = (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    setMessage(null);
    refresh.reset();
    save.mutate({
      enabled,
      listKey,
      media,
      itemCount: Number(itemCount),
      minHits: Number(minHits),
      intervalHours: Number(intervalHours),
      ...(kind?.needsUrl ? { listUrl: listUrl.trim() } : {}),
    });
  };
  const required = (value: string) => (value.trim() ? null : t('errors.field.required'));

  return (
    <Panel as="section" aria-labelledby={headingId}>
      <Text as="h2" variant="title">
        <span id={headingId}>{t('admin.spotlight.source.title')}</span>
      </Text>
      <Form className={styles.form} onSubmit={onSubmit} validationErrors={errors?.fields}>
        <div className={layout.fields}>
          <div className={layout.wide}>
            <RadioGroup
              name="listKey"
              label={t('admin.spotlight.source.list')}
              description={
                <>
                  {t('admin.spotlight.source.listHint')}{' '}
                  <TextLink href={paths.adminIntegrations}>
                    {t('admin.spotlight.source.listHintLink')}
                  </TextLink>
                </>
              }
              value={listKey}
              onChange={(value) =>
                setListKey(config.listKinds.find((entry) => entry.key === value)?.key ?? listKey)
              }
              options={config.listKinds.map((entry) => ({
                value: entry.key,
                label: t(`admin.spotlight.lists.${entry.key}`, { defaultValue: entry.key }),
              }))}
            />
            {kind?.needsUrl && (
              <TextField
                name="listUrl"
                type="url"
                label={t('admin.spotlight.source.listUrl')}
                description={t('admin.spotlight.source.listUrlHint')}
                autoComplete="off"
                isRequired
                value={listUrl}
                onChange={setListUrl}
                validate={required}
              />
            )}
          </div>
          <div className={layout.wide}>
            <RadioGroup
              name="media"
              label={t('admin.spotlight.source.media')}
              description={
                allowedMedia.length === 1
                  ? t('admin.spotlight.source.mediaFixed')
                  : kind?.mediaFilters
                    ? t('admin.spotlight.source.mediaFilters')
                    : undefined
              }
              value={media}
              isDisabled={allowedMedia.length === 1}
              onChange={(value) =>
                setChosenMedia(allowedMedia.find((entry) => entry === value) ?? chosenMedia)
              }
              options={allowedMedia.map((entry) => ({
                value: entry,
                label: t(`admin.spotlight.media.${entry}`),
              }))}
            />
          </div>
          <div className={layout.narrow}>
            <TextField
              name="itemCount"
              type="number"
              inputMode="numeric"
              label={t('admin.spotlight.source.itemCount')}
              description={t('admin.spotlight.source.itemCountHint')}
              isRequired
              value={itemCount}
              onChange={setItemCount}
              validate={required}
            />
          </div>
          <div className={layout.narrow}>
            <TextField
              name="minHits"
              type="number"
              inputMode="numeric"
              label={t('admin.spotlight.source.minHits')}
              description={t('admin.spotlight.source.minHitsHint')}
              isRequired
              value={minHits}
              onChange={setMinHits}
              validate={required}
            />
          </div>
          <div className={layout.narrow}>
            <TextField
              name="intervalHours"
              type="number"
              inputMode="numeric"
              label={t('admin.spotlight.source.intervalHours')}
              description={t('admin.spotlight.source.intervalHoursHint')}
              isRequired
              value={intervalHours}
              onChange={setIntervalHours}
              validate={required}
            />
          </div>
        </div>
        {failed && <Notice tone="danger">{problemMessage(t, failed)}</Notice>}
        <div role="status" className={curation.live}>
          {message && !failed && <Notice live={false}>{message}</Notice>}
        </div>
        <div className={layout.footer}>
          <FetchState status={status} config={config} entries={entries} />
          <div className={layout.buttons}>
            <AriaButton
              variant="secondary"
              icon="refresh"
              isPending={refresh.isPending}
              onPress={() => {
                setMessage(null);
                save.reset();
                refresh.mutate();
              }}
            >
              {t('admin.spotlight.source.refresh')}
            </AriaButton>
            <AriaButton type="submit" variant="primary" isPending={save.isPending}>
              {save.isPending ? t('common.saving') : t('common.save')}
            </AriaButton>
          </div>
        </div>
      </Form>
    </Panel>
  );
}

/**
 * "Anheften" (the place in the row) or "Notiz" of one entry. An emptied note removes it; an
 * unchanged one closes the dialog without a request, because an empty body would remove the whole
 * override.
 */
function EntryDialog({
  entry,
  name,
  mode,
  onClose,
  onSaved,
}: {
  entry: SpotlightEntry;
  name: string;
  mode: 'pin' | 'note';
  onClose: () => void;
  onSaved: (saved: SpotlightEntry, body: SpotlightEntryOverride) => void;
}) {
  const { t } = useTranslation();
  const update = useMutation({
    mutationFn: (body: SpotlightEntryOverride) =>
      setSpotlightEntryOverride(entry.media, entry.tmdbId, body),
    onSuccess: onSaved,
  });
  const errors = update.error ? formErrors(t, update.error) : null;
  const onSubmit = (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    const data = new FormData(event.currentTarget);
    if (mode === 'pin') {
      update.mutate({ pinnedRank: Number(formText(data, 'pinnedRank')) });
      return;
    }
    const note = formText(data, 'note') || null;
    if (note === (entry.note ?? null)) {
      onClose();
      return;
    }
    update.mutate({ note });
  };
  return (
    <FormDialog
      isOpen
      onOpenChange={(open) => {
        if (!open) onClose();
      }}
      title={
        mode === 'pin'
          ? t('admin.spotlight.pin.title', { title: name })
          : t('admin.spotlight.note.title', { title: name })
      }
    >
      <Form className={styles.form} onSubmit={onSubmit} validationErrors={errors?.fields}>
        {errors?.form && <Notice tone="danger">{errors.form}</Notice>}
        {mode === 'pin' ? (
          <TextField
            name="pinnedRank"
            type="number"
            inputMode="numeric"
            label={t('admin.spotlight.pin.rank')}
            description={t('admin.spotlight.pin.rankHint')}
            isRequired
            validate={(value) => (value.trim() ? null : t('errors.field.required'))}
            autoComplete="off"
          />
        ) : (
          <TextField
            name="note"
            label={t('admin.spotlight.note.field')}
            description={t('admin.spotlight.note.hint', { max: MAX_NOTE })}
            defaultValue={entry.note}
            maxLength={MAX_NOTE}
            autoComplete="off"
          />
        )}
        <div className={styles.dialogActions}>
          <AriaButton variant="ghost" onPress={onClose}>
            {t('common.cancel')}
          </AriaButton>
          <AriaButton type="submit" variant="primary" isPending={update.isPending}>
            {update.isPending ? t('common.saving') : t('common.save')}
          </AriaButton>
        </div>
      </Form>
    </FormDialog>
  );
}

/**
 * Spotlight (M11, ADR 0011, mockup H): the public Trakt list shown as a row of the start page. In
 * the head the switch that shows it; the card "Quelle" with the list, its numbers and the state of
 * the fetch ("Jetzt abrufen"); the card "Einträge der Liste" with the fetched entries, each to pin
 * to a place, hide or annotate; overrides whose title left the list stay deletable. Order and
 * hiding of the whole row stay on the page "Startseite".
 */
export function SpotlightPage() {
  const { t } = useTranslation();
  useDocumentTitle(t('admin.spotlight.title'));
  const session = useActiveSession();
  const allowed = session.account.role === 'owner' && session.profile.isPrimary;
  const queryClient = useQueryClient();
  const [pollUntil, setPollUntil] = useState(0);
  const poll = () => (Date.now() < pollUntil ? POLL_MS : false);
  const config = useQuery({ ...spotlightConfigQuery, enabled: allowed });
  const status = useQuery({ ...traktStatusQuery, enabled: allowed, refetchInterval: poll });
  const entries = useQuery({ ...spotlightEntriesQuery, enabled: allowed, refetchInterval: poll });
  // The switch in the head belongs to the settings: "Speichern" stores it with them.
  const [enabled, setEnabled] = useState<boolean | null>(null);
  const [dialog, setDialog] = useState<{ entry: SpotlightEntry; mode: 'pin' | 'note' } | null>(
    null,
  );
  const [message, setMessage] = useState<string | null>(null);
  const header = useRef<HTMLDivElement>(null);
  const entriesId = useId();
  const orphansId = useId();

  const name = (entry: SpotlightEntry) =>
    entry.title ?? t('admin.spotlight.entries.untitled', { id: entry.tmdbId });

  // The answer replaces its entry at once, then the list and the start page are asked again.
  const settle = async (saved: SpotlightEntry) => {
    queryClient.setQueryData(spotlightEntriesQuery.queryKey, (data) =>
      data
        ? {
            ...data,
            entries: data.entries.map((entry) => (sameEntry(entry, saved) ? saved : entry)),
            orphans: data.orphans.map((entry) => (sameEntry(entry, saved) ? saved : entry)),
          }
        : data,
    );
    await Promise.all([
      queryClient.invalidateQueries({ queryKey: spotlightEntriesQuery.queryKey }),
      queryClient.invalidateQueries({ queryKey: ['home'] }),
    ]);
  };
  const outcome = (entry: SpotlightEntry, body: SpotlightEntryOverride) => {
    const title = name(entry);
    if (Object.keys(body).length === 0) return t('admin.spotlight.orphans.removed', { title });
    if (body.hidden !== undefined) {
      return t(body.hidden ? 'admin.spotlight.entries.hidden' : 'admin.spotlight.entries.shown', {
        title,
      });
    }
    if (body.pinnedRank === null) return t('admin.spotlight.unpin.done', { title });
    if (body.pinnedRank !== undefined) {
      return t('admin.spotlight.pin.done', { title, rank: body.pinnedRank });
    }
    return body.note === null
      ? t('admin.spotlight.note.removed', { title })
      : t('admin.spotlight.note.saved', { title });
  };
  const override = useMutation({
    mutationFn: ({ entry, body }: { entry: SpotlightEntry; body: SpotlightEntryOverride }) =>
      setSpotlightEntryOverride(entry.media, entry.tmdbId, body),
    onSuccess: async (saved, { entry, body }) => {
      setMessage(outcome(entry, body));
      await settle(saved);
      // An orphan's row and its button are gone: the focus goes to the page heading.
      if (Object.keys(body).length === 0) header.current?.focus();
    },
  });
  const change = (entry: SpotlightEntry, body: SpotlightEntryOverride) => {
    setMessage(null);
    override.mutate({ entry, body });
  };
  const queued = async () => {
    setPollUntil(Date.now() + POLL_FOR_MS);
    await queryClient.invalidateQueries({ queryKey: traktStatusQuery.queryKey });
  };

  const titleText = (entry: SpotlightEntry, className = '') => (
    <span className={`${styles.stack} ${className}`}>
      <span className={layout.title}>{name(entry)}</span>
      <span className={styles.muted}>
        {entry.year
          ? t('admin.spotlight.entries.meta', {
              media: t(`admin.spotlight.entries.media.${entry.media}`),
              year: entry.year,
            })
          : t(`admin.spotlight.entries.media.${entry.media}`)}
      </span>
    </span>
  );
  // A hidden entry: poster and title at half strength (mockup H).
  const titleCell = (entry: SpotlightEntry) => (
    <PosterTitle posterUrl={entry.posterUrl} className={entry.hidden ? layout.dimmed : ''}>
      {titleText(entry)}
    </PosterTitle>
  );
  const pinnedText = (entry: SpotlightEntry) =>
    entry.pinnedRank === undefined
      ? null
      : t('admin.spotlight.entries.pinned', { rank: entry.pinnedRank });
  const noteCell = (entry: SpotlightEntry) =>
    entry.note === undefined ? (
      t('admin.spotlight.entries.none')
    ) : (
      <span className={layout.note}>
        {t('admin.spotlight.entries.quoted', { note: entry.note })}
      </span>
    );
  const statusCell = (entry: SpotlightEntry) => {
    const [state, word] =
      entry.itemId === undefined
        ? (['warning', 'admin.spotlight.entries.state.missing'] as const)
        : entry.hidden
          ? (['pending', 'admin.spotlight.entries.state.hidden'] as const)
          : (['success', 'admin.spotlight.entries.state.visible'] as const);
    return (
      <span className={styles.stack}>
        <span className={layout.dot}>
          <StatusDot state={state} />
          {t(word)}
        </span>
        {pinnedText(entry) && <span className={styles.muted}>{pinnedText(entry)}</span>}
      </span>
    );
  };

  const columns: DataColumn<SpotlightEntry>[] = [
    {
      key: 'rank',
      header: t('admin.spotlight.entries.columns.rank'),
      align: 'end',
      cell: (entry) => <span className={styles.numbers}>{entry.position}</span>,
    },
    {
      key: 'title',
      header: t('admin.spotlight.entries.columns.title'),
      rowHeader: true,
      cell: titleCell,
    },
    { key: 'status', header: t('admin.spotlight.entries.columns.status'), cell: statusCell },
    { key: 'note', header: t('admin.spotlight.entries.columns.note'), cell: noteCell },
    {
      key: 'actions',
      header: t('admin.spotlight.entries.columns.actions'),
      cell: (entry) => {
        if (entry.itemId === undefined) {
          return <span className={layout.grey}>{t('admin.spotlight.entries.notInLibrary')}</span>;
        }
        const title = name(entry);
        return (
          <span className={styles.rowActions}>
            {entry.pinnedRank === undefined ? (
              <AriaButton
                variant="ghost"
                aria-label={t('admin.spotlight.pin.label', { title })}
                onPress={() => setDialog({ entry, mode: 'pin' })}
              >
                {t('admin.spotlight.pin.action')}
              </AriaButton>
            ) : (
              <AriaButton
                variant="ghost"
                aria-label={t('admin.spotlight.unpin.label', { title })}
                onPress={() => change(entry, { pinnedRank: null })}
              >
                {t('admin.spotlight.unpin.action')}
              </AriaButton>
            )}
            <AriaButton
              variant="ghost"
              aria-label={t(
                entry.hidden
                  ? 'admin.spotlight.entries.showLabel'
                  : 'admin.spotlight.entries.hideLabel',
                { title },
              )}
              onPress={() => change(entry, { hidden: !entry.hidden })}
            >
              {t(entry.hidden ? 'admin.spotlight.entries.show' : 'admin.spotlight.entries.hide')}
            </AriaButton>
            <AriaButton
              variant="ghost"
              aria-label={t('admin.spotlight.note.label', { title })}
              onPress={() => setDialog({ entry, mode: 'note' })}
            >
              {t('admin.spotlight.note.action')}
            </AriaButton>
          </span>
        );
      },
    },
  ];

  const orphanSummary = (entry: SpotlightEntry) =>
    [
      entry.hidden ? t('admin.spotlight.entries.state.hidden') : null,
      pinnedText(entry),
      entry.note === undefined ? null : t('admin.spotlight.entries.quoted', { note: entry.note }),
    ]
      .filter(Boolean)
      .join(t('admin.spotlight.state.separator'));
  const orphanColumns: DataColumn<SpotlightEntry>[] = [
    {
      key: 'title',
      header: t('admin.spotlight.entries.columns.title'),
      rowHeader: true,
      cell: (entry) => titleText(entry, entry.hidden ? layout.dimmed : ''),
    },
    { key: 'override', header: t('admin.spotlight.orphans.override'), cell: orphanSummary },
    {
      key: 'actions',
      header: t('admin.spotlight.entries.columns.actions'),
      hideHeader: true,
      align: 'end',
      cell: (entry) => (
        <AriaButton
          variant="ghost"
          icon="trash"
          aria-label={t('admin.spotlight.orphans.removeLabel', { title: name(entry) })}
          onPress={() => change(entry, {})}
        >
          {t('admin.spotlight.orphans.remove')}
        </AriaButton>
      ),
    },
  ];

  const loadError = (error: unknown, retry: () => void) => (
    <Notice
      tone="danger"
      title={problemMessage(t, error)}
      actions={
        <AriaButton icon="refresh" onPress={retry}>
          {t('common.retry')}
        </AriaButton>
      }
    />
  );

  return (
    <Page>
      <div className={layout.top}>
        <div ref={header} tabIndex={-1} className={layout.intro}>
          <Text as="h1" variant="display">
            {t('admin.spotlight.title')}
          </Text>
          <Text tone="secondary">{t('admin.spotlight.intro')}</Text>
        </div>
        {allowed && config.data && (
          <Switch isSelected={enabled ?? config.data.enabled} onChange={setEnabled}>
            {t('admin.spotlight.enabled')}
          </Switch>
        )}
      </div>
      {!allowed ? (
        <Notice title={t('admin.access.notAllowed.title')}>
          {t('admin.access.notAllowed.body')}
        </Notice>
      ) : (
        <div className={layout.cards}>
          {status.isError && loadError(status.error, () => void status.refetch())}
          {config.isPending && <Skeleton shape="block" />}
          {config.isError && loadError(config.error, () => void config.refetch())}
          {config.data && (
            <SourceCard
              config={config.data}
              status={status.data}
              entries={entries.data}
              enabled={enabled ?? config.data.enabled}
              onSaved={() => setEnabled(null)}
              onQueued={queued}
            />
          )}

          <Panel as="section" aria-labelledby={entriesId}>
            <div className={layout.cardHead}>
              <Text as="h2" variant="title">
                <span id={entriesId}>{t('admin.spotlight.entries.title')}</span>
              </Text>
              <span className={layout.small}>{t('admin.spotlight.entries.intro')}</span>
            </div>
            {override.isError && <Notice tone="danger">{problemMessage(t, override.error)}</Notice>}
            <div role="status" className={curation.live}>
              {message && !override.isError && <Notice live={false}>{message}</Notice>}
            </div>
            {entries.isPending && <Skeleton shape="block" />}
            {entries.isError && loadError(entries.error, () => void entries.refetch())}
            {entries.data?.entries.length === 0 && (
              <Notice>{t('admin.spotlight.entries.empty')}</Notice>
            )}
            {entries.data && entries.data.entries.length > 0 && (
              <DataTable
                caption={t('admin.spotlight.entries.title')}
                columns={columns}
                rows={entries.data.entries}
                rowKey={(entry) => `${entry.media}-${entry.tmdbId}`}
              />
            )}
          </Panel>

          {entries.data && entries.data.orphans.length > 0 && (
            <Panel as="section" aria-labelledby={orphansId}>
              <div className={styles.stack}>
                <Text as="h2" variant="title">
                  <span id={orphansId}>{t('admin.spotlight.orphans.title')}</span>
                </Text>
                <Text tone="secondary">{t('admin.spotlight.orphans.intro')}</Text>
              </div>
              <DataTable
                caption={t('admin.spotlight.orphans.title')}
                columns={orphanColumns}
                rows={entries.data.orphans}
                rowKey={(entry) => `${entry.media}-${entry.tmdbId}`}
              />
            </Panel>
          )}

          <Text tone="secondary">
            {t('admin.spotlight.rowSettings')}{' '}
            <TextLink href={paths.adminHomeRows}>{t('admin.spotlight.rowSettingsLink')}</TextLink>
          </Text>
        </div>
      )}
      {dialog && (
        <EntryDialog
          entry={dialog.entry}
          name={name(dialog.entry)}
          mode={dialog.mode}
          onClose={() => setDialog(null)}
          onSaved={(saved, body) => {
            setMessage(outcome(dialog.entry, body));
            setDialog(null);
            void settle(saved);
          }}
        />
      )}
    </Page>
  );
}
