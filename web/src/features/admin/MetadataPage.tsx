import type {
  ImageLangs,
  Library,
  LibraryImages,
  LibraryMetadata,
  LibraryMetadataUpdate,
  MetadataProviderInfo,
  MetadataSettings,
  MetadataSettingsUpdate,
} from '@slouchcat/api-client';
import type { TFunction } from 'i18next';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { useId, useMemo, useState, type ReactNode } from 'react';
import { useTranslation } from 'react-i18next';
import { getRouteApi } from '@tanstack/react-router';
import { formErrors, problemMessage } from '@/api/errors';
import { paths } from '@/app/paths';
import { useActiveSession } from '@/app/session';
import { formatDateTime } from '@/lib/format';
import { useDocumentTitle } from '@/lib/useDocumentTitle';
import { Chip, Notice, Page, Panel, SelectField, Skeleton, Text, TextLink } from '@/ui';
import { AriaButton, CheckboxGroup, RadioGroup, SaveBar, Switch } from '@/ui/aria';
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
import table from './AdminTablePage.module.css';

const route = getRouteApi('/_authed/_active/_chrome/admin/metadata');

/** The chain kinds in the order of the page. */
const CHAIN_KINDS = ['poster', 'backdrop', 'logo'] as const;
type ChainKind = (typeof CHAIN_KINDS)[number];

/** The switchable image kinds (poster and backdrop are always on, E-1). */
const SWITCHABLE = ['logo', 'season_poster', 'episode_still'] as const;
type Switchable = (typeof SWITCHABLE)[number];

const POSTER_WIDTHS = [0, 500, 1000, 1500, 2000] as const;
const BACKDROP_WIDTHS = [0, 1280, 1920, 3840] as const;
const COUNTRIES = ['DE', 'AT', 'CH', 'GB', 'US'] as const;
type Country = (typeof COUNTRIES)[number];

/** The YouTube quota: 10'000 units a day, a search costs 100 (ADR 0006). */
const YOUTUBE_SEARCHES_PER_DAY = 100;

/** The select value of the chain entry "no text" (the empty language code cannot be an option value). */
const NO_TEXT = '__none__';

/** The segments show up to five libraries; above that a select picks the scope. */
const MAX_SEGMENTS = 5;

/** The server part of the form: everything but the counts. */
type ServerValues = MetadataSettingsUpdate;

/** The library part of the form: the two switches with the values they carry. */
interface LibraryValues {
  textsDeviate: boolean;
  textLangs: string[];
  imagesDeviate: boolean;
  imageLangs: ImageLangs;
  imageKinds: LibraryImages['kinds'];
}

function serverValues(s: MetadataSettings): ServerValues {
  return {
    textLangs: [...s.textLangs],
    imageLangs: {
      poster: [...s.imageLangs.poster],
      backdrop: [...s.imageLangs.backdrop],
      logo: [...s.imageLangs.logo],
    },
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
    textsDeviate: lm.textLangs != null,
    textLangs: [...(lm.textLangs ?? lm.effective.textLangs)],
    imagesDeviate: lm.images != null,
    imageLangs: {
      poster: [...(lm.images?.langs.poster ?? lm.effective.imageLangs.poster)],
      backdrop: [...(lm.images?.langs.backdrop ?? lm.effective.imageLangs.backdrop)],
      logo: [...(lm.images?.langs.logo ?? lm.effective.imageLangs.logo)],
    },
    imageKinds: [...(lm.images?.kinds ?? lm.effective.imageKinds)],
  };
}

function libraryUpdate(v: LibraryValues): LibraryMetadataUpdate {
  return {
    textLangs: v.textsDeviate ? v.textLangs : null,
    images: v.imagesDeviate ? { langs: v.imageLangs, kinds: v.imageKinds } : null,
  };
}

