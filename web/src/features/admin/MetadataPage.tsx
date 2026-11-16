import type {
  ImageLangs,
  Library,
  MetadataSettings,
  MetadataSettingsUpdate,
} from '@slouchcat/api-client';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import {
  useEffect,
  useLayoutEffect,
  useMemo,
  useRef,
  useState,
  type ReactNode,
  type RefObject,
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
  DisclosureGroup,
  RadioGroup,
  SaveBar,
  SectionHeader,
  SettingRow,
  StatusRegion,
  Switch,
  afterDialogsClosed,
  focusTitleIfLost,
  useDrafts,
  useStatusMessage,
  useUnsavedChanges,
} from '@/ui/aria';
import { serviceHref } from './integrations/services';
import { LibraryFoldersDialog } from './libraries/LibraryFoldersDialog';
import {
  applyMetadata,
  jobTypesQuery,
  librariesQuery,
  metadataProvidersForQuery,
  metadataSettingsQuery,
  updateMetadataSettings,
} from './queries';
import { ratingCountryOptions, ratingStepsText } from '../shared/choices';
import {
  LanguageRanks,
  chainText,
  unknownLanguages,
  useFolderMetadataText,
  useLanguageName,
  useListOf,
  youtubeTitles,
} from './metadata/languages';
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

/** The draft id of the default, the only settings this page edits (concept 24 3.4). */
const SERVER = 'server';

/** The fold sections of the page, top to bottom (concept 24 3.4); `?item=` opens one. */
const SECTIONS = [
  'languages',
  'images',
  'quality',
  'sources',
  'rating',
  'apply',
  'folders',
] as const;
type Section = (typeof SECTIONS)[number];
const isSection = (value: unknown): value is Section =>
  typeof value === 'string' && (SECTIONS as readonly string[]).includes(value);

/** The draft of the server default: everything `PUT` takes. */
type ServerValues = MetadataSettingsUpdate;

/** What «Anwenden» brings to the titles. */
type ApplyWhat = 'both' | 'texts' | 'images';

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

/** The fields of the default that each section edits, for its «geändert». */
const SERVER_FIELDS: Record<Exclude<Section, 'apply' | 'folders'>, (keyof ServerValues)[]> = {
  languages: ['textLangs'],
  images: ['imageLangs', 'imageKinds'],
  quality: ['imageMinWidth', 'imageByVote'],
  sources: ['tmdbIncludeAdult', 'localImages', 'fanartEnabled'],
  rating: ['ratingCountry'],
};

/** The video folders of the libraries, each with its library (concept 24 3.1). */
const videoFoldersOf = (libraries: readonly Library[]) =>
  libraries
    .filter((library) => library.kind === 'video')
    .flatMap((library) =>
      library.roots
        .filter((root) => root.content === 'movie' || root.content === 'series')
        .map((root) => ({ library, root })),
    );

/** The body of one fold section: the area's intro, then its fields. */
function SectionBody({ intro, children }: { intro: string; children: ReactNode }) {
  return (
    <div className={styles.body}>
      <Text tone="secondary">{intro}</Text>
      {children}
    </div>
  );
}

/**
 * The section «Anwenden»: what and where (every folder or one video folder with the titles whose
 * home folder it is, concept 24 3.3), the button, the waiting children of the jobs page and the
 * last run. What and where are the page's, so the closed header can name them.
 */
function ApplySection({
  libraries,
  paused,
  what,
  onWhatChange,
  where,
  onWhereChange,
  whatOptions,
  whereOptions,
}: {
  libraries: readonly Library[];
  paused: boolean;
  what: ApplyWhat;
  onWhatChange: (what: ApplyWhat) => void;
  where: string;
  onWhereChange: (where: string) => void;
  whatOptions: { value: ApplyWhat; label: string }[];
  whereOptions: { value: string; label: string }[];
}) {
  const { t, i18n } = useTranslation();
  const queryClient = useQueryClient();
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
  // A folder that left the libraries meanwhile falls back to every folder.
  const known =
    where === 'all' || videoFoldersOf(libraries).some((f) => String(f.root.id) === where);
  return (
    <SectionBody intro={t('admin.metadata.apply.intro')}>
      <div className={styles.fields}>
        <RadioGroup
          name="what"
          label={t('admin.metadata.apply.what')}
          value={what}
          onChange={(value) => onWhatChange(value as ApplyWhat)}
          options={whatOptions}
        />
        <RadioGroup
          name="where"
          label={t('admin.metadata.apply.where')}
          orientation="vertical"
          value={known ? where : 'all'}
          onChange={onWhereChange}
          options={whereOptions}
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
              ...(where === 'all' || !known ? {} : { rootIds: [Number(where)] }),
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
    </SectionBody>
  );
}

