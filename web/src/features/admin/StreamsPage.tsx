import type { AdminStream } from '@slouchcat/api-client';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import type { TFunction } from 'i18next';
import { useState } from 'react';
import { useTranslation } from 'react-i18next';
import { api, request } from '@/api/client';
import { problemMessage } from '@/api/errors';
import { useActiveSession } from '@/app/session';
import { formatBitrate, formatDateTime, formatPlayerTime } from '@/lib/format';
import { useDocumentTitle } from '@/lib/useDocumentTitle';
import { Notice, Page, Skeleton, Text } from '@/ui';
import { AriaButton, ConfirmDialog, DataTable, type DataColumn } from '@/ui/aria';
import { streamsQuery } from './queries';
import styles from './AdminTablePage.module.css';

/** "Direct", "Remux" or "Transcode (HW)" / "Transcode (SW)" (W-6). */
export function streamMode(t: TFunction, stream: AdminStream): string {
  if (stream.mode !== 'transcode') return t(`admin.streams.modes.${stream.mode}`);
  return t('admin.streams.modes.transcode', {
    encoder: stream.hardware ? t('admin.streams.hw') : t('admin.streams.sw'),
  });
}

/** The output of the stream: rung or height plus bit rate; the source as it is for direct play. */
function streamQuality(t: TFunction, stream: AdminStream, language: string): string {
  const rung =
    stream.quality && stream.quality !== 'auto'
      ? stream.quality
      : stream.height
        ? `${stream.height}p`
        : undefined;
  if (!rung) return t('admin.streams.original');
  return stream.bitrate
    ? t('admin.streams.qualityValue', {
        quality: rung,
        bitrate: formatBitrate(stream.bitrate, language),
      })
    : rung;
}

/**
 * Streams (W-6): the running playbacks, refreshed every 5 s, with profile, device, title, quality
 * and mode. "Beenden" revokes the playback session; the viewer's player stops within seconds.
 */
export function StreamsPage() {
  const { t, i18n } = useTranslation();
  useDocumentTitle(t('admin.streams.title'));
  const session = useActiveSession();
  const allowed = session.account.role === 'owner' && session.profile.isPrimary;
  const queryClient = useQueryClient();
  const streams = useQuery({ ...streamsQuery, enabled: allowed });
  const [stopped, setStopped] = useState<string | null>(null);
  const stop = useMutation({
    mutationFn: (stream: AdminStream) =>
      request(api.DELETE('/admin/streams/{id}', { params: { path: { id: stream.id } } })),
    onSuccess: async (_data, stream) => {
      setStopped(stream.profileName);
      await queryClient.invalidateQueries({ queryKey: streamsQuery.queryKey });
    },
  });

  const columns: DataColumn<AdminStream>[] = [
    {
      key: 'profile',
      header: t('admin.streams.columns.profile'),
      rowHeader: true,
      cell: (stream) => (
        <span className={styles.stack}>
          <span>{stream.profileName}</span>
          <span className={styles.muted}>{stream.accountEmail}</span>
        </span>
      ),
    },
    {
      key: 'device',
      header: t('admin.streams.columns.device'),
      cell: (stream) => stream.device?.name ?? t('admin.streams.unknownDevice'),
    },
    { key: 'title', header: t('admin.streams.columns.title'), cell: (stream) => stream.itemTitle },
    {
      key: 'quality',
      header: t('admin.streams.columns.quality'),
      cell: (stream) => (
        <span className={styles.nowrap}>{streamQuality(t, stream, i18n.language)}</span>
      ),
    },
    {
      key: 'mode',
      header: t('admin.streams.columns.mode'),
      cell: (stream) => <span className={styles.nowrap}>{streamMode(t, stream)}</span>,
    },
    {
      key: 'position',
      header: t('admin.streams.columns.position'),
      cell: ({ positionMs, durationMs }) =>
        positionMs === undefined ? (
          '–'
        ) : (
          <span className={styles.numbers}>
            {durationMs
              ? t('admin.streams.position', {
                  position: formatPlayerTime(positionMs, durationMs),
                  duration: formatPlayerTime(durationMs, durationMs),
                })
              : formatPlayerTime(positionMs, positionMs)}
          </span>
        ),
    },
    {
      key: 'since',
      header: t('admin.streams.columns.since'),
      cell: (stream) => (
        <time className={styles.nowrap} dateTime={stream.startedAt}>
          {formatDateTime(stream.startedAt, i18n.language)}
        </time>
      ),
    },
    {
      key: 'actions',
      header: t('admin.streams.columns.actions'),
      hideHeader: true,
      align: 'end',
      cell: (stream) => (
        <ConfirmDialog
          trigger={
            <AriaButton
              variant="ghost"
              aria-label={t('admin.streams.stop.label', { profile: stream.profileName })}
            >
              {t('admin.streams.stop.action')}
            </AriaButton>
          }
          title={t('admin.streams.stop.title', { profile: stream.profileName })}
          body={t('admin.streams.stop.body', { title: stream.itemTitle })}
          confirmLabel={t('admin.streams.stop.confirm')}
          cancelLabel={t('common.cancel')}
          tone="danger"
          onConfirm={() => stop.mutate(stream)}
        />
      ),
    },
  ];

  return (
    <Page>
      <div className={styles.header}>
        <Text as="h1" variant="display">
          {t('admin.streams.title')}
        </Text>
        <Text tone="secondary">{t('admin.streams.intro')}</Text>
      </div>
      {!allowed ? (
        <Notice title={t('admin.access.notAllowed.title')}>
          {t('admin.access.notAllowed.body')}
        </Notice>
      ) : (
        <div className={styles.body} aria-busy={streams.isPending}>
          {stop.error && <Notice tone="danger">{problemMessage(t, stop.error)}</Notice>}
          {stopped && !stop.error && (
            <Notice>{t('admin.streams.stopped', { profile: stopped })}</Notice>
          )}
          {streams.isPending && <Skeleton shape="block" />}
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
          {streams.data?.length === 0 && <Notice>{t('admin.streams.empty')}</Notice>}
          {streams.data && streams.data.length > 0 && (
            <DataTable
              caption={t('admin.streams.list')}
              columns={columns}
              rows={streams.data}
              rowKey={(stream) => stream.id}
            />
          )}
        </div>
      )}
    </Page>
  );
}