/** The display name of a language code in the interface language (rule 9: no name in the code). */
function useLanguageName(): (code: string) => string {
  const { i18n } = useTranslation();
  const names = useMemo(
    () => new Intl.DisplayNames([i18n.language], { type: 'language' }),
    [i18n.language],
  );
  return (code: string) => (code ? (names.of(code) ?? code) : '');
}

function moveItem<T>(list: readonly T[], from: number, to: number): T[] {
  if (to < 0 || to >= list.length) return [...list];
  const out = [...list];
  const [item] = out.splice(from, 1);
  out.splice(to, 0, item as T);
  return out;
}

/**
 * An ordered list of language codes as chips (7.1): the position, the name, move forward or back,
 * remove; a picker adds one of the known codes. The last de/en of a text list cannot be removed
 * (E-9); a chain may hold "" (no text).
 */
function LanguageChips({
  label,
  value,
  onChange,
  known,
  allowNoText,
  protectUI,
  disabled,
}: {
  label: string;
  value: string[];
  onChange: (value: string[]) => void;
  known: readonly string[];
  allowNoText?: boolean;
  protectUI?: boolean;
  disabled?: boolean;
}) {
  const { t } = useTranslation();
  const nameOf = useLanguageName();
  const id = useId();
  const [adding, setAdding] = useState('');
  const display = (code: string) =>
    code === '' ? t('admin.metadata.images.noText') : nameOf(code);
  const uiCount = value.filter((code) => code === 'de' || code === 'en').length;
  const options = useMemo(() => {
    const rest = known.filter((code) => !value.includes(code));
    const list = rest.map((code) => ({ code, name: nameOf(code) }));
    list.sort((a, b) => a.name.localeCompare(b.name));
    return allowNoText && !value.includes('')
      ? [{ code: '', name: t('admin.metadata.images.noText') }, ...list]
      : list;
  }, [known, value, allowNoText, nameOf, t]);
  return (
    <div className={styles.block} role="group" aria-labelledby={`${id}-label`}>
      <span id={`${id}-label`} className={styles.hint}>
        {label}
      </span>
      <ol className={styles.chips}>
        {value.map((code, index) => {
          const removable =
            !disabled && !(protectUI && (code === 'de' || code === 'en') && uiCount <= 1);
          return (
            <li key={code || 'none'} className={styles.chipRow}>
              <span className={styles.chipIndex}>
                {t('admin.metadata.languages.position', { index: index + 1 })}
              </span>
              <span className={styles.chipName}>{display(code)}</span>
              <span className={styles.chipButtons}>
                <AriaButton
                  variant="ghost"
                  icon="chevronLeft"
                  isDisabled={disabled || index === 0}
                  aria-label={t('admin.metadata.languages.moveUp', { name: display(code) })}
                  onPress={() => onChange(moveItem(value, index, index - 1))}
                />
                <AriaButton
                  variant="ghost"
                  icon="chevronRight"
                  isDisabled={disabled || index === value.length - 1}
                  aria-label={t('admin.metadata.languages.moveDown', { name: display(code) })}
                  onPress={() => onChange(moveItem(value, index, index + 1))}
                />
                <AriaButton
                  variant="ghost"
                  icon="close"
                  isDisabled={!removable}
                  aria-label={t('admin.metadata.languages.remove', { name: display(code) })}
                  onPress={() => onChange(value.filter((_, i) => i !== index))}
                />
              </span>
            </li>
          );
        })}
      </ol>
      {!disabled && options.length > 0 && (
        <div className={styles.add}>
          <SelectField
            label={t('admin.metadata.languages.addLabel')}
            value={adding}
            onChange={setAdding}
            options={[
              { value: '', label: t('admin.metadata.languages.add') },
              ...options.map((option) => ({ value: option.code || NO_TEXT, label: option.name })),
            ]}
          />
          <AriaButton
            variant="secondary"
            isDisabled={adding === ''}
            onPress={() => {
              onChange([...value, adding === NO_TEXT ? '' : adding]);
              setAdding('');
            }}
          >
            {t('admin.metadata.languages.addButton')}
          </AriaButton>
        </div>
      )}
    </div>
  );
}

