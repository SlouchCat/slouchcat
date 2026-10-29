import type { AdminPodcast, Library, PodcastCreate } from '@slouchcat/api-client';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { useRef, useState, type FormEvent } from 'react';
import { useTranslation } from 'react-i18next';
import { api, request } from '@/api/client';
import { codeMessage, formErrors, problemMessage } from '@/api/errors';
import { paths } from '@/app/paths';
import { useActiveSession } from '@/app/session';
import { formText } from '@/lib/form';
import { formatDateTime } from '@/lib/format';
import { useDocumentTitle } from '@/lib/useDocumentTitle';
import { ButtonLink, Chip, Notice, Page, Skeleton, Text } from '@/ui';
import {
  AriaButton,
  ConfirmDialog,
  DataTable,
  Form,
  FormDialog,
  RadioGroup,
  TextField,
  type DataColumn,
} from '@/ui/aria';
import { librariesQuery, podcastsQuery } from './queries';
import styles from './AdminTablePage.module.css';

/** After "Jetzt abrufen" the list asks again this often for a while, until the fetch shows. */
const REFRESH_POLL_MS = 2000;
const REFRESH_POLL_FOR_MS = 30_000;

/** A feed address the server can take: http or https (the server checks the rest). */
const isFeedUrl = (value: string) => /^https?:\/\/\S+$/i.test(value.trim());

/** "Feed abonnieren" (H-2): a podcast library and the feed's URL; the server fetches it at once. */
function SubscribeDialog({
  libraries,
  onClose,
  onSubscribed,
}: {
  libraries: readonly Library[];
  onClose: () => void;
  onSubscribed: (podcast: AdminPodcast) => void;
}) {
  const { t } = useTranslation();
  const [libraryId, setLibraryId] = useState(String(libraries[0]?.id ?? ''));
  const create = useMutation({
    mutationFn: (body: PodcastCreate): Promise<AdminPodcast> =>
      request(api.POST('/admin/podcasts', { body })),
    onSuccess: onSubscribed,
  });
  // The feed's problems (unreachable, no RSS, already subscribed) belong to the URL field.
  const errors = create.error ? formErrors(t, create.error) : null;
  const onSubmit = (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    const data = new FormData(event.currentTarget);
    create.mutate({ libraryId: Number(libraryId), feedUrl: formText(data, 'feedUrl') });
  };
  return (
    <FormDialog
      isOpen
      onOpenChange={(open) => {
        if (!open) onClose();
      }}
      title={t('admin.podcasts.subscribe.title')}
      description={t('admin.podcasts.subscribe.description')}
    >
      <Form className={styles.form} onSubmit={onSubmit} validationErrors={errors?.fields}>
        {errors?.form && <Notice tone="danger">{errors.form}</Notice>}
        <TextField
          name="feedUrl"
          type="url"
          label={t('admin.podcasts.subscribe.url')}
          description={t('admin.podcasts.subscribe.urlHint')}
          autoComplete="off"
          spellCheck="false"
          mono
          isRequired
          // A server error (unreachable, no RSS, already subscribed) sits on this field until the
          // address changes; Firefox keeps the field invalid otherwise and never submits again.
          onChange={() => {
            if (create.error) create.reset();
          }}
          validate={(value) => {
            if (!value.trim()) return t('errors.field.required');
            return isFeedUrl(value) ? null : t('errors.field.invalid_url');
          }}
        />
        <RadioGroup
          name="libraryId"
          label={t('admin.podcasts.subscribe.library')}
          value={libraryId}
          onChange={setLibraryId}
          options={libraries.map((library) => ({
            value: String(library.id),
            label: library.name,
          }))}
        />
        <div className={styles.dialogActions}>
          <AriaButton variant="ghost" onPress={onClose}>
            {t('common.cancel')}
          </AriaButton>
          <AriaButton type="submit" variant="primary" icon="plus" isPending={create.isPending}>
            {create.isPending
              ? t('admin.podcasts.subscribe.submitting')
              : t('admin.podcasts.subscribe.submit')}
          </AriaButton>
        </div>
      </Form>
    </FormDialog>
  );
}

/**
 * Podcasts (M6, H-2): the subscribed feeds and the podcasts read from folders (ADR 0015) with
 * library, episodes, last fetch and error; "Feed abonnieren", "Jetzt abrufen" and "Abo löschen".
 * The server fetches every feed hourly.
 */
