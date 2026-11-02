import type {
  ImageLangs,
  Library,
  LibraryImages,
  LibraryMetadata,
  MetadataProviderInfo,
  MetadataSettings,
  MetadataSettingsUpdate,
} from '@slouchcat/api-client';
import type { TFunction } from 'i18next';
import { useMutation, useQueries, useQuery, useQueryClient } from '@tanstack/react-query';
import {
  useEffect,
  useId,
  useLayoutEffect,
  useMemo,
  useRef,
  useState,
  type ReactNode,
} from 'react';
import { useTranslation } from 'react-i18next';
import { getRouteApi } from '@tanstack/react-router';
import { formErrors, problemMessage } from '@/api/errors';
import { paths } from '@/app/paths';
import { useActiveSession } from '@/app/session';
import { formatDateTime } from '@/lib/format';
import { useDocumentTitle } from '@/lib/useDocumentTitle';
import { Chip, Notice, Panel, SelectField, Skeleton, Text, TextLink, VisuallyHidden } from '@/ui';
import {
  AdminPage,
  AriaButton,
  CheckboxGroup,
  ConfirmDialog,
  MasterDetail,
  RadioGroup,
  RankedChoiceList,
  SaveBar,
  SectionHeader,
  SettingRow,
  Switch,
  useDrafts,
  useSearchSelection,
  useUnsavedChanges,
  type MasterDetailItem,
} from '@/ui/aria';
import {
  applyMetadata,
  jobTypesQuery,
  librariesQuery,
  libraryMetadataQuery,
  metadataProvidersQuery,
  metadataSettingsQuery,
  updateLibraryMetadata,
  updateMetadataSettings,
} from './queries';
import styles from './MetadataPage.module.css';

const route = getRouteApi('/_authed/_active/_chrome/admin/metadata');

/** The chain kinds in the order of the page. */
const CHAIN_KINDS = ['poster', 'backdrop', 'logo'] as const;
type ChainKind = (typeof CHAIN_KINDS)[number];

/** The image kinds that are always fetched. */
const ALWAYS_KINDS = ['poster', 'backdrop'] as const;

/** The switchable image kinds (poster and backdrop are always on, E-1). */
const SWITCHABLE = ['logo', 'season_poster', 'episode_still'] as const;
type Switchable = (typeof SWITCHABLE)[number];

const POSTER_WIDTHS = [0, 500, 1000, 1500, 2000] as const;
const BACKDROP_WIDTHS = [0, 1280, 1920, 3840] as const;
const COUNTRIES = ['DE', 'AT', 'CH', 'GB', 'US'] as const;
type Country = (typeof COUNTRIES)[number];

/** The YouTube quota: 10'000 units a day, a search costs 100 (ADR 0006). */
const YOUTUBE_SEARCHES_PER_DAY = 100;

/** The scope id of the server default; a library's scope id is its id. */
const SERVER = 'server';

/** The settings areas of the list (Met-B); a library has only the two it can deviate in. */
const SERVER_AREAS = ['languages', 'images', 'quality', 'sources', 'rating', 'apply'] as const;
const LIBRARY_AREAS = ['languages', 'images'] as const;
type Area = (typeof SERVER_AREAS)[number];

/** The draft of the server default: everything `PUT` takes. */
type ServerValues = MetadataSettingsUpdate;

/** The draft of a library: its two deviations, null where it follows the default. */
interface LibraryValues {
  textLangs: string[] | null;
  images: LibraryImages | null;
}

/** What the drafts hold per scope id (S-17): the default under SERVER, a library under its id. */
type ScopeValues = ServerValues | LibraryValues;

/** The image part a page shows: the chains and the switchable kinds. */
interface ImagesView {
  langs: ImageLangs;
  kinds: Switchable[];
}

const switchable = (kinds: readonly string[]): Switchable[] =>
  kinds.filter((kind): kind is Switchable => (SWITCHABLE as readonly string[]).includes(kind));

const copyLangs = (langs: ImageLangs): ImageLangs => ({
  poster: [...langs.poster],
  backdrop: [...langs.backdrop],
  logo: [...langs.logo],
});

function serverValues(s: MetadataSettings): ServerValues {
  return {
    textLangs: [...s.textLangs],
    imageLangs: copyLangs(s.imageLangs),
    imageKinds: [...s.imageKinds],
    imageMinWidth: { ...s.imageMinWidth },
    imageByVote: s.imageByVote,
    localImages: s.localImages,
    ratingCountry: s.ratingCountry,
    tmdbIncludeAdult: s.tmdbIncludeAdult,
    fanartEnabled: s.fanartEnabled,
  };
}

function libraryValues(lm: LibraryMetadata): LibraryValues {
  return {
    textLangs: lm.textLangs ? [...lm.textLangs] : null,
    images: lm.images ? { langs: copyLangs(lm.images.langs), kinds: [...lm.images.kinds] } : null,
  };
}

