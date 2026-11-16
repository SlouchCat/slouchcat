import type {
  ImageLangs,
  Library,
  MetadataProviderInfo,
  MetadataSettings,
  MetadataSettingsUpdate,
} from '@slouchcat/api-client';
import type { TFunction } from 'i18next';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
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
import {
  ButtonLink,
  Chip,
  Notice,
  Panel,
  SelectField,
  Skeleton,
  Text,
  TextLink,
  VisuallyHidden,
} from '@/ui';
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
  StatusRegion,
  Switch,
  useDrafts,
  useSearchSelection,
  useStatusMessage,
  useUnsavedChanges,
  type MasterDetailItem,
} from '@/ui/aria';
import { serviceHref } from './integrations/services';
import {
  applyMetadata,
  jobTypesQuery,
  librariesQuery,
  metadataProvidersQuery,
  metadataSettingsQuery,
  updateMetadataSettings,
} from './queries';
import { ratingCountryOptions, ratingStepsText } from '../shared/choices';
import styles from './metadata/Metadata.module.css';
import { intlLocale } from '@/i18n';

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

/** The YouTube quota: 10'000 units a day, a search costs 100 (ADR 0006). */
const YOUTUBE_SEARCHES_PER_DAY = 100;

/** The draft id of the default, the only settings this page edits (concept 24 3.4). */
const SERVER = 'server';

/** The settings areas of the list (Met-B). */
const SERVER_AREAS = ['languages', 'images', 'quality', 'sources', 'rating', 'apply'] as const;
type Area = (typeof SERVER_AREAS)[number];

/** The draft of the server default: everything `PUT` takes. */
type ServerValues = MetadataSettingsUpdate;

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
    () => new Intl.DisplayNames([intlLocale(i18n.language)], { type: 'language' }),
    [i18n.language],
  );
  return (code: string) => (code ? (names.of(code) ?? code) : '');
}

/**
 * An ordered list of language codes as fixed places (concept 12, 10.2): «1. Sprache», «2.
 * Sprache» … each with a select of every known language; picking one that sits on another place
 * swaps the two. «Weitere Sprache» appends a place, «×» removes one, except the last one; a chain
 * offers "" (no text). No list has to hold German or English (concept 22 F-7, ADR 0033).
 */
