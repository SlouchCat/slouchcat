import type { KeyState, Library } from '@slouchcat/api-client';
import type { TFunction } from 'i18next';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { useEffect, useId, useRef, useState, type RefObject } from 'react';
import { useTranslation } from 'react-i18next';
import { formErrors, problemMessage } from '@/api/errors';
import { paths } from '@/app/paths';
import { useActiveSession } from '@/app/session';
import { formatBookLength, formatDateTime } from '@/lib/format';
import { useDocumentTitle } from '@/lib/useDocumentTitle';
import { Chip, Notice, SelectField, Skeleton, Text, TextLink, VisuallyHidden } from '@/ui';
import {
  AdminPage,
  afterDialogsClosed,
  afterRender,
  AriaButton,
  ConfirmDialog,
  DataTable,
  focusTitleIfLost,
  Form,
  providerChain,
  RadioGroup,
  SaveBar,
  SectionHeader,
  SettingRow,
  StatTile,
  StatusRegion,
  Switch,
  TextField,
  useStatusMessage,
  useUnsavedChanges,
  type DataColumn,
} from '@/ui/aria';
import { MetadataEditor } from './editor/MetadataEditor';
import { SourceChip } from './integrations/shared';
import { providerName } from './item/facts';
import { LibraryFoldersDialog } from './libraries/LibraryFoldersDialog';
import {
  audiobookReviewQuery,
  audiobookSourcesQuery,
  AUDIOBOOK_MATCH_POLL_MS,
  librariesQuery,
  matchAllAudiobooks,
  updateAudiobookSources,
  type AudibleRegion,
  type AudiobookAutoMatch,
  type AudiobookSources,
  type AudiobookSourcesUpdate,
  type BookReviewEntry,
} from './queries';
import styles from './metadata/AudiobookSources.module.css';

/** Every Audible marketplace with its text key (a dot cannot stand in a key). */
const REGIONS = {
  de: 'admin.audiobookSources.region.de',
  com: 'admin.audiobookSources.region.com',
  'co.uk': 'admin.audiobookSources.region.couk',
  fr: 'admin.audiobookSources.region.fr',
  it: 'admin.audiobookSources.region.it',
  es: 'admin.audiobookSources.region.es',
  ca: 'admin.audiobookSources.region.ca',
  'com.au': 'admin.audiobookSources.region.comau',
  in: 'admin.audiobookSources.region.in',
  'co.jp': 'admin.audiobookSources.region.cojp',
} as const satisfies Record<AudibleRegion, string>;
const isRegion = (value: string): value is AudibleRegion => value in REGIONS;

const AUTO = {
  off: ['admin.audiobookSources.auto.off', 'admin.audiobookSources.auto.offHint'],
  new: ['admin.audiobookSources.auto.new', 'admin.audiobookSources.auto.newHint'],
  all: ['admin.audiobookSources.auto.all', 'admin.audiobookSources.auto.allHint'],
} as const satisfies Record<AudiobookAutoMatch, readonly [string, string]>;
const isAuto = (value: string): value is AudiobookAutoMatch => value in AUTO;

/** Where a signed-in user creates a key for Google Books (an address, not a text). */

/** The tolerances the page offers by name (mockup `Quellen`); the server allows 1 to 10. */
const TOLERANCES = [1, 3, 5] as const;

/** The text of each reason of a failed run (`BookReviewEntry.error`, `<source>:<reason>`). */
const REASONS = {
  paused: 'admin.audiobookSources.review.reason.paused',
  blocked: 'admin.audiobookSources.review.reason.blocked',
  not_configured: 'admin.audiobookSources.review.reason.notConfigured',
  region: 'admin.audiobookSources.review.reason.region',
  unreachable: 'admin.audiobookSources.review.reason.unreachable',
} as const;
const isReason = (value: string): value is keyof typeof REASONS => value in REASONS;