/**
 * The section «Ordner»: every video folder with its languages and images (default or its own) and
 * the way to its library's folder dialog, where «Ändern» edits them (concept 24 3.4, stage 2).
 */
function FoldersSection({
  libraries,
  titleRef,
}: {
  libraries: readonly Library[];
  titleRef: RefObject<HTMLHeadingElement | null>;
}) {
  const { t } = useTranslation();
  const queryClient = useQueryClient();
  const text = useFolderMetadataText();
  const [openId, setOpenId] = useState<number | null>(null);
  const [open, setOpen] = useState(false);
  const library = libraries.find((entry) => entry.id === openId);
  const folders = videoFoldersOf(libraries);

  const changed = async (updated: Library) => {
    queryClient.setQueryData<Library[]>(librariesQuery.queryKey, (list) =>
      list?.map((entry) => (entry.id === updated.id ? updated : entry)),
    );
    await queryClient.invalidateQueries({ queryKey: metadataSettingsQuery.queryKey });
    await afterDialogsClosed();
    focusTitleIfLost(titleRef);
  };

  return (
    <SectionBody intro={t('admin.metadata.folders.intro')}>
      {folders.length === 0 ? (
        <Text tone="secondary">{t('admin.metadata.folders.empty')}</Text>
      ) : (
        <ul className={styles.folders}>
          {folders.map(({ library: owner, root }) => (
            <li key={root.id} className={styles.folder}>
              <code className={styles.path}>{root.path}</code>
              <span className={styles.folderMeta}>
                <Text tone="secondary" as="span">
                  {owner.name}
                </Text>
                <Chip tone={root.textLangs === null ? 'outline' : 'info'}>{text.langs(root)}</Chip>
                <Chip tone={root.images === null ? 'outline' : 'info'}>{text.images(root)}</Chip>
              </span>
              <AriaButton
                icon="folder"
                aria-label={t('admin.metadata.folders.openLabel', {
                  name: owner.name,
                  path: root.path,
                })}
                onPress={() => {
                  setOpenId(owner.id);
                  setOpen(true);
                }}
              >
                {t('admin.libraries.openFolders')}
              </AriaButton>
            </li>
          ))}
        </ul>
      )}
      <Text tone="secondary" variant="caption">
        {t('admin.metadata.folders.audio')}
      </Text>
      {library && (
        <LibraryFoldersDialog
          key={library.id}
          library={library}
          isOpen={open}
          onOpenChange={setOpen}
          onChanged={(updated) => void changed(updated)}
        />
      )}
    </SectionBody>
  );
}

/**
 * The page "Metadaten" (concept 10, ADR 0018): the default settings, which every folder without
 * settings of its own follows (concept 24 3.4). The areas are fold sections, all closed, each
 * header with its summary and «geändert» while it holds unsaved changes; `?item=` opens the named
 * section (concept 24 5, exception 1). Unsaved changes stay while a section folds away (useDrafts,
 * S-17); the save bar stores them. `?library=` of the earlier per-library scope leaves the address.
 */
