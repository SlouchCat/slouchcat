import {
  keepPreviousData,
  queryOptions,
  useMutation,
  useQuery,
  useQueryClient,
} from '@tanstack/react-query';
import { useEffect, useId, useMemo, useRef, useState, type RefObject } from 'react';
import { useTranslation } from 'react-i18next';
import { imageUrl, isApiError } from '@/api/client';
import { problemMessage } from '@/api/errors';
import { countryName } from '@/app/radio/labels';
import { radioKeys } from '@/app/radio/queries';
import { usePagedList, type PlaceRange } from '@/lib/pagedList';
import { FactList, Notice, Skeleton, StationLogo, Text } from '@/ui';
import {
  AriaButton,
  BandPanel,
  DataTable,
  SelectCheck,
  Switch,
  TextField,
  afterRender,
  focusTitleIfLost,
  type DataColumn,
} from '@/ui/aria';
import {
  RADIO_CONTINENTS,
  applyRadioOffer,
  previewRadioOffer,
  radioAdminKeys,
  radioOfferQuery,
  type RadioContinent,
  type RadioOfferCountry,
  type RadioOfferStation,
  type RadioOfferStep,
} from '../queries';
import styles from './Radio.module.css';

/** Changes wait this long before the preview follows; typing in a search as well. */
const DEBOUNCE_MS = 300;

/** The pages of the station list of a country (the preview's `list`). */
const LIST_KEY = [...radioAdminKeys.all, 'offer', 'list'] as const;

const previewQuery = (steps: readonly RadioOfferStep[]) =>
  queryOptions({
    queryKey: [...radioAdminKeys.all, 'offer', 'preview', steps],
    queryFn: ({ signal }) => previewRadioOffer({ steps: [...steps] }, signal),
  });

/**
 * What a control shows until the preview of its step is back: the admin's own input (a tick, a
 * chosen country, the switch), never a result of the rule. `gen` is the change it belongs to.
 */
interface Override {
  value: boolean;
  gen: number;
}

const isStationStep = (step: RadioOfferStep | undefined, id: number) =>
  (step?.op === 'mark' || step?.op === 'demark') &&
  step.stationIds?.length === 1 &&
  step.stationIds[0] === id;

/**
 * The steps with one more. A tick undone at once takes its step back (the station is as it was),
 * and the switch keeps one step: the list stays as short as the admin's intent.
 */
function withStep(steps: readonly RadioOfferStep[], step: RadioOfferStep): RadioOfferStep[] {
  const last = steps.at(-1);
  const id = step.stationIds?.[0];
  if (
    id !== undefined &&
    isStationStep(step, id) &&
    isStationStep(last, id) &&
    last?.op !== step.op
  ) {
    return steps.slice(0, -1);
  }
  if (step.op === 'setHideBroken' && last?.op === 'setHideBroken') {
    return [...steps.slice(0, -1), step];
  }
  return [...steps, step];
}

/**
 * A ref for a panel that closes on Escape from anywhere inside it (B9.4): a native listener, the
 * panel is no widget of its own. The latest `close` runs.
 */
function useEscape(close: () => void) {
  const latest = useRef(close);
  useEffect(() => {
    latest.current = close;
  });
  const [node, setNode] = useState<HTMLElement | null>(null);
  useEffect(() => {
    if (!node) return;
    const onKeyDown = (event: globalThis.KeyboardEvent) => {
      if (event.key !== 'Escape' || event.defaultPrevented) return;
      event.preventDefault();
      latest.current();
    };
    node.addEventListener('keydown', onKeyDown);
    return () => node.removeEventListener('keydown', onKeyDown);
  }, [node]);
  return setNode;
}

/** Lower case without accents, for the country search. */
const fold = (text: string) =>
  text
    .normalize('NFD')
    .replace(/\p{Diacritic}/gu, '')
    .toLocaleLowerCase();

interface OfferTabProps {
  /** The draft: the steps on the stored offer, kept by the page across its tabs (S-17). */
  steps: RadioOfferStep[];
  onStepsChange: (steps: RadioOfferStep[]) => void;
  /** The page's h1: it takes the focus when «Verwerfen» went with the changes (ALL-2). */
  titleRef: RefObject<HTMLHeadingElement | null>;
}

