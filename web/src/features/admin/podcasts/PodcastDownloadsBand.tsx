import type { Library, LibraryPodcastDownloads, PodcastFolder } from '@slouchcat/api-client';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { useCallback, useState, type FormEvent } from 'react';
import { useTranslation } from 'react-i18next';
import { api, request } from '@/api/client';
import { problemMessage } from '@/api/errors';
import { formatBytes } from '@/lib/format';
import { Notice, Text, TextLink } from '@/ui';
import {
  AriaButton,
  BandPanel,
  RadioGroup,
  StatusRegion,
  TextField,
  useStatusMessage,
} from '@/ui/aria';
import { librariesQuery, podcastsQuery } from '../queries';
import {
  estimatePodcastDownloads,
  LAST_N_MAX,
  libraryDetailQuery,
  PARALLEL_VALUES,
  PODCAST_FOLDER_GUIDE,
  useLoadConfirmation,
} from './downloads';
import styles from './Podcasts.module.css';

type Mode = LibraryPodcastDownloads['mode'];
type Scope = LibraryPodcastDownloads['scope'];

/**
 * The band «Neue Folgen» of a podcast library (concept 19 part A, 12.3, variant B): folded, one
 * line that sums the setting up; open, «Speichern / Nur Link», the scope, the folder for new
 * subscriptions with the result of the folder check, and «Gleichzeitig laden». A change that loads
 * nothing once applies at once («Gespeichert»); one that does shows the server's estimate with
 * «Laden» and «Abbrechen» first (useLoadConfirmation). «Speichern» is locked while the folder fails
 * the check; the server decides that and what the setting means.
 */
