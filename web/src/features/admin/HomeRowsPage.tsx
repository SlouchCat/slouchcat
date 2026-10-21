import type { HomeRowKey, HomeRowSetting } from '@slouchcat/api-client';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { useState } from 'react';
import { useTranslation } from 'react-i18next';
import { codeMessage, problemMessage } from '@/api/errors';
import { useActiveSession } from '@/app/session';
import { formatDateTime } from '@/lib/format';
import { useDocumentTitle } from '@/lib/useDocumentTitle';
import { FactList, Notice, Page, Skeleton, Text } from '@/ui';
import { AriaButton, DataTable, type DataColumn } from '@/ui/aria';
import { moveBy, useMoveButtons } from './CurationParts';
import { homeRowsQuery, refreshTrakt, setHomeRows, traktStatusQuery } from './queries';
import styles from './AdminTablePage.module.css';
import curation from './Curation.module.css';

/** The row names of the start page (the same keys as its headings). */
const rowTitleKey = {
  picks: 'home.picks',
  continue: 'home.rows.continue',
  recent: 'home.rows.recent',
  watchlist: 'home.rows.watchlist',
  collections: 'home.rows.collections',
  genres: 'home.rows.genres',
  trakt_trending: 'home.rows.traktTrending',
  trakt_popular: 'home.rows.traktPopular',
} as const satisfies Record<HomeRowKey, string>;

/** After "Jetzt aktualisieren" the status is asked again this often for a while. */
const TRAKT_POLL_MS = 2000;
const TRAKT_POLL_FOR_MS = 30_000;

/**
 * Trakt (E-7, I-7): whether the server has a client id, when the lists were fetched, the last
 * error and the number of entries; "Jetzt aktualisieren" queues the daily job at once.
 */
function TraktSection() {
  const { t, i18n } = useTranslation();
  const queryClient = useQueryClient();
  const [pollUntil, setPollUntil] = useState(0);
  const status = useQuery({
    ...traktStatusQuery,
    refetchInterval: () => (Date.now() < pollUntil ? TRAKT_POLL_MS : false),
  });
  const refresh = useMutation({
    mutationFn: refreshTrakt,
    onSuccess: async () => {
      setPollUntil(Date.now() + TRAKT_POLL_FOR_MS);
      await queryClient.invalidateQueries({ queryKey: traktStatusQuery.queryKey });
    },
  });
  const date = (iso: string | undefined) =>
    iso ? formatDateTime(iso, i18n.language) : t('admin.trakt.never');
  // `trakt_unavailable: HTTP 503`: the code in words when the UI knows it, else the server's text.
  const lastError = (text: string | undefined) => {
    if (!text) return t('admin.trakt.noError');
    const [code = ''] = text.split(':');
    return codeMessage(t, code.trim()) ?? text;
  };

  return (
    <section className={curation.section} aria-labelledby="admin-trakt">
      <div className={styles.stack}>
        <Text as="h2" variant="title">
          <span id="admin-trakt">{t('admin.trakt.title')}</span>
        </Text>
        <Text tone="secondary">{t('admin.trakt.intro')}</Text>
      </div>
      {status.isPending && <Skeleton shape="block" />}
      {status.isError && <Notice tone="danger">{problemMessage(t, status.error)}</Notice>}
      {status.data && (
        <>
          {!status.data.configured && <Notice>{t('admin.trakt.notConfigured')}</Notice>}
          <FactList
            facts={[
              [
                t('admin.trakt.configured'),
                status.data.configured ? t('admin.trakt.yes') : t('admin.trakt.no'),
              ],
              [t('admin.trakt.lastSuccess'), date(status.data.lastSuccess)],
              [t('admin.trakt.lastAttempt'), date(status.data.lastAttempt)],
              [t('admin.trakt.lastError'), lastError(status.data.lastError)],
              [t('admin.trakt.trending'), String(status.data.trending)],
              [t('admin.trakt.popular'), String(status.data.popular)],
            ]}
          />
        </>
      )}
      {refresh.isError && <Notice tone="danger">{problemMessage(t, refresh.error)}</Notice>}
      {refresh.isSuccess && <Notice>{t('admin.trakt.queued')}</Notice>}
      <div>
        <AriaButton icon="refresh" isPending={refresh.isPending} onPress={() => refresh.mutate()}>
          {t('admin.trakt.refresh')}
        </AriaButton>
      </div>
    </section>
  );
}

/**
 * Start page (M7, E-9): the order of its eight rows for every profile, each with "Nach oben",
 * "Nach unten" and "Ausblenden"; "Speichern" stores them at once. Below, the Trakt lists.
 */