export function LanguageRanks({
  value,
  onChange,
  known,
  allowNoText,
  disabled,
}: {
  value: string[];
  onChange: (value: string[]) => void;
  known: readonly string[];
  allowNoText?: boolean;
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

/**
 * The area "Anwenden": what, where (every folder or one video folder, concept 24 3.3), the button,
 * the waiting children of the jobs page.
 */
function ApplySection({
  libraries,
  settings,
  paused,
}: {
  libraries: Library[];
  settings: MetadataSettings;
  paused: boolean;
}) {
  const { t, i18n } = useTranslation();
  const queryClient = useQueryClient();
  const [what, setWhat] = useState<'both' | 'texts' | 'images'>('both');
  const [where, setWhere] = useState('all');
  const [message, setMessage] = useStatusMessage();
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
            ...libraries.flatMap((library) =>
              library.roots
                .filter((root) => root.content === 'movie' || root.content === 'series')
                .map((root) => ({
                  value: String(root.id),
                  label: `${library.name}: ${root.path}`,
                })),
            ),
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
              ...(where === 'all' ? {} : { rootIds: [Number(where)] }),
            });
          }}
        >
          {run.isPending ? t('admin.metadata.apply.running') : t('admin.metadata.apply.run')}
        </AriaButton>
        <TextLink href={paths.adminJobs}>{t('admin.metadata.apply.jobsLink')}</TextLink>
      </div>
      {run.error ? <Notice tone="danger">{problemMessage(t, run.error)}</Notice> : null}
      <StatusRegion message={!run.error ? message : null} />
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
                  intlLocale(i18n.language),
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
 * The page "Metadaten" (concept 10, ADR 0018; layout concept 12 Met-B): the default settings, which
 * every folder without settings of its own follows (concept 24 3.4); the settings areas in a list
 * on the left (`?item=`), the selected area on the right. A folder's own languages and images are
 * part of the folder. Unsaved changes are kept while the area changes (useDrafts, S-17); the save
 * bar stores them. `?library=` of the earlier per-library scope leaves the address.
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

  const originalOf = (id: string): ServerValues | undefined =>
    id === SERVER && settings.data ? serverValues(settings.data) : undefined;
  const drafts = useDrafts<ServerValues>(originalOf);
  const unsaved = useUnsavedChanges(drafts.dirty);
  // The drafts of the last render, for a save that is still waiting for the server: what was
  // typed meanwhile must not be dropped with the saved values.
  const latestDrafts = useRef(drafts);
  useLayoutEffect(() => {
    latestDrafts.current = drafts;
  });

  // The per-library scope is gone (concept 24 3.4): an old `?library=` leaves the address.
  const { library: stale } = route.useSearch();
  useEffect(() => {
    if (stale !== undefined)
      void navigate({
        to: '.',
        replace: true,
        search: (previous) => ({ ...previous, library: undefined }),
      });
  }, [stale, navigate]);
  const [area, selectArea] = useSearchSelection('item', SERVER_AREAS, 'languages');

  const defaultName = t('admin.metadata.default');
  const listOf = (names: string[]) =>
    new Intl.ListFormat(intlLocale(i18n.language), { type: 'conjunction' }).format(names);

  const [saving, setSaving] = useState(false);
  const [savedMessage, setSavedMessage] = useState<string | null>(null);
  const [failure, setFailure] = useState<{ error: unknown } | null>(null);
  const [discardOpen, setDiscardOpen] = useState(false);
  // A new rating country reads every rating anew (concept 22 F-16): ask first when there are titles.
  const [countryConfirmOpen, setCountryConfirmOpen] = useState(false);
  // Input that changed while the save waited for the server, to be put back as a draft once the
  // page sees the saved values as the original (L-5): a field set back to the old value during the
  // save would otherwise turn into the saved one.
  const [restores, setRestores] = useState<{ value: ServerValues; original: ServerValues }[]>([]);
  const due = restores.filter((entry) => same(originalOf(SERVER), entry.original));
  if (due.length > 0) {
    setRestores(restores.filter((entry) => !due.includes(entry)));
    for (const entry of due) drafts.set(SERVER, entry.value);
  }
  const errors = failure ? formErrors(t, failure.error) : null;
  const fieldError = (field: string) => errors?.fields?.[field];

  const onSave = async () => {
    const values = drafts.valueOf(SERVER);
    if (!values) return;
    setSaving(true);
    setSavedMessage(null);
    setFailure(null);
    try {
      const stored = await updateMetadataSettings(values);
      const latest = latestDrafts.current.valueOf(SERVER);
      queryClient.setQueryData(metadataSettingsQuery.queryKey, stored);
      setSavedMessage(
        t('admin.metadata.save.savedScope', { name: defaultName, count: stored.affectedTitles }),
      );
      // The draft is spent only while it is still what was sent; what was typed meanwhile (also a
      // field set back to its old value) stays as a draft against the saved values.
      if (latest === undefined || same(latest, values)) drafts.discard(SERVER);
      else
        setRestores((current) => [...current, { value: latest, original: serverValues(stored) }]);
      await queryClient.invalidateQueries({ queryKey: ['session'] });
    } catch (error) {
      setFailure({ error });
    }
    setSaving(false);
  };
  const onDiscard = () => {
    drafts.discardAll();
    setRestores([]);
    setSavedMessage(null);
    setFailure(null);
    // «Verwerfen» went with the changes; the focus goes to the page head (ALL-2).
    requestAnimationFrame(() => titleRef.current?.focus());
  };

  const serverDraft = drafts.valueOf(SERVER);
  const known = settings.data?.knownLanguages ?? [];
  const ratingCountries = useMemo(
    () => ratingCountryOptions(t, intlLocale(i18n.language), settings.data?.ratingSystems ?? []),
    [t, i18n.language, settings.data?.ratingSystems],
  );

  const setServer = (change: (values: ServerValues) => ServerValues) =>
    drafts.set(SERVER, (values) => change(values));

  const textLangs = serverDraft?.textLangs;
  const images: ImagesView | undefined = serverDraft && {
    langs: serverDraft.imageLangs,
    kinds: switchable(serverDraft.imageKinds),
  };

  const setTextLangs = (langs: string[]) =>
    setServer((values) => ({ ...values, textLangs: langs }));
  const setImages = (change: (view: ImagesView) => ImagesView) =>
    setServer((values) => {
      const next = change({ langs: values.imageLangs, kinds: switchable(values.imageKinds) });
      return { ...values, imageLangs: next.langs, imageKinds: next.kinds };
    });
  const setChain = (kind: ChainKind, chain: string[]) =>
    setImages((view) => ({ ...view, langs: { ...view.langs, [kind]: chain } }));

  /** Whether an area differs from what the server holds. */
  const areaDirty = (id: Area): boolean => {
    if (id === 'apply' || !drafts.isDirty(SERVER)) return false;
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
      summary =
        ratingCountries.find((option) => option.value === serverDraft.ratingCountry)?.label ??
        serverDraft.ratingCountry;
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
  const areaItems: MasterDetailItem[] = SERVER_AREAS.map((id) => ({
    id,
    label: areaLabels[id],
    description: summaryOf(id),
  }));

  const youtubeTitles = textLangs
    ? Math.floor(
        YOUTUBE_SEARCHES_PER_DAY / Math.max(1, new Set(textLangs.map((c) => c.split('-')[0])).size),
      )
    : 0;
  const textLangsError = fieldError('textLangs');
  const ratingCountryError = fieldError('ratingCountry');

  let detail: ReactNode = null;
  if (area === 'languages' && textLangs) {
    detail = (
      <AreaPanel
        title={t('admin.metadata.languages.title')}
        intro={t('admin.metadata.languages.intro')}
      >
        <SettingRow label={t('admin.metadata.languages.list')}>
          <LanguageRanks value={textLangs} onChange={setTextLangs} known={known} />
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
      <AreaPanel title={t('admin.metadata.images.title')} intro={t('admin.metadata.images.intro')}>
        <div>
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
            <TextLink href={serviceHref('keys')}>{t('admin.metadata.links.keys')}</TextLink>
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
    const chosen = settings.data?.ratingSystems.find(
      (rs) => rs.country === serverDraft.ratingCountry,
    );
    detail = (
      <AreaPanel title={t('admin.metadata.rating.title')} intro={t('admin.metadata.rating.intro')}>
        <SettingRow label={t('admin.metadata.rating.country')}>
          <SelectField
            label={t('admin.metadata.rating.country')}
            hideLabel
            value={serverDraft.ratingCountry}
            onChange={(value) => setServer((values) => ({ ...values, ratingCountry: value }))}
            options={ratingCountries}
          />
        </SettingRow>
        {chosen && (
          <Text tone="secondary">
            {t('admin.metadata.rating.steps', { steps: ratingStepsText(t, chosen) })}
          </Text>
        )}
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
          {/* Audiobooks take their sources from a page of their own (concept 19 D6.6); a caption
              link under the areas went unnoticed, so the entry stands above them. */}
          {libraries.data?.some((library) => library.kind === 'audiobooks') && (
            <Notice
              live={false}
              title={t('admin.metadata.audiobooks.title')}
              actions={
                <ButtonLink href={paths.adminAudiobookSources} icon="book">
                  {t('admin.metadata.audiobooks.open')}
                </ButtonLink>
              }
            >
              {t('admin.metadata.audiobooks.body')}
            </Notice>
          )}
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
          {/* A direct child of the page's column, so it sticks to the bottom while the areas
              scroll (a wrapper only as tall as the bar would hold it in place). */}
          <SaveBar
            dirty={drafts.dirty}
            saving={saving}
            onSave={() => {
              const country = drafts.valueOf(SERVER);
              if (
                settings.data &&
                settings.data.affectedTitles > 0 &&
                country &&
                country.ratingCountry !== settings.data.ratingCountry
              )
                setCountryConfirmOpen(true);
              else void onSave();
            }}
            onDiscard={() => setDiscardOpen(true)}
          />
          {failure && errors ? (
            <Notice tone="danger">
              {t('admin.metadata.save.failed', {
                name: defaultName,
                reason: errors.form ?? Object.values(errors.fields)[0] ?? '',
              })}
            </Notice>
          ) : null}
          <StatusRegion>
            {savedMessage && <Notice live={false}>{savedMessage}</Notice>}
          </StatusRegion>
          <ConfirmDialog
            isOpen={discardOpen}
            onOpenChange={setDiscardOpen}
            title={t('admin.metadata.discard.title')}
            body={t('admin.metadata.discard.body', { names: listOf([defaultName]) })}
            confirmLabel={t('admin.metadata.discard.confirm')}
            cancelLabel={t('common.cancel')}
            tone="danger"
            defaultFocus="cancel"
            onConfirm={onDiscard}
          />
          <ConfirmDialog
            isOpen={countryConfirmOpen}
            onOpenChange={setCountryConfirmOpen}
            title={t('admin.metadata.rating.confirm.title')}
            body={t('admin.metadata.rating.confirm.body')}
            confirmLabel={t('admin.metadata.rating.confirm.confirm')}
            cancelLabel={t('common.cancel')}
            defaultFocus="cancel"
            onConfirm={() => void onSave()}
          />
          <ConfirmDialog {...unsaved.dialog} />
        </>
      )}
    </AdminPage>
  );
}
