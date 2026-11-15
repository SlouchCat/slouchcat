import type { MyMediaSetting } from '@slouchcat/api-client';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { useCallback, useId, useMemo, useRef, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { problemMessage } from '@/api/errors';
import { paths } from '@/app/paths';
import { useActiveSession } from '@/app/session';
import { useDocumentTitle } from '@/lib/useDocumentTitle';
import { Chip, Notice, Skeleton, StatusDot, Text, TextLink, type StatusDotState } from '@/ui';
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
import {
  contentOf,
  entryName,
  setHidden as withHidden,
  stateOf,
  toRows,
  toSettings,
  type MyMediaPlace,
  type MyMediaRow,
  type MyMediaState,
} from './mymedia/entries';
import { MyMediaPreview } from './mymedia/MyMediaPreview';
import { myMediaQuery, setMyMedia } from './queries';
import styles from './homerows/HomeRows.module.css';
import { intlLocale } from '@/i18n';

/** The tabs in the band: the order (the default, no `?tab=`) and the preview. */
const ORDER = 'order';
const PREVIEW = 'preview';
const TAB_IDS = [ORDER, PREVIEW] as const;

/** The one draft of the page: the whole list, order and hiding together (S-17). */
const DRAFT = 'entries';

/** The dot of each state: visible in both places, in one, in none. */
const DOT: Record<MyMediaState, StatusDotState> = {
  visible: 'success',
  sidebarOnly: 'active',
  homeOnly: 'active',
  hidden: 'pending',
};

/**
 * «Meine Medien» (concept 21 P-11, variant A; O-20, O-21): one order of the group for the side
 * navigation and the tiles of the start page, for every profile, built like the start page rows
 * (HomeRowsPage). Every library, «Radio», «Sammlungen» and «User-Empfehlungen» get a position
 * (RankSelect, S-16), what they hold and a box «Ausblenden» for each place; the tab «Vorschau»
 * shows both places with the draft. The SaveBar stores everything at once or discards it. A new
 * library comes at the end, visible (the server's trigger).
 */
export function MyMediaPage() {
  const { t } = useTranslation();
  useDocumentTitle(t('admin.myMedia.title'));
  const session = useActiveSession();
  const allowed = session.account.role === 'admin' && session.profile.isPrimary;
  if (!allowed) {
    return (
      <AdminPage title={t('admin.myMedia.title')}>
        <Notice title={t('admin.access.notAllowed.title')}>
          {t('admin.access.notAllowed.body')}
        </Notice>
      </AdminPage>
    );
  }
  return <MyMediaContent />;
}

function MyMediaContent() {
  const { t, i18n } = useTranslation();
  const queryClient = useQueryClient();
  const number = useMemo(() => new Intl.NumberFormat(intlLocale(i18n.language)), [i18n.language]);
  const titleRef = useRef<HTMLHeadingElement>(null);
  const hintId = useId();

  // The draft needs what the server holds before the query knows whether to pause for it: a
  // passive observer reads the same cache entry without fetching.
  const cached = useQuery({ ...myMediaQuery, enabled: false }).data;
  const original = useMemo(() => (cached ? toRows(cached) : undefined), [cached]);
  const originalOf = useCallback((id: string) => (id === DRAFT ? original : undefined), [original]);
  const drafts = useDrafts<MyMediaRow[]>(originalOf);
  const unsaved = useUnsavedChanges(drafts.dirty);
  const [confirmOpen, setConfirmOpen] = useState(false);
  // No refetch while the list is changed (S-13); the questions open only then.
  const stored = useQuery({
    ...myMediaQuery,
    refetchOnWindowFocus: !unsaved.pausePolling,
    refetchOnReconnect: !unsaved.pausePolling,
  });

  // An unknown `?tab=` shows «Reihenfolge», and the address says so too (replaced, not pushed).
  const [tab, selectTab] = useSearchSelection('tab', TAB_IDS, ORDER, { normalise: true });

  // Every message is a node of its own, so the same sentence twice is announced twice.
  const [message, say] = useStatusMessage();

  const save = useMutation({
    mutationFn: (rows: MyMediaRow[]) => setMyMedia(toSettings(rows)),
    onSuccess: async (saved: MyMediaSetting[]) => {
      // The server holds the draft now: it is spent and drops out (useDrafts).
      queryClient.setQueryData(myMediaQuery.queryKey, saved);
      drafts.discard(DRAFT);
      say(t('admin.myMedia.saved'));
      // The side navigation and the start page read the order.
      await Promise.all([
        queryClient.invalidateQueries({ queryKey: ['home'] }),
        queryClient.invalidateQueries({ queryKey: ['my-media'] }),
      ]);
      // The controls were locked while saving; the focus stays where it was, or the h1 takes it.
      await afterRender();
      focusTitleIfLost(titleRef);
    },
  });

  const rows = drafts.valueOf(DRAFT) ?? [];
  const name = (row: MyMediaRow) => entryName(t, row);
  const change = (next: MyMediaRow[], text: string | null) => {
    drafts.set(DRAFT, next);
    say(text);
    if (save.isError) save.reset();
  };
  const moveTo = (row: MyMediaRow, position: number) =>
    change(
      rankTo(rows, rows.indexOf(row), position, 'move'),
      t('admin.myMedia.moved', { entry: name(row), position: position + 1 }),
    );
  const setHidden = (row: MyMediaRow, place: MyMediaPlace, hidden: boolean) =>
    change(withHidden(rows, row.id, place, hidden), null);

  const onDiscard = () => {
    drafts.discard(DRAFT);
    if (save.isError) save.reset();
    say(t('admin.myMedia.discarded'));
  };
  const onConfirmOpenChange = useCallback((open: boolean) => {
    setConfirmOpen(open);
    if (open) return;
    // «Abbrechen» gives the focus back to «Verwerfen»; after discarding, that button is gone
    // and the h1 takes the focus (ALL-2).
    void afterDialogsClosed().then(() => focusTitleIfLost(titleRef));
  }, []);

  const hideBox = (row: MyMediaRow, place: MyMediaPlace) => {
    const checked = place === 'sidebar' ? row.hideSidebar : row.hideHome;
    return (
      <label className={styles.hide}>
        <input
          type="checkbox"
          checked={checked}
          disabled={save.isPending}
          aria-label={t(
            place === 'sidebar' ? 'admin.myMedia.hideSidebarLabel' : 'admin.myMedia.hideHomeLabel',
            { entry: name(row) },
          )}
          onChange={(event) => setHidden(row, place, event.target.checked)}
        />
        {t('admin.myMedia.hide')}
      </label>
    );
  };

  // Fixed widths: ticking «Ausblenden» changes the state's word, not the columns (K12-9).
  const columns: DataColumn<MyMediaRow>[] = [
    {
      key: 'position',
      width: '14%',
      header: t('admin.myMedia.columns.position'),
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
      key: 'entry',
      width: '26%',
      header: t('admin.myMedia.columns.entry'),
      rowHeader: true,
      cell: (row) => (
        <span className={styles.name}>
          {name(row)} <Chip>{t(`admin.myMedia.kind.${row.key}`)}</Chip>
        </span>
      ),
    },
    {
      key: 'content',
      width: '14%',
      header: t('admin.myMedia.columns.content'),
      cell: (row) => contentOf(t, number, row),
    },
    {
      key: 'state',
      width: '18%',
      header: t('admin.myMedia.columns.state'),
      cell: (row) => {
        const state = stateOf(row);
        return (
          <span className={styles.state}>
            <StatusDot state={DOT[state]} />
            {t(`admin.myMedia.state.${state}`)}
          </span>
        );
      },
    },
    {
      key: 'sidebar',
      width: '14%',
      header: t('admin.myMedia.columns.sidebar'),
      cell: (row) => hideBox(row, 'sidebar'),
    },
    {
      key: 'home',
      header: t('admin.myMedia.columns.home'),
      cell: (row) => hideBox(row, 'home'),
    },
  ];

  const tabs: TabItem[] = [
    {
      id: ORDER,
      label: t('admin.myMedia.tabs.order'),
      count: number.format(rows.length),
      countLabel: t('admin.myMedia.count', {
        count: rows.length,
        number: number.format(rows.length),
      }),
      content: (
        <div className={styles.tab}>
          <RankHint id={hintId} />
          <DataTable
            caption={t('admin.myMedia.list')}
            columns={columns}
            rows={rows}
            rowKey={(row) => row.id}
            layout="fixed"
          />
        </div>
      ),
    },
    {
      id: PREVIEW,
      label: t('admin.myMedia.tabs.preview'),
      content: <MyMediaPreview rows={rows} />,
    },
  ];

  return (
    <AdminPage title={t('admin.myMedia.title')} help={t('admin.myMedia.intro')} titleRef={titleRef}>
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
            title={t('admin.myMedia.band')}
            icon="grid"
            tabs={{
              label: t('admin.myMedia.tabs.label'),
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
            {t('admin.myMedia.homeRows')}{' '}
            <TextLink href={paths.adminHomeRows}>{t('admin.myMedia.homeRowsLink')}</TextLink>
          </Text>
        </>
      )}
      <ConfirmDialog
        isOpen={confirmOpen}
        onOpenChange={onConfirmOpenChange}
        title={t('admin.myMedia.discard.title')}
        body={t('admin.myMedia.discard.body')}
        confirmLabel={t('admin.myMedia.discard.confirm')}
        cancelLabel={t('common.cancel')}
        tone="danger"
        defaultFocus="cancel"
        onConfirm={onDiscard}
      />
      <ConfirmDialog {...unsaved.dialog} />
    </AdminPage>
  );
}
