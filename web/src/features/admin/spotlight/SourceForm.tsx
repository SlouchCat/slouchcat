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
  type SpotlightConfigUpdate,
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
 * A draft as it is compared: numbers and the address without surrounding blanks. The draft keeps
 * what was typed; only the comparison and the request trim it.
 */
const normalised = (draft: SourceDraft): SourceDraft => ({
  ...draft,
  listUrl: draft.listUrl.trim(),
  itemCount: draft.itemCount.trim(),
  minHits: draft.minHits.trim(),
  intervalHours: draft.intervalHours.trim(),
  rotationSeconds: draft.rotationSeconds.trim(),
});

/**
 * Whether the source holds unsaved input: a draft that differs from the stored config once both
 * are trimmed. A field typed back to its stored value, blanks around it or not, is no change.
 */
export function sourceChanged(drafts: Drafts<SourceDraft>, config: SpotlightConfig | undefined) {
  const draft = drafts.valueOf(SOURCE_DRAFT);
  if (!config || !draft || !drafts.isDirty(SOURCE_DRAFT)) return false;
  return JSON.stringify(normalised(draft)) !== JSON.stringify(normalised(sourceDraftOf(config)));
}

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
  statusFailed,
  entries,
  drafts,
  onStart,
  onSaved,
  onQueued,
  onFailed,
}: {
  config: SpotlightConfig;
  status?: TraktStatus;
  /** The state could not be loaded: the page shows the error, the foot no placeholder. */
  statusFailed: boolean;
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
    drafts.set(SOURCE_DRAFT, (current) => ({ ...current, ...patch }));

  const kind = config.listKinds.find((entry) => entry.key === draft.listKey);
  const allowedMedia = kind?.media ?? [draft.media];
  // A choice the new kind does not offer falls back to the kind's first medium; the choice itself
  // stays, so going back to a kind with all media shows it again.
  const media = allowedMedia.includes(draft.media) ? draft.media : (allowedMedia[0] ?? draft.media);

  const save = useMutation({
    mutationFn: ({ body }: { body: SpotlightConfigUpdate; sent: SourceDraft }) =>
      setSpotlightConfig(body),
    onSuccess: async (saved, { sent }) => {
      queryClient.setQueryData(spotlightConfigQuery.queryKey, saved);
      // The draft is spent when it still says what was sent; what was typed while the save ran
      // stays a draft (decision 18, addendum 3).
      drafts.set(SOURCE_DRAFT, (current) =>
        JSON.stringify(normalised(current)) === JSON.stringify(normalised(sent))
          ? sourceDraftOf(saved)
          : current,
      );
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
    // Nothing changed: nothing to send, the focus stays on «Speichern» (decision 18).
    if (!sourceChanged(drafts, config)) return;
    onStart();
    save.mutate({
      body: {
        enabled: draft.enabled,
        listKey: draft.listKey,
        media,
        itemCount: Number(draft.itemCount.trim()),
        minHits: Number(draft.minHits.trim()),
        intervalHours: Number(draft.intervalHours.trim()),
        rotationSeconds: Number(draft.rotationSeconds.trim()),
        ...(kind?.needsUrl ? { listUrl: draft.listUrl.trim() } : {}),
      },
      sent: draft,
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
      <SettingRow
        label={t('admin.spotlight.source.list')}
        help={
          <>
            {t('admin.spotlight.source.listHint')}{' '}
            <TextLink href={paths.adminIntegrations}>
              {t('admin.spotlight.source.listHintLink')}
            </TextLink>
          </>
        }
      >
        <div className={styles.control}>
          <RadioGroup
            name="listKey"
            label={t('admin.spotlight.source.list')}
            hideLabel
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
      </SettingRow>
      <SettingRow
        label={t('admin.spotlight.source.media')}
        help={
          allowedMedia.length === 1
            ? t('admin.spotlight.source.mediaFixed')
            : kind?.mediaFilters
              ? t('admin.spotlight.source.mediaFilters')
              : undefined
        }
      >
        <RadioGroup
          name="media"
          label={t('admin.spotlight.source.media')}
          hideLabel
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
      </SettingRow>
      {NUMBERS.map(([field, label, hint]) => (
        <SettingRow
          key={field}
          label={t(`admin.spotlight.source.${label}`)}
          help={t(`admin.spotlight.source.${hint}`)}
        >
          <div className={styles.number}>
            <TextField
              name={field}
              type="number"
              inputMode="numeric"
              label={t(`admin.spotlight.source.${label}`)}
              hideLabel
              isRequired
              value={draft[field]}
              onChange={(value) => change({ [field]: value })}
              validate={required}
            />
          </div>
        </SettingRow>
      ))}
      <div className={styles.footer}>
        <FetchState status={status} statusFailed={statusFailed} config={config} entries={entries} />
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
