import type { HomeRowKey, HomeRowSetting } from '@slouchcat/api-client';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { useState } from 'react';
import { useTranslation } from 'react-i18next';
import { problemMessage } from '@/api/errors';
import { paths } from '@/app/paths';
import { useActiveSession } from '@/app/session';
import { useDocumentTitle } from '@/lib/useDocumentTitle';
import { Notice, Page, Skeleton, Text, TextLink } from '@/ui';
import { AriaButton, DataTable, type DataColumn } from '@/ui/aria';
import { moveBy, useMoveButtons } from './CurationParts';
import { homeRowsQuery, setHomeRows } from './queries';
import styles from './AdminTablePage.module.css';
import curation from './Curation.module.css';

/** The row names of the start page (the same keys as its headings). */
const rowTitleKey = {
  spotlight: 'home.rows.spotlight',
  picks: 'home.picks',
  continue: 'home.rows.continue',
  recent: 'home.rows.recent',
  released: 'home.rows.released',
  watchlist: 'home.rows.watchlist',
  collections: 'home.rows.collections',
  genres: 'home.rows.genres',
} as const satisfies Record<HomeRowKey, string>;

/**
 * Start page (M7, E-9): the order of its eight rows for every profile, each with "Nach oben",
 * "Nach unten" and "Ausblenden"; "Speichern" stores them at once. Order and hiding live only here;
 * the Trakt list behind Spotlight and its state are on the spotlight page, one line links there.
 */
export function HomeRowsPage() {
  const { t } = useTranslation();
  useDocumentTitle(t('admin.homeRows.title'));
  const session = useActiveSession();
  const allowed = session.account.role === 'admin' && session.profile.isPrimary;
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
            <div role="status" className={curation.live}>
              {message && !save.isError && <Notice live={false}>{message}</Notice>}
            </div>
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
          <Text tone="secondary">
            {t('admin.homeRows.spotlight')}{' '}
            <TextLink href={paths.adminSpotlight}>{t('admin.homeRows.spotlightLink')}</TextLink>
          </Text>
        </>
      )}
    </Page>
  );
}
