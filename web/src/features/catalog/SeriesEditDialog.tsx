import type { components } from '@slouchcat/api-client';
import { useMutation, useQueryClient } from '@tanstack/react-query';
import { useState, type FormEvent } from 'react';
import { useTranslation } from 'react-i18next';
import { api, request } from '@/api/client';
import { formErrors } from '@/api/errors';
import { formText } from '@/lib/form';
import { Notice } from '@/ui';
import { AriaButton, CheckboxGroup, Form, FormDialog, TextField } from '@/ui/aria';
import styles from './SeriesPage.module.css';

type AudioSeries = components['schemas']['AudioSeries'];
type AdminAudioSeries = components['schemas']['AdminAudioSeries'];
type AudioSeriesRequest = components['schemas']['AudioSeriesRequest'];

/** Longest series name the server takes (`AudioSeriesRequest.name`). */
const MAX_NAME = 200;
const WRITE_TO_VOLUMES = 'writeToVolumes';

/**
 * «Reihe bearbeiten» (concept 19 D5.4): the displayed name, locked by the server; with «In alle
 * Bände schreiben» also the series name of every visible volume, which may merge two series. The
 * answer names the series the volumes belong to afterwards; `onSaved` gets it.
 */
export function SeriesEditDialog({
  series,
  onClose,
  onSaved,
}: {
  series: AudioSeries;
  onClose: () => void;
  onSaved: (saved: AdminAudioSeries) => void;
}) {
  const { t } = useTranslation();
  const queryClient = useQueryClient();
  const [options, setOptions] = useState<string[]>([]);
  const save = useMutation({
    mutationFn: (body: AudioSeriesRequest): Promise<AdminAudioSeries> =>
      request(
        api.PATCH('/admin/audio-series/{seriesId}', {
          params: { path: { seriesId: series.id } },
          body,
        }),
      ),
    onSuccess: (saved) => {
      // The row, the series page and the volumes' detail pages (their series line) change.
      void queryClient.invalidateQueries({ queryKey: ['library'] });
      void queryClient.invalidateQueries({ queryKey: ['item'] });
      void queryClient.invalidateQueries({ queryKey: ['home'] });
      onSaved(saved);
    },
  });
  const errors = save.error ? formErrors(t, save.error) : null;
  const onSubmit = (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    save.mutate({
      name: formText(new FormData(event.currentTarget), 'name'),
      writeToVolumes: options.includes(WRITE_TO_VOLUMES),
    });
  };
  return (
    <FormDialog
      isOpen
      onOpenChange={(open) => {
        if (!open) onClose();
      }}
      title={t('audioSeries.dialog.title')}
    >
      <Form className={styles.form} onSubmit={onSubmit} validationErrors={errors?.fields}>
        {errors?.form && <Notice tone="danger">{errors.form}</Notice>}
        <TextField
          name="name"
          label={t('audioSeries.dialog.name')}
          description={t('audioSeries.dialog.nameHint')}
          defaultValue={series.name}
          maxLength={MAX_NAME}
          autoComplete="off"
          isRequired
          onChange={() => {
            if (save.error) save.reset();
          }}
          validate={(value) => (value.trim() ? null : t('errors.field.required'))}
        />
        <CheckboxGroup
          label={t('audioSeries.dialog.options')}
          value={options}
          onChange={setOptions}
          options={[
            {
              value: WRITE_TO_VOLUMES,
              label: t('audioSeries.dialog.writeToVolumes'),
              description: t('audioSeries.dialog.writeToVolumesHint'),
            },
          ]}
        />
        <div className={styles.dialogActions}>
          <AriaButton variant="ghost" onPress={onClose}>
            {t('common.cancel')}
          </AriaButton>
          <AriaButton type="submit" variant="primary" isPending={save.isPending}>
            {save.isPending ? t('common.saving') : t('common.save')}
          </AriaButton>
        </div>
      </Form>
    </FormDialog>
  );
}