/** «Audible pausiert nach wiederholten Absagen.» from `audible:paused`; an unknown reason reads as a failure. */
function failureText(t: TFunction, error: string): string {
  const colon = error.indexOf(':');
  const source = providerName(t, colon < 0 ? error : error.slice(0, colon));
  const reason = colon < 0 ? '' : error.slice(colon + 1);
  return isReason(reason)
    ? t(REASONS[reason], { source })
    : t('admin.audiobookSources.review.reason.unknown', { source });
}

/** The form's state: the server-wide settings of the book sources without the key. */
interface Draft {
  audibleRegion: AudibleRegion;
  audnexusUrl: string;
  autoMatch: AudiobookAutoMatch;
  tolerancePercent: number;
  normalizeAuthors: boolean;
  normalizeSeries: boolean;
}

const draftOf = (sources: AudiobookSources): Draft => ({
  audibleRegion: sources.audibleRegion,
  audnexusUrl: sources.audnexusUrl,
  autoMatch: sources.autoMatch,
  tolerancePercent: sources.tolerancePercent,
  normalizeAuthors: sources.normalizeAuthors,
  normalizeSeries: sources.normalizeSeries,
});

/** What differs from the stored settings (PATCH: absent fields keep their value). */
function changes(sources: AudiobookSources, draft: Draft): AudiobookSourcesUpdate {
  const stored = draftOf(sources);
  const update: AudiobookSourcesUpdate = {};
  for (const key of Object.keys(stored) as (keyof Draft)[]) {
    if (draft[key] !== stored[key]) Object.assign(update, { [key]: draft[key] });
  }
  if (update.audnexusUrl !== undefined) update.audnexusUrl = update.audnexusUrl.trim();
  return update;
}

/**
 * The page «Hörbücher: Quellen» (concept 19 D6.6, mockup `Quellen`): the ordered list of every
 * audiobook folder (shown only, changed in the folder's dialog, D6.2), the server-wide settings
 * with one save bar, the Google Books key, the terms of Audible, the state of the sources and of
 * the library, «Jetzt alle ohne Zuordnung zuordnen» and the list «Prüfen» with the pen of the
 * editor.
 */
export function AudiobookSourcesPage() {
  const { t } = useTranslation();
  useDocumentTitle(t('admin.audiobookSources.title'));
  const session = useActiveSession();
  const allowed = session.account.role === 'admin' && session.profile.isPrimary;
  const titleRef = useRef<HTMLHeadingElement>(null);
  const sources = useQuery({ ...audiobookSourcesQuery, enabled: allowed });

  return (
    <AdminPage
      title={t('admin.audiobookSources.title')}
      help={t('admin.audiobookSources.intro')}
      titleRef={titleRef}
    >
      {!allowed ? (
        <Notice title={t('admin.access.notAllowed.title')}>
          {t('admin.access.notAllowed.body')}
        </Notice>
      ) : sources.isError ? (
        <Notice
          tone="danger"
          title={problemMessage(t, sources.error)}
          actions={
            <AriaButton icon="refresh" onPress={() => void sources.refetch()}>
              {t('common.retry')}
            </AriaButton>
          }
        />
      ) : !sources.data ? (
        <Skeleton shape="block" />
      ) : (
        <SourcesContent sources={sources.data} titleRef={titleRef} />
      )}
    </AdminPage>
  );
}

function SourcesContent({
  sources,
  titleRef,
}: {
  sources: AudiobookSources;
  titleRef: RefObject<HTMLHeadingElement | null>;
}) {
  return (
    <div className={styles.page}>
      <Folders sources={sources} titleRef={titleRef} />
      <Settings sources={sources} titleRef={titleRef} />
      <State sources={sources} />
      <Review running={sources.running} titleRef={titleRef} />
    </div>
  );
}

