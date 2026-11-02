import type { HomeRowSetting } from '@slouchcat/api-client';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { useCallback, useId, useMemo, useRef, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { problemMessage } from '@/api/errors';
import { paths } from '@/app/paths';
import { useActiveSession } from '@/app/session';
import { useDocumentTitle } from '@/lib/useDocumentTitle';
import { Notice, Skeleton, StatusDot, Text, TextLink } from '@/ui';
import {
  AdminPage,
  AriaButton,
  BandPanel,
  ConfirmDialog,
  DataTable,
  RankHint,
  RankSelect,
  SaveBar,
  StatusRegion,
  afterDialogsClosed,
  afterRender,
  focusTitleIfLost,
  rankTo,
  useDrafts,
  useSearchSelection,
  useStatusMessage,
  useUnsavedChanges,
  type DataColumn,
  type TabItem,
} from '@/ui/aria';
import { HomeRowsPreview } from './homerows/HomeRowsPreview';
import { pageOf, rowTitleKey, toRows, toSettings, type HomeRow } from './homerows/rows';
import { homeRowsQuery, setHomeRows } from './queries';
import styles from './homerows/HomeRows.module.css';

/** The tabs in the band: the order (the default, no `?tab=`) and the preview. */
const ORDER = 'order';
const PREVIEW = 'preview';
const TAB_IDS = [ORDER, PREVIEW] as const;

/** The one draft of the page: the whole list, order and hiding together (S-17). */
const DRAFT = 'rows';

/**
 * Start page (M7, E-9; concept 12, variant C): the order of its eight rows for every profile.
 * The tab «Reihenfolge» gives each row a position (RankSelect, S-16) and «Ausblenden», and folds
 * out what the row shows with a link to the page that keeps its content; the tab «Vorschau»
 * sketches the start page in the order on the page, saved or not. The SaveBar stores everything
 * at once or discards it. The Trakt list behind Spotlight and its state are on the spotlight
 * page; one line links there.
 */
export function HomeRowsPage() {
  const { t } = useTranslation();
  useDocumentTitle(t('admin.homeRows.title'));
  const session = useActiveSession();
  const allowed = session.account.role === 'admin' && session.profile.isPrimary;
  if (!allowed) {
    return (
      <AdminPage title={t('admin.homeRows.title')}>
        <Notice title={t('admin.access.notAllowed.title')}>
          {t('admin.access.notAllowed.body')}
        </Notice>
      </AdminPage>
    );
  }
  return <HomeRowsContent />;
}

function HomeRowsContent() {
  const { t, i18n } = useTranslation();
  const queryClient = useQueryClient();
  const number = useMemo(() => new Intl.NumberFormat(i18n.language), [i18n.language]);
  const titleRef = useRef<HTMLHeadingElement>(null);
  const hintId = useId();

  // The draft needs what the server holds before the query knows whether to pause for it: a
  // passive observer reads the same cache entry without fetching.
  const cached = useQuery({ ...homeRowsQuery, enabled: false }).data;
  const original = useMemo(() => (cached ? toRows(cached) : undefined), [cached]);
  const originalOf = useCallback((id: string) => (id === DRAFT ? original : undefined), [original]);
  const drafts = useDrafts<HomeRow[]>(originalOf);
  const unsaved = useUnsavedChanges(drafts.dirty);
  const [confirmOpen, setConfirmOpen] = useState(false);
  // No refetch while the list is changed (S-13); the questions open only then.
  const stored = useQuery({
    ...homeRowsQuery,
    refetchOnWindowFocus: !unsaved.pausePolling,
    refetchOnReconnect: !unsaved.pausePolling,
  });

  // An unknown `?tab=` shows «Reihenfolge», and the address says so too (replaced, not pushed).
  const [tab, selectTab] = useSearchSelection('tab', TAB_IDS, ORDER, { normalise: true });

  // Open rows, held by the page: switching to the preview and back keeps them open.
  const [expanded, setExpanded] = useState<ReadonlySet<string>>(() => new Set());
  const onExpandedChange = useCallback(
    (key: string, open: boolean) =>
      setExpanded((current) => {
        const next = new Set(current);
        if (open) next.add(key);
        else next.delete(key);
        return next;
      }),
    [],
  );

  // Every message is a node of its own, so the same sentence twice is announced twice.
  const [message, say] = useStatusMessage();

  const save = useMutation({
    mutationFn: (rows: HomeRow[]) => setHomeRows(toSettings(rows)),
    onSuccess: async (saved: HomeRowSetting[]) => {
      // The server holds the draft now: it is spent and drops out (useDrafts).
      queryClient.setQueryData(homeRowsQuery.queryKey, saved);
      drafts.discard(DRAFT);
      say(t('admin.homeRows.saved'));
      await queryClient.invalidateQueries({ queryKey: ['home'] });
      // «Speichern» went disabled with the draft.
      await afterRender();
      focusTitleIfLost(titleRef);
    },
  });

  const rows = drafts.valueOf(DRAFT) ?? [];
  const name = (row: HomeRow) => t(rowTitleKey[row.key]);
  const change = (next: HomeRow[], text: string | null) => {
    drafts.set(DRAFT, next);
    say(text);
    if (save.isError) save.reset();
  };
  const moveTo = (row: HomeRow, position: number) =>
    change(
      rankTo(rows, rows.indexOf(row), position, 'move'),
      t('admin.homeRows.moved', { row: name(row), position: position + 1 }),
    );
  const setHidden = (row: HomeRow, hidden: boolean) =>
    change(
      rows.map((entry) => (entry.key === row.key ? { ...entry, hidden } : entry)),
      null,
    );

  const onDiscard = () => {
    drafts.discard(DRAFT);
    if (save.isError) save.reset();
    say(t('admin.homeRows.discarded'));
  };
  const onConfirmOpenChange = useCallback((open: boolean) => {
    setConfirmOpen(open);
    if (open) return;
    // «Abbrechen» gives the focus back to «Verwerfen»; after discarding, that button is
    // disabled and the h1 takes the focus (ALL-2).
    void afterDialogsClosed().then(() => focusTitleIfLost(titleRef));
  }, []);

  const columns: DataColumn<HomeRow>[] = [
    {
      key: 'position',
      header: t('admin.homeRows.columns.position'),
      cell: (row) => (
        <RankSelect
          name={name(row)}
          position={rows.indexOf(row)}
          count={rows.length}
          describedBy={hintId}
          isDisabled={save.isPending}
          onChange={(position) => moveTo(row, position)}
        />
      ),
    },
    {
      key: 'row',
      header: t('admin.homeRows.columns.row'),
      rowHeader: true,
      cell: (row) => name(row),
    },
    {
      key: 'state',
      header: t('admin.homeRows.columns.state'),
      cell: (row) => (
        <span className={styles.state}>
          <StatusDot state={row.hidden ? 'pending' : 'success'} />
          {row.hidden ? t('admin.homeRows.state.hidden') : t('admin.homeRows.state.visible')}
        </span>
      ),
    },
    {
      key: 'hidden',
      header: t('admin.homeRows.columns.hidden'),
      cell: (row) => (
        <label className={styles.hide}>
          <input
            type="checkbox"
            checked={row.hidden}
            disabled={save.isPending}
            aria-label={t('admin.homeRows.hideLabel', { row: name(row) })}
            onChange={(event) => setHidden(row, event.target.checked)}
          />
          {t('admin.homeRows.hide')}
        </label>
      ),
    },
  ];

  const detail = (row: HomeRow) => {
    const page = pageOf(row.key);
    return (
      <div className={styles.detail}>
        <p>{t(`admin.homeRows.rows.${row.key}`)}</p>
        {page && (
          <TextLink href={page.href}>
            {t('admin.homeRows.pageLink', { page: t(page.titleKey) })}
          </TextLink>
        )}
      </div>
    );
  };

  const visible = rows.filter((row) => !row.hidden).length;
  const tabs: TabItem[] = [
    {
      id: ORDER,
      label: t('admin.homeRows.tabs.order'),
      count: number.format(rows.length),
      countLabel: t('admin.homeRows.count', {
        count: rows.length,
        number: number.format(rows.length),
      }),
      content: (
        <div className={styles.tab}>
          <RankHint id={hintId} />
          <DataTable
            caption={t('admin.homeRows.list')}
            columns={columns}
            rows={rows}
            rowKey={(row) => row.key}
            expand={{ render: detail, expandedKeys: expanded, onExpandedChange }}
          />
        </div>
      ),
    },
    {
      id: PREVIEW,
      label: t('admin.homeRows.tabs.preview'),
      count: number.format(visible),
      countLabel: t('admin.homeRows.visibleCount', {
        count: visible,
        number: number.format(visible),
      }),
      content: <HomeRowsPreview rows={rows} />,
    },
  ];

  return (
    <AdminPage
      title={t('admin.homeRows.title')}
      help={t('admin.homeRows.intro')}
      titleRef={titleRef}
    >
      {stored.isPending && <Skeleton shape="block" />}
      {stored.isError && (
        <Notice
          tone="danger"
          title={problemMessage(t, stored.error)}
          actions={
            <AriaButton icon="refresh" onPress={() => void stored.refetch()}>
              {t('common.retry')}
            </AriaButton>
          }
        />
      )}
      {rows.length > 0 && (
        <>
          <BandPanel
            title={t('admin.homeRows.band')}
            icon="list"
            tabs={{
              label: t('admin.homeRows.tabs.label'),
              selectedKey: tab,
              onSelectionChange: selectTab,
              tabs,
            }}
          />
          {/* A direct child of the page's column, so it sticks to the bottom while the rows scroll
              (a wrapper only as tall as the bar would hold it in place). */}
          <SaveBar
            dirty={drafts.dirty}
            saving={save.isPending}
            onSave={() => save.mutate(rows)}
            onDiscard={() => setConfirmOpen(true)}
          />
          {save.isError && <Notice tone="danger">{problemMessage(t, save.error)}</Notice>}
          <StatusRegion message={!save.isError ? message : null} />
          <Text tone="secondary">
            {t('admin.homeRows.spotlight')}{' '}
            <TextLink href={paths.adminSpotlight}>{t('admin.homeRows.spotlightLink')}</TextLink>
          </Text>
        </>
      )}
      <ConfirmDialog
        isOpen={confirmOpen}
        onOpenChange={onConfirmOpenChange}
        title={t('admin.homeRows.discard.title')}
        body={t('admin.homeRows.discard.body')}
        confirmLabel={t('admin.homeRows.discard.confirm')}
        cancelLabel={t('common.cancel')}
        tone="danger"
        defaultFocus="cancel"
        onConfirm={onDiscard}
      />
      <ConfirmDialog {...unsaved.dialog} />
    </AdminPage>
  );
}
