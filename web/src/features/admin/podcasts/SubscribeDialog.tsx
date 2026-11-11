import type { AdminPodcast, Library, PodcastCreate } from '@slouchcat/api-client';
import { useMutation } from '@tanstack/react-query';
import { useState, type FormEvent } from 'react';
import { useTranslation } from 'react-i18next';
import { api, request } from '@/api/client';
import { formErrors } from '@/api/errors';
import { formText } from '@/lib/form';
import { Notice } from '@/ui';
import { AriaButton, Form, FormDialog, RadioGroup, TextField } from '@/ui/aria';
import styles from './Podcasts.module.css';

/** A feed address the server can take: http or https (the server checks the rest). */
const isFeedUrl = (value: string) => /^https?:\/\/\S+$/i.test(value.trim());

/** "Feed abonnieren" (H-2, POD-1): a podcast library and the feed's URL; the server fetches it at once. */
export function SubscribeDialog({
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
  // What the chosen library loads after the first fetch (concept 19 part A, 5.2): nothing for
  // «Nur Link» and «Nur neue Folgen», the N newest or the whole feed otherwise.
  const downloads = libraries.find((library) => String(library.id) === libraryId)?.podcastDownloads;
  const loads =
    downloads?.mode === 'save'
      ? t(`admin.podcasts.subscribe.loads.${downloads.scope}`, { n: downloads.lastN })
      : null;
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
        {loads && (
          <Notice tone="info" live={false}>
            {loads}
          </Notice>
        )}
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