/** The ordered source list of every audiobook folder, with the way into the folder's dialog. */
function Folders({
  sources,
  titleRef,
}: {
  sources: AudiobookSources;
  titleRef: RefObject<HTMLHeadingElement | null>;
}) {
  const { t } = useTranslation();
  const queryClient = useQueryClient();
  const libraries = useQuery(librariesQuery);
  const [openId, setOpenId] = useState<number | null>(null);
  const [open, setOpen] = useState(false);
  const library = libraries.data?.find((entry) => entry.id === openId);
  const headingId = useId();

  const changed = async (updated: Library) => {
    queryClient.setQueryData<Library[]>(librariesQuery.queryKey, (list) =>
      list?.map((entry) => (entry.id === updated.id ? updated : entry)),
    );
    await queryClient.invalidateQueries({ queryKey: audiobookSourcesQuery.queryKey });
    await afterDialogsClosed();
    focusTitleIfLost(titleRef);
  };

  return (
    <section className={styles.section} aria-labelledby={headingId}>
      <SectionHeader
        id={headingId}
        title={t('admin.audiobookSources.folders.title')}
        description={t('admin.audiobookSources.folders.intro')}
      />
      {sources.folders.length === 0 ? (
        <Text tone="secondary">{t('admin.audiobookSources.folders.empty')}</Text>
      ) : (
        sources.folders.map((folder) => (
          <SettingRow
            key={folder.rootId}
            label={folder.path}
            help={t('admin.audiobookSources.folders.library', { name: folder.libraryName })}
          >
            <div className={styles.inline}>
              <Text>{providerChain(t, folder.providers)}</Text>
              <AriaButton
                icon="pen"
                isDisabled={!libraries.data?.some((entry) => entry.id === folder.libraryId)}
                aria-label={t('admin.audiobookSources.folders.editLabel', { path: folder.path })}
                onPress={() => {
                  setOpenId(folder.libraryId);
                  setOpen(true);
                }}
              >
                {t('admin.audiobookSources.folders.edit')}
              </AriaButton>
            </div>
          </SettingRow>
        ))
      )}
      {library && (
        <LibraryFoldersDialog
          key={library.id}
          library={library}
          isOpen={open}
          onOpenChange={setOpen}
          onChanged={(updated) => void changed(updated)}
        />
      )}
    </section>
  );
}