/**
 * «Angebot» (concept 19 part B, B9.2, mockup AngebotB): the countries by search and continent,
 * the chosen countries with the station list of one of them, the summary and the bar with
 * «Übernehmen» and «Verwerfen». A change is a step of the draft; the server computes the preview
 * of the steps (totals, difference, «n von m», the ticks of the list) and stores them on
 * «Übernehmen» with the version they are based on (B4.3). The client applies no rule (rule 12).
 */
export function OfferTab({ steps, onStepsChange, titleRef }: OfferTabProps) {
  const { t, i18n } = useTranslation();
  const language = i18n.language;
  const number = useMemo(() => new Intl.NumberFormat(language), [language]);
  const queryClient = useQueryClient();
  const id = useId();

  // Every change counts; the preview of a change replaces what the controls showed meanwhile.
  const [generation, setGeneration] = useState(0);
  const [overrides, setOverrides] = useState<ReadonlyMap<string, Override>>(() => new Map());
  const change = (next: RadioOfferStep[], shown: [string, boolean][] = [], drop?: string) => {
    const gen = generation + 1;
    setGeneration(gen);
    setOverrides((previous) => {
      const map = new Map(previous);
      if (drop) for (const key of map.keys()) if (key.startsWith(drop)) map.delete(key);
      for (const [key, value] of shown) map.set(key, { value, gen });
      return map;
    });
    onStepsChange(next);
  };
  const add = (step: RadioOfferStep, shown: [string, boolean][] = [], drop?: string) =>
    change(withStep(steps, step), shown, drop);

  // The steps the previews are computed for: the draft once it rested.
  const [settled, setSettled] = useState({ steps, gen: 0 });
  useEffect(() => {
    const gen = generation;
    // An empty draft (discarded, applied) at once.
    const timer = window.setTimeout(
      () => setSettled({ steps, gen }),
      steps.length === 0 ? 0 : DEBOUNCE_MS,
    );
    return () => window.clearTimeout(timer);
  }, [generation, steps]);
  // The station list reads its pages with the settled steps; a new draft reads them again in
  // place (an invalidation keeps the rows, so the focus stays on its tick).
  const settledSteps = useRef(settled.steps);
  useEffect(() => {
    settledSteps.current = settled.steps;
    let live = true;
    const work: Promise<unknown>[] = [queryClient.invalidateQueries({ queryKey: LIST_KEY })];
    if (settled.steps.length > 0) work.push(queryClient.fetchQuery(previewQuery(settled.steps)));
    void Promise.allSettled(work).then(() => {
      if (!live) return;
      setOverrides((previous) => {
        const map = new Map([...previous].filter(([, entry]) => entry.gen > settled.gen));
        return map.size === previous.size ? previous : map;
      });
    });
    return () => {
      live = false;
    };
  }, [queryClient, settled]);

  const offer = useQuery(radioOfferQuery);
  const preview = useQuery({
    ...previewQuery(settled.steps),
    enabled: settled.steps.length > 0,
    placeholderData: keepPreviousData,
  });
  const dirty = steps.length > 0;
  const view = dirty && preview.data ? preview.data : offer.data;
  const version = (dirty ? preview.data?.version : undefined) ?? offer.data?.version;

  const [note, setNote] = useState<'applied' | 'conflict' | null>(null);
  const apply = useMutation({
    mutationFn: () => applyRadioOffer(version ?? 0, steps),
    onSuccess: async (applied) => {
      queryClient.setQueryData(radioOfferQuery.queryKey, applied.offer);
      setNote('applied');
      change([]);
      await queryClient.invalidateQueries({ queryKey: radioAdminKeys.all });
      // The radio page shows the new offer at once.
      await queryClient.invalidateQueries({ queryKey: radioKeys.all });
    },
    onError: async (error) => {
      if (!isApiError(error, 409)) return;
      // Another tab stored meanwhile: the same steps on the new offer, for a second «Übernehmen».
      setNote('conflict');
      await queryClient.invalidateQueries({ queryKey: radioAdminKeys.all });
    },
  });

  const countries = useMemo(
    () =>
      (view?.countries ?? [])
        .map((country) => ({ ...country, name: countryName(language, country.code) }))
        .sort((a, b) => a.name.localeCompare(b.name, language)),
    [language, view?.countries],
  );
  type Country = (typeof countries)[number];
  const isChosen = (country: RadioOfferCountry) =>
    overrides.get(`c:${country.code}`)?.value ?? country.mode !== undefined;
  const choose = (country: Country, on: boolean) =>
    add(
      { op: on ? 'chooseCountry' : 'removeCountry', country: country.code },
      [[`c:${country.code}`, on]],
      on ? undefined : 's:',
    );

  // Search: the name in the app's language or the code.
  const [search, setSearch] = useState('');
  const term = fold(search.trim());
  const hits = term
    ? countries.filter(
        (country) => fold(country.name).includes(term) || country.code.toLowerCase() === term,
      )
    : [];

  // One continent open at a time.
  const [continent, setContinent] = useState<RadioContinent | null>(null);
  const continentButton = (key: RadioContinent) => `${id}-continent-${key}`;
  const inContinent = continent
    ? countries.filter((country) => country.continent === continent)
    : [];
  const chooseMany = (list: readonly Country[], on: boolean) => {
    const todo = list.filter((country) => isChosen(country) !== on);
    if (todo.length === 0) return;
    let next = steps;
    for (const country of todo) {
      next = withStep(next, { op: on ? 'chooseCountry' : 'removeCountry', country: country.code });
    }
    change(
      next,
      todo.map((country): [string, boolean] => [`c:${country.code}`, on]),
      on ? undefined : 's:',
    );
  };
  // Escape in the open continent closes it, with the focus back on its button. A native
  // listener: the group is no widget of its own, its checkboxes are.
  const continentPanel = useEscape(() => {
    if (!continent) return;
    const key = continent;
    setContinent(null);
    void afterRender().then(() => document.getElementById(continentButton(key))?.focus());
  });

  // The station list of one chosen country.
  const [open, setOpen] = useState<string | null>(null);
  const openButton = (code: string) => `${id}-open-${code}`;
  const chosen = countries.filter(isChosen);
  const opened = open ? countries.find((country) => country.code === open) : undefined;
  const openedChosen = opened !== undefined && isChosen(opened);
  const closeStations = () => {
    if (!open) return;
    const code = open;
    setOpen(null);
    void afterRender().then(() => document.getElementById(openButton(code))?.focus());
  };

  const countryColumns = (caption: string): DataColumn<Country>[] => [
    {
      key: 'choose',
      width: '3.5rem',
      header: t('admin.radio.offer.columns.choose'),
      hideHeader: true,
      cell: (country) => (
        <SelectCheck
          label={t('admin.radio.offer.chooseCountry', { country: country.name })}
          isSelected={isChosen(country)}
          onChange={(value) => choose(country, value)}
        />
      ),
    },
    {
      key: 'country',
      header: caption,
      rowHeader: true,
      cell: (country) => country.name,
    },
    {
      key: 'stations',
      width: '7rem',
      align: 'end',
      header: t('admin.radio.offer.columns.stations'),
      cell: (country) => number.format(country.total),
    },
  ];

  const chosenColumns: DataColumn<Country>[] = [
    {
      key: 'country',
      header: t('admin.radio.offer.columns.country'),
      rowHeader: true,
      cell: (country) => country.name,
    },
    {
      key: 'count',
      header: t('admin.radio.offer.columns.stations'),
      cell: (country) =>
        t('admin.radio.offer.chosen.count', {
          offered: number.format(country.offered),
          total: number.format(country.total),
        }),
    },
    {
      key: 'actions',
      width: '18rem',
      header: t('admin.radio.offer.columns.actions'),
      hideHeader: true,
      align: 'end',
      cell: (country) => {
        const isOpen = open === country.code;
        return (
          <span className={styles.actions}>
            <AriaButton
              id={openButton(country.code)}
              variant="ghost"
              size="sm"
              aria-expanded={isOpen}
              aria-controls={isOpen ? `${id}-stations` : undefined}
              aria-label={t(
                isOpen
                  ? 'admin.radio.offer.chosen.closeLabel'
                  : 'admin.radio.offer.chosen.openLabel',
                { country: country.name },
              )}
              onPress={() => setOpen(isOpen ? null : country.code)}
            >
              {t(isOpen ? 'admin.radio.offer.chosen.close' : 'admin.radio.offer.chosen.open')}
            </AriaButton>
            <AriaButton
              variant="dangerQuiet"
              size="sm"
              aria-label={t('admin.radio.offer.chosen.removeLabel', { country: country.name })}
              onPress={() => {
                if (isOpen) setOpen(null);
                choose(country, false);
              }}
            >
              {t('admin.radio.offer.chosen.remove')}
            </AriaButton>
          </span>
        );
      },
    },
  ];

  if (offer.isPending) return <Skeleton shape="block" />;
  if (offer.isError) return <Notice tone="danger" title={problemMessage(t, offer.error)} />;

  const totals = view?.totals;
  const hideBroken = overrides.get('h')?.value ?? view?.hideBroken ?? true;
  const diff = dirty ? preview.data?.diff : undefined;
  const diffText = !diff
    ? t('admin.radio.offer.bar.same')
    : diff.added > 0 && diff.removed > 0
      ? t('admin.radio.offer.bar.both', {
          added: number.format(diff.added),
          removed: number.format(diff.removed),
        })
      : diff.added > 0
        ? t('admin.radio.offer.bar.added', { count: diff.added, n: number.format(diff.added) })
        : diff.removed > 0
          ? t('admin.radio.offer.bar.removed', {
              count: diff.removed,
              n: number.format(diff.removed),
            })
          : t('admin.radio.offer.bar.same');
  const barText = dirty
    ? note === 'conflict'
      ? `${t('admin.radio.offer.bar.conflict')} ${diffText}`
      : diffText
    : note === 'applied'
      ? t('admin.radio.offer.bar.applied')
      : t('admin.radio.offer.bar.clean');

  return (
    <div className={styles.tab}>
      <div className={styles.offer}>
        <div className={styles.offerCountries}>
          <BandPanel
            title={t('admin.radio.offer.countries.title')}
            icon="list"
            meta={t('admin.radio.offer.countries.note', {
              chosen: number.format(chosen.length),
              count: countries.length,
              total: number.format(countries.length),
            })}
          >
            <div className={styles.section}>
              <TextField
                label={t('admin.radio.offer.search')}
                description={t('admin.radio.offer.searchHelp')}
                value={search}
                onChange={setSearch}
              />
              {term && (
                <>
                  <Text tone="secondary">
                    {hits.length === 0
                      ? t('admin.radio.offer.noHits')
                      : t('admin.radio.offer.hits', { count: hits.length })}
                  </Text>
                  {hits.length > 0 && (
                    <div className={styles.scrollList}>
                      <DataTable
                        caption={t('admin.radio.offer.hitsCaption')}
                        columns={countryColumns(t('admin.radio.offer.columns.country'))}
                        rows={hits}
                        rowKey={(country) => country.code}
                      />
                    </div>
                  )}
                </>
              )}
              <Text tone="secondary">{t('admin.radio.offer.continents.label')}</Text>
              <div className={styles.continents}>
                {RADIO_CONTINENTS.map((key) => {
                  const list = countries.filter((country) => country.continent === key);
                  const stations = list.reduce((sum, country) => sum + country.total, 0);
                  const picked = list.filter(isChosen).length;
                  const isOpen = continent === key;
                  return (
                    <AriaButton
                      key={key}
                      id={continentButton(key)}
                      variant={isOpen ? 'primary' : 'secondary'}
                      block
                      aria-expanded={isOpen}
                      aria-controls={isOpen ? `${id}-countries` : undefined}
                      onPress={() => setContinent(isOpen ? null : key)}
                    >
                      <span className={styles.continent}>
                        <span className={styles.relayName}>
                          {t(`admin.radio.offer.continent.${key}`)}
                        </span>
                        <span className={styles.continentSub}>
                          {[
                            t('admin.radio.offer.continentCountries', { count: list.length }),
                            t('admin.radio.offer.continentStations', {
                              count: stations,
                              n: number.format(stations),
                            }),
                            picked > 0 && t('admin.radio.offer.continentChosen', { count: picked }),
                          ]
                            .filter(Boolean)
                            .join(' · ')}
                        </span>
                      </span>
                    </AriaButton>
                  );
                })}
              </div>
              {continent && (
                <div
                  ref={continentPanel}
                  id={`${id}-countries`}
                  className={styles.section}
                  role="group"
                  aria-label={t('admin.radio.offer.countryCaption', {
                    continent: t(`admin.radio.offer.continent.${continent}`),
                  })}
                >
                  <span className={styles.actions}>
                    <AriaButton size="sm" onPress={() => chooseMany(inContinent, true)}>
                      {t('admin.radio.offer.chooseAll')}
                    </AriaButton>
                    <AriaButton
                      size="sm"
                      variant="ghost"
                      onPress={() => chooseMany(inContinent, false)}
                    >
                      {t('admin.radio.offer.removeAll')}
                    </AriaButton>
                  </span>
                  <div className={styles.scrollList}>
                    <DataTable
                      caption={t('admin.radio.offer.countryCaption', {
                        continent: t(`admin.radio.offer.continent.${continent}`),
                      })}
                      columns={countryColumns(t('admin.radio.offer.columns.country'))}
                      rows={inContinent}
                      rowKey={(country) => country.code}
                    />
                  </div>
                </div>
              )}
            </div>
          </BandPanel>
        </div>

        <aside className={styles.offerSummary} aria-labelledby={`${id}-summary`}>
          <div className={styles.summaryPanel}>
            <h2 id={`${id}-summary`} className={styles.summaryTitle}>
              {t('admin.radio.offer.summary.title')}
            </h2>
            {totals && (
              <FactList
                facts={[
                  [t('admin.radio.offer.summary.offered'), number.format(totals.offered)],
                  [
                    t('admin.radio.offer.summary.unreachable'),
                    number.format(totals.offeredUnreachable),
                  ],
                  [t('admin.radio.offer.summary.countries'), number.format(totals.countries)],
                  [t('admin.radio.offer.summary.marked'), number.format(totals.marked)],
                ]}
              />
            )}
            <Switch
              isSelected={hideBroken}
              onChange={(value) => add({ op: 'setHideBroken', value }, [['h', value]])}
            >
              {t('admin.radio.offer.summary.hideBroken')}
            </Switch>
            <Text tone="secondary">{t('admin.radio.offer.summary.hideBrokenHelp')}</Text>
          </div>
        </aside>

        <div className={styles.offerStations}>
          <BandPanel title={t('admin.radio.offer.chosen.title')} icon="radio">
            <div className={styles.section}>
              {chosen.length === 0 ? (
                <Text tone="secondary">{t('admin.radio.offer.chosen.empty')}</Text>
              ) : (
                <DataTable
                  caption={t('admin.radio.offer.chosen.title')}
                  columns={chosenColumns}
                  rows={chosen}
                  rowKey={(country) => country.code}
                />
              )}
              {opened && openedChosen && (
                <OfferStations
                  key={opened.code}
                  id={`${id}-stations`}
                  country={opened}
                  stepsRef={settledSteps}
                  overrides={overrides}
                  onStep={add}
                  onClose={closeStations}
                />
              )}
            </div>
          </BandPanel>
        </div>
      </div>

      {apply.error && !isApiError(apply.error, 409) && (
        <Notice tone="danger" title={problemMessage(t, apply.error)} />
      )}
      <div className={styles.offerBar} role="region" aria-label={t('admin.radio.offer.bar.label')}>
        <p className={styles.offerBarText} role="status">
          {barText}
        </p>
        <div className={styles.actions}>
          {dirty && (
            <AriaButton
              variant="ghost"
              isDisabled={apply.isPending}
              onPress={() => {
                setNote(null);
                change([]);
                void afterRender().then(() => focusTitleIfLost(titleRef));
              }}
            >
              {t('admin.radio.offer.bar.discard')}
            </AriaButton>
          )}
          <AriaButton
            variant="primary"
            isPending={apply.isPending}
            onPress={() => {
              if (dirty && version !== undefined) apply.mutate();
            }}
          >
            {apply.isPending
              ? t('admin.radio.offer.bar.applying')
              : t('admin.radio.offer.bar.apply')}
          </AriaButton>
        </div>
      </div>
    </div>
  );
}