/** The image settings a library inherits from the default. */
const inheritedImages = (lm: LibraryMetadata): ImagesView => ({
  langs: copyLangs(lm.effective.imageLangs),
  kinds: switchable(lm.effective.imageKinds),
});

const same = (a: unknown, b: unknown) => JSON.stringify(a) === JSON.stringify(b);

/** The fields of the default that each area edits, for the «geändert» mark in the list. */
const SERVER_FIELDS: Record<Exclude<Area, 'apply'>, (keyof ServerValues)[]> = {
  languages: ['textLangs'],
  images: ['imageLangs', 'imageKinds'],
  quality: ['imageMinWidth', 'imageByVote'],
  sources: ['tmdbIncludeAdult', 'localImages', 'fanartEnabled'],
  rating: ['ratingCountry'],
};

/** The display name of a language code in the interface language (rule 9: no name in the code). */
function useLanguageName(): (code: string) => string {
  const { i18n } = useTranslation();
  const names = useMemo(
    () => new Intl.DisplayNames([i18n.language], { type: 'language' }),
    [i18n.language],
  );
  return (code: string) => (code ? (names.of(code) ?? code) : '');
}

/**
 * The rule of a text language list (MET-2, as the server checks it in
 * `metadata/settings.go`): the default needs German, a library German or English.
 */
const satisfiesDefault = (list: readonly string[]) => list.includes('de');
const satisfiesLibrary = (list: readonly string[]) => list.includes('de') || list.includes('en');

/**
 * An ordered list of language codes as fixed places (concept 12, 10.2): «1. Sprache», «2.
 * Sprache» … each with a select of every known language; picking one that sits on another place
 * swaps the two. «Weitere Sprache» appends a place, «×» removes one. `satisfies` is the rule of
 * the list: choices and removals that would break it are off; a chain offers "" (no text).
 */
function LanguageRanks({
  value,
  onChange,
  known,
  allowNoText,
  satisfies,
  disabled,
}: {
  value: string[];
  onChange: (value: string[]) => void;
  known: readonly string[];
  allowNoText?: boolean;
  satisfies?: (list: readonly string[]) => boolean;
  disabled?: boolean;
}) {
  const { t } = useTranslation();
  const nameOf = useLanguageName();
  const choices = useMemo(() => {
    const codes = [...new Set([...known, ...value.filter((code) => code !== '')])];
    const list = codes.map((code) => ({ value: code, label: nameOf(code) }));
    list.sort((a, b) => a.label.localeCompare(b.label));
    return allowNoText ? [{ value: '', label: t('admin.metadata.images.noText') }, ...list] : list;
  }, [known, value, allowNoText, nameOf, t]);
  return (
    <RankedChoiceList
      choices={choices}
      value={value}
      onChange={onChange}
      rankLabel={(index) => t('admin.metadata.languages.rank', { position: index + 1 })}
      addLabel={t('admin.metadata.languages.more')}
      isAllowed={satisfies}
      isDisabled={disabled}
    />
  );
}

/** One settings area in the detail: a section named by its h2 (S-3). */
function AreaPanel({
  title,
  intro,
  actions,
  children,
}: {
  title: string;
  intro: string;
  actions?: ReactNode;
  children: ReactNode;
}) {
  const id = useId();
  return (
    <Panel as="section" aria-labelledby={id}>
      <SectionHeader id={id} title={title} description={intro} actions={actions} />
      {children}
    </Panel>
  );
}

/** Which of the server's languages a provider does not know (6.1, 12.3). */
function unknownLanguages(
  t: TFunction,
  nameOf: (code: string) => string,
  listOf: (items: string[]) => string,
  providers: MetadataProviderInfo[],
  langs: string[],
): string[] {
  const lines: string[] = [];
  for (const p of providers) {
    if (p.id === 'local') continue;
    const missing = langs.filter((code) => !p.languages.includes(code));
    if (missing.length > 0 && p.languages.length > 0) {
      lines.push(
        t('admin.metadata.languages.unknownAt', {
          provider: p.id.toUpperCase(),
          languages: listOf(missing.map(nameOf)),
        }),
      );
    }
  }
  return lines;
}

