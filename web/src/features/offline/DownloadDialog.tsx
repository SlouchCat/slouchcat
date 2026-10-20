import type { ItemRef } from '@slouchcat/api-client';
import { useMutation, useQuery } from '@tanstack/react-query';
import { useEffect, useRef, useState, type FormEvent } from 'react';
import { Form } from 'react-aria-components';
import { useTranslation } from 'react-i18next';
import { formErrors, problemMessage } from '@/api/errors';
import { useActiveSession } from '@/app/session';
import { trackLabel } from '@/features/player/labels';
import { formatBytes } from '@/lib/format';
import { Notice, Skeleton, Text } from '@/ui';
import { AriaButton, FormDialog, RadioGroup } from '@/ui/aria';
import { createDownloads, persistAsked, persistOnFirstDownload } from './actions';
import { qualityLabel } from './labels';
import { downloadOptionsQuery } from './queries';
import type { DownloadQuality, DownloadRequest, DownloadState } from './types';
import styles from './OfflinePages.module.css';

const DEFAULT_QUALITY: DownloadQuality = 'medium';

interface DownloadDialogProps {
  /** A movie, an episode or a season. */
  target: Pick<ItemRef, 'id' | 'kind' | 'title'>;
  /** The version of a single title (B-5); the primary file otherwise. */
  mediaFileId?: string;
  onClose: () => void;
  onCreated: (downloads: DownloadState[]) => void;
}

/**
 * "Herunterladen" (O-1, O-2): the three download profiles with the expected size, the audio track
 * of a single title, the number of episodes of a season. Creating the download asks the browser
 * once to keep the storage (persist) and wakes the engine, which fetches once the server is ready.
 */
export function DownloadDialog({ target, mediaFileId, onClose, onCreated }: DownloadDialogProps) {
  const { t, i18n } = useTranslation();
  const session = useActiveSession();
  const season = target.kind === 'season';
  const options = useQuery(downloadOptionsQuery(target.id, season ? undefined : mediaFileId));
  const [quality, setQuality] = useState<string | null>(null);
  const [audio, setAudio] = useState<string | null>(null);
  const asked = useRef(true);
  useEffect(() => {
    void persistAsked().then((value) => {
      asked.current = value;
    });
  }, []);
  const create = useMutation({
    mutationFn: (body: DownloadRequest) => createDownloads(target.id, body, session.account.id),
    onSuccess: onCreated,
  });
  const errors = create.error ? formErrors(t, create.error) : null;
  const data = options.data;
  const qualities = data?.qualities ?? [];
  const chosenQuality =
    quality ??
    qualities.find((option) => option.quality === DEFAULT_QUALITY)?.quality ??
    qualities[0]?.quality ??
    null;
  const audioTracks = season ? [] : (data?.audioTracks ?? []);
  const defaultAudio = audioTracks.find((track) => track.default) ?? audioTracks[0];
  const chosenAudio = audio ?? (defaultAudio ? String(defaultAudio.index) : null);
  const language = i18n.language;

  const onSubmit = (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    if (!chosenQuality || create.isPending) return;
    // Within the click: Firefox asks the viewer and wants the user's gesture.
    persistOnFirstDownload(asked.current);
    asked.current = true;
    create.mutate({
      quality: chosenQuality as DownloadQuality,
      ...(!season && chosenAudio !== null && audioTracks.length > 1
        ? { audioTrack: Number(chosenAudio) }
        : {}),
      ...(!season && mediaFileId ? { mediaFileId } : {}),
    });
  };

  const subtitleNames = (data?.subtitles ?? []).map((track, index) =>
    trackLabel(t, track, index, language),
  );

  return (
    <FormDialog
      isOpen
      onOpenChange={(open) => {
        if (!open) onClose();
      }}
      title={t('offline.dialog.title', { title: target.title })}
      description={
        season && data?.episodeCount !== undefined
          ? t('offline.dialog.seasonIntro', { count: data.episodeCount })
          : t('offline.dialog.intro')
      }
    >
      {options.isPending && <Skeleton shape="block" />}
      {options.isError && (
        <div className={styles.form}>
          <Notice tone="danger">{problemMessage(t, options.error)}</Notice>
          <div className={styles.formActions}>
            <AriaButton variant="ghost" onPress={onClose} autoFocus>
              {t('common.close')}
            </AriaButton>
          </div>
        </div>
      )}
      {data && (
        <Form className={styles.form} onSubmit={onSubmit} validationErrors={errors?.fields}>
          {errors?.form && <Notice tone="danger">{errors.form}</Notice>}
          <RadioGroup
            name="quality"
            label={t('offline.dialog.quality')}
            value={chosenQuality ?? ''}
            onChange={setQuality}
            options={qualities.map((option) => ({
              value: option.quality,
              label: t('offline.dialog.option', {
                quality: qualityLabel(t, option.quality),
                height: option.height,
                size: formatBytes(option.estimatedBytes, language),
              }),
            }))}
          />
          {audioTracks.length > 1 && (
            <RadioGroup
              name="audioTrack"
              label={t('offline.dialog.audio')}
              value={chosenAudio ?? ''}
              onChange={setAudio}
              options={audioTracks.map((track, index) => ({
                value: String(track.index),
                label: trackLabel(t, track, index, language),
              }))}
            />
          )}
          {!season && (
            <Text tone="secondary" variant="caption">
              {subtitleNames.length > 0
                ? t('offline.dialog.subtitles', { list: subtitleNames.join(t('format.separator')) })
                : t('offline.dialog.noSubtitles')}
            </Text>
          )}
          <div className={styles.formActions}>
            <AriaButton variant="ghost" onPress={onClose}>
              {t('common.cancel')}
            </AriaButton>
            <AriaButton
              type="submit"
              variant="primary"
              icon="download"
              isDisabled={!chosenQuality}
              isPending={create.isPending}
            >
              {create.isPending ? t('offline.dialog.submitting') : t('offline.dialog.submit')}
            </AriaButton>
          </div>
        </Form>
      )}
    </FormDialog>
  );
}
