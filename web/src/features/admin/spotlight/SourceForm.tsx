import type { TraktStatus } from '@slouchcat/api-client';
import { useMutation, useQueryClient } from '@tanstack/react-query';
import type { FormEvent } from 'react';
import { useTranslation } from 'react-i18next';
import { formErrors } from '@/api/errors';
import { paths } from '@/app/paths';
import { TextLink, VisuallyHidden } from '@/ui';
import {
  AriaButton,
  Form,
  RadioGroup,
  SettingRow,
  Switch,
  TextField,
  type Drafts,
} from '@/ui/aria';
import {
  refreshSpotlight,
  setSpotlightConfig,
  spotlightConfigQuery,
  spotlightEntriesQuery,
  traktStatusQuery,
  type SpotlightConfig,
  type SpotlightEntries,
} from '../queries';
import { FetchState } from './FetchState';
import styles from './Spotlight.module.css';

/** The id of the source's draft (useDrafts): the page has one form. */
export const SOURCE_DRAFT = 'source';

/** What the form holds while it is edited: the numbers as typed. */
export interface SourceDraft {
  enabled: boolean;
  listKey: SpotlightConfig['listKey'];
  listUrl: string;
  media: SpotlightConfig['media'];
  itemCount: string;
  minHits: string;
  intervalHours: string;
  rotationSeconds: string;
}

export const sourceDraftOf = (config: SpotlightConfig): SourceDraft => ({
  enabled: config.enabled,
  listKey: config.listKey,
  listUrl: config.listUrl ?? '',
  media: config.media,
  itemCount: String(config.itemCount),
  minHits: String(config.minHits),
  intervalHours: String(config.intervalHours),
  rotationSeconds: String(config.rotationSeconds),
});

/**
 * A draft as the form compares it: numbers and the address without surrounding blanks, so a
 * field typed back to its stored value is no change.
 */
const normalised = (draft: SourceDraft): SourceDraft => ({
  ...draft,
  listUrl: draft.listUrl.trim(),
  itemCount: draft.itemCount.trim(),
  minHits: draft.minHits.trim(),
  intervalHours: draft.intervalHours.trim(),
  rotationSeconds: draft.rotationSeconds.trim(),
});

/** The four numbers in the order of the form, with their locale keys. */
const NUMBERS = [
  ['itemCount', 'itemCount', 'itemCountHint'],
  ['minHits', 'minHits', 'minHitsHint'],
  ['intervalHours', 'intervalHours', 'intervalHoursHint'],
  // The band's rhythm on the start page (concept 08, F-10).
  ['rotationSeconds', 'rotation', 'rotationHint'],
] as const;

/**
 * «Quelle» (SPO-1, SPO-2): the switch «Auf der Startseite zeigen», the list and its numbers, with
 * the state of the fetch, «Jetzt abrufen» and «Speichern» at its foot. The switch is stored only
 * with «Speichern», together with the rest (S-2). Which media a list kind allows, whether it needs
 * an address and whether the media filters after the fetch all come from `listKinds` (plan 5.6):
 * the page keeps no table of its own, so a kind with a single medium shows that medium, locked.
 * Unsaved input lives in the page (useDrafts), so folding the band keeps it.
 */
