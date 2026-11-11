import {
  keepPreviousData,
  queryOptions,
  useMutation,
  useQuery,
  useQueryClient,
} from '@tanstack/react-query';
import {
  useEffect,
  useId,
  useMemo,
  useRef,
  useState,
  type Dispatch,
  type RefObject,
  type SetStateAction,
} from 'react';
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
  OFFER_STEP_IDS_MAX,
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

/** The most steps a draft may hold (contract 1.28.0, `steps` maxItems). */
const OFFER_STEPS_MAX = 1000;

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

const isTick = (step: RadioOfferStep | undefined) => step?.op === 'mark' || step?.op === 'demark';

interface StepContext {
  /** The steps before this place are on their way to the server («Übernehmen»): untouched. */
  floor: number;
  /** The stored switch «Nicht erreichbare weglassen» (a value of the server, no rule). */
  storedHideBroken: boolean | undefined;
}

/**
 * The steps with one more, as short as the admin's intent: the last step takes in what only
 * repeats or undoes it, and nothing before `floor` changes.
 *
 * - A tick undone at once leaves the last tick step (the station is as it was before it); more
 *   ticks of the same kind join it (up to OFFER_STEP_IDS_MAX ids).
 * - «Land entfernen» right after «Land wählen» of the same country takes the choice back: a
 *   `removeCountry` would also drop the stored exceptions of its stations.
 * - The switch keeps one step, none when it is back at the stored value.
 */
