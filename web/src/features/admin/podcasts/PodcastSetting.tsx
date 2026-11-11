import type { AdminPodcast, PodcastUpdate } from '@slouchcat/api-client';
import { useMutation, useQueryClient } from '@tanstack/react-query';
import { useCallback, useState, type FormEvent } from 'react';
import { useTranslation } from 'react-i18next';
import { api, request } from '@/api/client';
import { problemMessage } from '@/api/errors';
import { formatBytes } from '@/lib/format';
import { Notice, Text } from '@/ui';
import { AriaButton, RadioGroup, StatusRegion, TextField, useStatusMessage } from '@/ui/aria';
import { podcastsQuery } from '../queries';
import { estimatePodcastDownloads, LAST_N_MAX, useLoadConfirmation } from './downloads';
import styles from './Podcasts.module.css';

/** The own setting of a podcast as `updatePodcast` takes it; `lastN` with an own scope. */
interface OwnSetting {
  mode: NonNullable<PodcastUpdate['mode']>;
  scope: NonNullable<PodcastUpdate['scope']>;
  lastN?: number;
}

/** What a confirmation waits for: the own setting, or «Fehlende nachladen». */
type Change = { kind: 'setting'; setting: OwnSetting } | { kind: 'missing' };

const MODES = ['inherit', 'save', 'link'] as const;
const SCOPES = ['inherit', 'new', 'last_n', 'all'] as const;

/**
 * The stored episodes of one podcast in its fold-out (concept 19 part A, 12.3, stage 1): the own
 * setting «Wie Bibliothek», «Speichern» or «Nur Link» with its scope, what applies (the server's
 * `effective*`), and «Fehlende nachladen». A change that loads once and the button show the
 * server's estimate with «Laden» and «Abbrechen» first. «Speichern» is locked when the server says
 * the podcast cannot store (`savable`).
 */
