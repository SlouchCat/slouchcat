import { useQuery } from '@tanstack/react-query';
import { useEffect, useMemo, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { imageUrl } from '@/api/client';
import { problemMessage } from '@/api/errors';
import { countryName, facetValueName, stationPlace } from '@/app/radio/labels';
import { radioQueryParams } from '@/features/radio/filters';
import { usePagedList, type PlaceRange } from '@/lib/pagedList';
import { radioFacetsQuery } from '@/features/radio/queries';
import { Notice, SelectField, Skeleton, StationLogo, StatusDot, Text } from '@/ui';
import { AriaButton, DataTable, TextField, type DataColumn } from '@/ui/aria';
import {
  adminRadioStationsSource,
  type AdminRadioStation,
  type AdminRadioStationFilter,
} from '../queries';
import styles from './Radio.module.css';

/** Typing waits this long before the list follows. */
const DEBOUNCE_MS = 300;

const STATES = ['reachable', 'broken'] as const;
type StateFilter = (typeof STATES)[number];

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
 * «Sender» (concept 16, 5.5 and 11): every station of the catalog, in the offer or not (contract
 * 1.28.0), searched and narrowed by country and state; the row action «Bearbeiten». A station
 * leaves the offer through the offer, no longer by hiding it (concept 19 part B, B4).
 */
export function StationsTab({ onEdit }: StationsTabProps) {
  const { t, i18n } = useTranslation();
  const language = i18n.language;

  const [text, setText] = useState('');
  const [q, setQ] = useState('');
  useEffect(() => {
    const timer = window.setTimeout(() => setQ(text.trim()), DEBOUNCE_MS);
    return () => window.clearTimeout(timer);
  }, [text]);
  const [country, setCountry] = useState('');
  const [state, setState] = useState<StateFilter | ''>('');

  const filter: AdminRadioStationFilter = useMemo(
    () => ({
      ...(q ? { q } : {}),
      ...(country ? { country } : {}),
      ...(state ? { state } : {}),
      offered: false,
    }),
    [country, q, state],
  );
  // Every station of the filter, read page by page as the table scrolls; the table is always
  // virtual, the catalog has some 40'000 stations (concept 18 C-3).
  const [span, setSpan] = useState<PlaceRange>({ start: 0, end: 0 });
  const paged = usePagedList(adminRadioStationsSource(filter), span);
  const stations = paged.first;
  // The values to filter by: the countries as the app counts them.
  const facets = useQuery(radioFacetsQuery(radioQueryParams({})));

  const total = paged.total;

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
      width: '14%',
      header: t('admin.radio.stations.columns.place'),
      cell: (station) => stationPlace(t, language, station) ?? '–',
    },
    {
      key: 'genres',
      width: '16%',
      header: t('admin.radio.stations.columns.genres'),
      cell: (station) =>
        station.genres.map((key) => facetValueName(t, 'genre', key)).join(', ') || '–',
    },
    {
      key: 'state',
      width: '9rem',
      header: t('admin.radio.stations.columns.state'),
      cell: (station) => {
        const shown = stationState(station);
        return (
          <span className={styles.stateCell}>
            <StatusDot state={shown.dot} />
            {t(`admin.radio.stations.stateText.${shown.key}`)}
          </span>
        );
      },
    },
    {
      key: 'marks',
      width: '10%',
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
      width: '15rem',
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
        </span>
      ),
    },
  ];

  return (
    <div className={styles.tab}>
      <div className={styles.filters}>
        <TextField label={t('admin.radio.stations.search')} value={text} onChange={setText} />
        <SelectField
          label={t('admin.radio.stations.country')}
          value={country}
          options={[
            { value: '', label: t('admin.radio.stations.allCountries') },
            ...(facets.data?.country ?? [])
              .map((entry) => ({ value: entry.value, label: countryName(language, entry.value) }))
              .sort((a, b) => a.label.localeCompare(b.label, language)),
          ]}
          onChange={setCountry}
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
      </div>

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
    </div>
  );
}
