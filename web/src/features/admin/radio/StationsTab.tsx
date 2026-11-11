import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { useEffect, useMemo, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { imageUrl } from '@/api/client';
import { problemMessage } from '@/api/errors';
import { countryName, stationPlace } from '@/app/radio/labels';
import { radioKeys } from '@/app/radio/queries';
import { formatBitrate } from '@/lib/format';
import { usePagedList, type PlaceRange } from '@/lib/pagedList';
import { Notice, SelectField, Skeleton, StationLogo, StatusDot, Text } from '@/ui';
import {
  AriaButton,
  DataTable,
  FormDialog,
  StatusRegion,
  Switch,
  TextField,
  useStatusMessage,
  type DataColumn,
} from '@/ui/aria';
import {
  adminRadioStationsSource,
  applyRadioOfferNow,
  markSteps,
  radioAdminKeys,
  radioOfferQuery,
  type AdminRadioStation,
  type AdminRadioStationFilter,
} from '../queries';
import styles from './Radio.module.css';

/** Typing waits this long before the list follows. */
const DEBOUNCE_MS = 300;

const STATES = ['reachable', 'broken', 'noLogo'] as const;
type StateFilter = (typeof STATES)[number];
const SORTS = ['popularity', 'name'] as const;
type Sort = (typeof SORTS)[number];

/** The state of a station as one word and its dot. */
function stationState(station: AdminRadioStation) {
  if (station.state === 'missing') return { key: 'missing', dot: 'warning' } as const;
  if (station.state === 'broken') return { key: 'broken', dot: 'danger' } as const;
  return { key: 'ok', dot: 'success' } as const;
}

interface StationsTabProps {
  /** Opens the editor of a station (`?item=`). */
  onEdit: (stationId: number) => void;
}

/**
 * «Sender» (concept 19 part B, B9.3): the stations of the offer, or of the whole catalog with the
 * switch, searched and narrowed by country, provider (a provider in a row filters by it) and state,
 * by popularity or name. Above the list «Nicht erreichbare entfernen (n)» (B6) with «Rückgängig»
 * while its message stands; per row the streams, «Bearbeiten» and «Aus dem Angebot nehmen» or
 * «Ins Angebot nehmen». Each of these is one step stored at once on the current offer (B4.3).
 */
export function StationsTab({ onEdit }: StationsTabProps) {
  const { t, i18n } = useTranslation();
  const language = i18n.language;
  const queryClient = useQueryClient();
  const [message, setMessage] = useStatusMessage();

  const [text, setText] = useState('');
  const [q, setQ] = useState('');
  const [providerText, setProviderText] = useState('');
  const [provider, setProvider] = useState('');
  useEffect(() => {
    const timer = window.setTimeout(() => {
      setQ(text.trim());
      setProvider(providerText.trim());
    }, DEBOUNCE_MS);
    return () => window.clearTimeout(timer);
  }, [text, providerText]);
  const [country, setCountry] = useState('');
  const [state, setState] = useState<StateFilter | ''>('');
  const [sort, setSort] = useState<Sort>('popularity');
  const [wholeCatalog, setWholeCatalog] = useState(false);

  const filter: AdminRadioStationFilter = useMemo(
    () => ({
      ...(q ? { q } : {}),
      ...(country ? { country } : {}),
      ...(provider ? { provider } : {}),
      ...(state ? { state } : {}),
      ...(sort !== 'popularity' ? { sort } : {}),
      offered: !wholeCatalog,
    }),
    [country, provider, q, sort, state, wholeCatalog],
  );
  // Every station of the filter, read page by page as the table scrolls; the table is always
  // virtual, the catalog has some 55'000 stations (concept 18 C-3).
  const [span, setSpan] = useState<PlaceRange>({ start: 0, end: 0 });
  const paged = usePagedList(adminRadioStationsSource(filter), span);
  const stations = paged.first;
  const total = paged.total;
  // The countries of the catalog and the stations «Nicht erreichbare entfernen» takes out now.
  const offer = useQuery(radioOfferQuery);
  const removable = offer.data?.totals.removableUnreachable ?? 0;

  const refresh = async () => {
    await queryClient.invalidateQueries({ queryKey: radioAdminKeys.all });
    await queryClient.invalidateQueries({ queryKey: radioKeys.all });
  };

  // «Nicht erreichbare entfernen» and its undo, while the message stands.
  const [removed, setRemoved] = useState<number[] | null>(null);
  const removeUnreachable = useMutation({
    mutationFn: () => applyRadioOfferNow([{ op: 'demarkUnreachable' }]),
    onSuccess: async (applied) => {
      setRemoved(applied.demarkedStationIds);
      setMessage(null);
      await refresh();
    },
  });
  const undo = useMutation({
    mutationFn: (ids: number[]) => applyRadioOfferNow(markSteps(ids)),
    onSuccess: async () => {
      setRemoved(null);
      setMessage(t('admin.radio.stations.undone'));
      await refresh();
    },
  });
  // One station in or out of the offer.
  const toggle = useMutation({
    mutationFn: (station: AdminRadioStation) =>
      applyRadioOfferNow([{ op: station.offered ? 'demark' : 'mark', stationIds: [station.id] }]),
    onSuccess: async (_applied, station) => {
      setRemoved(null);
      setMessage(
        t(station.offered ? 'admin.radio.stations.takenOut' : 'admin.radio.stations.takenIn', {
          name: station.name,
        }),
      );
      await refresh();
    },
  });
  const failure = removeUnreachable.error ?? undo.error ?? toggle.error;

  const [streamsOf, setStreamsOf] = useState<AdminRadioStation | null>(null);

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
      key: 'place',
      width: '12%',
      header: t('admin.radio.stations.columns.place'),
      cell: (station) => stationPlace(t, language, station) ?? '–',
    },
    {
      key: 'provider',
      width: '14%',
      header: t('admin.radio.stations.columns.provider'),
      cell: (station) =>
        station.provider ? (
          <AriaButton
            variant="ghost"
            size="sm"
            aria-label={t('admin.radio.stations.providerFilter', { provider: station.provider })}
            onPress={() => {
              setProviderText(station.provider ?? '');
              setProvider(station.provider ?? '');
            }}
          >
            {station.provider}
          </AriaButton>
        ) : (
          '–'
        ),
    },
    {
      key: 'state',
      width: '10rem',
      header: t('admin.radio.stations.columns.state'),
      cell: (station) => {
        const shown = stationState(station);
        return (
          <span className={styles.offerStation}>
            <span className={styles.stateCell}>
              <StatusDot state={shown.dot} />
              {t(`admin.radio.stations.stateText.${shown.key}`)}
            </span>
            <span className={styles.muted}>
              {station.offered
                ? t('admin.radio.stations.offeredYes')
                : t('admin.radio.stations.offeredNo')}
            </span>
          </span>
        );
      },
    },
    {
      key: 'marks',
      width: '9%',
      header: t('admin.radio.stations.columns.marks'),
      cell: (station) =>
        [
          station.locks.length > 0 &&
            t('admin.radio.stations.locked', { count: station.locks.length }),
          station.recommended && t('admin.radio.stations.recommended'),
        ]
          .filter(Boolean)
          .join(' · ') || '–',
    },
    {
      key: 'actions',
      width: '24rem',
      header: t('admin.radio.stations.columns.actions'),
      hideHeader: true,
      align: 'end',
      cell: (station) => (
        <span className={styles.actions}>
          <AriaButton
            variant="ghost"
            size="sm"
            aria-label={t('admin.radio.stations.streamsLabel', { name: station.name })}
            onPress={() => setStreamsOf(station)}
          >
            {t('admin.radio.stations.streams')}
          </AriaButton>
          <AriaButton
            variant="ghost"
            size="sm"
            aria-label={t('admin.radio.stations.editLabel', { name: station.name })}
            onPress={() => onEdit(station.id)}
          >
            {t('admin.radio.stations.edit')}
          </AriaButton>
          <AriaButton
            variant={station.offered ? 'dangerQuiet' : 'ghost'}
            size="sm"
            isDisabled={toggle.isPending}
            aria-label={t(
              station.offered
                ? 'admin.radio.stations.takeOutLabel'
                : 'admin.radio.stations.takeInLabel',
              { name: station.name },
            )}
            onPress={() => toggle.mutate(station)}
          >
            {t(station.offered ? 'admin.radio.stations.takeOut' : 'admin.radio.stations.takeIn')}
          </AriaButton>
        </span>
      ),
    },
  ];

  return (
    <div className={styles.tab}>
      <StatusRegion message={message} />
      <div className={styles.filters}>
        <TextField label={t('admin.radio.stations.search')} value={text} onChange={setText} />
        <SelectField
          label={t('admin.radio.stations.country')}
          value={country}
          options={[
            { value: '', label: t('admin.radio.stations.allCountries') },
            ...(offer.data?.countries ?? [])
              .map((entry) => ({ value: entry.code, label: countryName(language, entry.code) }))
              .sort((a, b) => a.label.localeCompare(b.label, language)),
          ]}
          onChange={setCountry}
        />
        <TextField
          label={t('admin.radio.stations.provider')}
          value={providerText}
          onChange={setProviderText}
        />
        <SelectField<StateFilter | ''>
          label={t('admin.radio.stations.state')}
          value={state}
          options={[
            { value: '', label: t('admin.radio.stations.anyState') },
            ...STATES.map((value) => ({
              value,
              label: t(`admin.radio.stations.states.${value}`),
            })),
          ]}
          onChange={setState}
        />
        <SelectField<Sort>
          label={t('admin.radio.stations.sort')}
          value={sort}
          options={SORTS.map((value) => ({
            value,
            label: t(`admin.radio.stations.sorts.${value}`),
          }))}
          onChange={setSort}
        />
        <Switch isSelected={wholeCatalog} onChange={setWholeCatalog}>
          {t('admin.radio.stations.wholeCatalog')}
        </Switch>
      </div>

      <div className={styles.offerStationsHead}>
        <Text tone="secondary">{t('admin.radio.stations.removeUnreachableHelp')}</Text>
        <AriaButton
          variant="dangerQuiet"
          isDisabled={removable === 0}
          isPending={removeUnreachable.isPending}
          onPress={() => removeUnreachable.mutate()}
        >
          {t('admin.radio.stations.removeUnreachable', {
            count: removable,
            n: new Intl.NumberFormat(language).format(removable),
          })}
        </AriaButton>
      </div>
      {removed && (
        <Notice
          title={t('admin.radio.stations.removedUnreachable', { count: removed.length })}
          actions={
            removed.length > 0 && (
              <AriaButton size="sm" isPending={undo.isPending} onPress={() => undo.mutate(removed)}>
                {t('admin.radio.stations.undo')}
              </AriaButton>
            )
          }
        />
      )}
      {failure && <Notice tone="danger" title={problemMessage(t, failure)} />}

      {stations.isError && <Notice tone="danger" title={problemMessage(t, stations.error)} />}
      {stations.isPending ? (
        <Skeleton shape="block" />
      ) : total === 0 ? (
        <Text tone="secondary">{t('admin.radio.stations.empty')}</Text>
      ) : (
        <>
          <Text tone="secondary">{t('admin.radio.stations.count', { count: total })}</Text>
          <DataTable
            caption={t('admin.radio.stations.caption')}
            columns={columns}
            rows={paged.slots}
            rowKey={(station) => String(station.id)}
            layout="fixed"
            virtual
            onRange={setSpan}
          />
          {paged.failed && (
            <Notice
              tone="danger"
              title={t('library.page.pageFailed')}
              actions={
                <AriaButton icon="refresh" onPress={paged.retry}>
                  {t('common.retry')}
                </AriaButton>
              }
            />
          )}
        </>
      )}

      {streamsOf && (
        <FormDialog
          isOpen
          onOpenChange={(open) => {
            if (!open) setStreamsOf(null);
          }}
          title={t('admin.radio.stations.streamsTitle', { name: streamsOf.name })}
          initialFocus="[data-streams-close]"
        >
          <div className={styles.form}>
            {streamsOf.streams.length === 0 ? (
              <Text tone="secondary">{t('admin.radio.stations.streamsEmpty')}</Text>
            ) : (
              <ul className={styles.listeners}>
                {streamsOf.streams.map((stream) => (
                  <li key={stream.id} className={styles.offerStation}>
                    <span className={styles.url}>{stream.urlResolved ?? stream.url}</span>
                    <span className={styles.muted}>
                      {[
                        stream.codec,
                        stream.bitrateKbps !== undefined &&
                          formatBitrate(stream.bitrateKbps * 1000, language),
                        stream.hls && t('admin.radio.stations.streamHls'),
                        stream.preferred && t('admin.radio.stations.streamPreferred'),
                        !stream.reachable && t('admin.radio.stations.streamUnreachable'),
                      ]
                        .filter(Boolean)
                        .join(' · ')}
                    </span>
                  </li>
                ))}
              </ul>
            )}
            <div className={styles.dialogActions}>
              <AriaButton data-streams-close="" onPress={() => setStreamsOf(null)}>
                {t('admin.radio.stations.streamsClose')}
              </AriaButton>
            </div>
          </div>
        </FormDialog>
      )}
    </div>
  );
}