export function PodcastDownloadsBand({
  library,
  titled,
}: {
  library: Library & { podcastDownloads: LibraryPodcastDownloads };
  /** The band names its library (the tab «Alle» with several podcast libraries). */
  titled: boolean;
}) {
  const { t, i18n } = useTranslation();
  const queryClient = useQueryClient();
  const saved = library.podcastDownloads;
  const detail = useQuery(libraryDetailQuery(library.id));
  const folder = detail.data?.podcastFolder;
  const [expanded, setExpanded] = useState(false);
  const [message, say] = useStatusMessage();
  const [failure, setFailure] = useState<unknown>(null);

  const save = useMutation({
    mutationFn: (podcastDownloads: LibraryPodcastDownloads): Promise<Library> =>
      request(
        api.PATCH('/admin/libraries/{id}', {
          params: { path: { id: library.id } },
          body: { podcastDownloads },
        }),
      ),
    onSuccess: async () => {
      setFailure(null);
      say(t('admin.podcasts.downloads.saved'));
      await Promise.all([
        queryClient.invalidateQueries({ queryKey: librariesQuery.queryKey }),
        queryClient.invalidateQueries({ queryKey: libraryDetailQuery(library.id).queryKey }),
        queryClient.invalidateQueries({ queryKey: podcastsQuery.queryKey }),
      ]);
    },
    onError: (error) => {
      say(null);
      setFailure(error);
    },
  });
  const { mutate } = save;
  const apply = useCallback((value: LibraryPodcastDownloads) => mutate(value), [mutate]);
  const confirmation = useLoadConfirmation(apply);
  const value = confirmation.shown ?? (save.isPending ? save.variables : undefined) ?? saved;

  // What the setting loads once is the server's estimate of the new against the current one.
  const loads = (next: LibraryPodcastDownloads) => next.mode === 'save' && next.scope !== 'new';
  const change = (next: LibraryPodcastDownloads) => {
    say(null);
    if (!loads(next)) {
      confirmation.reset();
      apply(next);
      return;
    }
    const body = (d: LibraryPodcastDownloads) => ({ mode: d.mode, scope: d.scope, lastN: d.lastN });
    confirmation.ask(
      next,
      () => estimatePodcastDownloads(library.id, body(next)),
      () =>
        loads(saved) ? estimatePodcastDownloads(library.id, body(saved)) : Promise.resolve(null),
    );
  };

  const [lastN, setLastN] = useState(String(value.lastN));
  const [shownN, setShownN] = useState(value.lastN);
  if (shownN !== value.lastN) {
    setShownN(value.lastN);
    setLastN(String(value.lastN));
  }
  const onLastN = (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    const n = Number(lastN.trim());
    if (!Number.isInteger(n) || n < 1 || n > LAST_N_MAX || n === value.lastN) return;
    change({ ...value, lastN: n });
  };

  const scopeName = (scope: Scope, n: number) =>
    scope === 'last_n'
      ? t('admin.podcasts.downloads.scope.lastNOf', { n })
      : t(`admin.podcasts.downloads.scope.${scope}`);
  const summary =
    saved.mode === 'save'
      ? t('admin.podcasts.downloads.summary.save', {
          scope: scopeName(saved.scope, saved.lastN),
          parallel: saved.parallel,
        })
      : t('admin.podcasts.downloads.summary.link');
  const locked = !folder?.writable;
  const podcastRoots = library.roots.filter((root) => root.content === 'podcast');
  const pending = confirmation.pending;
  const error = failure ?? confirmation.failure;

  return (
    <BandPanel
      title={
        titled
          ? t('admin.podcasts.downloads.titleOf', { library: library.name })
          : t('admin.podcasts.downloads.title')
      }
      icon="download"
      meta={<span className={styles.summary}>{summary}</span>}
      collapsible
      isExpanded={expanded}
      onExpandedChange={setExpanded}
    >
      <div className={styles.band}>
        <FolderCheck folder={folder} pending={detail.isPending} language={i18n.language} />
        <RadioGroup
          label={t('admin.podcasts.downloads.mode.label')}
          value={value.mode}
          onChange={(mode) => change({ ...value, mode: mode as Mode })}
          options={[
            {
              value: 'save',
              label: t('admin.podcasts.downloads.mode.save'),
              // Locked while the folder fails the check, unless it stores already.
              isDisabled: locked && saved.mode !== 'save',
            },
            { value: 'link', label: t('admin.podcasts.downloads.mode.link') },
          ]}
        />
        <RadioGroup
          label={t('admin.podcasts.downloads.scope.label')}
          description={
            value.mode === 'save'
              ? t(`admin.podcasts.downloads.hint.${value.scope}`, { n: value.lastN })
              : t('admin.podcasts.downloads.hint.link')
          }
          value={value.scope}
          onChange={(scope) => change({ ...value, scope: scope as Scope })}
          options={(['new', 'last_n', 'all'] as const).map((scope) => ({
            value: scope,
            label: t(`admin.podcasts.downloads.scope.${scope}`),
          }))}
        />
        {value.scope === 'last_n' && (
          <form className={styles.lastN} onSubmit={onLastN}>
            <TextField
              name="lastN"
              type="number"
              inputMode="numeric"
              label={t('admin.podcasts.downloads.lastN.label')}
              description={t('admin.podcasts.downloads.lastN.hint', { max: LAST_N_MAX })}
              value={lastN}
              onChange={setLastN}
            />
            <AriaButton type="submit">{t('admin.podcasts.downloads.lastN.apply')}</AriaButton>
          </form>
        )}
        {confirmation.estimating && (
          <Text tone="secondary">{t('admin.podcasts.downloads.estimating')}</Text>
        )}
        {pending && (
          <Notice
            title={t('admin.podcasts.downloads.confirm.text', {
              count: pending.episodes,
              number: new Intl.NumberFormat(i18n.language).format(pending.episodes),
              size: formatBytes(pending.bytes, i18n.language),
              free: formatBytes(pending.freeBytes, i18n.language),
            })}
            actions={
              <>
                <AriaButton variant="primary" icon="download" onPress={confirmation.confirm}>
                  {t('admin.podcasts.downloads.confirm.load')}
                </AriaButton>
                <AriaButton variant="ghost" onPress={confirmation.cancel}>
                  {t('common.cancel')}
                </AriaButton>
              </>
            }
          />
        )}
        {podcastRoots.length > 1 && (
          <RadioGroup
            label={t('admin.podcasts.downloads.folder.target')}
            orientation="vertical"
            value={String(value.rootId ?? folder?.rootId ?? '')}
            onChange={(id) => change({ ...value, rootId: Number(id) })}
            options={podcastRoots.map((root) => ({ value: String(root.id), label: root.path }))}
          />
        )}
        <RadioGroup
          label={t('admin.podcasts.downloads.parallel.label')}
          description={t('admin.podcasts.downloads.parallel.hint')}
          value={String(value.parallel)}
          onChange={(parallel) => change({ ...value, parallel: Number(parallel) })}
          options={PARALLEL_VALUES.map((n) => ({ value: n, label: n }))}
        />
        {error !== null && <Notice tone="danger">{problemMessage(t, error)}</Notice>}
        <StatusRegion message={error === null ? message : null} />
      </div>
    </BandPanel>
  );
}

/** The folder for new subscriptions with the result of its check (5.4), or why there is none. */
function FolderCheck({
  folder,
  pending,
  language,
}: {
  folder: PodcastFolder | undefined;
  pending: boolean;
  language: string;
}) {
  const { t } = useTranslation();
  if (pending) return <Text tone="secondary">{t('admin.podcasts.downloads.folder.checking')}</Text>;
  const guide = (
    <TextLink href={PODCAST_FOLDER_GUIDE} external>
      {t('admin.podcasts.downloads.folder.guide')}
    </TextLink>
  );
  if (!folder) {
    return (
      <div className={styles.folder}>
        <Text tone="secondary">{t('admin.podcasts.downloads.folder.none')}</Text>
      </div>
    );
  }
  return (
    <div className={styles.folder}>
      <span className={styles.muted}>{t('admin.podcasts.downloads.folder.label')}</span>
      <span className={styles.url}>{folder.path}</span>
      {folder.writable ? (
        <span>
          {folder.freeBytes > 0
            ? t('admin.podcasts.downloads.folder.writable', {
                free: formatBytes(folder.freeBytes, language),
              })
            : t('admin.podcasts.downloads.folder.writableUnknown')}
        </span>
      ) : (
        <>
          <span className={styles.problem}>
            {t(`admin.podcasts.downloads.folder.problem.${folder.problem ?? 'read_only'}`)}
          </span>
          <span>
            {t('admin.podcasts.downloads.folder.locked')} {guide}
          </span>
        </>
      )}
    </div>
  );
}
