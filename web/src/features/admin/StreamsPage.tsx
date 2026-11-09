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
import { Notice, Skeleton, Text } from '@/ui';
import {
  AdminPage,
  AriaButton,
  BandPanel,
  ConfirmDialog,
  StatusRegion,
  StatusStrip,
  SectionHeader,
  StreamTile,
  ToggleToolbar,
  afterDialogsClosed,
  afterRender,
  focusTitleIfLost,
  useStatusMessage,
  type StreamFact,
  type StreamKind,
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
type RadioListener = RadioRelay['listeners'][number];

/** The kinds of the filter (concept 18, 17.3); «Video» holds films, episodes and extras. */
type Group = 'video' | 'audiobook' | 'podcast' | 'radio';
type Filter = 'all' | Group;
const groups: readonly Group[] = ['video', 'audiobook', 'podcast', 'radio'];

/** One tile of «Läuft gerade»: a playback or a radio listener with the relay it hears (D-11). */
type Entry =
  | { kind: 'stream'; key: string; started: number; group: Group; stream: AdminStream }
  | {
      kind: 'radio';
      key: string;
      started: number;
      group: 'radio';
      relay: RadioRelay;
      listener: RadioListener;
    };

/** The key of a tile in `data-stream`, unique across both lists. */
const radioKey = (listenId: string) => `radio:${listenId}`;

const groupOf = (stream: AdminStream): Group =>
  stream.kind === 'audiobook'
    ? 'audiobook'
    : stream.kind === 'podcast_episode'
      ? 'podcast'
      : 'video';

/** The badge of a stream tile: a podcast episode is a «Podcast». */
const tileKind = (stream: AdminStream): StreamKind =>
  stream.kind === 'podcast_episode' ? 'podcast' : stream.kind;

/** Newest first by start; the id breaks a tie, so the tiles keep their places across polls. */
function byStart(a: Entry, b: Entry): number {
  if (a.started !== b.started) return b.started - a.started;
  return a.key < b.key ? 1 : a.key > b.key ? -1 : 0;
}

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
 * Streams (W-6; concept 12, mockup Str-D2; concept 18, 17.3): everything that runs in one list
 * «Läuft gerade», newest first: films, episodes and extras with their backdrop, audiobooks and
 * podcasts with their square cover, the radio's listeners with their station's logo (read from the
 * radio's relays, D-11). Above it «Serverlast» with the streams per mode, the radio listeners and
 * the bandwidth, and the kind filter with its counts. Both lists are asked again every 5 s, paused
 * while the confirmation is open; when one of them fails, the other half stays. «Beenden» revokes
 * the playback or the listen session; the viewer's player stops within seconds.
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
  const [stopping, setStopping] = useState<Entry | null>(null);
  const [confirmOpen, setConfirmOpen] = useState(false);
  // The kind filter holds across the polls, also when its kind runs out (17.3).
  const [filter, setFilter] = useState<Filter>('all');
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
  // Each message gets its own key, so the same text twice in a row is announced again.
  const [message, setMessage] = useStatusMessage();

  const stop = useMutation({
    mutationFn: (target: Entry) =>
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

  const askStop = useCallback((target: Entry) => {
    setStopping(target);
    setConfirmOpen(true);
  }, []);
  const onConfirmOpenChange = useCallback((open: boolean) => {
    if (open) return;
    setConfirmOpen(false);
    // The button that opened it may be gone by now.
    void afterDialogsClosed().then(() => focusTitleIfLost(titleRef));
  }, []);

  // A half whose request failed is unknown: its last answer is not shown (17.3, Teilausfall).
  const running = streams.isError ? undefined : streams.data;
  const radio = relays.isError ? undefined : relays.data;
  const entries: Entry[] = useMemo(
    () =>
      [
        ...(running ?? []).map((stream): Entry => ({
          kind: 'stream',
          key: stream.id,
          started: Date.parse(stream.startedAt),
          group: groupOf(stream),
          stream,
        })),
        ...(radio?.relays ?? []).flatMap((relay) =>
          relay.listeners.map((listener): Entry => ({
            kind: 'radio',
            key: radioKey(listener.listenId),
            started: listener.since,
            group: 'radio',
            relay,
            listener,
          })),
        ),
      ].sort(byStart),
    [running, radio],
  );
  const shown = filter === 'all' ? entries : entries.filter((entry) => entry.group === filter);
  const known = (group: Group) => (group === 'radio' ? radio !== undefined : running !== undefined);
  const countOf = (value: Filter) => {
    const covered = value === 'all' ? groups : [value];
    if (!covered.every(known)) return '–';
    return number.format(entries.filter((entry) => covered.includes(entry.group)).length);
  };
  const listeners = entries.filter((entry) => entry.kind === 'radio').length;

  // A stream may end on its own while its «Beenden» has the focus: when the poll takes its tile
  // away, the h1 takes the focus instead of the body.
  const focusedStream = useRef<string | null>(null);
  useEffect(() => {
    const key = focusedStream.current;
    if (key === null) return;
    // A list that has not answered yet keeps its tiles.
    const pending = key.startsWith(radioKey('')) ? !relays.data : !streams.data;
    if (pending || shown.some((entry) => entry.key === key)) return;
    focusedStream.current = null;
    void afterRender().then(() => focusTitleIfLost(titleRef));
  }, [streams.data, relays.data, shown]);

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

  const count = (load: StreamLoad) =>
    (running ?? []).filter((stream) => streamLoad(stream) === load).length;
  const separator = t('format.separator');
  const kbps = (bitsPerSecond: number) =>
    t('radio.player.kbps', { count: Math.round(bitsPerSecond / 1000) });
  const since = (iso: string) => (
    <time dateTime={iso}>
      {t('admin.streams.since', { time: formatDateTime(iso, i18n.language) })}
    </time>
  );
  const device = (named?: { name?: string }) => ({
    key: 'device',
    label: t('admin.streams.facts.device'),
    value: named?.name ?? t('admin.streams.unknownDevice'),
    icon: 'monitor' as const,
  });

  /** The tile of a playback (concept 18, 17.3: title, image and facts by kind). */
  const streamTile = (entry: Extract<Entry, { kind: 'stream' }>) => {
    const { stream } = entry;
    const position = streamPosition(t, stream);
    const audio = stream.kind === 'audiobook' || stream.kind === 'podcast_episode';
    const title = [
      stream.kind === 'extra' || stream.kind === 'podcast_episode' ? stream.parentTitle : undefined,
      stream.itemTitle,
      stream.kind === 'audiobook' ? stream.chapterTitle : undefined,
    ]
      .filter(Boolean)
      .join(separator);
    const format =
      [
        stream.audioCodec?.toUpperCase(),
        stream.audioBitrate !== undefined && kbps(stream.audioBitrate),
        stream.partCount !== undefined && t('admin.streams.parts', { count: stream.partCount }),
      ]
        .filter(Boolean)
        .join(separator) || t('admin.streams.noPosition');
    const facts: StreamFact[] = audio
      ? [
          device(stream.device),
          {
            key: 'format',
            label: t('admin.streams.facts.format'),
            value: format,
            icon: 'audioTrack',
          },
          ...(stream.kind === 'podcast_episode'
            ? [
                {
                  key: 'source',
                  label: t('admin.streams.facts.source'),
                  value: t(`admin.streams.source.${stream.source}`),
                  icon: stream.source === 'feed' ? ('external' as const) : ('disk' as const),
                },
              ]
            : []),
        ]
      : [
          device(stream.device),
          {
            key: 'quality',
            label: t('admin.streams.facts.quality'),
            value: streamQuality(t, stream, i18n.language),
            icon: 'film',
          },
          // Why it is not played directly, in the words of the player's stream info; none for
          // direct play.
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
        ];
    return (
      <StreamTile
        label={t('admin.streams.tileLabel', { profile: stream.profileName, title })}
        title={title}
        kind={tileKind(stream)}
        kindLabel={t(`admin.streams.kinds.${tileKind(stream)}`)}
        image={
          audio
            ? { form: 'cover', src: stream.posterUrl }
            : { form: 'backdrop', src: stream.backdropUrl }
        }
        mode={streamMode(t, stream)}
        modeTone={loadTone[streamLoad(stream)]}
        profile={stream.profileName}
        account={stream.accountEmail}
        facts={facts}
        progress={position.percent}
        progressLabel={t('admin.streams.progress', { title })}
        position={position.text}
        since={since(stream.startedAt)}
        action={
          <AriaButton
            variant="dangerQuiet"
            aria-label={t('admin.streams.stop.label', { profile: stream.profileName })}
            onPress={() => askStop(entry)}
          >
            {t('admin.streams.stop.action')}
          </AriaButton>
        }
      />
    );
  };

  /** The tile of a radio listener: the station's logo, format, listeners and state. */
  const radioTile = (entry: Extract<Entry, { kind: 'radio' }>) => {
    const { relay, listener } = entry;
    const format =
      [
        relay.codec,
        relay.bitrateKbps !== undefined && t('radio.player.kbps', { count: relay.bitrateKbps }),
        relay.codec !== undefined &&
          (relay.transcoded
            ? t('admin.streams.radio.transcoded')
            : t('admin.streams.radio.copied')),
      ]
        .filter(Boolean)
        .join(separator) || t('admin.streams.noPosition');
    const state =
      relay.state === 'failed'
        ? t('admin.streams.radio.failed')
        : listener.playing
          ? t('admin.streams.radio.playing')
          : t('admin.streams.radio.paused');
    return (
      <StreamTile
        label={t('admin.streams.tileLabel', { profile: listener.profileName, title: relay.name })}
        title={relay.name}
        kind="radio"
        kindLabel={t('admin.streams.kinds.radio')}
        image={{ form: 'logo', src: relay.logo?.url }}
        profile={listener.profileName}
        account={listener.accountEmail}
        facts={[
          device(listener.device),
          { key: 'format', label: t('admin.streams.facts.format'), value: format, icon: 'radio' },
          {
            key: 'listeners',
            label: t('admin.streams.facts.listeners'),
            value: t('admin.streams.radio.listeners', { count: relay.listeners.length }),
            icon: 'users',
          },
        ]}
        progressLabel={t('admin.streams.progress', { title: relay.name })}
        position={state}
        since={since(new Date(listener.since).toISOString())}
        action={
          <AriaButton
            variant="dangerQuiet"
            aria-label={t('admin.streams.radio.stopLabel', { profile: listener.profileName })}
            onPress={() => askStop(entry)}
          >
            {t('admin.streams.stop.action')}
          </AriaButton>
        }
      />
    );
  };

  const pending = streams.isPending || relays.isPending;
  // Nothing runs: no filter says the page's own text, a filter the kind's; a half that failed
  // leaves its kinds unknown, so no text claims they are empty.
  const empty =
    !pending && shown.length === 0 && (filter === 'all' ? groups : [filter]).every(known);

  return (
    <AdminPage
      title={t('admin.streams.title')}
      help={t('admin.streams.intro')}
      liveSeconds={confirmOpen ? undefined : STREAMS_POLL_MS / 1000}
      titleRef={titleRef}
    >
      {stop.error && <Notice tone="danger">{problemMessage(t, stop.error)}</Notice>}
      <StatusRegion message={!stop.error ? message : null} />
      {running && (
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
                value: radio ? number.format(listeners) : '–',
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
      <section aria-labelledby={listHeading} aria-busy={pending}>
        <SectionHeader
          id={listHeading}
          title={t('admin.streams.list')}
          description={t('admin.streams.listHelp')}
        />
        <div className={styles.list}>
          <ToggleToolbar
            label={t('admin.streams.filter.label')}
            value={filter}
            onChange={(value) => setFilter(value as Filter)}
            options={(['all', ...groups] as const).map((value) => ({
              value,
              label: t(`admin.streams.filter.${value}`),
              count: countOf(value),
              countLabel: countOf(value) === '–' ? t('admin.streams.filter.unknown') : undefined,
            }))}
          />
          {streams.isError && (
            <Notice
              tone="danger"
              title={problemMessage(t, streams.error)}
              actions={
                <AriaButton icon="refresh" onPress={() => void streams.refetch()}>
                  {t('common.retry')}
                </AriaButton>
              }
            >
              {t('admin.streams.partial.streams')}
            </Notice>
          )}
          {relays.isError && (
            <Notice
              tone="danger"
              title={problemMessage(t, relays.error)}
              actions={
                <AriaButton icon="refresh" onPress={() => void relays.refetch()}>
                  {t('common.retry')}
                </AriaButton>
              }
            >
              {t('admin.streams.partial.radio')}
            </Notice>
          )}
          {pending ? (
            <Skeleton shape="block" />
          ) : empty ? (
            <Text tone="secondary">
              {filter === 'all' ? t('admin.streams.empty') : t('admin.streams.emptyKind')}
            </Text>
          ) : (
            shown.length > 0 && (
              <ul ref={setList} className={styles.tiles}>
                {shown.map((entry) => (
                  <li key={entry.key} data-stream={entry.key}>
                    {entry.kind === 'stream' ? streamTile(entry) : radioTile(entry)}
                  </li>
                ))}
              </ul>
            )
          )}
        </div>
      </section>
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