export function PodcastsPage() {
  const { t, i18n } = useTranslation();
  useDocumentTitle(t('admin.podcasts.title'));
  const session = useActiveSession();
  const allowed = session.account.role === 'admin' && session.profile.isPrimary;
  const queryClient = useQueryClient();
  const [pollUntil, setPollUntil] = useState(0);
  const podcasts = useQuery({
    ...podcastsQuery,
    enabled: allowed,
    refetchInterval: () => (Date.now() < pollUntil ? REFRESH_POLL_MS : false),
  });
  const libraries = useQuery({ ...librariesQuery, enabled: allowed });
  const podcastLibraries = (libraries.data ?? []).filter((library) => library.kind === 'podcasts');
  const libraryName = (id: number) =>
    libraries.data?.find((library) => library.id === id)?.name ?? String(id);
  const [subscribing, setSubscribing] = useState(false);
  const [message, setMessage] = useState<string | null>(null);
  const header = useRef<HTMLDivElement>(null);
  const subscribeButton = useRef<HTMLButtonElement>(null);
  const invalidate = () => queryClient.invalidateQueries({ queryKey: podcastsQuery.queryKey });

  const refresh = useMutation({
    mutationFn: (podcast: AdminPodcast): Promise<AdminPodcast> =>
      request(api.POST('/admin/podcasts/{id}/refresh', { params: { path: { id: podcast.id } } })),
    onSuccess: async (_data, podcast) => {
      setMessage(t('admin.podcasts.refresh.queued', { title: podcast.title }));
      setPollUntil(Date.now() + REFRESH_POLL_FOR_MS);
      await invalidate();
    },
  });
  const remove = useMutation({
    mutationFn: (podcast: AdminPodcast) =>
      request(api.DELETE('/admin/podcasts/{id}', { params: { path: { id: podcast.id } } })),
    onSuccess: async (_data, podcast) => {
      setMessage(t('admin.podcasts.remove.done', { title: podcast.title }));
      await invalidate();
      // The row and its buttons are gone: the focus goes to the page heading.
      header.current?.focus();
    },
  });
  const actionError = refresh.error ?? remove.error;

  const columns: DataColumn<AdminPodcast>[] = [
    {
      key: 'title',
      header: t('admin.podcasts.columns.title'),
      rowHeader: true,
      cell: (podcast) => (
        <span className={styles.stack}>
          <span>{podcast.title}</span>
          {podcast.author && <span className={styles.muted}>{podcast.author}</span>}
          {podcast.feedUrl && (
            <span className={`${styles.muted} ${styles.url}`}>{podcast.feedUrl}</span>
          )}
          {podcast.feedOrigin === 'file' && (
            <span>
              <Chip tone="outline" icon="folder">
                {t('admin.podcasts.fromFolder')}
              </Chip>
            </span>
          )}
          {podcast.explicit && (
            <span>
              <Chip tone="warning">{t('admin.podcasts.explicit')}</Chip>
            </span>
          )}
        </span>
      ),
    },
    {
      key: 'library',
      header: t('admin.podcasts.columns.library'),
      cell: (podcast) => libraryName(podcast.libraryId),
    },
    {
      key: 'episodes',
      header: t('admin.podcasts.columns.episodes'),
      align: 'end',
      cell: (podcast) => (
        <span className={styles.stack}>
          <span className={styles.numbers}>{podcast.episodeCount}</span>
          {podcast.localEpisodeCount > 0 && (
            <span className={`${styles.muted} ${styles.numbers}`}>
              {t('admin.podcasts.localEpisodes', { count: podcast.localEpisodeCount })}
            </span>
          )}
        </span>
      ),
    },
    {
      key: 'lastFetch',
      header: t('admin.podcasts.columns.lastFetch'),
      cell: (podcast) =>
        podcast.lastCheckedAt ? (
          <time className={styles.nowrap} dateTime={podcast.lastCheckedAt}>
            {formatDateTime(podcast.lastCheckedAt, i18n.language)}
          </time>
        ) : (
          t('admin.podcasts.never')
        ),
    },
    {
      key: 'error',
      header: t('admin.podcasts.columns.error'),
      cell: (podcast) => {
        if (!podcast.lastError) return t('admin.podcasts.noError');
        // `feed_unreachable: HTTP 404`: the code in words, the server's text as the detail.
        const [code = '', ...detail] = podcast.lastError.split(':');
        const message = codeMessage(t, code.trim());
        return (
          <span className={styles.stack}>
            <Chip tone="danger" icon="warning">
              {message ?? podcast.lastError}
            </Chip>
            {message && <span className={styles.muted}>{detail.join(':').trim()}</span>}
          </span>
        );
      },
    },
    {
      key: 'actions',
      header: t('admin.podcasts.columns.actions'),
      hideHeader: true,
      align: 'end',
      cell: (podcast) => (
        <span className={styles.rowActions}>
          {/* A folder podcast without feed has nothing to fetch. */}
          {podcast.feedUrl && (
            <AriaButton
              variant="ghost"
              icon="refresh"
              aria-label={t('admin.podcasts.refresh.label', { title: podcast.title })}
              isPending={refresh.isPending && refresh.variables.id === podcast.id}
              onPress={() => refresh.mutate(podcast)}
            >
              {t('admin.podcasts.refresh.action')}
            </AriaButton>
          )}
          {/* A podcast a folder still owns comes back with the next scan: the folder decides, so
              there is nothing to remove here. Once the folder is gone (rootId absent), the
              subscription it left behind may go. */}
          {podcast.rootId === undefined && (
            <ConfirmDialog
              trigger={
                <AriaButton
                  variant="ghost"
                  icon="trash"
                  aria-label={t('admin.podcasts.remove.label', { title: podcast.title })}
                >
                  {t('admin.podcasts.remove.action')}
                </AriaButton>
              }
              title={t('admin.podcasts.remove.title', { title: podcast.title })}
              body={t('admin.podcasts.remove.body')}
              confirmLabel={t('admin.podcasts.remove.confirm')}
              cancelLabel={t('common.cancel')}
              tone="danger"
              onConfirm={() => remove.mutate(podcast)}
            />
          )}
        </span>
      ),
    },
  ];

  return (
    <Page>
      <div ref={header} tabIndex={-1} className={styles.header}>
        <Text as="h1" variant="display">
          {t('admin.podcasts.title')}
        </Text>
        <Text tone="secondary">{t('admin.podcasts.intro')}</Text>
      </div>
      {!allowed ? (
        <Notice title={t('admin.access.notAllowed.title')}>
          {t('admin.access.notAllowed.body')}
        </Notice>
      ) : (
        <div className={styles.body} aria-busy={podcasts.isPending}>
          {libraries.data && podcastLibraries.length === 0 ? (
            <Notice
              actions={
                <ButtonLink href={paths.adminLibraries} variant="secondary" icon="folder">
                  {t('admin.podcasts.librariesLink')}
                </ButtonLink>
              }
            >
              {t('admin.podcasts.noLibrary')}
            </Notice>
          ) : (
            <div>
              <AriaButton
                ref={subscribeButton}
                variant="primary"
                icon="plus"
                isDisabled={podcastLibraries.length === 0}
                onPress={() => setSubscribing(true)}
              >
                {t('admin.podcasts.subscribe.action')}
              </AriaButton>
            </div>
          )}
          {actionError && <Notice tone="danger">{problemMessage(t, actionError)}</Notice>}
          {message && !actionError && <Notice>{message}</Notice>}
          {podcasts.isPending && <Skeleton shape="block" />}
          {podcasts.isError && (
            <Notice
              tone="danger"
              title={problemMessage(t, podcasts.error)}
              actions={
                <AriaButton icon="refresh" onPress={() => void podcasts.refetch()}>
                  {t('common.retry')}
                </AriaButton>
              }
            />
          )}
          {podcasts.data?.length === 0 && <Notice>{t('admin.podcasts.empty')}</Notice>}
          {podcasts.data && podcasts.data.length > 0 && (
            <DataTable
              caption={t('admin.podcasts.list')}
              columns={columns}
              rows={podcasts.data}
              rowKey={(podcast) => podcast.id}
            />
          )}
        </div>
      )}
      {subscribing && (
        <SubscribeDialog
          libraries={podcastLibraries}
          onClose={() => {
            setSubscribing(false);
            subscribeButton.current?.focus();
          }}
          onSubscribed={(podcast) => {
            setSubscribing(false);
            setMessage(t('admin.podcasts.subscribe.done', { title: podcast.title }));
            void invalidate();
            subscribeButton.current?.focus();
          }}
        />
      )}
    </Page>
  );
}