function Section({
  title,
  intro,
  aside,
  children,
}: {
  title: string;
  intro: string;
  aside?: ReactNode;
  children: ReactNode;
}) {
  const id = useId();
  return (
    <Panel as="section" aria-labelledby={id}>
      <div className={styles.head}>
        <div className={styles.headText}>
          <h2 id={id} className={styles.title}>
            {title}
          </h2>
          <Text tone="secondary">{intro}</Text>
        </div>
        {aside}
      </div>
      {children}
    </Panel>
  );
}

/** Which of the server's languages a provider does not know (6.1, 12.3). */
function unknownLanguages(
  t: TFunction,
  nameOf: (code: string) => string,
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
          languages: missing.map(nameOf).join(', '),
        }),
      );
    }
  }
  return lines;
}

/** The "Anwenden" section: what, where, the button, the waiting children of the jobs page. */
function ApplySection({
  libraries,
  settings,
}: {
  libraries: Library[];
  settings: MetadataSettings;
}) {
  const { t, i18n } = useTranslation();
  const queryClient = useQueryClient();
  const [what, setWhat] = useState<'both' | 'texts' | 'images'>('both');
  const [where, setWhere] = useState('all');
  const [message, setMessage] = useState<string | null>(null);
  const jobTypes = useQuery({ ...jobTypesQuery, refetchInterval: 10_000 });
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
  const video = libraries.filter((library) => library.kind === 'video');
  return (
    <Section title={t('admin.metadata.apply.title')} intro={t('admin.metadata.apply.intro')}>
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
              label: `${t('admin.metadata.apply.all')} (${t('admin.metadata.apply.titles', { count: settings.affectedTitles })})`,
            },
            ...video.map((library) => ({ value: String(library.id), label: library.name })),
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
    </Section>
  );
}

/**
 * The page "Metadaten" (concept 10, 7.1, ADR 0018): scope (server default or one library),
 * languages, images, quality, sources, rating country and the apply run; a sticky save bar. A
 * library edits only its two deviations; everything else is dimmed and read-only there.
 */
