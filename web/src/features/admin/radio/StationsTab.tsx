import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { useEffect, useMemo, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { imageUrl } from '@/api/client';
import { problemMessage } from '@/api/errors';
import { countryName, facetValueName, stationPlace } from '@/app/radio/labels';
import { radioQueryParams } from '@/features/radio/filters';
import { usePagedList, type PlaceRange } from '@/lib/pagedList';
import { radioFacetsQuery } from '@/features/radio/queries';
import { Notice, SelectField, Skeleton, StationLogo, StatusDot, Text } from '@/ui';
import {
  AriaButton,
  DataTable,
  StatusRegion,
  Switch,
  TextField,
  useStatusMessage,
  type DataColumn,
} from '@/ui/aria';
import {
  adminRadioStationsSource,
  radioAdminKeys,
  updateAdminRadioStation,
  type AdminRadioStation,
  type AdminRadioStationFilter,
} from '../queries';
import styles from './Radio.module.css';

/** Typing waits this long before the list follows. */
const DEBOUNCE_MS = 300;

const STATES = ['reachable', 'broken', 'hidden'] as const;
type StateFilter = (typeof STATES)[number];

/** The state of a station as one word and its dot. */
function stationState(station: AdminRadioStation) {
  if (station.hidden) return { key: 'hidden', dot: 'pending' } as const;
  if (station.state === 'missing') return { key: 'missing', dot: 'warning' } as const;
  if (station.state === 'broken') return { key: 'broken', dot: 'danger' } as const;
  return { key: 'ok', dot: 'success' } as const;
}

interface StationsTabProps {
  /** Opens the editor of a station (`?item=`). */
  onEdit: (stationId: number) => void;
}

/**
 * «Sender» (concept 16, 5.5 and 11): opens with «Nur Ausnahmen» (changed by hand, hidden,
 * recommended, broken); a search always covers every station. Two row actions: «Bearbeiten» and
 * «Ausblenden»/«Einblenden».
 */
export function StationsTab({ onEdit }: StationsTabProps) {
  const { t, i18n } = useTranslation();
  const language = i18n.language;
  const queryClient = useQueryClient();
  const [message, setMessage] = useStatusMessage();

  const [text, setText] = useState('');
  const [q, setQ] = useState('');
  useEffect(() => {
    const timer = window.setTimeout(() => setQ(text.trim()), DEBOUNCE_MS);
    return () => window.clearTimeout(timer);
  }, [text]);
  const [country, setCountry] = useState('');
  const [genre, setGenre] = useState('');
  const [state, setState] = useState<StateFilter | ''>('');
  const [onlyExceptions, setOnlyExceptions] = useState(true);

  const filter: AdminRadioStationFilter = useMemo(
    () => ({
      ...(q ? { q } : {}),
      ...(country ? { country } : {}),
      ...(genre ? { genre } : {}),
      ...(state ? { state } : {}),
      // The search goes over every station (5.5).
      exceptions: onlyExceptions && !q,
    }),
    [country, genre, onlyExceptions, q, state],
  );
  // Every station of the filter, read page by page as the table scrolls; the table is always
  // virtual, the catalog has some 40'000 stations (concept 18 C-3).
  const [span, setSpan] = useState<PlaceRange>({ start: 0, end: 0 });
  const paged = usePagedList(adminRadioStationsSource(filter), span);
  const stations = paged.first;
  // The values to filter by: the catalog's countries and genres as the app counts them.
  const facets = useQuery(radioFacetsQuery(radioQueryParams({})));

  const hide = useMutation({
    mutationFn: (station: AdminRadioStation) =>
      updateAdminRadioStation(station.id, { hidden: !station.hidden }),
    onSuccess: async (next) => {
      setMessage(
        t(next.hidden ? 'admin.radio.stations.hiddenDone' : 'admin.radio.stations.shownDone', {
          name: next.name,
        }),
      );
      await queryClient.invalidateQueries({ queryKey: radioAdminKeys.all });
    },
  });

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
      width: '16%',
      header: t('admin.radio.stations.columns.place'),
      cell: (station) => stationPlace(t, language, station) ?? '–',
    },
    {
      key: 'genres',
      width: '18%',
      header: t('admin.radio.stations.columns.genres'),
      cell: (station) =>
        station.genres.map((key) => facetValueName(t, 'genre', key)).join(', ') || '–',
    },
    {
      key: 'state',
      width: '10rem',
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
      width: '12%',
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
      width: '13rem',
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
            variant="ghost"
            size="sm"
            aria-label={t(
              station.hidden ? 'admin.radio.stations.showLabel' : 'admin.radio.stations.hideLabel',
              { name: station.name },
            )}
            isPending={hide.isPending && hide.variables.id === station.id}
            onPress={() => hide.mutate(station)}
          >
            {station.hidden ? t('admin.radio.stations.show') : t('admin.radio.stations.hide')}
          </AriaButton>
        </span>
      ),
    },
  ];

  return (
    <div className={styles.tab}>
      <StatusRegion message={message} />
      {hide.error && <Notice tone="danger">{problemMessage(t, hide.error)}</Notice>}
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
        <SelectField
          label={t('admin.radio.stations.genre')}
          value={genre}
          options={[
            { value: '', label: t('admin.radio.stations.allGenres') },
            ...(facets.data?.genre ?? []).map((entry) => ({
              value: entry.value,
              label: facetValueName(t, 'genre', entry.value),
            })),
          ]}
          onChange={setGenre}
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
        <Switch isSelected={onlyExceptions} onChange={setOnlyExceptions}>
          {t('admin.radio.stations.exceptions')}
        </Switch>
      </div>
      <Text tone="secondary">{t('admin.radio.stations.exceptionsHelp')}</Text>

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
