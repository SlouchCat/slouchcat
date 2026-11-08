import { useMutation, useQuery } from '@tanstack/react-query';
import { useState, type FormEvent } from 'react';
import { useTranslation } from 'react-i18next';
import { formErrors } from '@/api/errors';
import { countryName, facetValueName } from '@/app/radio/labels';
import { radioQueryParams } from '@/features/radio/filters';
import { radioFacetsQuery } from '@/features/radio/queries';
import { Notice, SelectField } from '@/ui';
import { AriaButton, Form, FormDialog, TextField } from '@/ui/aria';
import {
  createRadioStation,
  type AdminRadioStation,
  type AdminRadioStationCreate,
} from '../queries';
import { facetValues, KINDS } from './facets';
import { languageCodes } from './languages';
import styles from './Radio.module.css';

/** An address the server can take: http or https (it checks the rest, the local network too). */
const isWebUrl = (value: string) => /^https?:\/\/\S+$/i.test(value.trim());

interface Draft {
  name: string;
  streamUrl: string;
  homepage: string;
  country: string;
  region: string;
  languages: string;
  genre: string;
  kind: string;
}

const empty: Draft = {
  name: '',
  streamUrl: '',
  homepage: '',
  country: '',
  region: '',
  languages: '',
  genre: '',
  kind: '',
};

/** The body of `createRadioStation`: the fields given, the codes as the contract wants them. */
function bodyOf(draft: Draft): AdminRadioStationCreate {
  const homepage = draft.homepage.trim();
  const region = draft.region.trim().toUpperCase();
  const languages = languageCodes(draft.languages);
  return {
    name: draft.name.trim(),
    streamUrl: draft.streamUrl.trim(),
    ...(homepage ? { homepage } : {}),
    ...(draft.country ? { country: draft.country } : {}),
    ...(region ? { region } : {}),
    ...(languages.length > 0 ? { languages } : {}),
    ...(draft.genre ? { genres: [draft.genre] } : {}),
    ...(draft.kind ? { kinds: [draft.kind] } : {}),
  };
}

interface CreateStationDialogProps {
  onClose: () => void;
  /** The station the server added; the page opens its editor for the logo. */
  onCreated: (station: AdminRadioStation) => void;
}

/**
 * «Eigenen Sender hinzufügen» (concept 16, 5.6; D-8): name and stream address, optionally
 * homepage, country, region, languages, genre and kind. The server refuses an address in the
 * local network (`local_address`, F-15); the message stands at the field.
 */
export function CreateStationDialog({ onClose, onCreated }: CreateStationDialogProps) {
  const { t, i18n } = useTranslation();
  const language = i18n.language;
  const [draft, setDraft] = useState(empty);
  const facets = useQuery(radioFacetsQuery(radioQueryParams({})));
  const create = useMutation({ mutationFn: createRadioStation, onSuccess: onCreated });
  const set = <K extends keyof Draft>(key: K, value: Draft[K]) => {
    setDraft((previous) => ({ ...previous, [key]: value }));
    // A server error sits on its field until the value changes (SubscribeDialog).
    if (create.error) create.reset();
  };
  const errors = create.error ? formErrors(t, create.error) : null;

  const onSubmit = (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    create.mutate(bodyOf(draft));
  };
  const urlCheck = (required: boolean) => (value: string) => {
    if (!value.trim()) return required ? t('errors.field.required') : null;
    return isWebUrl(value) ? null : t('errors.field.invalid_url');
  };

  return (
    <FormDialog
      isOpen
      onOpenChange={(open) => {
        if (!open) onClose();
      }}
      title={t('admin.radio.own.create.title')}
      description={t('admin.radio.own.create.description')}
    >
      <Form className={styles.form} onSubmit={onSubmit} validationErrors={errors?.fields}>
        {errors?.form && <Notice tone="danger">{errors.form}</Notice>}
        <TextField
          name="name"
          label={t('admin.radio.own.fields.name')}
          value={draft.name}
          onChange={(value) => set('name', value)}
          isRequired
          maxLength={200}
          validate={(value) => (value.trim() ? null : t('errors.field.required'))}
        />
        <TextField
          name="streamUrl"
          type="url"
          label={t('admin.radio.own.fields.streamUrl')}
          description={t('admin.radio.own.fields.streamUrlHelp')}
          value={draft.streamUrl}
          onChange={(value) => set('streamUrl', value)}
          autoComplete="off"
          spellCheck="false"
          mono
          isRequired
          validate={urlCheck(true)}
        />
        <TextField
          name="homepage"
          type="url"
          label={t('admin.radio.own.fields.homepage')}
          description={t('admin.radio.own.fields.homepageHelp')}
          value={draft.homepage}
          onChange={(value) => set('homepage', value)}
          autoComplete="off"
          spellCheck="false"
          mono
          validate={urlCheck(false)}
        />
        <div className={styles.formRow}>
          <SelectField
            label={t('admin.radio.own.fields.country')}
            value={draft.country}
            options={[
              { value: '', label: t('admin.radio.own.fields.none') },
              ...(facets.data?.country ?? [])
                .map((entry) => ({
                  value: entry.value,
                  label: countryName(language, entry.value),
                }))
                .sort((a, b) => a.label.localeCompare(b.label, language)),
            ]}
            onChange={(value) => set('country', value)}
          />
          <TextField
            name="region"
            label={t('admin.radio.editor.region')}
            description={t('admin.radio.editor.regionHelp')}
            value={draft.region}
            onChange={(value) => set('region', value)}
          />
        </div>
        <TextField
          name="languages"
          label={t('admin.radio.own.fields.languages')}
          description={t('admin.radio.own.fields.languagesHelp')}
          value={draft.languages}
          onChange={(value) => set('languages', value)}
        />
        <div className={styles.formRow}>
          <SelectField
            label={t('admin.radio.editor.genre')}
            value={draft.genre}
            options={[
              { value: '', label: t('admin.radio.own.fields.none') },
              ...facetValues(
                'genre',
                (facets.data?.genre ?? []).map((entry) => entry.value),
              ).map((value) => ({ value, label: facetValueName(t, 'genre', value) })),
            ]}
            onChange={(value) => set('genre', value)}
          />
          <SelectField
            label={t('admin.radio.editor.kind')}
            value={draft.kind}
            options={[
              { value: '', label: t('admin.radio.own.fields.kindAuto') },
              ...KINDS.map((value) => ({ value, label: facetValueName(t, 'kind', value) })),
            ]}
            onChange={(value) => set('kind', value)}
          />
        </div>
        <div className={styles.dialogActions}>
          <AriaButton variant="ghost" onPress={onClose}>
            {t('common.cancel')}
          </AriaButton>
          <AriaButton type="submit" variant="primary" icon="plus" isPending={create.isPending}>
            {t('admin.radio.own.create.submit')}
          </AriaButton>
        </div>
      </Form>
    </FormDialog>
  );
}