/** The area "Anwenden": what, where, the button, the waiting children of the jobs page. */
function ApplySection({
  libraries,
  titlesOf,
  settings,
  paused,
}: {
  libraries: Library[];
  titlesOf: (id: number) => number | undefined;
  settings: MetadataSettings;
  paused: boolean;
}) {
  const { t, i18n } = useTranslation();
  const queryClient = useQueryClient();
  const [what, setWhat] = useState<'both' | 'texts' | 'images'>('both');
  const [where, setWhere] = useState('all');
  const [message, setMessage] = useState<string | null>(null);
  const jobTypes = useQuery({ ...jobTypesQuery, refetchInterval: paused ? false : 10_000 });
  const run = useMutation({
    mutationFn: applyMetadata,
    onSuccess: async (result) => {
      setMessage(t('admin.metadata.apply.queued', { id: result.jobId, count: result.titles }));
      await queryClient.invalidateQueries({ queryKey: jobTypesQuery.queryKey });
    },
  });
  const waiting = (type: string) =>
    jobTypes.data?.types.find((entry) => entry.type === type)?.counts.queued ?? 0;
  const lastRun = jobTypes.data?.types.find((entry) => entry.type === 'metadata.apply')?.lastRun;
  const withTitles = (name: string, count: number | undefined) =>
    count === undefined ? name : `${name} (${t('admin.metadata.apply.titles', { count })})`;
  return (
    <AreaPanel title={t('admin.metadata.apply.title')} intro={t('admin.metadata.apply.intro')}>
      <div className={styles.fields}>
        <RadioGroup
          name="what"
          label={t('admin.metadata.apply.what')}
          value={what}
          onChange={(value) => setWhat(value as 'both' | 'texts' | 'images')}
          options={[
            { value: 'both', label: t('admin.metadata.apply.both') },
            { value: 'texts', label: t('admin.metadata.apply.texts') },
            { value: 'images', label: t('admin.metadata.apply.images') },
          ]}
        />
        <RadioGroup
          name="where"
          label={t('admin.metadata.apply.where')}
          value={where}
          onChange={setWhere}
          options={[
            {
              value: 'all',
              label: withTitles(t('admin.metadata.apply.all'), settings.affectedTitles),
            },
            ...libraries.map((library) => ({
              value: String(library.id),
              label: withTitles(library.name, titlesOf(library.id)),
            })),
          ]}
        />
      </div>
      <div className={styles.actions}>
        <AriaButton
          variant="primary"
          isPending={run.isPending}
          onPress={() => {
            setMessage(null);
            void run.mutate({
              what: what === 'both' ? ['texts', 'images'] : [what],
              ...(where === 'all' ? {} : { libraryIds: [Number(where)] }),
            });
          }}
        >
          {run.isPending ? t('admin.metadata.apply.running') : t('admin.metadata.apply.run')}
        </AriaButton>
        <TextLink href={paths.adminJobs}>{t('admin.metadata.apply.jobsLink')}</TextLink>
      </div>
      {run.error ? <Notice tone="danger">{problemMessage(t, run.error)}</Notice> : null}
      <div role="status" className={styles.live}>
        {message && !run.error && <Notice live={false}>{message}</Notice>}
      </div>
      {jobTypes.data && (
        <Text tone="secondary" variant="caption">
          {t('admin.metadata.apply.waiting', {
            matches: waiting('metadata.match'),
            images: waiting('image.fetch'),
          })}{' '}
          {lastRun
            ? t('admin.metadata.apply.lastRun', {
                when: formatDateTime(
                  lastRun.finishedAt ?? lastRun.startedAt ?? lastRun.updatedAt,
                  i18n.language,
                ),
                state: t(`admin.jobs.runs.tab.${lastRun.state}`),
              })
            : t('admin.metadata.apply.noRun')}
        </Text>
      )}
    </AreaPanel>
  );
}

/**
 * The page "Metadaten" (concept 10, ADR 0018; layout concept 12 Met-B): the scope (the server
 * default or one video library) as a select in the header, bound to `?library=`; the settings
 * areas in a list on the left (`?item=`), the selected area on the right. A library edits only
 * its two deviations. Unsaved changes are kept per scope (useDrafts, S-17), so switching scope or
 * area loses nothing; one save bar saves every changed scope, the default first, then the
 * libraries in list order, and stops at the first failure.
 */