export function MetadataPage() {
  const { t } = useTranslation();
  useDocumentTitle(t('admin.metadata.title'));
  const session = useActiveSession();
  const allowed = session.account.role === 'admin' && session.profile.isPrimary;
  const queryClient = useQueryClient();
  const nameOf = useLanguageName();

  const settings = useQuery({ ...metadataSettingsQuery, enabled: allowed });
  const libraries = useQuery({ ...librariesQuery, enabled: allowed });
  const providers = useQuery({ ...metadataProvidersQuery, enabled: allowed });
  const videoLibraries = (libraries.data ?? []).filter((library) => library.kind === 'video');

  const { library: wanted } = route.useSearch();
  const [scope, setScope] = useState<string>(wanted ? String(wanted) : 'server');
  const libraryId = scope === 'server' ? undefined : Number(scope);
  const libraryMeta = useQuery({
    ...libraryMetadataQuery(libraryId ?? 0),
    enabled: allowed && libraryId !== undefined,
  });

  // The form values follow the loaded data: when a query delivers a new object (first load, a
  // save, another scope), the values are seeded from it again during the render (the React way
  // of adjusting state to a changed prop; no effect, no extra render pass).
  const [server, setServer] = useState<ServerValues | null>(null);
  const [serverSource, setServerSource] = useState<MetadataSettings | undefined>(undefined);
  const [library, setLibrary] = useState<LibraryValues | null>(null);
  const [librarySource, setLibrarySource] = useState<LibraryMetadata | undefined>(undefined);
  const [message, setMessage] = useState<string | null>(null);
  if (settings.data !== serverSource) {
    setServerSource(settings.data);
    setServer(settings.data ? serverValues(settings.data) : null);
  }
  if (libraryMeta.data !== librarySource) {
    setLibrarySource(libraryMeta.data);
    setLibrary(libraryMeta.data ? libraryValues(libraryMeta.data) : null);
  }

  const dirty =
    (settings.data &&
      server &&
      JSON.stringify(server) !== JSON.stringify(serverValues(settings.data))) ||
    (libraryMeta.data &&
      library &&
      JSON.stringify(library) !== JSON.stringify(libraryValues(libraryMeta.data)));

  const saveServer = useMutation({
    mutationFn: updateMetadataSettings,
    onSuccess: async (stored) => {
      queryClient.setQueryData(metadataSettingsQuery.queryKey, stored);
      setMessage(t('admin.metadata.save.saved', { count: stored.affectedTitles }));
      await queryClient.invalidateQueries({ queryKey: ['admin', 'library-metadata'] });
      await queryClient.invalidateQueries({ queryKey: ['session'] });
    },
  });
  const saveLibrary = useMutation({
    mutationFn: (body: LibraryMetadataUpdate) => updateLibraryMetadata(libraryId ?? 0, body),
    onSuccess: async (stored) => {
      queryClient.setQueryData(libraryMetadataQuery(libraryId ?? 0).queryKey, stored);
      setMessage(
        t('admin.metadata.save.savedLibrary', {
          name: videoLibraries.find((l) => l.id === libraryId)?.name ?? '',
        }),
      );
      await queryClient.invalidateQueries({ queryKey: ['session'] });
    },
  });
  const saving = saveServer.isPending || saveLibrary.isPending;
  const saveError = saveServer.error ?? saveLibrary.error;
  const errors = saveError ? formErrors(t, saveError) : null;
  const fieldError = (field: string) => errors?.fields?.[field];
  const textLangsError = fieldError('textLangs');
  const ratingCountryError = fieldError('ratingCountry');

  const onSave = () => {
    setMessage(null);
    saveServer.reset();
    saveLibrary.reset();
    if (libraryId !== undefined && library) void saveLibrary.mutate(libraryUpdate(library));
    else if (server) void saveServer.mutate(server);
  };
  const onDiscard = () => {
    setMessage(null);
    if (settings.data) setServer(serverValues(settings.data));
    if (libraryMeta.data) setLibrary(libraryValues(libraryMeta.data));
  };

  const known = settings.data?.knownLanguages ?? [];
  const effective = libraryId !== undefined ? libraryMeta.data?.effective : settings.data;
  const inLibrary = libraryId !== undefined;
  const textsEditable = !inLibrary || (library?.textsDeviate ?? false);
  const imagesEditable = !inLibrary || (library?.imagesDeviate ?? false);
  const textLangs = inLibrary ? library?.textLangs : server?.textLangs;
  const imageLangs = inLibrary ? library?.imageLangs : server?.imageLangs;
  const imageKinds = inLibrary ? library?.imageKinds : server?.imageKinds;
  const setTextLangs = (langs: string[]) =>
    inLibrary
      ? setLibrary((v) => v && { ...v, textLangs: langs })
      : setServer((v) => v && { ...v, textLangs: langs });
  const setChain = (kind: ChainKind, chain: string[]) =>
    inLibrary
      ? setLibrary((v) => v && { ...v, imageLangs: { ...v.imageLangs, [kind]: chain } })
      : setServer((v) => v && { ...v, imageLangs: { ...v.imageLangs, [kind]: chain } });
  const setKinds = (kinds: string[]) => {
    const typed = kinds.filter((kind): kind is Switchable =>
      (SWITCHABLE as readonly string[]).includes(kind),
    );
    if (inLibrary) setLibrary((v) => v && { ...v, imageKinds: typed });
    else setServer((v) => v && { ...v, imageKinds: typed });
  };

  const scopeOptions = [
    { value: 'server', label: t('admin.metadata.scope.server') },
    ...videoLibraries.map((l) => ({ value: String(l.id), label: l.name })),
  ];
  const scopeName = videoLibraries.find((l) => String(l.id) === scope)?.name ?? '';
  const youtubeTitles = textLangs
    ? Math.floor(
        YOUTUBE_SEARCHES_PER_DAY / Math.max(1, new Set(textLangs.map((c) => c.split('-')[0])).size),
      )
    : 0;

  return (
    <Page>
      <div className={table.header}>
        <Text as="h1" variant="display">
          {t('admin.metadata.title')}
        </Text>
        <Text tone="secondary">{t('admin.metadata.intro')}</Text>
        <Text tone="secondary" variant="caption">
          <TextLink href={paths.adminLibraries}>{t('admin.metadata.links.roots')}</TextLink>{' '}
          <TextLink href={paths.adminJobs}>{t('admin.metadata.links.jobs')}</TextLink>
        </Text>
      </div>
      {!allowed ? (
        <Notice title={t('admin.access.notAllowed.title')}>
          {t('admin.access.notAllowed.body')}
        </Notice>
      ) : (
        <div className={styles.sections} aria-busy={settings.isPending}>
          {settings.isPending && <Skeleton shape="block" />}
          {settings.isError && (
            <Notice
              tone="danger"
              title={problemMessage(t, settings.error)}
              actions={
                <AriaButton icon="refresh" onPress={() => void settings.refetch()}>
                  {t('common.retry')}
                </AriaButton>
              }
            />
          )}
          {settings.data && server && (
            <>
              <Panel as="section" aria-label={t('admin.metadata.scope.label')}>
                {scopeOptions.length <= MAX_SEGMENTS + 1 ? (
                  <RadioGroup
                    name="scope"
                    label={t('admin.metadata.scope.label')}
                    value={scope}
                    onChange={setScope}
                    options={scopeOptions}
                  />
                ) : (
                  <SelectField
                    label={t('admin.metadata.scope.label')}
                    value={scope}
                    onChange={setScope}
                    options={scopeOptions}
                  />
                )}
                <Text tone="secondary" variant="caption">
                  {inLibrary
                    ? t('admin.metadata.scope.libraryHint', { name: scopeName })
                    : t('admin.metadata.scope.serverHint')}
                </Text>
                {inLibrary && library && (
                  <div className={styles.toggles}>
                    <Switch
                      isSelected={library.textsDeviate}
                      onChange={(on) => setLibrary((v) => v && { ...v, textsDeviate: on })}
                    >
                      {t('admin.metadata.scope.textsDeviate')}
                    </Switch>
                    <Switch
                      isSelected={library.imagesDeviate}
                      onChange={(on) => setLibrary((v) => v && { ...v, imagesDeviate: on })}
                    >
                      {t('admin.metadata.scope.imagesDeviate')}
                    </Switch>
                  </div>
                )}
              </Panel>

              <Section
                title={t('admin.metadata.languages.title')}
                intro={t('admin.metadata.languages.intro')}
                aside={
                  inLibrary && !textsEditable ? (
                    <Chip tone="outline">{t('admin.metadata.scope.likeServer')}</Chip>
                  ) : undefined
                }
              >
                <div className={textsEditable ? styles.block : `${styles.block} ${styles.dimmed}`}>
                  {textLangs && (
                    <LanguageChips
                      label={t('admin.metadata.languages.list')}
                      value={textLangs}
                      onChange={setTextLangs}
                      known={known}
                      protectUI
                      disabled={!textsEditable}
                    />
                  )}
                  {textLangsError && <Notice tone="danger">{textLangsError}</Notice>}
                  <Text tone="secondary" variant="caption">
                    {t('admin.metadata.languages.fallback')}
                  </Text>
                  {providers.data && textLangs && (
                    <Text tone="secondary" variant="caption">
                      {unknownLanguages(t, nameOf, providers.data, textLangs).join(' ')}{' '}
                      {t('admin.metadata.languages.youtube', {
                        count: textLangs.length,
                        titles: youtubeTitles,
                      })}
                    </Text>
                  )}
                </div>
              </Section>

              <Section
                title={t('admin.metadata.images.title')}
                intro={t('admin.metadata.images.intro')}
                aside={
                  inLibrary && !imagesEditable ? (
                    <Chip tone="outline">{t('admin.metadata.scope.likeServer')}</Chip>
                  ) : undefined
                }
              >
                <div className={imagesEditable ? styles.block : `${styles.block} ${styles.dimmed}`}>
                  <div className={styles.kindRow}>
                    <span className={styles.chipName}>
                      {t('admin.metadata.images.kind.poster')}
                    </span>
                    <span className={styles.hint}>
                      {t('admin.metadata.images.kindHint.poster')}
                    </span>
                    <Chip tone="outline">{t('admin.metadata.images.always')}</Chip>
                  </div>
                  <div className={styles.kindRow}>
                    <span className={styles.chipName}>
                      {t('admin.metadata.images.kind.backdrop')}
                    </span>
                    <span className={styles.hint}>
                      {t('admin.metadata.images.kindHint.backdrop')}
                    </span>
                    <Chip tone="outline">{t('admin.metadata.images.always')}</Chip>
                  </div>
                  {imageKinds && (
                    <CheckboxGroup
                      label={t('admin.metadata.images.kinds')}
                      value={imageKinds}
                      onChange={setKinds}
                      isDisabled={!imagesEditable}
                      options={SWITCHABLE.map((kind: Switchable) => ({
                        value: kind,
                        label: t(`admin.metadata.images.kind.${kind}`),
                        description: t(`admin.metadata.images.kindHint.${kind}`),
                      }))}
                    />
                  )}
                  <Text tone="secondary" variant="caption">
                    {t('admin.metadata.images.notBuilt')}
                  </Text>
                  <h3 className={styles.title}>{t('admin.metadata.images.chains')}</h3>
                  <Text tone="secondary" variant="caption">
                    {t('admin.metadata.images.chainsIntro')}
                  </Text>
                  {imageLangs &&
                    CHAIN_KINDS.map((kind) => (
                      <LanguageChips
                        key={kind}
                        label={t('admin.metadata.images.chainOf', {
                          kind: t(`admin.metadata.images.kind.${kind}`),
                        })}
                        value={imageLangs[kind]}
                        onChange={(chain) => setChain(kind, chain)}
                        known={known}
                        allowNoText
                        disabled={!imagesEditable}
                      />
                    ))}
                  {CHAIN_KINDS.map((kind) => fieldError(`imageLangs.${kind}`))
                    .filter(Boolean)
                    .map((e) => (
                      <Notice key={e} tone="danger">
                        {e}
                      </Notice>
                    ))}
                </div>
              </Section>

              {!inLibrary && (
                <>
                  <Section
                    title={t('admin.metadata.quality.title')}
                    intro={t('admin.metadata.quality.intro')}
                  >
                    <div className={styles.fields}>
                      <RadioGroup
                        name="minPoster"
                        label={t('admin.metadata.quality.minPoster')}
                        value={String(server.imageMinWidth.poster)}
                        onChange={(value) =>
                          setServer(
                            (v) =>
                              v && {
                                ...v,
                                imageMinWidth: { ...v.imageMinWidth, poster: Number(value) },
                              },
                          )
                        }
                        options={POSTER_WIDTHS.map((w) => ({
                          value: String(w),
                          label:
                            w === 0
                              ? t('admin.metadata.quality.any')
                              : t('admin.metadata.quality.px', { width: w }),
                        }))}
                      />
                      <RadioGroup
                        name="minBackdrop"
                        label={t('admin.metadata.quality.minBackdrop')}
                        value={String(server.imageMinWidth.backdrop)}
                        onChange={(value) =>
                          setServer(
                            (v) =>
                              v && {
                                ...v,
                                imageMinWidth: { ...v.imageMinWidth, backdrop: Number(value) },
                              },
                          )
                        }
                        options={BACKDROP_WIDTHS.map((w) => ({
                          value: String(w),
                          label:
                            w === 0
                              ? t('admin.metadata.quality.any')
                              : t('admin.metadata.quality.px', { width: w }),
                        }))}
                      />
                    </div>
                    <Switch
                      isSelected={server.imageByVote}
                      onChange={(on) => setServer((v) => v && { ...v, imageByVote: on })}
                    >
                      {t('admin.metadata.quality.byVote')}
                    </Switch>
                    <Text tone="secondary" variant="caption">
                      {t('admin.metadata.quality.byVoteHint')}{' '}
                      {t('admin.metadata.quality.fallback')}
                    </Text>
                  </Section>

                  <Section
                    title={t('admin.metadata.sources.title')}
                    intro={t('admin.metadata.sources.intro')}
                  >
                    <div className={styles.toggles}>
                      <Switch
                        isSelected={server.tmdbIncludeAdult}
                        onChange={(on) => setServer((v) => v && { ...v, tmdbIncludeAdult: on })}
                      >
                        {t('admin.metadata.sources.tmdbAdult')}
                      </Switch>
                      <Text tone="secondary" variant="caption">
                        {t('admin.metadata.sources.tmdbAdultHint')}
                      </Text>
                      <Switch
                        isSelected={server.localImages}
                        onChange={(on) => setServer((v) => v && { ...v, localImages: on })}
                      >
                        {t('admin.metadata.sources.local')}
                      </Switch>
                      <Text tone="secondary" variant="caption">
                        {t('admin.metadata.sources.localHint')}{' '}
                        {t('admin.metadata.sources.localCount', {
                          count: settings.data.localImageTitles,
                        })}
                        . {t('admin.metadata.sources.localSmall')}
                      </Text>
                      <Switch
                        isSelected={server.fanartEnabled}
                        onChange={(on) => setServer((v) => v && { ...v, fanartEnabled: on })}
                      >
                        {t('admin.metadata.sources.fanart')}
                      </Switch>
                      <Text tone="secondary" variant="caption">
                        {t('admin.metadata.sources.fanartHint')}{' '}
                        <TextLink href={paths.adminIntegrations}>
                          {t('admin.metadata.links.keys')}
                        </TextLink>
                      </Text>
                    </div>
                  </Section>

                  <Section
                    title={t('admin.metadata.rating.title')}
                    intro={t('admin.metadata.rating.intro')}
                  >
                    <RadioGroup
                      name="ratingCountry"
                      label={t('admin.metadata.rating.country')}
                      value={server.ratingCountry}
                      onChange={(value) => setServer((v) => v && { ...v, ratingCountry: value })}
                      options={COUNTRIES.map((c: Country) => ({
                        value: c,
                        label: t(`admin.metadata.rating.countries.${c}`),
                      }))}
                    />
                    {ratingCountryError && <Notice tone="danger">{ratingCountryError}</Notice>}
                    <Text tone="secondary" variant="caption">
                      {t('admin.metadata.rating.hint')}
                    </Text>
                  </Section>

                  <ApplySection libraries={libraries.data ?? []} settings={settings.data} />
                </>
              )}

              <SaveBar
                dirty={Boolean(dirty)}
                saving={saving}
                onSave={onSave}
                onDiscard={onDiscard}
              />
              {errors?.form ? <Notice tone="danger">{errors.form}</Notice> : null}
              <div role="status" className={styles.live}>
                {message && !saveError && <Notice live={false}>{message}</Notice>}
              </div>
              {effective && inLibrary && libraryMeta.data && (
                <Text tone="secondary" variant="caption">
                  {t('admin.metadata.apply.titles', { count: libraryMeta.data.affectedTitles })}
                </Text>
              )}
            </>
          )}
        </div>
      )}
    </Page>
  );
}