export function PodcastSetting({ podcast }: { podcast: AdminPodcast }) {
  const { t, i18n } = useTranslation();
  const queryClient = useQueryClient();
  const number = new Intl.NumberFormat(i18n.language);
  const [message, say] = useStatusMessage();
  const [failure, setFailure] = useState<unknown>(null);
  const saved: OwnSetting = {
    mode: podcast.mode ?? 'inherit',
    scope: podcast.scope ?? 'inherit',
    lastN: podcast.lastN,
  };

  const done = async (text: string) => {
    setFailure(null);
    say(text);
    await queryClient.invalidateQueries({ queryKey: podcastsQuery.queryKey });
  };
  const failed = (error: unknown) => {
    say(null);
    setFailure(error);
  };
  const update = useMutation({
    mutationFn: (setting: OwnSetting): Promise<AdminPodcast> =>
      request(
        api.PATCH('/admin/podcasts/{id}', {
          params: { path: { id: podcast.id } },
          body:
            setting.scope === 'last_n'
              ? { mode: setting.mode, scope: setting.scope, lastN: setting.lastN }
              : { mode: setting.mode, scope: setting.scope },
        }),
      ),
    onSuccess: () => done(t('admin.podcasts.setting.saved')),
    onError: failed,
  });
  const download = useMutation({
    mutationFn: (): Promise<{ queued: number }> =>
      request(api.POST('/admin/podcasts/{id}/download', { params: { path: { id: podcast.id } } })),
    onSuccess: ({ queued }) =>
      done(
        queued === 0
          ? t('admin.podcasts.setting.missing.none')
          : t('admin.podcasts.setting.missing.queued', {
              count: queued,
              number: number.format(queued),
            }),
      ),
    onError: failed,
  });
  const { mutate: mutateSetting } = update;
  const { mutate: mutateDownload } = download;
  const apply = useCallback(
    (change: Change) => {
      if (change.kind === 'setting') mutateSetting(change.setting);
      else mutateDownload();
    },
    [mutateSetting, mutateDownload],
  );
  const confirmation = useLoadConfirmation(apply);
  const asked = confirmation.shown;
  const value =
    (asked?.kind === 'setting' ? asked.setting : undefined) ??
    (update.isPending ? update.variables : undefined) ??
    saved;

  const estimate = (s: OwnSetting) =>
    estimatePodcastDownloads(podcast.libraryId, {
      podcastId: podcast.id,
      mode: s.mode,
      scope: s.scope,
      ...(s.scope === 'last_n' ? { lastN: s.lastN } : {}),
    });
  const change = (setting: OwnSetting) => {
    say(null);
    // «Nur Link» never loads; everything else asks the server what it loads more than now.
    if (setting.mode === 'link') {
      confirmation.reset();
      apply({ kind: 'setting', setting });
      return;
    }
    confirmation.ask(
      { kind: 'setting', setting },
      () => estimate(setting),
      () => estimate(saved),
    );
  };
  const askMissing = () => {
    say(null);
    confirmation.ask(
      { kind: 'missing' },
      () =>
        estimatePodcastDownloads(podcast.libraryId, {
          podcastId: podcast.id,
          mode: 'save',
          scope: 'all',
        }),
      () => Promise.resolve(null),
    );
  };

  const [lastN, setLastN] = useState(String(value.lastN ?? podcast.effectiveLastN ?? ''));
  const onLastN = (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    const n = Number(lastN.trim());
    if (!Number.isInteger(n) || n < 1 || n > LAST_N_MAX || n === value.lastN) return;
    change({ ...value, scope: 'last_n', lastN: n });
  };

  const scopeName = (scope: string, n?: number) =>
    scope === 'last_n'
      ? t('admin.podcasts.downloads.scope.lastNOf', { n })
      : t(`admin.podcasts.downloads.scope.${scope as 'new' | 'all'}`);
  const effective =
    podcast.effectiveMode === 'save'
      ? t('admin.podcasts.setting.effective.save', {
          scope: scopeName(podcast.effectiveScope ?? 'new', podcast.effectiveLastN),
        })
      : t('admin.podcasts.setting.effective.link');
  const pending = confirmation.pending;
  const error = failure ?? confirmation.failure;
  const missing = Math.max(0, podcast.episodeCount - podcast.localEpisodeCount);

  return (
    <div className={styles.setting}>
      <Text tone="secondary">{effective}</Text>
      {!podcast.savable && <Text tone="secondary">{t('admin.podcasts.setting.notSavable')}</Text>}
      <RadioGroup
        label={t('admin.podcasts.setting.mode.label')}
        value={value.mode}
        onChange={(mode) => change({ ...value, mode: mode as OwnSetting['mode'] })}
        options={MODES.map((mode) => ({
          value: mode,
          label: t(`admin.podcasts.setting.mode.${mode}`),
          isDisabled: mode === 'save' && !podcast.savable && saved.mode !== 'save',
        }))}
      />
      <RadioGroup
        label={t('admin.podcasts.setting.scope.label')}
        value={value.scope}
        onChange={(scope) =>
          change({
            ...value,
            scope: scope as OwnSetting['scope'],
            lastN: scope === 'last_n' ? (value.lastN ?? podcast.effectiveLastN) : undefined,
          })
        }
        options={SCOPES.map((scope) => ({
          value: scope,
          label: t(`admin.podcasts.setting.scope.${scope}`),
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
      {podcast.unclearEpisodeCount > 0 && (
        <Text tone="secondary">
          {t('admin.podcasts.setting.unclear', {
            count: podcast.unclearEpisodeCount,
            number: number.format(podcast.unclearEpisodeCount),
          })}
        </Text>
      )}
      <div className={styles.actions}>
        <AriaButton
          variant="secondary"
          icon="download"
          aria-label={t('admin.podcasts.setting.missing.label', { title: podcast.title })}
          isDisabled={!podcast.savable || missing === 0}
          isPending={download.isPending}
          onPress={askMissing}
        >
          {t('admin.podcasts.setting.missing.action')}
        </AriaButton>
      </div>
      {confirmation.estimating && (
        <Text tone="secondary">{t('admin.podcasts.downloads.estimating')}</Text>
      )}
      {pending && (
        <Notice
          title={t('admin.podcasts.downloads.confirm.text', {
            count: pending.episodes,
            number: number.format(pending.episodes),
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
      {error !== null && <Notice tone="danger">{problemMessage(t, error)}</Notice>}
      <StatusRegion message={error === null ? message : null} />
    </div>
  );
}
