import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { useCallback, useEffect, useMemo, useRef, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { problemMessage } from '@/api/errors';
import { paths } from '@/app/paths';
import { useActiveSession } from '@/app/session';
import { useDocumentTitle } from '@/lib/useDocumentTitle';
import { Chip, Notice, Skeleton, Text, TextLink } from '@/ui';
import {
  AdminPage,
  AriaButton,
  BandPanel,
  ConfirmDialog,
  DataTable,
  useDrafts,
  useUnsavedChanges,
} from '@/ui/aria';
import {
  setSpotlightEntryOverride,
  spotlightConfigQuery,
  spotlightEntriesQuery,
  traktStatusQuery,
  type SpotlightEntry,
  type SpotlightEntryOverride,
} from './queries';
import { entryColumns, entryKey, entryName, orphanColumns } from './spotlight/columns';
import { NoteDialog, PinDialog } from './spotlight/EntryDialogs';
import { RowChip, rowState } from './spotlight/FetchState';
import { SOURCE_DRAFT, SourceForm, sourceDraftOf, type SourceDraft } from './spotlight/SourceForm';
import styles from './spotlight/Spotlight.module.css';

/** After "Jetzt abrufen" the status and the entries are asked again this often for a while. */
const POLL_MS = 2000;
const POLL_FOR_MS = 30_000;

const sameEntry = (a: SpotlightEntry, b: SpotlightEntry) => entryKey(a) === entryKey(b);

/** Resolves once React has rendered what the last cache update brought. */
const afterRender = () =>
  new Promise<void>((resolve) => {
    window.setTimeout(() => window.requestAnimationFrame(() => resolve()), 0);
  });

/** A dialog of one entry; `opening` gives every opening a fresh form. */
interface EntryDialog {
  entry: SpotlightEntry;
  mode: 'pin' | 'note';
  opening: number;
}

/**
 * Spotlight (M11, ADR 0011; concept 12, variant A «Kompakte Liste», mockup Spo-A): the public
 * Trakt list shown as a row of the start page, in three bands that fold away. «Quelle» holds the
 * switch, the list and its numbers with one «Speichern», the state of the fetch, why the row is
 * missing and «Jetzt abrufen»; «Einträge der Liste» lists the fetched entries, each to pin to a
 * place, hide or annotate; «Verwaiste Anpassungen» lists overrides whose title left the list.
 * Order and hiding of the whole row stay on the page «Startseite».
 */
export function SpotlightPage() {
  const { t } = useTranslation();
  useDocumentTitle(t('admin.spotlight.title'));
  const session = useActiveSession();
  const allowed = session.account.role === 'admin' && session.profile.isPrimary;
  if (!allowed) {
    return (
      <AdminPage title={t('admin.spotlight.title')}>
        <Notice title={t('admin.access.notAllowed.title')}>
          {t('admin.access.notAllowed.body')}
        </Notice>
      </AdminPage>
    );
  }
  return <SpotlightContent />;
}

function SpotlightContent() {
  const { t } = useTranslation();
  const queryClient = useQueryClient();
  const titleRef = useRef<HTMLHeadingElement>(null);

  const [dialog, setDialog] = useState<EntryDialog | null>(null);
  const [dialogOpen, setDialogOpen] = useState(false);

  // The config is asked again on focus only while nothing is typed or asked: a draft compares
  // against it. `hold` comes from the draft, which needs the config, so a ref carries it back.
  const held = useRef(false);
  const config = useQuery({ ...spotlightConfigQuery, refetchOnWindowFocus: () => !held.current });
  const configData = config.data;
  const drafts = useDrafts<SourceDraft>(
    useCallback(
      (id: string) => (configData && id === SOURCE_DRAFT ? sourceDraftOf(configData) : undefined),
      [configData],
    ),
  );
  const unsaved = useUnsavedChanges(drafts.dirty);
  // Nothing is asked again while a dialog is open or the source holds unsaved input (S-6, S-13).
  const hold = dialogOpen || unsaved.dialog.isOpen || unsaved.pausePolling;
  useEffect(() => {
    held.current = hold;
  }, [hold]);

  // «Jetzt abrufen» polls the state and the entries every 2 s for 30 s.
  const [pollUntil, setPollUntil] = useState(0);
  useEffect(() => {
    if (pollUntil === 0) return;
    const timer = window.setTimeout(() => setPollUntil(0), Math.max(0, pollUntil - Date.now()));
    return () => window.clearTimeout(timer);
  }, [pollUntil]);
  const polling = pollUntil > 0 && !hold;

  const status = useQuery({
    ...traktStatusQuery,
    refetchInterval: polling ? POLL_MS : false,
    refetchOnWindowFocus: !hold,
  });
  const entries = useQuery({
    ...spotlightEntriesQuery,
    refetchInterval: polling ? POLL_MS : false,
    refetchOnWindowFocus: !hold,
  });
  // The outcome of the last action. Each message is a new node in the status region, so the
  // same sentence twice is announced twice.
  const [message, setMessage] = useState<{ text: string; id: number } | null>(null);
  const [failure, setFailure] = useState<unknown>(null);
  const said = useRef(0);
  const clear = useCallback(() => {
    setMessage(null);
    setFailure(null);
  }, []);
  const say = useCallback((text: string) => {
    said.current += 1;
    setFailure(null);
    setMessage({ text, id: said.current });
  }, []);
  const fail = useCallback((error: unknown) => {
    setMessage(null);
    setFailure(error);
  }, []);

  /** The focus fell to the body (its button or row went with a change): the h1 takes it. */
  const focusTitleIfLost = useCallback(() => {
    const active = document.activeElement;
    if (!active || active === document.body) titleRef.current?.focus();
  }, []);

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
    const title = entryName(t, entry);
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
      say(outcome(entry, body));
      await settle(saved);
      await afterRender();
      // An orphan's row and its button are gone: the h1 takes the focus.
      if (Object.keys(body).length === 0) titleRef.current?.focus();
      else focusTitleIfLost();
    },
    onError: fail,
  });
  const { mutate: mutateOverride } = override;
  const change = useCallback(
    (entry: SpotlightEntry, body: SpotlightEntryOverride) => {
      clear();
      mutateOverride({ entry, body });
    },
    [clear, mutateOverride],
  );

  const openDialog = useCallback((entry: SpotlightEntry, mode: EntryDialog['mode']) => {
    setDialog((current) => ({ entry, mode, opening: (current?.opening ?? 0) + 1 }));
    setDialogOpen(true);
  }, []);
  const closeDialog = useCallback(() => {
    setDialogOpen(false);
    // The button that opened it may be gone by now.
    void afterRender().then(focusTitleIfLost);
  }, [focusTitleIfLost]);

  const queued = async (text: string) => {
    say(text);
    setPollUntil(Date.now() + POLL_FOR_MS);
    await queryClient.invalidateQueries({ queryKey: traktStatusQuery.queryKey });
  };

  const columns = useMemo(
    () =>
      entryColumns(t, {
        onPin: (entry) => openDialog(entry, 'pin'),
        onNote: (entry) => openDialog(entry, 'note'),
        onChange: change,
      }),
    [t, openDialog, change],
  );
  const orphans = useMemo(() => orphanColumns(t, (entry) => change(entry, {})), [t, change]);

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

  const list = entries.data;
  const state = config.data && status.data && rowState(t, status.data, config.data, list);

  return (
    <AdminPage
      title={t('admin.spotlight.title')}
      help={t('admin.spotlight.intro')}
      liveSeconds={polling ? POLL_MS / 1000 : undefined}
      titleRef={titleRef}
    >
      {failure !== null && <Notice tone="danger">{problemMessage(t, failure)}</Notice>}
      <div role="status" className={styles.live}>
        {message && failure === null && (
          <Notice key={message.id} live={false}>
            {message.text}
          </Notice>
        )}
      </div>
      {status.isError && loadError(status.error, () => void status.refetch())}
      {config.isPending && <Skeleton shape="block" />}
      {config.isError && loadError(config.error, () => void config.refetch())}
      {config.data && (
        <BandPanel
          collapsible
          icon="spotlight"
          title={t('admin.spotlight.source.title')}
          meta={
            <>
              {drafts.dirty && <Chip tone="warning">{t('admin.spotlight.unsaved')}</Chip>}
              {state && <RowChip state={state} />}
            </>
          }
        >
          <SourceForm
            config={config.data}
            status={status.data}
            entries={list}
            drafts={drafts}
            onStart={clear}
            onSaved={say}
            onQueued={queued}
            onFailed={fail}
          />
        </BandPanel>
      )}

      <BandPanel
        collapsible
        icon="list"
        title={t('admin.spotlight.entries.title')}
        meta={list ? t('admin.spotlight.entries.count', { count: list.entries.length }) : undefined}
      >
        <Text tone="secondary">{t('admin.spotlight.entries.intro')}</Text>
        {entries.isPending && <Skeleton shape="block" />}
        {entries.isError && loadError(entries.error, () => void entries.refetch())}
        {list?.entries.length === 0 && (
          <Text tone="secondary">{t('admin.spotlight.entries.empty')}</Text>
        )}
        {list && list.entries.length > 0 && (
          <DataTable
            caption={t('admin.spotlight.entries.title')}
            columns={columns}
            rows={list.entries}
            rowKey={entryKey}
          />
        )}
      </BandPanel>

      {list && list.orphans.length > 0 && (
        <BandPanel
          collapsible
          icon="archive"
          title={t('admin.spotlight.orphans.title')}
          meta={t('admin.spotlight.orphans.count', { count: list.orphans.length })}
        >
          <Text tone="secondary">{t('admin.spotlight.orphans.intro')}</Text>
          <DataTable
            caption={t('admin.spotlight.orphans.title')}
            columns={orphans}
            rows={list.orphans}
            rowKey={entryKey}
          />
        </BandPanel>
      )}

      <Text tone="secondary">
        {t('admin.spotlight.rowSettings')}{' '}
        <TextLink href={paths.adminHomeRows}>{t('admin.spotlight.rowSettingsLink')}</TextLink>
      </Text>

      {dialog?.mode === 'pin' && config.data && (
        <PinDialog
          key={dialog.opening}
          entry={dialog.entry}
          isOpen={dialogOpen}
          places={config.data.itemCount}
          others={[...(list?.entries ?? []), ...(list?.orphans ?? [])]}
          onClose={closeDialog}
          onSaved={(saved, body) => {
            say(outcome(dialog.entry, body));
            setDialogOpen(false);
            void settle(saved);
          }}
        />
      )}
      {dialog?.mode === 'note' && (
        <NoteDialog
          key={dialog.opening}
          entry={dialog.entry}
          isOpen={dialogOpen}
          onClose={closeDialog}
          onSaved={(saved, body) => {
            say(outcome(dialog.entry, body));
            setDialogOpen(false);
            void settle(saved);
          }}
        />
      )}
      <ConfirmDialog {...unsaved.dialog} />
    </AdminPage>
  );
}