export function MetadataPage() {
  const { t, i18n } = useTranslation();
  useDocumentTitle(t('admin.metadata.title'));
  const session = useActiveSession();
  const allowed = session.account.role === 'admin' && session.profile.isPrimary;
  const queryClient = useQueryClient();
  const nameOf = useLanguageName();
  const navigate = route.useNavigate();
  const titleRef = useRef<HTMLHeadingElement>(null);

  const settings = useQuery({ ...metadataSettingsQuery, enabled: allowed });
  const libraries = useQuery({ ...librariesQuery, enabled: allowed });
  const providers = useQuery({ ...metadataProvidersQuery, enabled: allowed });
  const videoLibraries = useMemo(
    () => (libraries.data ?? []).filter((library) => library.kind === 'video'),
    [libraries.data],
  );
  // Every library's deviation is loaded: its draft must keep its original while another scope
  // is shown, and the save messages and the apply list name its titles.
  const libraryMetas = useQueries({
    queries: videoLibraries.map((library) => ({
      ...libraryMetadataQuery(library.id),
      enabled: allowed,
    })),
  });
  const metaQueryOf = (id: string) =>
    libraryMetas[videoLibraries.findIndex((library) => String(library.id) === id)];

  const originalOf = (id: string): ScopeValues | undefined => {
    if (id === SERVER) return settings.data ? serverValues(settings.data) : undefined;
    const meta = metaQueryOf(id)?.data;
    return meta ? libraryValues(meta) : undefined;
  };
  const drafts = useDrafts<ScopeValues>(originalOf);
  const unsaved = useUnsavedChanges(drafts.dirty, { keep: ['library'] });
  // The drafts of the last render, for a save that is still waiting for the server: what was
  // typed meanwhile must not be dropped with the saved scope.
  const latestDrafts = useRef(drafts);
  useLayoutEffect(() => {
    latestDrafts.current = drafts;
  });

  const { library: wanted } = route.useSearch();
  const wantedKnown =
    wanted !== undefined && videoLibraries.some((library) => library.id === wanted);
  const scope = wantedKnown ? String(wanted) : SERVER;
  // An unknown `?library=` (deleted, or not a video library) leaves the address once the list is
  // there; the page shows the default meanwhile.
  const staleLibrary = libraries.data !== undefined && wanted !== undefined && !wantedKnown;
  useEffect(() => {
    if (staleLibrary)
      void navigate({
        to: '.',
        replace: true,
        search: (previous) => ({ ...previous, library: undefined }),
      });
  }, [staleLibrary, navigate]);
  const inLibrary = scope !== SERVER;
  const selectScope = (id: string) =>
    void navigate({
      to: '.',
      replace: true,
      search: (previous) => ({ ...previous, library: id === SERVER ? undefined : Number(id) }),
    });
  const [area, selectArea] = useSearchSelection(
    'item',
    inLibrary ? LIBRARY_AREAS : SERVER_AREAS,
    'languages',
  );

  const scopeIds = [SERVER, ...videoLibraries.map((library) => String(library.id))];
  const scopeName = (id: string) =>
    id === SERVER
      ? t('admin.metadata.scope.server')
      : (videoLibraries.find((library) => String(library.id) === id)?.name ?? id);
  /** The changed scopes in save order: the default first, then the libraries in list order. */
  const dirtyScopes = scopeIds.filter((id) => drafts.isDirty(id));
  const listOf = (names: string[]) =>
    new Intl.ListFormat(i18n.language, { type: 'conjunction' }).format(names);

  const [saving, setSaving] = useState(false);
  const [savedMessages, setSavedMessages] = useState<{ scope: string; text: string }[]>([]);
  const [failure, setFailure] = useState<{ scope: string; error: unknown } | null>(null);
  const [discardOpen, setDiscardOpen] = useState(false);
  // The last deviating lists of a library whose «abweichend» was switched off, so switching it on
  // again brings them back (S-13).
  const [parked, setParked] = useState<
    Record<string, { textLangs?: string[]; images?: LibraryImages }>
  >({});
  // Input of a scope that changed while its save waited for the server, to be put back as a
  // draft once the page sees the saved values as the original (L-5): a field set back to the
  // old value during the save would otherwise turn into the saved one.
  const [restores, setRestores] = useState<
    { id: string; value: ScopeValues; original: ScopeValues }[]
  >([]);
  const due = restores.filter((entry) => same(originalOf(entry.id), entry.original));
  if (due.length > 0) {
    setRestores(restores.filter((entry) => !due.includes(entry)));
    for (const entry of due) drafts.set(entry.id, entry.value);
  }
  const errors = failure ? formErrors(t, failure.error) : null;
  const fieldError = (field: string) =>
    failure?.scope === scope ? errors?.fields?.[field] : undefined;

  const onSave = async () => {
    const queue = dirtyScopes.map((id) => ({ id, values: drafts.valueOf(id) }));
    setSaving(true);
    setSavedMessages([]);
    setFailure(null);
    const messages: { scope: string; text: string }[] = [];
    let serverSaved = false;
    for (const { id, values } of queue) {
      if (!values) continue;
      try {
        let latest: ScopeValues | undefined;
        let original: ScopeValues;
        let titles: number;
        if (id === SERVER) {
          const stored = await updateMetadataSettings(values as ServerValues);
          latest = latestDrafts.current.valueOf(id);
          queryClient.setQueryData(metadataSettingsQuery.queryKey, stored);
          serverSaved = true;
          original = serverValues(stored);
          titles = stored.affectedTitles;
        } else {
          const stored = await updateLibraryMetadata(Number(id), values);
          latest = latestDrafts.current.valueOf(id);
          queryClient.setQueryData(libraryMetadataQuery(Number(id)).queryKey, stored);
          original = libraryValues(stored);
          titles = stored.affectedTitles;
        }
        messages.push({
          scope: id,
          text: t('admin.metadata.save.savedScope', { name: scopeName(id), count: titles }),
        });
        // The draft is spent only while it is still what was sent; what was typed meanwhile
        // (also a field set back to its old value) stays as a draft against the saved values.
        if (latest === undefined || same(latest, values)) drafts.discard(id);
        else setRestores((current) => [...current, { id, value: latest, original }]);
        setSavedMessages([...messages]);
      } catch (error) {
        // Stop here: this scope and the ones after it keep their drafts.
        setFailure({ scope: id, error });
        break;
      }
    }
    setSaving(false);
    // A library that follows the default shows the new default.
    if (serverSaved)
      await queryClient.invalidateQueries({ queryKey: ['admin', 'library-metadata'] });
    if (messages.length > 0) await queryClient.invalidateQueries({ queryKey: ['session'] });
  };
  const onDiscard = () => {
    drafts.discardAll();
    setParked({});
    setRestores([]);
    setSavedMessages([]);
    setFailure(null);
    // The bar's buttons are disabled now; the focus goes to the page head (ALL-2).
    requestAnimationFrame(() => titleRef.current?.focus());
  };

  const selectedMeta = inLibrary ? metaQueryOf(scope) : undefined;
  const lm = selectedMeta?.data;
  const serverDraft = drafts.valueOf(SERVER) as ServerValues | undefined;
  const libraryDraft = inLibrary ? (drafts.valueOf(scope) as LibraryValues | undefined) : undefined;
  const known = settings.data?.knownLanguages ?? [];

  const setServer = (change: (values: ServerValues) => ServerValues) =>
    drafts.set(SERVER, (values) => change(values as ServerValues));
  const setLibrary = (change: (values: LibraryValues) => LibraryValues) =>
    drafts.set(scope, (values) => change(values as LibraryValues));

  const textLangs = inLibrary
    ? lm && libraryDraft && (libraryDraft.textLangs ?? lm.effective.textLangs)
    : serverDraft?.textLangs;
  const textsEditable = !inLibrary || (libraryDraft?.textLangs ?? null) !== null;
  const images: ImagesView | undefined = inLibrary
    ? lm && libraryDraft && (libraryDraft.images ?? inheritedImages(lm))
    : serverDraft && { langs: serverDraft.imageLangs, kinds: switchable(serverDraft.imageKinds) };
  const imagesEditable = !inLibrary || (libraryDraft?.images ?? null) !== null;

  const setTextLangs = (langs: string[]) =>
    inLibrary
      ? setLibrary((values) => ({ ...values, textLangs: langs }))
      : setServer((values) => ({ ...values, textLangs: langs }));
  const setImages = (change: (view: ImagesView) => ImagesView) => {
    if (inLibrary) {
      if (!lm) return;
      setLibrary((values) => ({ ...values, images: change(values.images ?? inheritedImages(lm)) }));
    } else {
      setServer((values) => {
        const next = change({ langs: values.imageLangs, kinds: switchable(values.imageKinds) });
        return { ...values, imageLangs: next.langs, imageKinds: next.kinds };
      });
    }
  };
  const setChain = (kind: ChainKind, chain: string[]) =>
    setImages((view) => ({ ...view, langs: { ...view.langs, [kind]: chain } }));

  /**
   * «abweichend» of a library: off keeps the deviating list aside, on brings it back (or starts
   * from the default when there is none).
   */
  const toggleTexts = (on: boolean) => {
    if (!lm || !libraryDraft) return;
    if (!on && libraryDraft.textLangs)
      setParked((all) => ({
        ...all,
        [scope]: { ...all[scope], textLangs: libraryDraft.textLangs! },
      }));
    setLibrary((values) => ({
      ...values,
      textLangs: on ? (parked[scope]?.textLangs ?? [...lm.effective.textLangs]) : null,
    }));
  };
  const toggleImages = (on: boolean) => {
    if (!lm || !libraryDraft) return;
    if (!on && libraryDraft.images)
      setParked((all) => ({ ...all, [scope]: { ...all[scope], images: libraryDraft.images! } }));
    setLibrary((values) => ({
      ...values,
      images: on ? (parked[scope]?.images ?? inheritedImages(lm)) : null,
    }));
  };

  /** Whether the default has unsaved changes in an area (a library shows the saved default). */
  const serverAreaDirty = (id: Exclude<Area, 'apply'>): boolean => {
    if (!settings.data || !serverDraft || !drafts.isDirty(SERVER)) return false;
    const original = serverValues(settings.data);
    return SERVER_FIELDS[id].some((field) => !same(serverDraft[field], original[field]));
  };
  const serverDirtyNote = (id: 'languages' | 'images') =>
    inLibrary && serverAreaDirty(id) ? (
      <Notice live={false}>{t('admin.metadata.scope.serverDirty')}</Notice>
    ) : null;

  /** Whether an area of the shown scope differs from what the server holds. */
  const areaDirty = (id: Area): boolean => {
    if (id === 'apply' || !drafts.isDirty(scope)) return false;
    if (inLibrary) {
      const original = lm ? libraryValues(lm) : undefined;
      if (!original || !libraryDraft) return false;
      if (id === 'languages') return !same(libraryDraft.textLangs, original.textLangs);
      if (id === 'images') return !same(libraryDraft.images, original.images);
      return false;
    }
    if (!settings.data || !serverDraft) return false;
    const original = serverValues(settings.data);
    return SERVER_FIELDS[id].some((field) => !same(serverDraft[field], original[field]));
  };

  const widthLabel = (width: number) =>
    width === 0 ? t('admin.metadata.quality.any') : t('admin.metadata.quality.px', { width });
  const summaryOf = (id: Area): string | undefined => {
    let summary: string | undefined;
    if (id === 'languages' && textLangs)
      summary = textLangs.map(nameOf).join(t('admin.metadata.areas.separator'));
    if (id === 'images' && images)
      summary = listOf([
        t('admin.metadata.images.kind.poster'),
        t('admin.metadata.images.kind.backdrop'),
        ...images.kinds.map((kind) => t(`admin.metadata.images.kind.${kind}`)),
      ]);
    if (id === 'quality' && serverDraft)
      summary = t('admin.metadata.areas.quality', {
        poster: widthLabel(serverDraft.imageMinWidth.poster),
        backdrop: widthLabel(serverDraft.imageMinWidth.backdrop),
      });
    if (id === 'sources' && serverDraft) {
      const on = [
        serverDraft.localImages ? t('admin.metadata.areas.sources.local') : '',
        serverDraft.fanartEnabled ? t('admin.metadata.areas.sources.fanart') : '',
        serverDraft.tmdbIncludeAdult ? t('admin.metadata.areas.sources.tmdbAdult') : '',
      ].filter(Boolean);
      summary = on.length > 0 ? listOf(on) : t('admin.metadata.areas.sources.none');
    }
    if (id === 'rating' && serverDraft)
      summary = t(`admin.metadata.rating.countries.${serverDraft.ratingCountry as Country}`);
    if (summary !== undefined && inLibrary) {
      const deviates = id === 'languages' ? textsEditable : imagesEditable;
      summary = t('admin.metadata.areas.scoped', {
        state: deviates ? t('admin.metadata.scope.deviates') : t('admin.metadata.scope.likeServer'),
        summary,
      });
    }
    if (summary !== undefined && areaDirty(id))
      summary = t('admin.metadata.areas.changed', { summary });
    return summary;
  };
  const areaLabels: Record<Area, string> = {
    languages: t('admin.metadata.languages.title'),
    images: t('admin.metadata.images.title'),
    quality: t('admin.metadata.quality.title'),
    sources: t('admin.metadata.sources.title'),
    rating: t('admin.metadata.rating.title'),
    apply: t('admin.metadata.apply.short'),
  };
  const areaItems: MasterDetailItem[] = (inLibrary ? LIBRARY_AREAS : SERVER_AREAS).map((id) => ({
    id,
    label: areaLabels[id],
    description: summaryOf(id),
  }));

  const scopeHint = (
    <Text tone="secondary" variant="caption">
      {inLibrary
        ? t('admin.metadata.scope.libraryHint', { name: scopeName(scope) })
        : t('admin.metadata.scope.serverHint')}{' '}
      {t('admin.metadata.apply.titles', {
        count: (inLibrary ? lm?.affectedTitles : settings.data?.affectedTitles) ?? 0,
      })}
    </Text>
  );
  const youtubeTitles = textLangs
    ? Math.floor(
        YOUTUBE_SEARCHES_PER_DAY / Math.max(1, new Set(textLangs.map((c) => c.split('-')[0])).size),
      )
    : 0;
  const textLangsError = fieldError('textLangs');
  const ratingCountryError = fieldError('ratingCountry');

  const deviation = (label: string, on: boolean, toggle: (on: boolean) => void) => (
    <>
      {!on && <Chip tone="outline">{t('admin.metadata.scope.likeServer')}</Chip>}
      <Switch isSelected={on} onChange={toggle}>
        {label}
      </Switch>
    </>
  );

  let detail: ReactNode = null;
  if (inLibrary && !lm) {
    detail = selectedMeta?.isError ? (
      <Notice
        tone="danger"
        title={problemMessage(t, selectedMeta.error)}
        actions={
          <AriaButton icon="refresh" onPress={() => void selectedMeta.refetch()}>
            {t('common.retry')}
          </AriaButton>
        }
      />
    ) : (
      <Skeleton shape="block" />
    );
  } else if (area === 'languages' && textLangs) {
    detail = (
      <AreaPanel
        title={t('admin.metadata.languages.title')}
        intro={t('admin.metadata.languages.intro')}
        actions={
          inLibrary && lm
            ? deviation(t('admin.metadata.scope.textsDeviate'), textsEditable, toggleTexts)
            : undefined
        }
      >
        {scopeHint}
        {!textsEditable && serverDirtyNote('languages')}
        <SettingRow label={t('admin.metadata.languages.list')}>
          <LanguageRanks
            value={textLangs}
            onChange={setTextLangs}
            known={known}
            satisfies={inLibrary ? satisfiesLibrary : satisfiesDefault}
            disabled={!textsEditable}
          />
        </SettingRow>
        {textLangsError && <Notice tone="danger">{textLangsError}</Notice>}
        <Text tone="secondary" variant="caption">
          {t('admin.metadata.languages.fallback')}
        </Text>
        {providers.data && (
          <Text tone="secondary" variant="caption">
            {unknownLanguages(t, nameOf, listOf, providers.data, textLangs).join(' ')}{' '}
            {t('admin.metadata.languages.youtube', {
              count: textLangs.length,
              titles: youtubeTitles,
            })}
          </Text>
        )}
      </AreaPanel>
    );
  } else if (area === 'images' && images) {
    detail = (
      <AreaPanel
        title={t('admin.metadata.images.title')}
        intro={t('admin.metadata.images.intro')}
        actions={
          inLibrary && lm
            ? deviation(t('admin.metadata.scope.imagesDeviate'), imagesEditable, toggleImages)
            : undefined
        }
      >
        {scopeHint}
        {!imagesEditable && serverDirtyNote('images')}
        <div className={imagesEditable ? undefined : styles.dimmed}>
          {ALWAYS_KINDS.map((kind) => (
            <SettingRow
              key={kind}
              label={t(`admin.metadata.images.kind.${kind}`)}
              help={t(`admin.metadata.images.kindHint.${kind}`)}
            >
              <Chip tone="outline">{t('admin.metadata.images.always')}</Chip>
            </SettingRow>
          ))}
        </div>
        <CheckboxGroup
          label={t('admin.metadata.images.kinds')}
          value={images.kinds}
          onChange={(kinds) => setImages((view) => ({ ...view, kinds: switchable(kinds) }))}
          isDisabled={!imagesEditable}
          options={SWITCHABLE.map((kind: Switchable) => ({
            value: kind,
            label: t(`admin.metadata.images.kind.${kind}`),
            description: t(`admin.metadata.images.kindHint.${kind}`),
          }))}
        />
        <Text tone="secondary" variant="caption">
          {t('admin.metadata.images.notBuilt')}
        </Text>
        <SectionHeader
          level={3}
          title={t('admin.metadata.images.chains')}
          description={t('admin.metadata.images.chainsIntro')}
        />
        {CHAIN_KINDS.map((kind) => (
          <SettingRow
            key={kind}
            label={t('admin.metadata.images.chainOf', {
              kind: t(`admin.metadata.images.kind.${kind}`),
            })}
          >
            <LanguageRanks
              value={images.langs[kind]}
              onChange={(chain) => setChain(kind, chain)}
              known={known}
              allowNoText
              disabled={!imagesEditable}
            />
          </SettingRow>
        ))}
        {CHAIN_KINDS.map((kind) => fieldError(`imageLangs.${kind}`))
          .filter(Boolean)
          .map((error) => (
            <Notice key={error} tone="danger">
              {error}
            </Notice>
          ))}
      </AreaPanel>
    );
  } else if (area === 'quality' && serverDraft) {
    detail = (
      <AreaPanel
        title={t('admin.metadata.quality.title')}
        intro={t('admin.metadata.quality.intro')}
      >
        <div className={styles.fields}>
          <RadioGroup
            name="minPoster"
            label={t('admin.metadata.quality.minPoster')}
            value={String(serverDraft.imageMinWidth.poster)}
            onChange={(value) =>
              setServer((values) => ({
                ...values,
                imageMinWidth: { ...values.imageMinWidth, poster: Number(value) },
              }))
            }
            options={POSTER_WIDTHS.map((width) => ({
              value: String(width),
              label: widthLabel(width),
            }))}
          />
          <RadioGroup
            name="minBackdrop"
            label={t('admin.metadata.quality.minBackdrop')}
            value={String(serverDraft.imageMinWidth.backdrop)}
            onChange={(value) =>
              setServer((values) => ({
                ...values,
                imageMinWidth: { ...values.imageMinWidth, backdrop: Number(value) },
              }))
            }
            options={BACKDROP_WIDTHS.map((width) => ({
              value: String(width),
              label: widthLabel(width),
            }))}
          />
        </div>
        <SettingRow
          label={t('admin.metadata.quality.byVote')}
          help={`${t('admin.metadata.quality.byVoteHint')} ${t('admin.metadata.quality.fallback')}`}
        >
          <Switch
            isSelected={serverDraft.imageByVote}
            onChange={(on) => setServer((values) => ({ ...values, imageByVote: on }))}
          >
            <VisuallyHidden>{t('admin.metadata.quality.byVote')}</VisuallyHidden>
          </Switch>
        </SettingRow>
      </AreaPanel>
    );
  } else if (area === 'sources' && serverDraft && settings.data) {
    detail = (
      <AreaPanel
        title={t('admin.metadata.sources.title')}
        intro={t('admin.metadata.sources.intro')}
      >
        <div>
          <SettingRow
            label={t('admin.metadata.sources.tmdbAdult')}
            help={t('admin.metadata.sources.tmdbAdultHint')}
          >
            <Switch
              isSelected={serverDraft.tmdbIncludeAdult}
              onChange={(on) => setServer((values) => ({ ...values, tmdbIncludeAdult: on }))}
            >
              <VisuallyHidden>{t('admin.metadata.sources.tmdbAdult')}</VisuallyHidden>
            </Switch>
          </SettingRow>
          <SettingRow
            label={t('admin.metadata.sources.local')}
            help={`${t('admin.metadata.sources.localHint')} ${t('admin.metadata.sources.localCount', { count: settings.data.localImageTitles })}.`}
          >
            <Switch
              isSelected={serverDraft.localImages}
              onChange={(on) => setServer((values) => ({ ...values, localImages: on }))}
            >
              <VisuallyHidden>{t('admin.metadata.sources.local')}</VisuallyHidden>
            </Switch>
          </SettingRow>
          <SettingRow
            label={t('admin.metadata.sources.fanart')}
            help={t('admin.metadata.sources.fanartHint')}
          >
            <TextLink href={paths.adminIntegrations}>{t('admin.metadata.links.keys')}</TextLink>
            <Switch
              isSelected={serverDraft.fanartEnabled}
              onChange={(on) => setServer((values) => ({ ...values, fanartEnabled: on }))}
            >
              <VisuallyHidden>{t('admin.metadata.sources.fanart')}</VisuallyHidden>
            </Switch>
          </SettingRow>
        </div>
        <Text tone="secondary" variant="caption">
          {t('admin.metadata.sources.localSmall')}
        </Text>
      </AreaPanel>
    );
  } else if (area === 'rating' && serverDraft) {
    detail = (
      <AreaPanel title={t('admin.metadata.rating.title')} intro={t('admin.metadata.rating.intro')}>
        <RadioGroup
          name="ratingCountry"
          label={t('admin.metadata.rating.country')}
          value={serverDraft.ratingCountry}
          onChange={(value) => setServer((values) => ({ ...values, ratingCountry: value }))}
          options={COUNTRIES.map((country: Country) => ({
            value: country,
            label: t(`admin.metadata.rating.countries.${country}`),
          }))}
        />
        {ratingCountryError && <Notice tone="danger">{ratingCountryError}</Notice>}
        <Text tone="secondary" variant="caption">
          {t('admin.metadata.rating.hint')}
        </Text>
      </AreaPanel>
    );
  } else if (area === 'apply' && settings.data) {
    detail = (
      <ApplySection
        libraries={videoLibraries}
        titlesOf={(id) => metaQueryOf(String(id))?.data?.affectedTitles}
        settings={settings.data}
        paused={unsaved.pausePolling}
      />
    );
  }

  const ready = allowed && settings.data && libraries.data;
  const loadError = settings.error ?? libraries.error;

  return (
    <AdminPage
      title={t('admin.metadata.title')}
      help={t('admin.metadata.intro')}
      titleRef={titleRef}
      actions={
        ready ? (
          <SelectField
            label={t('admin.metadata.scope.label')}
            value={scope}
            onChange={selectScope}
            options={scopeIds.map((id) => ({
              value: id,
              label: drafts.isDirty(id)
                ? t('admin.metadata.scope.changed', { name: scopeName(id) })
                : scopeName(id),
            }))}
          />
        ) : undefined
      }
    >
      {!allowed ? (
        <Notice title={t('admin.access.notAllowed.title')}>
          {t('admin.access.notAllowed.body')}
        </Notice>
      ) : loadError ? (
        <Notice
          tone="danger"
          title={problemMessage(t, loadError)}
          actions={
            <AriaButton
              icon="refresh"
              onPress={() => {
                void settings.refetch();
                void libraries.refetch();
              }}
            >
              {t('common.retry')}
            </AriaButton>
          }
        />
      ) : !ready ? (
        <Skeleton shape="block" />
      ) : (
        <>
          <MasterDetail
            label={t('admin.metadata.areas.label')}
            items={areaItems}
            selectedId={area}
            onSelect={selectArea}
          >
            {detail}
          </MasterDetail>
          <Text tone="secondary" variant="caption">
            <TextLink href={paths.adminLibraries}>{t('admin.metadata.links.roots')}</TextLink>{' '}
            <TextLink href={paths.adminJobs}>{t('admin.metadata.links.jobs')}</TextLink>
          </Text>
          <div className={styles.save}>
            <SaveBar
              dirty={drafts.dirty}
              saving={saving}
              onSave={() => void onSave()}
              onDiscard={() => setDiscardOpen(true)}
            />
            {failure && errors ? (
              <Notice tone="danger">
                {t('admin.metadata.save.failed', {
                  name: scopeName(failure.scope),
                  reason: errors.form ?? Object.values(errors.fields)[0] ?? '',
                })}
              </Notice>
            ) : null}
            <div role="status" className={styles.live}>
              {savedMessages.map((message) => (
                <Notice key={message.scope} live={false}>
                  {message.text}
                </Notice>
              ))}
            </div>
          </div>
          <ConfirmDialog
            isOpen={discardOpen}
            onOpenChange={setDiscardOpen}
            title={t('admin.metadata.discard.title')}
            body={t('admin.metadata.discard.body', { names: listOf(dirtyScopes.map(scopeName)) })}
            confirmLabel={t('admin.metadata.discard.confirm')}
            cancelLabel={t('common.cancel')}
            tone="danger"
            defaultFocus="cancel"
            onConfirm={onDiscard}
          />
          <ConfirmDialog {...unsaved.dialog} />
        </>
      )}
    </AdminPage>
  );
}