interface OfferStationsProps {
  id: string;
  country: RadioOfferCountry & { name: string };
  /** The settled steps the pages are read with. */
  stepsRef: RefObject<readonly RadioOfferStep[]>;
  overrides: ReadonlyMap<string, Override>;
  onStep: (step: RadioOfferStep, shown?: [string, boolean][], drop?: string) => void;
  onClose: () => void;
}

/**
 * The stations of one chosen country as the draft has them (the preview's `list`, by
 * popularity, pages of 100, a virtual table as in concept 18 C-3): search, «Alle markieren» and
 * «Alle demarkieren» (with a search «Gezeigte …», resolved by the server), the mode of the
 * country, per station its tick, logo or initials, name, tags and «nicht erreichbar». Escape
 * closes the list.
 */
function OfferStations({ id, country, stepsRef, overrides, onStep, onClose }: OfferStationsProps) {
  const { t, i18n } = useTranslation();
  const number = useMemo(() => new Intl.NumberFormat(i18n.language), [i18n.language]);
  const [text, setText] = useState('');
  const [q, setQ] = useState('');
  useEffect(() => {
    const timer = window.setTimeout(() => setQ(text.trim()), DEBOUNCE_MS);
    return () => window.clearTimeout(timer);
  }, [text]);

  const code = country.code;
  const source = useMemo(
    () => ({
      key: [...LIST_KEY, code, q],
      fetch: (offset: number, limit: number, signal: AbortSignal) =>
        previewRadioOffer(
          {
            steps: [...(stepsRef.current ?? [])],
            list: { country: code, ...(q ? { q } : {}), offset, limit },
          },
          signal,
        ),
      items: (page: Awaited<ReturnType<typeof previewRadioOffer>>) => page.list?.items ?? [],
      total: (page: Awaited<ReturnType<typeof previewRadioOffer>>) => page.list?.total ?? 0,
      id: (station: RadioOfferStation) => station.id,
    }),
    [code, q, stepsRef],
  );
  const [span, setSpan] = useState<PlaceRange>({ start: 0, end: 0 });
  const paged = usePagedList(source, span);
  const total = paged.total;

  const offered = (station: RadioOfferStation) =>
    overrides.get(`s:${station.id}`)?.value ?? station.offered;

  const columns: DataColumn<RadioOfferStation>[] = [
    {
      key: 'tick',
      width: '3.5rem',
      header: t('admin.radio.offer.columns.tick'),
      hideHeader: true,
      cell: (station) => (
        <SelectCheck
          label={t('admin.radio.offer.stations.mark', { name: station.name })}
          isSelected={offered(station)}
          onChange={(value) =>
            onStep({ op: value ? 'mark' : 'demark', stationIds: [station.id] }, [
              [`s:${station.id}`, value],
            ])
          }
        />
      ),
    },
    {
      key: 'station',
      header: t('admin.radio.stations.columns.station'),
      rowHeader: true,
      cell: (station) => (
        <span className={styles.stationCell}>
          <StationLogo name={station.name} src={imageUrl(station.logo)} size="sm" />
          <span className={styles.offerStation}>
            <span>{station.name}</span>
            <span className={styles.muted}>
              {[station.broken && t('admin.radio.offer.stations.broken'), station.tags.join(', ')]
                .filter(Boolean)
                .join(' · ') || '–'}
            </span>
          </span>
        </span>
      ),
    },
  ];

  const many = (on: boolean) =>
    onStep(
      q
        ? { op: on ? 'markMatching' : 'demarkMatching', country: code, q }
        : { op: on ? 'markAll' : 'demarkAll', country: code },
      [],
      's:',
    );

  const panel = useEscape(onClose);

  return (
    <section ref={panel} id={id} className={styles.section} aria-labelledby={`${id}-title`}>
      <div className={styles.offerStationsHead}>
        <h3 id={`${id}-title`} className={styles.summaryTitle}>
          {t('admin.radio.offer.stations.title', { country: country.name })}
        </h3>
        <Text tone="secondary">
          {t('admin.radio.offer.stations.count', {
            offered: number.format(country.offered),
            total: number.format(country.total),
          })}
        </Text>
      </div>
      <Text tone="secondary">
        {country.mode === 'marked'
          ? t('admin.radio.offer.stations.modeMarked')
          : t('admin.radio.offer.stations.modeAll')}
      </Text>
      <div className={styles.filters}>
        <TextField label={t('admin.radio.offer.stations.search')} value={text} onChange={setText} />
        <span className={styles.actions}>
          <AriaButton size="sm" onPress={() => many(true)}>
            {q
              ? t('admin.radio.offer.stations.markShown', { count: total })
              : t('admin.radio.offer.stations.markAll')}
          </AriaButton>
          <AriaButton size="sm" variant="ghost" onPress={() => many(false)}>
            {q
              ? t('admin.radio.offer.stations.demarkShown', { count: total })
              : t('admin.radio.offer.stations.demarkAll')}
          </AriaButton>
        </span>
      </div>
      {paged.first.isError && <Notice tone="danger" title={problemMessage(t, paged.first.error)} />}
      {paged.first.isPending ? (
        <Skeleton shape="block" />
      ) : total === 0 ? (
        <Text tone="secondary">{t('admin.radio.offer.stations.empty')}</Text>
      ) : (
        <DataTable
          caption={t('admin.radio.offer.stations.title', { country: country.name })}
          columns={columns}
          rows={paged.slots}
          rowKey={(station) => String(station.id)}
          layout="fixed"
          virtual
          onRange={setSpan}
        />
      )}
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
    </section>
  );
}
