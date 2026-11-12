import type { PlaybackPrefs } from '@slouchcat/api-client';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { useId, useState, type FormEvent } from 'react';
import { useTranslation } from 'react-i18next';
import { api, request } from '@/api/client';
import { formErrors, problemMessage } from '@/api/errors';
import { paths } from '@/app/paths';
import { useActiveSession } from '@/app/session';
import { useDocumentTitle } from '@/lib/useDocumentTitle';
import { ButtonLink, Notice, Panel, Skeleton } from '@/ui';
import { AdminPage, AriaButton, ConfirmDialog, Form, SectionHeader } from '@/ui/aria';
import { PlaybackPrefsFields } from './PlaybackPrefsFields';
import { adoptPlaybackPrefs, myPlaybackPrefsQuery, putMyPlaybackPrefs } from './playbackPrefs';
import styles from './PlaybackSettingsPage.module.css';

/**
 * «Einstellungen» (concept 13 11.4, F-13; named so since concept 20, P-13): every profile sets its
 * own audio languages, subtitle rule and subtitle languages, saved together, and forgets the
 * languages it chose per series (after a confirmation). The server chooses the tracks of a start
 * from them (rule 12). A third field leads to the notification channels of the account, which
 * have no entry in the sidebar (concept 20, O-8).
 */
export function PlaybackSettingsPage() {
  const { t } = useTranslation();
  useDocumentTitle(t('settings.playback.title'));
  const session = useActiveSession();
  const queryClient = useQueryClient();
  const languagesId = useId();
  const seriesId = useId();
  const notificationsId = useId();
  const prefs = useQuery(myPlaybackPrefsQuery);
  // The changed preferences until they are saved; null shows what the server holds.
  const [draft, setDraft] = useState<PlaybackPrefs | null>(null);
  const [saved, setSaved] = useState(false);
  const save = useMutation({
    mutationFn: putMyPlaybackPrefs,
    onSuccess: (stored) => {
      adoptPlaybackPrefs(queryClient, stored);
      setDraft(null);
      setSaved(true);
    },
  });
  const reset = useMutation({
    mutationFn: () => request(api.DELETE('/me/playback-prefs/series')),
  });
  const value = draft ?? prefs.data;
  const errors = save.error ? formErrors(t, save.error) : null;

  const onSubmit = (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    if (value) save.mutate(value);
  };

  return (
    <AdminPage
      title={t('settings.playback.title')}
      help={t('settings.playback.intro', { name: session.profile.name })}
    >
      <Panel as="section" aria-labelledby={languagesId}>
        <SectionHeader id={languagesId} title={t('settings.playback.languages')} />
        {prefs.isError ? (
          <Notice tone="danger">{problemMessage(t, prefs.error)}</Notice>
        ) : !value ? (
          <Skeleton shape="block" />
        ) : (
          <Form className={styles.form} onSubmit={onSubmit} validationErrors={errors?.fields}>
            <PlaybackPrefsFields
              value={value}
              errors={errors?.fields}
              onChange={(next) => {
                setDraft(next);
                setSaved(false);
              }}
            />
            {errors?.form && <Notice tone="danger">{errors.form}</Notice>}
            <div className={styles.actions}>
              <AriaButton type="submit" variant="primary" isPending={save.isPending}>
                {save.isPending ? t('common.saving') : t('common.save')}
              </AriaButton>
              <span role="status" className={styles.status}>
                {saved ? t('settings.playback.saved') : ''}
              </span>
            </div>
          </Form>
        )}
      </Panel>
      <Panel as="section" aria-labelledby={seriesId}>
        <SectionHeader
          id={seriesId}
          title={t('settings.playback.series.title')}
          description={t('settings.playback.series.body')}
        />
        <div className={styles.actions}>
          <ConfirmDialog
            trigger={
              <AriaButton variant="secondary" isPending={reset.isPending}>
                {t('settings.playback.series.reset')}
              </AriaButton>
            }
            title={t('settings.playback.series.confirmTitle')}
            body={t('settings.playback.series.confirmBody')}
            confirmLabel={t('settings.playback.series.confirm')}
            cancelLabel={t('common.cancel')}
            tone="danger"
            onConfirm={() => reset.mutate()}
          />
          <span role="status" className={styles.status}>
            {reset.isSuccess ? t('settings.playback.series.done') : ''}
          </span>
        </div>
        {reset.isError && <Notice tone="danger">{problemMessage(t, reset.error)}</Notice>}
      </Panel>
      <Panel as="section" aria-labelledby={notificationsId}>
        <SectionHeader
          id={notificationsId}
          title={t('settings.playback.notifications.title')}
          description={t('settings.playback.notifications.body')}
        />
        <div className={styles.actions}>
          <ButtonLink variant="secondary" icon="bell" href={paths.notificationSettings}>
            {t('settings.playback.notifications.link')}
          </ButtonLink>
        </div>
      </Panel>
    </AdminPage>
  );
}