export function HomeRowsPage() {
  const { t } = useTranslation();
  useDocumentTitle(t('admin.homeRows.title'));
  const session = useActiveSession();
  const allowed = session.account.role === 'owner' && session.profile.isPrimary;
  const queryClient = useQueryClient();
  const stored = useQuery({ ...homeRowsQuery, enabled: allowed });
  const [edited, setEdited] = useState<HomeRowSetting[] | null>(null);
  const [message, setMessage] = useState<string | null>(null);
  const rows = edited ?? stored.data ?? [];
  const { buttonRef, keepFocus } = useMoveButtons(rows.map((row) => row.key).join());
  const save = useMutation({
    mutationFn: (list: HomeRowSetting[]) =>
      setHomeRows(list.map((row, position) => ({ ...row, position }))),
    onSuccess: async (saved) => {
      queryClient.setQueryData(homeRowsQuery.queryKey, saved);
      setEdited(null);
      setMessage(t('admin.homeRows.saved'));
      await queryClient.invalidateQueries({ queryKey: ['home'] });
    },
  });

  const name = (row: HomeRowSetting) => t(rowTitleKey[row.key]);
  const change = (next: HomeRowSetting[]) => {
    setEdited(next);
    setMessage(null);
    if (save.error) save.reset();
  };
  const move = (row: HomeRowSetting, delta: -1 | 1) => {
    const index = rows.indexOf(row);
    change(moveBy(rows, index, delta));
    setMessage(t('admin.homeRows.moved', { row: name(row), position: index + delta + 1 }));
    keepFocus(row.key, delta);
  };

  const columns: DataColumn<HomeRowSetting>[] = [
    {
      key: 'row',
      header: t('admin.homeRows.columns.row'),
      rowHeader: true,
      cell: (row) => name(row),
    },
    {
      key: 'hidden',
      header: t('admin.homeRows.columns.hidden'),
      cell: (row) => (
        <label className={curation.toggle}>
          <input
            type="checkbox"
            checked={row.hidden}
            aria-label={t('admin.homeRows.hideLabel', { row: name(row) })}
            onChange={(event) =>
              change(
                rows.map((entry) =>
                  entry.key === row.key ? { ...entry, hidden: event.target.checked } : entry,
                ),
              )
            }
          />
          {t('admin.homeRows.hide')}
        </label>
      ),
    },
    {
      key: 'actions',
      header: t('admin.homeRows.columns.actions'),
      hideHeader: true,
      align: 'end',
      cell: (row) => {
        const index = rows.indexOf(row);
        return (
          <span className={styles.rowActions}>
            <AriaButton
              ref={buttonRef(row.key, -1)}
              variant="ghost"
              aria-label={t('admin.homeRows.upLabel', { row: name(row) })}
              isDisabled={index === 0}
              onPress={() => move(row, -1)}
            >
              {t('admin.homeRows.up')}
            </AriaButton>
            <AriaButton
              ref={buttonRef(row.key, 1)}
              variant="ghost"
              aria-label={t('admin.homeRows.downLabel', { row: name(row) })}
              isDisabled={index === rows.length - 1}
              onPress={() => move(row, 1)}
            >
              {t('admin.homeRows.down')}
            </AriaButton>
          </span>
        );
      },
    },
  ];

  return (
    <Page>
      <div className={styles.header}>
        <Text as="h1" variant="display">
          {t('admin.homeRows.title')}
        </Text>
        <Text tone="secondary">{t('admin.homeRows.intro')}</Text>
      </div>
      {!allowed ? (
        <Notice title={t('admin.access.notAllowed.title')}>
          {t('admin.access.notAllowed.body')}
        </Notice>
      ) : (
        <>
          <div className={styles.body} aria-busy={stored.isPending}>
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
              <DataTable
                caption={t('admin.homeRows.list')}
                columns={columns}
                rows={rows}
                rowKey={(row) => row.key}
              />
            )}
            {save.isError && <Notice tone="danger">{problemMessage(t, save.error)}</Notice>}
            {message && !save.isError && <Notice>{message}</Notice>}
            {rows.length > 0 && (
              <div>
                <AriaButton
                  variant="primary"
                  isPending={save.isPending}
                  onPress={() => save.mutate(rows)}
                >
                  {save.isPending ? t('common.saving') : t('common.save')}
                </AriaButton>
              </div>
            )}
          </div>
          <TraktSection />
        </>
      )}
    </Page>
  );
}