/** Region, Audnexus, the key, the automatic match with its tolerance and the two switches. */
function Settings({
  sources,
  titleRef,
}: {
  sources: AudiobookSources;
  titleRef: RefObject<HTMLHeadingElement | null>;
}) {
  const { t } = useTranslation();
  const queryClient = useQueryClient();
  const [message, setMessage] = useStatusMessage();
  const [draft, setDraft] = useState(() => draftOf(sources));
  const set = <K extends keyof Draft>(key: K, value: Draft[K]) =>
    setDraft((previous) => ({ ...previous, [key]: value }));
  const update = changes(sources, draft);
  const dirty = Object.keys(update).length > 0;
  const unsaved = useUnsavedChanges(dirty);
  const headingId = useId();

  const save = useMutation({
    mutationFn: () => updateAudiobookSources(update),
    onSuccess: (next) => {
      queryClient.setQueryData(audiobookSourcesQuery.queryKey, next);
      setDraft(draftOf(next));
      setMessage(t('admin.audiobookSources.settings.saved'));
    },
  });
  const errors = save.error ? formErrors(t, save.error) : null;
  const tolerances: number[] = [...TOLERANCES];
  if (!tolerances.includes(draft.tolerancePercent)) {
    tolerances.push(draft.tolerancePercent);
    tolerances.sort((a, b) => a - b);
  }

  return (
    <section className={styles.section} aria-labelledby={headingId}>
      <SectionHeader id={headingId} title={t('admin.audiobookSources.settings.title')} />
      <StatusRegion message={message} />
      <SettingRow
        label={t('admin.audiobookSources.region.label')}
        help={t('admin.audiobookSources.region.help')}
      >
        <SelectField
          label={t('admin.audiobookSources.region.label')}
          hideLabel
          value={draft.audibleRegion}
          onChange={(value) => {
            if (isRegion(value)) set('audibleRegion', value);
          }}
          options={(Object.keys(REGIONS) as AudibleRegion[]).map((region) => ({
            value: region,
            label: t(REGIONS[region]),
          }))}
        />
      </SettingRow>
      <SettingRow
        label={t('admin.audiobookSources.audnexus.label')}
        help={t('admin.audiobookSources.audnexus.help')}
      >
        <Form
          validationErrors={errors?.fields}
          // Enter in the field saves like the save bar, without a native submit (no reload).
          onSubmit={(event) => {
            event.preventDefault();
            if (dirty && !save.isPending) save.mutate();
          }}
        >
          <TextField
            name="audnexusUrl"
            label={t('admin.audiobookSources.audnexus.label')}
            hideLabel
            type="url"
            mono
            spellCheck="false"
            value={draft.audnexusUrl}
            onChange={(value) => set('audnexusUrl', value)}
            validate={(value) =>
              /^https?:\/\/\S+$/.test(value.trim())
                ? null
                : t('admin.audiobookSources.audnexus.invalid')
            }
          />
        </Form>
      </SettingRow>
      <GoogleBooksKey state={sources.googleBooksKey} />
      <SettingRow label={t('admin.audiobookSources.auto.label')}>
        <RadioGroup
          label={t('admin.audiobookSources.auto.label')}
          hideLabel
          description={t(AUTO[draft.autoMatch][1])}
          value={draft.autoMatch}
          onChange={(value) => {
            if (isAuto(value)) set('autoMatch', value);
          }}
          options={(Object.keys(AUTO) as AudiobookAutoMatch[]).map((mode) => ({
            value: mode,
            label: t(AUTO[mode][0]),
          }))}
        />
      </SettingRow>
      <SettingRow
        label={t('admin.audiobookSources.tolerance.label')}
        help={t('admin.audiobookSources.tolerance.help')}
      >
        <RadioGroup
          label={t('admin.audiobookSources.tolerance.label')}
          hideLabel
          value={String(draft.tolerancePercent)}
          onChange={(value) => set('tolerancePercent', Number(value))}
          options={tolerances.map((count) => ({
            value: String(count),
            label: t('admin.audiobookSources.tolerance.option', { count }),
          }))}
        />
      </SettingRow>
      <SectionHeader level={3} title={t('admin.audiobookSources.normalize.title')} />
      <SettingRow
        label={t('admin.audiobookSources.normalize.authors')}
        help={t('admin.audiobookSources.normalize.authorsHint')}
      >
        <Switch isSelected={draft.normalizeAuthors} onChange={(on) => set('normalizeAuthors', on)}>
          <VisuallyHidden>{t('admin.audiobookSources.normalize.authors')}</VisuallyHidden>
        </Switch>
      </SettingRow>
      <SettingRow
        label={t('admin.audiobookSources.normalize.series')}
        help={t('admin.audiobookSources.normalize.seriesHint')}
      >
        <Switch isSelected={draft.normalizeSeries} onChange={(on) => set('normalizeSeries', on)}>
          <VisuallyHidden>{t('admin.audiobookSources.normalize.series')}</VisuallyHidden>
        </Switch>
      </SettingRow>
      <Notice live={false}>{t('admin.audiobookSources.terms')}</Notice>
      <SaveBar
        dirty={dirty}
        saving={save.isPending}
        onSave={() => {
          if (dirty) save.mutate();
        }}
        onDiscard={() => {
          save.reset();
          setDraft(draftOf(sources));
          void afterRender().then(() => focusTitleIfLost(titleRef));
        }}
      />
      {errors?.form && <Notice tone="danger">{errors.form}</Notice>}
      <ConfirmDialog {...unsaved.dialog} />
    </section>
  );
}

/**
 * Google Books' key: stored and checked on the page of the provider keys since concept 20 (O-7);
 * here its state and the way there.
 */
function GoogleBooksKey({ state }: { state: KeyState }) {
  const { t } = useTranslation();
  return (
    <SettingRow
      label={t('admin.audiobookSources.googleBooks.name')}
      labelAddon={<SourceChip source={state.source} hint={state.hint} />}
      help={t('admin.audiobookSources.googleBooks.description')}
    >
      <TextLink href={paths.adminIntegrations}>
        {t('admin.audiobookSources.googleBooks.link')}
      </TextLink>
    </SettingRow>
  );
}