function withStep(
  steps: readonly RadioOfferStep[],
  step: RadioOfferStep,
  { floor, storedHideBroken }: StepContext,
): RadioOfferStep[] {
  const frozen = steps.slice(0, floor);
  const open = steps.slice(floor);
  const last = open.at(-1);
  const rest = open.slice(0, -1);
  if (isTick(step) && isTick(last) && last && step.stationIds) {
    const ids = step.stationIds;
    const lastIds = last.stationIds ?? [];
    if (last.op !== step.op && ids.length === 1 && lastIds.includes(ids[0]!)) {
      const left = lastIds.filter((id) => id !== ids[0]);
      return [...frozen, ...rest, ...(left.length > 0 ? [{ ...last, stationIds: left }] : [])];
    }
    const joined = [...new Set([...lastIds, ...ids])];
    if (last.op === step.op && joined.length <= OFFER_STEP_IDS_MAX) {
      return [...frozen, ...rest, { ...last, stationIds: joined }];
    }
  }
  if (
    step.op === 'removeCountry' &&
    last?.op === 'chooseCountry' &&
    last.country === step.country
  ) {
    return [...frozen, ...rest];
  }
  if (step.op === 'setHideBroken') {
    const before = last?.op === 'setHideBroken' ? [...frozen, ...rest] : [...frozen, ...open];
    const switched = before.some((entry) => entry.op === 'setHideBroken');
    return (step.value ?? false) === storedHideBroken && !switched ? before : [...before, step];
  }
  return [...frozen, ...open, step];
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
  onStepsChange: Dispatch<SetStateAction<RadioOfferStep[]>>;
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

  const offer = useQuery(radioOfferQuery);
  // The steps an «Übernehmen» on its way sent: they stay as they are until it answered.
  const [sending, setSending] = useState(0);
  const context: StepContext = { floor: sending, storedHideBroken: offer.data?.hideBroken };
  // A change that would pass OFFER_STEPS_MAX is refused with a notice.
  const [full, setFull] = useState(false);

  // Every change counts; the preview of a change replaces what the controls showed meanwhile.
  const [generation, setGeneration] = useState(0);
  const [overrides, setOverrides] = useState<ReadonlyMap<string, Override>>(() => new Map());
  const change = (next: RadioOfferStep[], shown: [string, boolean][] = [], drop?: string) => {
    if (next.length > OFFER_STEPS_MAX) {
      setFull(true);
      return;
    }
    setFull(false);
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
    change(withStep(steps, step, context), shown, drop);

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
    void Promise.allSettled(work).then((results) => {
      // A preview that failed leaves the controls on the admin's input (and says so).
      if (!live || results.some((result) => result.status === 'rejected')) return;
      setOverrides((previous) => {
        const map = new Map([...previous].filter(([, entry]) => entry.gen > settled.gen));
        return map.size === previous.size ? previous : map;
      });
    });
    return () => {
      live = false;
    };
  }, [queryClient, settled]);

  const preview = useQuery({
    ...previewQuery(settled.steps),
    enabled: settled.steps.length > 0,
    placeholderData: keepPreviousData,
  });
  const dirty = steps.length > 0;
  const view = dirty && preview.data ? preview.data : offer.data;
  // The preview belongs to the draft as it is now: only then its numbers and its version count.
  const previewOfDraft = dirty && settled.steps === steps;
  const previewFailed = previewOfDraft && preview.isError && !preview.isFetching;
  const previewReady =
    previewOfDraft && preview.isSuccess && !preview.isPlaceholderData && !preview.isFetching;
  const version = previewReady ? preview.data.version : undefined;

  const [note, setNote] = useState<'applied' | 'conflict' | null>(null);
  const apply = useMutation({
    mutationFn: (sent: { version: number; steps: RadioOfferStep[] }) =>
      applyRadioOffer(sent.version, sent.steps),
    onMutate: (sent) => setSending(sent.steps.length),
    onSuccess: async (applied, sent) => {
      queryClient.setQueryData(radioOfferQuery.queryKey, applied.offer);
      setNote('applied');
      setSending(0);
      // What came while the apply was on its way stays: the steps are relative.
      onStepsChange((current) => current.slice(sent.steps.length));
      void afterRender().then(() => focusTitleIfLost(titleRef));
      await queryClient.invalidateQueries({ queryKey: radioAdminKeys.all });
      // The radio page shows the new offer at once.
      await queryClient.invalidateQueries({ queryKey: radioKeys.all });
    },
    onError: async (error) => {
      setSending(0);
      if (!isApiError(error, 409)) return;
      // Another tab stored meanwhile: the same steps on the new offer, for a second «Übernehmen»
      // once the preview of the new offer is there.
      await queryClient.invalidateQueries({ queryKey: radioAdminKeys.all });
      setNote('conflict');
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
  const choose = (country: Country, on: boolean) => {
    if (!on && open === country.code) setOpen(null);
    add(
      { op: on ? 'chooseCountry' : 'removeCountry', country: country.code },
      [[`c:${country.code}`, on]],
      on ? undefined : 's:',
    );
  };

  // The chosen country whose station list is open.
  const [open, setOpen] = useState<string | null>(null);

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
      next = withStep(
        next,
        { op: on ? 'chooseCountry' : 'removeCountry', country: country.code },
        context,
      );
    }
    if (!on && todo.some((country) => country.code === open)) setOpen(null);
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
  const openButton = (code: string) => `${id}-open-${code}`;
  const chosen = countries.filter(isChosen);
  const opened = open ? countries.find((country) => country.code === open) : undefined;
  const openedChosen = opened !== undefined && isChosen(opened);
  const removeButton = (code: string) => `${id}-remove-${code}`;
  // «Land entfernen»: its row goes, the focus to the same button of the next row (else the h1).
  const removeChosen = (country: Country) => {
    const at = chosen.findIndex((entry) => entry.code === country.code);
    const next = chosen[at + 1] ?? chosen[at - 1];
    choose(country, false);
    void afterRender().then(() => {
      const button = next ? document.getElementById(removeButton(next.code)) : null;
      if (button) button.focus();
      else focusTitleIfLost(titleRef);
    });
  };
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
              id={removeButton(country.code)}
              variant="dangerQuiet"
              size="sm"
              aria-label={t('admin.radio.offer.chosen.removeLabel', { country: country.name })}
              onPress={() => removeChosen(country)}
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
  const diff = previewReady ? preview.data.diff : undefined;
  const diffText = !diff
    ? previewFailed
      ? t('admin.radio.offer.bar.previewFailed')
      : t('admin.radio.offer.bar.computing')
    : diff.added === 0 && diff.removed === 0
      ? t('admin.radio.offer.bar.same')
      : [
          diff.added > 0 &&
            t('admin.radio.offer.bar.added', { count: diff.added, n: number.format(diff.added) }),
          diff.removed > 0 &&
            t('admin.radio.offer.bar.removed', {
              count: diff.removed,
              n: number.format(diff.removed),
            }),
        ]
          .filter(Boolean)
          .join(' ');
  const barText = dirty
    ? note === 'conflict'
      ? `${t('admin.radio.offer.bar.conflict')} ${diffText}`
      : diffText
    : note === 'applied'
      ? t('admin.radio.offer.bar.applied')
      : t('admin.radio.offer.bar.clean');
  const atLimit = full || steps.length >= OFFER_STEPS_MAX;

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

      {previewFailed && <Notice tone="danger" title={problemMessage(t, preview.error)} />}
      {atLimit && (
        <Notice tone="warning" title={t('admin.radio.offer.limit', { max: OFFER_STEPS_MAX })} />
      )}
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
                setFull(false);
                void afterRender().then(() => focusTitleIfLost(titleRef));
              }}
            >
              {t('admin.radio.offer.bar.discard')}
            </AriaButton>
          )}
          {/* Only with a draft whose preview is there: its version is the one to send. */}
          <AriaButton
            variant="primary"
            isPending={apply.isPending}
            isDisabled={!apply.isPending && version === undefined}
            onPress={() => {
              if (version !== undefined && !apply.isPending) apply.mutate({ version, steps });
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
