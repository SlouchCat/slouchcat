import type { AdminStream } from '@slouchcat/api-client';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { useCallback, useEffect, useId, useMemo, useRef, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { api, request } from '@/api/client';
import { problemMessage } from '@/api/errors';
import { useActiveSession } from '@/app/session';
import { formatBitrate, formatDateTime } from '@/lib/format';
import { useDocumentTitle } from '@/lib/useDocumentTitle';
import { Notice, Skeleton, Text, VisuallyHidden } from '@/ui';
import {
  AdminPage,
  AriaButton,
  BandPanel,
  ConfirmDialog,
  StatusStrip,
  StreamTile,
  afterRender,
  focusTitleIfLost,
} from '@/ui/aria';
import { STREAMS_POLL_MS, streamsQuery } from './queries';
import {
  loadTone,
  streamLoad,
  streamLoads,
  streamMode,
  streamPosition,
  streamQuality,
  totalBitrate,
  type StreamLoad,
} from './streams/format';
import styles from './streams/Streams.module.css';

/** The number of columns the tile grid has right now (its computed tracks); 1 without layout. */
function columnsOf(list: HTMLElement): number {
  const tracks = getComputedStyle(list).gridTemplateColumns.split(' ');
  return Math.max(1, tracks.filter((track) => track.endsWith('px')).length);
}

/**
 * The arrows move between the tiles' «Beenden»: ←/→ to the previous or next tile in reading order,
 * ↑/↓ to the tile above or below in the same column.
 */
function moveBetweenTiles(event: KeyboardEvent) {
  if (event.altKey || event.ctrlKey || event.metaKey || event.shiftKey) return;
  const list = event.currentTarget as HTMLElement;
  const steps: Record<string, () => number> = {
    ArrowRight: () => 1,
    ArrowLeft: () => -1,
    ArrowDown: () => columnsOf(list),
    ArrowUp: () => -columnsOf(list),
  };
  const step = steps[event.key]?.();
  if (step === undefined) return;
  const buttons = [...list.querySelectorAll<HTMLElement>('button')];
  const index = buttons.indexOf(event.target as HTMLElement);
  const next = index < 0 ? undefined : buttons[index + step];
  if (!next) return;
  event.preventDefault();
  next.focus();
}

/**
 * Streams (W-6; concept 12, mockup Str-D2): the running playbacks as tiles with the backdrop,
 * profile, device, quality and progress, and above them «Serverlast» with the streams per mode and
 * the bandwidth (the sum of the bit rates). The list is asked again every 5 s, paused while the
 * confirmation is open. «Beenden» revokes the playback session; the viewer's player stops within
 * seconds.
 */
export function StreamsPage() {
  const { t } = useTranslation();
  useDocumentTitle(t('admin.streams.title'));
  const session = useActiveSession();
  const allowed = session.account.role === 'admin' && session.profile.isPrimary;
  if (!allowed) {
    return (
      <AdminPage title={t('admin.streams.title')}>
        <Notice title={t('admin.access.notAllowed.title')}>
          {t('admin.access.notAllowed.body')}
        </Notice>
      </AdminPage>
    );
  }
  return <StreamsContent />;
}

function StreamsContent() {
  const { t, i18n } = useTranslation();
  const queryClient = useQueryClient();
  const number = useMemo(() => new Intl.NumberFormat(i18n.language), [i18n.language]);
  const titleRef = useRef<HTMLHeadingElement>(null);
  const listHeading = useId();

  // The confirmation is the page's own: a tile that goes while it is open (the stream ended)
  // cannot take the dialog with it. The poll waits while it is open.
  const [stopping, setStopping] = useState<AdminStream | null>(null);
  const [confirmOpen, setConfirmOpen] = useState(false);
  const streams = useQuery({
    ...streamsQuery,
    refetchInterval: confirmOpen ? false : STREAMS_POLL_MS,
    refetchOnWindowFocus: !confirmOpen,
  });
  // Each message gets its own key, so the same text twice in a row is announced again.
  const [message, setMessage] = useState<{ text: string; key: number } | null>(null);

  const stop = useMutation({
    mutationFn: (stream: AdminStream) =>
      request(api.DELETE('/admin/streams/{id}', { params: { path: { id: stream.id } } })),
    onSuccess: async (_data, stream) => {
      setMessage((current) => ({
        text: t('admin.streams.stopped', { profile: stream.profileName }),
        key: (current?.key ?? 0) + 1,
      }));
      await queryClient.invalidateQueries({ queryKey: streamsQuery.queryKey });
      // The tile and its button are gone: once React rendered that, the h1 takes the focus (the
      // dialog's focus restore may have put it on another tile meanwhile).
      await afterRender();
      const listed = queryClient.getQueryData(streamsQuery.queryKey);
      if (listed?.some((entry) => entry.id === stream.id)) focusTitleIfLost(titleRef);
      else titleRef.current?.focus();
    },
    onError: () => setMessage(null),
  });

  const askStop = useCallback((stream: AdminStream) => {
    setStopping(stream);
    setConfirmOpen(true);
  }, []);
  const onConfirmOpenChange = useCallback((open: boolean) => {
    if (open) return;
    setConfirmOpen(false);
    // The button that opened it may be gone by now.
    void afterRender().then(() => focusTitleIfLost(titleRef));
  }, []);

  // A stream may end on its own while its «Beenden» has the focus: when the poll takes its tile
  // away, the h1 takes the focus instead of the body.
  const focusedStream = useRef<string | null>(null);
  useEffect(() => {
    const id = focusedStream.current;
    if (id === null || !streams.data || streams.data.some((stream) => stream.id === id)) return;
    focusedStream.current = null;
    void afterRender().then(() => focusTitleIfLost(titleRef));
  }, [streams.data]);

  // ↑/↓ and ←/→ between the tiles: a native listener, the list is no widget of its own.
  const [list, setList] = useState<HTMLUListElement | null>(null);
  useEffect(() => {
    if (!list) return;
    const onFocusIn = (event: FocusEvent) => {
      const tile = (event.target as HTMLElement).closest<HTMLElement>('[data-stream]');
      focusedStream.current = tile?.dataset.stream ?? null;
    };
    const onFocusOut = (event: FocusEvent) => {
      // Gone to another element outside the list; a tile that went away leaves no new target.
      const to = event.relatedTarget;
      if (to instanceof Node && !list.contains(to)) focusedStream.current = null;
    };
    list.addEventListener('keydown', moveBetweenTiles);
    list.addEventListener('focusin', onFocusIn);
    list.addEventListener('focusout', onFocusOut);
    return () => {
      list.removeEventListener('keydown', moveBetweenTiles);
      list.removeEventListener('focusin', onFocusIn);
      list.removeEventListener('focusout', onFocusOut);
    };
  }, [list]);

  const running = streams.data ?? [];
  const count = (load: StreamLoad) =>
    running.filter((stream) => streamLoad(stream) === load).length;

  return (
    <AdminPage
      title={t('admin.streams.title')}
      help={t('admin.streams.intro')}
      liveSeconds={confirmOpen ? undefined : STREAMS_POLL_MS / 1000}
      titleRef={titleRef}
    >
      {stop.error && <Notice tone="danger">{problemMessage(t, stop.error)}</Notice>}
      <div role="status" className={styles.live}>
        {message && !stop.error && (
          <Notice key={message.key} live={false}>
            {message.text}
          </Notice>
        )}
      </div>
      {streams.isError && (
        <Notice
          tone="danger"
          title={problemMessage(t, streams.error)}
          actions={
            <AriaButton icon="refresh" onPress={() => void streams.refetch()}>
              {t('common.retry')}
            </AriaButton>
          }
        />
      )}
      {streams.data && (
        <BandPanel title={t('admin.streams.load.title')} icon="server">
          <StatusStrip
            label={t('admin.streams.load.title')}
            tiles={[
              ...streamLoads.map((load) => ({
                key: load,
                label: t(`admin.streams.load.${load}.label`),
                value: number.format(count(load)),
                note: t(`admin.streams.load.${load}.note`),
                tone: loadTone[load],
              })),
              {
                key: 'bandwidth',
                label: t('admin.streams.load.bandwidth.label'),
                value: formatBitrate(totalBitrate(running), i18n.language),
                // Direct play reports no bit rate: then the sum covers only the known ones.
                note: running.every((stream) => stream.bitrate !== undefined)
                  ? t('admin.streams.load.bandwidth.note')
                  : t('admin.streams.load.bandwidth.notePartial'),
              },
            ]}
          />
        </BandPanel>
      )}
      {!streams.isError && (
        <section aria-labelledby={listHeading} aria-busy={streams.isPending}>
          <VisuallyHidden as="h2" id={listHeading}>
            {t('admin.streams.list')}
          </VisuallyHidden>
          {streams.isPending ? (
            <Skeleton shape="block" />
          ) : running.length === 0 ? (
            <Text tone="secondary">{t('admin.streams.empty')}</Text>
          ) : (
            <ul ref={setList} className={styles.tiles}>
              {running.map((stream) => {
                const position = streamPosition(t, stream);
                return (
                  <li key={stream.id} data-stream={stream.id}>
                    <StreamTile
                      label={t('admin.streams.tileLabel', {
                        profile: stream.profileName,
                        title: stream.itemTitle,
                      })}
                      title={stream.itemTitle}
                      backdropUrl={stream.backdropUrl}
                      mode={streamMode(t, stream)}
                      modeTone={loadTone[streamLoad(stream)]}
                      profile={stream.profileName}
                      account={stream.accountEmail}
                      facts={[
                        {
                          key: 'device',
                          label: t('admin.streams.facts.device'),
                          value: stream.device?.name ?? t('admin.streams.unknownDevice'),
                          icon: 'monitor',
                        },
                        {
                          key: 'quality',
                          label: t('admin.streams.facts.quality'),
                          value: streamQuality(t, stream, i18n.language),
                          icon: 'film',
                        },
                      ]}
                      progress={position.percent}
                      progressLabel={t('admin.streams.progress', { title: stream.itemTitle })}
                      position={position.text}
                      since={
                        <time dateTime={stream.startedAt}>
                          {t('admin.streams.since', {
                            time: formatDateTime(stream.startedAt, i18n.language),
                          })}
                        </time>
                      }
                      action={
                        <AriaButton
                          variant="danger"
                          aria-label={t('admin.streams.stop.label', {
                            profile: stream.profileName,
                          })}
                          onPress={() => askStop(stream)}
                        >
                          {t('admin.streams.stop.action')}
                        </AriaButton>
                      }
                    />
                  </li>
                );
              })}
            </ul>
          )}
        </section>
      )}
      {stopping && (
        <ConfirmDialog
          isOpen={confirmOpen}
          onOpenChange={onConfirmOpenChange}
          title={t('admin.streams.stop.title', { profile: stopping.profileName })}
          body={t('admin.streams.stop.body', { title: stopping.itemTitle })}
          confirmLabel={t('admin.streams.stop.confirm')}
          cancelLabel={t('common.cancel')}
          tone="danger"
          onConfirm={() => stop.mutate(stopping)}
        />
      )}
    </AdminPage>
  );
}