export function MetadataPage() {
  const { t, i18n } = useTranslation();
  useDocumentTitle(t('admin.metadata.title'));
  const session = useActiveSession();
  const allowed = session.account.role === 'admin' && session.profile.isPrimary;
  const queryClient = useQueryClient();
  const nameOf = useLanguageName();
  const listOf = useListOf();
  const folderText = useFolderMetadataText();
  const navigate = route.useNavigate();
  const titleRef = useRef<HTMLHeadingElement>(null);

  const settings = useQuery({ ...metadataSettingsQuery, enabled: allowed });
  const libraries = useQuery({ ...librariesQuery, enabled: allowed });

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
  const { library: stale, item: askedItem } = route.useSearch();
  useEffect(() => {
    if (stale !== undefined)
      void navigate({
        to: '.',
        replace: true,
        search: (previous) => ({ ...previous, library: undefined }),
      });
  }, [stale, navigate]);

  // Every section starts closed (concept 24 P-3) and stays as the reader leaves it; `?item=`
  // opens its section once, adjusted while rendering so it is open in the first render.
  const [expanded, setExpanded] = useState<ReadonlySet<Section>>(() => new Set());
  const [openedItem, setOpenedItem] = useState<Section | undefined>(undefined);
  if (isSection(askedItem) && openedItem !== askedItem) {
    setOpenedItem(askedItem);
    setExpanded((current) => new Set(current).add(askedItem));
  } else if (!isSection(askedItem) && openedItem !== undefined) {
    setOpenedItem(undefined);
  }
  const toggle = (id: Section, open: boolean) =>
    setExpanded((current) => {
      const next = new Set(current);
      if (open) next.add(id);
      else next.delete(id);
      return next;
    });

  const [applyWhat, setApplyWhat] = useState<ApplyWhat>('both');
  const [applyWhere, setApplyWhere] = useState('all');

  const defaultName = t('admin.metadata.default');

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
      // A field error opens the section that holds the field (concept 24 5, exception 3).
      const fields = Object.keys(formErrors(t, error).fields);
      setExpanded((current) => {
        const next = new Set(current);
        for (const [id, names] of Object.entries(SERVER_FIELDS))
          if (fields.some((field) => names.some((name) => field.split('.')[0] === name)))
            next.add(id as Section);
        return next;
      });
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
  // Which source does not know a language: asked for the list as edited, not the saved one.
  const providers = useQuery({
    ...metadataProvidersForQuery(textLangs ?? []),
    enabled: allowed && textLangs !== undefined && textLangs.length > 0,
  });
  const providersOfList = providers.isPlaceholderData ? undefined : providers.data;
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

  /** Whether a section differs from what the server holds. */
  const sectionDirty = (id: Section): boolean => {
    if (id === 'apply' || id === 'folders' || !drafts.isDirty(SERVER)) return false;
    if (!settings.data || !serverDraft) return false;
    const original = serverValues(settings.data);
    return SERVER_FIELDS[id].some((field) => !same(serverDraft[field], original[field]));
  };

  const videoFolders = videoFoldersOf(libraries.data ?? []);
  const ownFolders = videoFolders.filter(({ root }) => folderText.isOwn(root)).length;
  const applyWhatOptions: { value: ApplyWhat; label: string }[] = [
    { value: 'both', label: t('admin.metadata.apply.both') },
    { value: 'texts', label: t('admin.metadata.apply.texts') },
    { value: 'images', label: t('admin.metadata.apply.images') },
  ];
  const withTitles = (name: string, count: number) =>
    `${name} (${t('admin.metadata.apply.titles', { count })})`;
  const applyWhereOptions = [
    {
      value: 'all',
      label: withTitles(t('admin.metadata.apply.all'), settings.data?.affectedTitles ?? 0),
    },
    ...videoFolders.map(({ library, root }) => ({
      value: String(root.id),
      label: withTitles(`${library.name}: ${root.path}`, root.homeTitleCount),
    })),
  ];

  const widthLabel = (width: number) =>
    width === 0 ? t('admin.metadata.quality.any') : t('admin.metadata.quality.px', { width });
  const summaryOf = (id: Section): string => {
    if (id === 'languages' && textLangs) return chainText(t, nameOf, textLangs);
    if (id === 'images' && images)
      return listOf([
        t('admin.metadata.images.kind.poster'),
        t('admin.metadata.images.kind.backdrop'),
        ...images.kinds.map((kind) => t(`admin.metadata.images.kind.${kind}`)),
      ]);
    if (id === 'quality' && serverDraft)
      return t('admin.metadata.areas.quality', {
        poster: widthLabel(serverDraft.imageMinWidth.poster),
        backdrop: widthLabel(serverDraft.imageMinWidth.backdrop),
      });
    if (id === 'sources' && serverDraft) {
      const on = [
        serverDraft.localImages ? t('admin.metadata.areas.sources.local') : '',
        serverDraft.fanartEnabled ? t('admin.metadata.areas.sources.fanart') : '',
        serverDraft.tmdbIncludeAdult ? t('admin.metadata.areas.sources.tmdbAdult') : '',
      ].filter(Boolean);
      return on.length > 0 ? listOf(on) : t('admin.metadata.areas.sources.none');
    }
    if (id === 'rating' && serverDraft)
      return (
        ratingCountries.find((option) => option.value === serverDraft.ratingCountry)?.label ??
        serverDraft.ratingCountry
      );
    if (id === 'apply') {
      const what = applyWhatOptions.find((option) => option.value === applyWhat)?.label ?? '';
      const where =
        applyWhereOptions.find((option) => option.value === applyWhere)?.label ??
        applyWhereOptions[0]?.label ??
        '';
      return t('admin.metadata.areas.apply', { what, where });
    }
    if (id === 'folders')
      return t('admin.metadata.areas.folders', {
        follow: t('admin.metadata.areas.foldersFollow', {
          count: videoFolders.length - ownFolders,
        }),
        own: t('admin.metadata.areas.foldersOwn', { count: ownFolders }),
      });
    return '';
  };
  const titles: Record<Section, string> = {
    languages: t('admin.metadata.languages.title'),
    images: t('admin.metadata.images.title'),
    quality: t('admin.metadata.quality.title'),
    sources: t('admin.metadata.sources.title'),
    rating: t('admin.metadata.rating.title'),
    apply: t('admin.metadata.apply.short'),
    folders: t('admin.metadata.folders.title'),
  };

  const textLangsError = fieldError('textLangs');
  const ratingCountryError = fieldError('ratingCountry');

  const contentOf = (id: Section): ReactNode => {
    if (id === 'languages' && textLangs)
      return (
        <SectionBody intro={t('admin.metadata.languages.intro')}>
          <SettingRow label={t('admin.metadata.languages.list')}>
            <LanguageRanks value={textLangs} onChange={setTextLangs} known={known} />
          </SettingRow>
          {textLangsError && <Notice tone="danger">{textLangsError}</Notice>}
          <Text tone="secondary" variant="caption">
            {t('admin.metadata.languages.fallback')}
          </Text>
          <Text tone="secondary" variant="caption">
            {[
              // The previous list's answer stays while this one loads: no hint until it is there.
              ...(providersOfList
                ? unknownLanguages(t, nameOf, listOf, providersOfList, textLangs)
                : []),
              t('admin.metadata.languages.youtube', {
                count: textLangs.length,
                titles: youtubeTitles(textLangs),
              }),
            ].join(' ')}
          </Text>
        </SectionBody>
      );
    if (id === 'images' && images)
      return (
        <SectionBody intro={t('admin.metadata.images.intro')}>
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
        </SectionBody>
      );
    if (id === 'quality' && serverDraft)
      return (
        <SectionBody intro={t('admin.metadata.quality.intro')}>
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
        </SectionBody>
      );
    if (id === 'sources' && serverDraft && settings.data)
      return (
        <SectionBody intro={t('admin.metadata.sources.intro')}>
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
        </SectionBody>
      );
    if (id === 'rating' && serverDraft) {
      const chosen = settings.data?.ratingSystems.find(
        (rs) => rs.country === serverDraft.ratingCountry,
      );
      return (
        <SectionBody intro={t('admin.metadata.rating.intro')}>
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
        </SectionBody>
      );
    }
    if (id === 'apply' && settings.data && libraries.data)
      return (
        <ApplySection
          libraries={libraries.data}
          // Folded content stays mounted: the jobs are polled only while «Anwenden» is open.
          paused={unsaved.pausePolling || !expanded.has('apply')}
          what={applyWhat}
          onWhatChange={setApplyWhat}
          where={applyWhere}
          onWhereChange={setApplyWhere}
          whatOptions={applyWhatOptions}
          whereOptions={applyWhereOptions}
        />
      );
    if (id === 'folders' && libraries.data)
      return <FoldersSection libraries={libraries.data} titleRef={titleRef} />;
    return null;
  };

  const ready = allowed && settings.data && libraries.data;
  const loadError = settings.error ?? libraries.error;

  return (
    <AdminPage
      title={t('admin.metadata.title')}
      help={t('admin.metadata.introDefault')}
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
          <Panel>
            <div className={styles.sections}>
              {SECTIONS.map((id) => (
                <div key={id} className={styles.section}>
                  <DisclosureGroup
                    level={2}
                    title={titles[id]}
                    meta={
                      <>
                        <span className={styles.summary}>{summaryOf(id)}</span>
                        {sectionDirty(id) && (
                          <Chip tone="warning">{t('admin.metadata.changed')}</Chip>
                        )}
                      </>
                    }
                    isExpanded={expanded.has(id)}
                    onExpandedChange={(open) => toggle(id, open)}
                  >
                    {contentOf(id)}
                  </DisclosureGroup>
                </div>
              ))}
            </div>
          </Panel>
          <Text tone="secondary" variant="caption">
            <TextLink href={paths.adminLibraries}>{t('admin.metadata.links.roots')}</TextLink>{' '}
            <TextLink href={paths.adminJobs}>{t('admin.metadata.links.jobs')}</TextLink>
          </Text>
          {/* A direct child of the page's column, so it sticks to the bottom while the sections
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
