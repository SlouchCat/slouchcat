import type { AdminStream } from '@slouchcat/api-client';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { useCallback, useEffect, useId, useMemo, useRef, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { api, request } from '@/api/client';
import { problemMessage } from '@/api/errors';
import { useActiveSession } from '@/app/session';
import { reasonText } from '@/features/player/labels';
import { formatBitrate, formatDateTime } from '@/lib/format';
import { useDocumentTitle } from '@/lib/useDocumentTitle';
import { Notice, Skeleton, Text, VisuallyHidden } from '@/ui';
import {
  AdminPage,
  AriaButton,
  BandPanel,
  ConfirmDialog,
  StatusRegion,
  StatusStrip,
  SectionHeader,
  StreamTile,
  afterDialogsClosed,
  afterRender,
  focusTitleIfLost,
  useStatusMessage,
} from '@/ui/aria';
import {
  radioRelaysQuery,
  STREAMS_POLL_MS,
  stopRadioListener,
  streamsQuery,
  type RadioRelays,
} from './queries';
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

type RadioRelay = RadioRelays['relays'][number];
/** A radio listener with the relay it hears (D-11: read from `listRadioRelays`). */
interface RadioListener {
  relay: RadioRelay;
  listener: RadioRelay['listeners'][number];
}
/** What «Beenden» stops: a playback session or a listen session of the radio. */
type Stopping = { kind: 'stream'; stream: AdminStream } | ({ kind: 'radio' } & RadioListener);

/** The key of a tile in `data-stream`, unique across both lists. */
const radioKey = (listenId: string) => `radio:${listenId}`;

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
 * the bandwidth (the sum of the bit rates). Since concept 16 stage 2 the radio's listeners follow
 * in a section «Radio», one tile each, read from the radio's relays (D-11). The lists are asked
 * again every 5 s, paused while the confirmation is open. «Beenden» revokes the playback or the
 * listen session; the viewer's player stops within seconds.
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
  const [stopping, setStopping] = useState<Stopping | null>(null);
  const [confirmOpen, setConfirmOpen] = useState(false);
  const streams = useQuery({
    ...streamsQuery,
    refetchInterval: confirmOpen ? false : STREAMS_POLL_MS,
    refetchOnWindowFocus: !confirmOpen,
  });
  const relays = useQuery({
    ...radioRelaysQuery,
    refetchInterval: confirmOpen ? false : STREAMS_POLL_MS,
    refetchOnWindowFocus: !confirmOpen,
  });
  const radioHeading = useId();
  // Each message gets its own key, so the same text twice in a row is announced again.
  const [message, setMessage] = useStatusMessage();

  const stop = useMutation({
    mutationFn: (target: Stopping) =>
      target.kind === 'stream'
        ? request(api.DELETE('/admin/streams/{id}', { params: { path: { id: target.stream.id } } }))
        : stopRadioListener(target.listener.listenId),
    onSuccess: async (_data, target) => {
      if (target.kind === 'stream') {
        setMessage(t('admin.streams.stopped', { profile: target.stream.profileName }));
        await queryClient.invalidateQueries({ queryKey: streamsQuery.queryKey });
      } else {
        setMessage(t('admin.streams.radio.stopped', { profile: target.listener.profileName }));
        await queryClient.invalidateQueries({ queryKey: radioRelaysQuery.queryKey });
      }
      // The tile and its button are gone: once React rendered that, the h1 takes the focus (the
      // dialog's focus restore may have put it on another tile meanwhile).
      await afterDialogsClosed();
      const still =
        target.kind === 'stream'
          ? queryClient
              .getQueryData(streamsQuery.queryKey)
              ?.some((entry) => entry.id === target.stream.id)
          : queryClient
              .getQueryData(radioRelaysQuery.queryKey)
              ?.relays.some((relay) =>
                relay.listeners.some((entry) => entry.listenId === target.listener.listenId),
              );
      if (still) focusTitleIfLost(titleRef);
      else titleRef.current?.focus();
    },
    onError: () => setMessage(null),
  });

  const askStop = useCallback((target: Stopping) => {
    setStopping(target);
    setConfirmOpen(true);
  }, []);
  const onConfirmOpenChange = useCallback((open: boolean) => {
    if (open) return;
    setConfirmOpen(false);
    // The button that opened it may be gone by now.
    void afterDialogsClosed().then(() => focusTitleIfLost(titleRef));
  }, []);

  const listeners: RadioListener[] = useMemo(
    () =>
      (relays.data?.relays ?? []).flatMap((relay) =>
        relay.listeners.map((listener) => ({ relay, listener })),
      ),
    [relays.data],
  );

  // A stream may end on its own while its «Beenden» has the focus: when the poll takes its tile
  // away, the h1 takes the focus instead of the body.
  const focusedStream = useRef<string | null>(null);
  useEffect(() => {
    const id = focusedStream.current;
    if (id === null) return;
    // A list that has not answered yet keeps its tiles.
    const listed = id.startsWith(radioKey(''))
      ? !relays.data || listeners.some(({ listener }) => radioKey(listener.listenId) === id)
      : !streams.data || streams.data.some((stream) => stream.id === id);
    if (listed) return;
    focusedStream.current = null;
    void afterRender().then(() => focusTitleIfLost(titleRef));
  }, [streams.data, relays.data, listeners]);

  // ↑/↓ and ←/→ between the tiles: a native listener, the lists are no widgets of their own.
  const [list, setList] = useState<HTMLUListElement | null>(null);
  const [radioList, setRadioList] = useState<HTMLUListElement | null>(null);
  useEffect(() => {
    const lists = [list, radioList].filter((entry): entry is HTMLUListElement => entry !== null);
    const onFocusIn = (event: FocusEvent) => {
      const tile = (event.target as HTMLElement).closest<HTMLElement>('[data-stream]');
      focusedStream.current = tile?.dataset.stream ?? null;
    };
    const onFocusOut = (event: FocusEvent) => {
      // Gone to another element outside the lists; a tile that went away leaves no new target.
      const to = event.relatedTarget;
      if (to instanceof Node && !lists.some((entry) => entry.contains(to))) {
        focusedStream.current = null;
      }
    };
    for (const entry of lists) {
      entry.addEventListener('keydown', moveBetweenTiles);
      entry.addEventListener('focusin', onFocusIn);
      entry.addEventListener('focusout', onFocusOut);
    }
    return () => {
      for (const entry of lists) {
        entry.removeEventListener('keydown', moveBetweenTiles);
        entry.removeEventListener('focusin', onFocusIn);
        entry.removeEventListener('focusout', onFocusOut);
      }
    };
  }, [list, radioList]);

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
      <StatusRegion message={!stop.error ? message : null} />
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
              // Radio listeners cost no transcoding: one relay per station, whoever listens.
              {
                key: 'radio',
                label: t('admin.streams.load.radio.label'),
                value: relays.data ? number.format(listeners.length) : '–',
                note: t('admin.streams.load.radio.note'),
              },
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
            listeners.length === 0 && <Text tone="secondary">{t('admin.streams.empty')}</Text>
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
                        // Why it is not played directly, in the words of the player's stream
                        // info; none for direct play.
                        ...(stream.mode !== 'direct' && stream.reason
                          ? [
                              {
                                key: 'reason',
                                label: t('admin.streams.facts.reason'),
                                value: reasonText(t, stream.reason),
                                icon: 'info' as const,
                              },
                            ]
                          : []),
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
                          variant="dangerQuiet"
                          aria-label={t('admin.streams.stop.label', {
                            profile: stream.profileName,
                          })}
                          onPress={() => askStop({ kind: 'stream', stream })}
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
      {relays.isError && <Notice tone="danger" title={problemMessage(t, relays.error)} />}
      {listeners.length > 0 && (
        <section aria-labelledby={radioHeading} data-testid="streams-radio">
          <SectionHeader
            id={radioHeading}
            title={t('admin.streams.radio.title')}
            description={t('admin.streams.radio.help')}
          />
          <ul ref={setRadioList} className={styles.tiles}>
            {listeners.map(({ relay, listener }) => (
              <li key={listener.listenId} data-stream={radioKey(listener.listenId)}>
                <StreamTile
                  label={t('admin.streams.tileLabel', {
                    profile: listener.profileName,
                    title: relay.name,
                  })}
                  title={relay.name}
                  mode={t('admin.streams.radio.mode')}
                  modeTone="neutral"
                  profile={listener.profileName}
                  account={listener.accountEmail}
                  facts={[
                    {
                      key: 'quality',
                      label: t('admin.streams.facts.quality'),
                      value:
                        [
                          relay.codec,
                          relay.bitrateKbps !== undefined &&
                            t('radio.player.kbps', { count: relay.bitrateKbps }),
                        ]
                          .filter(Boolean)
                          .join(t('format.separator')) || t('admin.streams.noPosition'),
                      icon: 'radio',
                    },
                  ]}
                  progressLabel={t('admin.streams.progress', { title: relay.name })}
                  position={
                    listener.playing
                      ? t('admin.streams.radio.playing')
                      : t('admin.streams.radio.paused')
                  }
                  since={
                    <time dateTime={new Date(listener.since).toISOString()}>
                      {t('admin.streams.since', {
                        time: formatDateTime(new Date(listener.since).toISOString(), i18n.language),
                      })}
                    </time>
                  }
                  action={
                    <AriaButton
                      variant="dangerQuiet"
                      aria-label={t('admin.streams.radio.stopLabel', {
                        profile: listener.profileName,
                      })}
                      onPress={() => askStop({ kind: 'radio', relay, listener })}
                    >
                      {t('admin.streams.stop.action')}
                    </AriaButton>
                  }
                />
              </li>
            ))}
          </ul>
        </section>
      )}
      {stopping && (
        <ConfirmDialog
          isOpen={confirmOpen}
          onOpenChange={onConfirmOpenChange}
          title={
            stopping.kind === 'stream'
              ? t('admin.streams.stop.title', { profile: stopping.stream.profileName })
              : t('admin.streams.radio.stopTitle', { profile: stopping.listener.profileName })
          }
          body={
            stopping.kind === 'stream'
              ? t('admin.streams.stop.body', { title: stopping.stream.itemTitle })
              : t('admin.streams.radio.stopBody', { station: stopping.relay.name })
          }
          confirmLabel={
            stopping.kind === 'stream'
              ? t('admin.streams.stop.confirm')
              : t('admin.streams.radio.stopConfirm')
          }
          cancelLabel={t('common.cancel')}
          tone="danger"
          onConfirm={() => stop.mutate(stopping)}
        />
      )}
    </AdminPage>
  );
}