/** The state of the sources and of the library, and «Jetzt alle ohne Zuordnung zuordnen». */
function State({ sources }: { sources: AudiobookSources }) {
  const { t, i18n } = useTranslation();
  const queryClient = useQueryClient();
  const [message, setMessage] = useStatusMessage();
  const number = new Intl.NumberFormat(i18n.language);
  const headingId = useId();
  const run = useMutation({
    mutationFn: matchAllAudiobooks,
    onSuccess: async ({ queued }) => {
      setMessage(t('admin.audiobookSources.matchAll.queued', { count: queued }));
      await queryClient.invalidateQueries({ queryKey: audiobookSourcesQuery.queryKey });
    },
  });
  const stats = sources.stats;
  const tiles = [
    ['total', stats.total],
    ['matched', stats.matched],
    ['review', stats.review],
    ['none', stats.none],
    ['error', stats.error],
    ['pending', stats.pending],
  ] as const;

  return (
    <section className={styles.section} aria-labelledby={headingId}>
      <SectionHeader id={headingId} title={t('admin.audiobookSources.stats.title')} />
      <div className={styles.tiles}>
        {tiles.map(([key, count]) => (
          <StatTile
            key={key}
            compact
            label={t(`admin.audiobookSources.stats.${key}`)}
            value={number.format(count)}
            tone={key === 'error' && count > 0 ? 'danger' : 'neutral'}
          />
        ))}
      </div>
      {sources.running && (
        <Text tone="secondary" variant="caption">
          {t('admin.audiobookSources.stats.running')}
        </Text>
      )}
      <div className={styles.inline}>
        <AriaButton
          variant="primary"
          icon="refresh"
          isPending={run.isPending}
          onPress={() => {
            setMessage(null);
            run.mutate();
          }}
        >
          {t('admin.audiobookSources.matchAll.run')}
        </AriaButton>
        <TextLink href={paths.adminJobs}>{t('admin.audiobookSources.matchAll.jobs')}</TextLink>
      </div>
      {run.isError && <Notice tone="danger">{problemMessage(t, run.error)}</Notice>}
      <StatusRegion message={run.isError ? null : message} />
      <SectionHeader level={3} title={t('admin.audiobookSources.sources.title')} />
      <ul className={styles.sources}>
        {sources.sources.map((source) => (
          <li key={source.id} className={styles.inline}>
            <Text>{providerName(t, source.id)}</Text>
            {source.pausedUntil ? (
              <Chip tone="warning">
                {t('admin.audiobookSources.sources.paused', {
                  time: formatDateTime(source.pausedUntil, i18n.language),
                })}
              </Chip>
            ) : source.configured ? (
              <Chip tone="success">{t('admin.audiobookSources.sources.ready')}</Chip>
            ) : (
              <Chip tone="outline">{t('admin.audiobookSources.sources.notConfigured')}</Chip>
            )}
          </li>
        ))}
      </ul>
    </section>
  );
}

/**
 * The list «Prüfen»: the stored candidates of each book and the pen of the editor. While a match
 * runs it is read again every few seconds, and once more when the run ends.
 */