export function SourceForm({
  config,
  status,
  entries,
  drafts,
  onStart,
  onSaved,
  onQueued,
  onFailed,
}: {
  config: SpotlightConfig;
  status?: TraktStatus;
  entries?: SpotlightEntries;
  drafts: Drafts<SourceDraft>;
  /** An action starts: the page clears its last outcome. */
  onStart: () => void;
  onSaved: (message: string) => void;
  onQueued: (message: string) => Promise<void>;
  onFailed: (error: unknown) => void;
}) {
  const { t } = useTranslation();
  const queryClient = useQueryClient();
  const draft = drafts.valueOf(SOURCE_DRAFT) ?? sourceDraftOf(config);
  const change = (patch: Partial<SourceDraft>) =>
    drafts.set(SOURCE_DRAFT, (current) => normalised({ ...current, ...patch }));

  const kind = config.listKinds.find((entry) => entry.key === draft.listKey);
  const allowedMedia = kind?.media ?? [draft.media];
  // A choice the new kind does not offer falls back to the kind's first medium; the choice itself
  // stays, so going back to a kind with all media shows it again.
  const media = allowedMedia.includes(draft.media) ? draft.media : (allowedMedia[0] ?? draft.media);

  const save = useMutation({
    mutationFn: setSpotlightConfig,
    onSuccess: async (saved) => {
      queryClient.setQueryData(spotlightConfigQuery.queryKey, saved);
      drafts.discard(SOURCE_DRAFT);
      onSaved(t('admin.spotlight.source.saved'));
      await Promise.all([
        queryClient.invalidateQueries({ queryKey: traktStatusQuery.queryKey }),
        queryClient.invalidateQueries({ queryKey: spotlightEntriesQuery.queryKey }),
        queryClient.invalidateQueries({ queryKey: ['home'] }),
      ]);
    },
    // A field's problem shows at the field; anything else goes to the page.
    onError: (error) => {
      if (formErrors(t, error).form) onFailed(error);
    },
  });
  const refresh = useMutation({
    mutationFn: refreshSpotlight,
    onSuccess: () => onQueued(t('admin.spotlight.source.queued')),
    onError: onFailed,
  });
  const errors = save.error ? formErrors(t, save.error) : null;

  const onSubmit = (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    onStart();
    save.mutate({
      enabled: draft.enabled,
      listKey: draft.listKey,
      media,
      itemCount: Number(draft.itemCount),
      minHits: Number(draft.minHits),
      intervalHours: Number(draft.intervalHours),
      rotationSeconds: Number(draft.rotationSeconds),
      ...(kind?.needsUrl ? { listUrl: draft.listUrl.trim() } : {}),
    });
  };
  const required = (value: string) => (value.trim() ? null : t('errors.field.required'));

  return (
    <Form className={styles.form} onSubmit={onSubmit} validationErrors={errors?.fields}>
      <SettingRow label={t('admin.spotlight.enabled')} help={t('admin.spotlight.enabledHint')}>
        <Switch isSelected={draft.enabled} onChange={(enabled) => change({ enabled })}>
          <VisuallyHidden>{t('admin.spotlight.enabled')}</VisuallyHidden>
        </Switch>
      </SettingRow>
      <div className={styles.group}>
        <RadioGroup
          name="listKey"
          label={t('admin.spotlight.source.list')}
          description={
            <>
              {t('admin.spotlight.source.listHint')}{' '}
              <TextLink href={paths.adminIntegrations}>
                {t('admin.spotlight.source.listHintLink')}
              </TextLink>
            </>
          }
          value={draft.listKey}
          onChange={(value) => {
            const picked = config.listKinds.find((entry) => entry.key === value);
            if (picked) change({ listKey: picked.key });
          }}
          options={config.listKinds.map((entry) => ({
            value: entry.key,
            label: t(`admin.spotlight.lists.${entry.key}`, { defaultValue: entry.key }),
          }))}
        />
        {kind?.needsUrl && (
          <TextField
            name="listUrl"
            type="url"
            label={t('admin.spotlight.source.listUrl')}
            description={t('admin.spotlight.source.listUrlHint')}
            autoComplete="off"
            spellCheck="false"
            isRequired
            value={draft.listUrl}
            onChange={(listUrl) => change({ listUrl })}
            validate={required}
          />
        )}
      </div>
      <RadioGroup
        name="media"
        label={t('admin.spotlight.source.media')}
        description={
          allowedMedia.length === 1
            ? t('admin.spotlight.source.mediaFixed')
            : kind?.mediaFilters
              ? t('admin.spotlight.source.mediaFilters')
              : undefined
        }
        value={media}
        isDisabled={allowedMedia.length === 1}
        onChange={(value) => {
          const picked = allowedMedia.find((entry) => entry === value);
          if (picked) change({ media: picked });
        }}
        options={allowedMedia.map((entry) => ({
          value: entry,
          label: t(`admin.spotlight.media.${entry}`),
        }))}
      />
      <div className={styles.numberFields}>
        {NUMBERS.map(([field, label, hint]) => (
          <TextField
            key={field}
            name={field}
            type="number"
            inputMode="numeric"
            label={t(`admin.spotlight.source.${label}`)}
            description={t(`admin.spotlight.source.${hint}`)}
            isRequired
            value={draft[field]}
            onChange={(value) => change({ [field]: value })}
            validate={required}
          />
        ))}
      </div>
      <div className={styles.footer}>
        <FetchState status={status} config={config} entries={entries} />
        <div className={styles.buttons}>
          <AriaButton
            variant="secondary"
            icon="refresh"
            isPending={refresh.isPending}
            onPress={() => {
              onStart();
              save.reset();
              refresh.mutate();
            }}
          >
            {t('admin.spotlight.source.refresh')}
          </AriaButton>
          <AriaButton type="submit" variant="primary" isPending={save.isPending}>
            {save.isPending ? t('common.saving') : t('common.save')}
          </AriaButton>
        </div>
      </div>
    </Form>
  );
}
