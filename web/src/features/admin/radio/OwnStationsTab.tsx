import { useInfiniteQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import { useState, type RefObject } from 'react';
import { useTranslation } from 'react-i18next';
import { imageUrl } from '@/api/client';
import { problemMessage } from '@/api/errors';
import { facetValueName, stationPlace } from '@/app/radio/labels';
import { Notice, Skeleton, StationLogo, Text } from '@/ui';
import {
  AriaButton,
  ConfirmDialog,
  DataTable,
  SectionHeader,
  StatusRegion,
  afterDialogsClosed,
  focusTitleIfLost,
  useStatusMessage,
  type DataColumn,
} from '@/ui/aria';
import {
  adminRadioStationsQuery,
  deleteRadioStation,
  radioAdminKeys,
  type AdminRadioStation,
} from '../queries';
import { CreateStationDialog } from './CreateStationDialog';
import styles from './Radio.module.css';

/** Every own station: they are exceptions by nature (contract 1.19.0, `source=own`). */
const OWN = { source: 'own', exceptions: false } as const;

interface OwnStationsTabProps {
  /** Opens the editor of a station (`?item=`). */
  onEdit: (stationId: number) => void;
  titleRef: RefObject<HTMLHeadingElement | null>;
}

/**
 * «Eigene Sender» (concept 16, 5.6 and 11): the stations the administration added because the
 * catalog lacks them. «Eigenen Sender hinzufügen» opens the form; after it the editor opens for
 * the logo. Two row actions: «Bearbeiten» and «Löschen» (after a confirmation; a catalog station
 * can only be hidden).
 */
export function OwnStationsTab({ onEdit, titleRef }: OwnStationsTabProps) {
  const { t, i18n } = useTranslation();
  const language = i18n.language;
  const queryClient = useQueryClient();
  const [message, setMessage] = useStatusMessage();
  const stations = useInfiniteQuery(adminRadioStationsQuery(OWN));
  const [creating, setCreating] = useState(false);
  const [deleting, setDeleting] = useState<AdminRadioStation | null>(null);
  const [confirmOpen, setConfirmOpen] = useState(false);

  const remove = useMutation({
    mutationFn: (station: AdminRadioStation) => deleteRadioStation(station.id),
    onSuccess: async (_data, station) => {
      setMessage(t('admin.radio.own.deleted', { name: station.name }));
      // The station leaves the app's lists and every profile's favourites as well.
      await queryClient.invalidateQueries({
        predicate: (query) =>
          query.queryKey[0] === 'radio' ||
          query.queryKey[0] === 'home' ||
          (query.queryKey[0] === radioAdminKeys.all[0] &&
            query.queryKey[1] === radioAdminKeys.all[1]),
      });
      await afterDialogsClosed();
      focusTitleIfLost(titleRef);
    },
  });

  const rows = stations.data?.pages.flatMap((page) => page.items) ?? [];
  const add = (
    <AriaButton variant="primary" icon="plus" onPress={() => setCreating(true)}>
      {t('admin.radio.own.add')}
    </AriaButton>
  );

  const columns: DataColumn<AdminRadioStation>[] = [
    {
      key: 'station',
      header: t('admin.radio.stations.columns.station'),
      rowHeader: true,
      cell: (station) => (
        <span className={styles.stationCell}>
          <StationLogo name={station.name} src={imageUrl(station.logo)} size="sm" />
          <span>{station.name}</span>
        </span>
      ),
    },
    {
      key: 'stream',
      header: t('admin.radio.own.columns.stream'),
      cell: (station) => <span className={styles.url}>{station.streams[0]?.url ?? '–'}</span>,
    },
    {
      key: 'place',
      header: t('admin.radio.stations.columns.place'),
      cell: (station) => stationPlace(t, language, station) ?? '–',
    },
    {
      key: 'genres',
      header: t('admin.radio.stations.columns.genres'),
      cell: (station) =>
        station.genres.map((key) => facetValueName(t, 'genre', key)).join(', ') || '–',
    },
    {
      key: 'actions',
      header: t('admin.radio.stations.columns.actions'),
      hideHeader: true,
      align: 'end',
      cell: (station) => (
        <span className={styles.actions}>
          <AriaButton
            variant="ghost"
            size="sm"
            aria-label={t('admin.radio.stations.editLabel', { name: station.name })}
            onPress={() => onEdit(station.id)}
          >
            {t('admin.radio.stations.edit')}
          </AriaButton>
          <AriaButton
            variant="dangerQuiet"
            size="sm"
            aria-label={t('admin.radio.own.deleteLabel', { name: station.name })}
            onPress={() => {
              setDeleting(station);
              setConfirmOpen(true);
            }}
          >
            {t('admin.radio.own.delete')}
          </AriaButton>
        </span>
      ),
    },
  ];

  return (
    <div className={styles.tab}>
      <StatusRegion message={message} />
      {remove.error && <Notice tone="danger">{problemMessage(t, remove.error)}</Notice>}
      <section className={styles.section}>
        <SectionHeader
          title={t('admin.radio.own.title')}
          description={t('admin.radio.own.help')}
          actions={rows.length > 0 ? add : undefined}
        />
        {stations.isError && <Notice tone="danger" title={problemMessage(t, stations.error)} />}
        {stations.isPending ? (
          <Skeleton shape="block" />
        ) : rows.length === 0 ? (
          <div className={styles.actions}>
            <Text tone="secondary">{t('admin.radio.own.empty')}</Text>
            {add}
          </div>
        ) : (
          <>
            <DataTable
              caption={t('admin.radio.own.title')}
              columns={columns}
              rows={rows}
              rowKey={(station) => String(station.id)}
            />
            {stations.hasNextPage && (
              <AriaButton
                isPending={stations.isFetchingNextPage}
                onPress={() => void stations.fetchNextPage()}
              >
                {t('admin.radio.stations.more')}
              </AriaButton>
            )}
          </>
        )}
      </section>
      {creating && (
        <CreateStationDialog
          onClose={() => setCreating(false)}
          onCreated={(station) => {
            setCreating(false);
            setMessage(t('admin.radio.own.created', { name: station.name }));
            void queryClient.invalidateQueries({ queryKey: radioAdminKeys.all });
            // The logo comes through the editor's upload (or the homepage's icon).
            onEdit(station.id);
          }}
        />
      )}
      {deleting && (
        <ConfirmDialog
          isOpen={confirmOpen}
          onOpenChange={(open) => {
            setConfirmOpen(open);
            if (!open) void afterDialogsClosed().then(() => focusTitleIfLost(titleRef));
          }}
          title={t('admin.radio.own.deleteTitle', { name: deleting.name })}
          body={t('admin.radio.own.deleteBody')}
          confirmLabel={t('admin.radio.own.delete')}
          cancelLabel={t('common.cancel')}
          tone="danger"
          onConfirm={() => remove.mutate(deleting)}
        />
      )}
    </div>
  );
}