function Review({
  running,
  titleRef,
}: {
  running: boolean;
  titleRef: RefObject<HTMLHeadingElement | null>;
}) {
  const { t } = useTranslation();
  const queryClient = useQueryClient();
  const review = useQuery({
    ...audiobookReviewQuery,
    refetchInterval: running ? AUDIOBOOK_MATCH_POLL_MS : false,
  });
  const { refetch } = review;
  const wasRunning = useRef(running);
  useEffect(() => {
    if (wasRunning.current && !running) void refetch();
    wasRunning.current = running;
  }, [running, refetch]);
  const [editing, setEditing] = useState<string | null>(null);
  const pens = useRef(new Map<string, HTMLButtonElement>());
  const headingId = useId();
  const lengthOf = (ms: number | undefined) =>
    ms ? formatBookLength(t, ms) : t('admin.editor.bookMatch.lengthUnknown');

  const columns: DataColumn<BookReviewEntry>[] = [
    {
      key: 'title',
      header: t('admin.audiobookSources.review.column.title'),
      rowHeader: true,
      cell: (entry) => entry.title,
    },
    {
      key: 'author',
      header: t('admin.audiobookSources.review.column.author'),
      cell: (entry) => entry.author,
    },
    {
      key: 'length',
      header: t('admin.audiobookSources.review.column.length'),
      cell: (entry) => lengthOf(entry.durationMs),
    },
    {
      key: 'state',
      header: t('admin.audiobookSources.review.column.state'),
      cell: (entry) => (
        <span className={styles.stack}>
          <Chip tone={entry.state === 'error' ? 'danger' : 'warning'}>
            {t(`admin.audiobookSources.review.state.${entry.state}`)}
          </Chip>
          {entry.error && (
            <Text tone="secondary" variant="caption">
              {failureText(t, entry.error)}
            </Text>
          )}
        </span>
      ),
    },
    {
      key: 'candidates',
      header: t('admin.audiobookSources.review.column.candidates'),
      cell: (entry) =>
        entry.candidates.length === 0 ? (
          t('admin.audiobookSources.review.noCandidates')
        ) : (
          <ul className={styles.candidates}>
            {entry.candidates.slice(0, 3).map((candidate) => (
              <li key={`${candidate.provider}-${candidate.providerId}`}>
                {t('admin.audiobookSources.review.candidate', {
                  source: providerName(t, candidate.provider),
                  title: candidate.subtitle
                    ? `${candidate.title} – ${candidate.subtitle}`
                    : candidate.title,
                  length: lengthOf(candidate.durationMs),
                })}
              </li>
            ))}
          </ul>
        ),
    },
    {
      key: 'actions',
      header: t('admin.audiobookSources.review.column.actions'),
      hideHeader: true,
      align: 'end',
      cell: (entry) => (
        <AriaButton
          ref={(element) => {
            if (element) pens.current.set(entry.itemId, element);
            else pens.current.delete(entry.itemId);
          }}
          variant="ghost"
          icon="pen"
          iconOnly
          aria-label={t('admin.audiobookSources.review.edit', { title: entry.title })}
          onPress={() => setEditing(entry.itemId)}
        />
      ),
    },
  ];

  return (
    <section className={styles.section} aria-labelledby={headingId}>
      <SectionHeader
        id={headingId}
        title={t('admin.audiobookSources.review.title')}
        description={t('admin.audiobookSources.review.intro')}
      />
      {review.isPending && <Skeleton shape="block" />}
      {review.isError && (
        <Notice
          tone="danger"
          title={problemMessage(t, review.error)}
          actions={
            <AriaButton icon="refresh" onPress={() => void review.refetch()}>
              {t('common.retry')}
            </AriaButton>
          }
        />
      )}
      {review.data?.length === 0 && (
        <Text tone="secondary">{t('admin.audiobookSources.review.empty')}</Text>
      )}
      {review.data && review.data.length > 0 && (
        <DataTable
          caption={t('admin.audiobookSources.review.caption')}
          columns={columns}
          rows={review.data}
          rowKey={(entry) => entry.itemId}
        />
      )}
      {editing && (
        <MetadataEditor
          itemId={editing}
          onClose={() => {
            const id = editing;
            const index = review.data?.findIndex((entry) => entry.itemId === id) ?? -1;
            setEditing(null);
            void (async () => {
              // The list and the numbers anew (a match takes the book off the list), then the
              // focus: the pen while its book is still to review, else the pen of the row that
              // took its place, else the page's title.
              await queryClient.invalidateQueries({ queryKey: audiobookSourcesQuery.queryKey });
              await afterDialogsClosed();
              const own = pens.current.get(id);
              if (own?.isConnected) {
                own.focus();
                return;
              }
              const rows = queryClient.getQueryData(audiobookReviewQuery.queryKey) ?? [];
              const next = rows[Math.min(Math.max(index, 0), rows.length - 1)];
              const pen = next && pens.current.get(next.itemId);
              if (pen?.isConnected) pen.focus();
              else focusTitleIfLost(titleRef);
            })();
          }}
        />
      )}
    </section>
  );
}
