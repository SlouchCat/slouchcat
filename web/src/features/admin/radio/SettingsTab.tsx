import { useMutation, useQueryClient, type UseQueryResult } from '@tanstack/react-query';
import { useState, type RefObject } from 'react';
import { useTranslation } from 'react-i18next';
import { problemMessage } from '@/api/errors';
import { Notice, Skeleton, Text } from '@/ui';
import {
  afterRender,
  focusTitleIfLost,
  RadioGroup,
  SaveBar,
  SettingRow,
  StatusRegion,
  Switch,
  TextField,
  useStatusMessage,
} from '@/ui/aria';
import {
  radioAdminKeys,
  radioConfigQuery,
  updateRadioConfig,
  type RadioConfig,
  type RadioConfigUpdate,
} from '../queries';
import styles from './Radio.module.css';

const BUFFERS = [10, 30, 60] as const;
type BufferMinutes = (typeof BUFFERS)[number];

/** The form's state: numbers as typed. */
interface Draft {
  bufferMinutes: BufferMinutes;
  budgetMb: string;
  maxRelays: string;
  graceSeconds: string;
  coverMusicbrainz: boolean;
  coverItunes: boolean;
  wikidataEnabled: boolean;
}

const draftOf = (config: RadioConfig): Draft => ({
  bufferMinutes: config.bufferMinutes,
  budgetMb: String(config.budgetMb),
  maxRelays: String(config.maxRelays),
  graceSeconds: String(config.graceSeconds),
  coverMusicbrainz: config.coverMusicbrainz,
  coverItunes: config.coverItunes,
  wikidataEnabled: config.wikidataEnabled,
});

/** What differs from the stored settings (PATCH: absent fields keep their value). */
function changes(config: RadioConfig, draft: Draft): RadioConfigUpdate {
  const update: RadioConfigUpdate = {};
  if (draft.bufferMinutes !== config.bufferMinutes) update.bufferMinutes = draft.bufferMinutes;
  const budget = Number(draft.budgetMb);
  if (Number.isInteger(budget) && budget !== config.budgetMb) update.budgetMb = budget;
  const relays = Number(draft.maxRelays);
  if (Number.isInteger(relays) && relays !== config.maxRelays) update.maxRelays = relays;
  const grace = Number(draft.graceSeconds);
  if (Number.isInteger(grace) && grace !== config.graceSeconds) update.graceSeconds = grace;
  if (draft.coverMusicbrainz !== config.coverMusicbrainz) {
    update.coverMusicbrainz = draft.coverMusicbrainz;
  }
  if (draft.coverItunes !== config.coverItunes) update.coverItunes = draft.coverItunes;
  if (draft.wikidataEnabled !== config.wikidataEnabled) {
    update.wikidataEnabled = draft.wikidataEnabled;
  }
  return update;
}

interface SettingsTabProps {
  config: UseQueryResult<RadioConfig>;
  /** The page's h1: it takes the focus when «Verwerfen» went with the changes (ALL-2). */
  titleRef: RefObject<HTMLHeadingElement | null>;
}

/**
 * «Einstellungen» (concept 16, 7.3, 8 and 11) as rows: time shift, budget, maximum stations,
 * run-on, the two cover lookups (D-10) and «Wikidata verwenden» (concept 19 B7.3, stage B2). Time shift, budget and maximum apply to relays that
 * start from now on (the running ones keep theirs, S2), the others at once. The countries and the
 * switch for broken stations belong to the offer since contract 1.28.0 (concept 19 part B).
 */
export function SettingsTab({ config, titleRef }: SettingsTabProps) {
  if (config.isPending) return <Skeleton shape="block" />;
  if (!config.data) return null;
  return <SettingsForm config={config.data} titleRef={titleRef} />;
}

function SettingsForm({
  config,
  titleRef,
}: {
  config: RadioConfig;
  titleRef: SettingsTabProps['titleRef'];
}) {
  const { t } = useTranslation();
  const queryClient = useQueryClient();
  const [message, setMessage] = useStatusMessage();
  const [draft, setDraft] = useState(() => draftOf(config));
  const set = <K extends keyof Draft>(key: K, value: Draft[K]) =>
    setDraft((previous) => ({ ...previous, [key]: value }));
  const update = changes(config, draft);
  const dirty = Object.keys(update).length > 0;

  const save = useMutation({
    mutationFn: () => updateRadioConfig(update),
    onSuccess: async (next) => {
      queryClient.setQueryData(radioConfigQuery.queryKey, next);
      setDraft(draftOf(next));
      setMessage(t('admin.radio.settings.saved'));
      await queryClient.invalidateQueries({ queryKey: radioAdminKeys.all });
    },
  });
  const newRelays = t('admin.radio.settings.newRelays');

  return (
    <div className={styles.tab}>
      <StatusRegion message={message} />
      <SettingRow
        label={t('admin.radio.settings.buffer')}
        help={`${t('admin.radio.settings.bufferHelp')} ${newRelays}`}
      >
        <RadioGroup
          label={t('admin.radio.settings.buffer')}
          hideLabel
          options={BUFFERS.map((minutes) => ({
            value: String(minutes),
            label: t('admin.radio.settings.minutes', { count: minutes }),
          }))}
          value={String(draft.bufferMinutes)}
          onChange={(value) => set('bufferMinutes', Number(value) as BufferMinutes)}
        />
      </SettingRow>
      <SettingRow
        label={t('admin.radio.settings.budget')}
        help={`${t('admin.radio.settings.budgetHelp')} ${newRelays}`}
      >
        <TextField
          label={t('admin.radio.settings.budget')}
          hideLabel
          inputMode="numeric"
          value={draft.budgetMb}
          onChange={(value) => set('budgetMb', value)}
        />
      </SettingRow>
      <SettingRow
        label={t('admin.radio.settings.maxRelays')}
        help={`${t('admin.radio.settings.maxRelaysHelp', { count: config.maxRelaysByBudget })} ${newRelays}`}
      >
        <TextField
          label={t('admin.radio.settings.maxRelays')}
          hideLabel
          inputMode="numeric"
          value={draft.maxRelays}
          onChange={(value) => set('maxRelays', value)}
        />
      </SettingRow>
      <SettingRow
        label={t('admin.radio.settings.grace')}
        help={t('admin.radio.settings.graceHelp')}
      >
        <TextField
          label={t('admin.radio.settings.grace')}
          hideLabel
          inputMode="numeric"
          value={draft.graceSeconds}
          onChange={(value) => set('graceSeconds', value)}
        />
      </SettingRow>
      <SettingRow
        label={t('admin.radio.settings.coverMusicbrainz')}
        help={t('admin.radio.settings.coverMusicbrainzHelp')}
      >
        <Switch
          isSelected={draft.coverMusicbrainz}
          onChange={(value) => set('coverMusicbrainz', value)}
        >
          {t('admin.radio.settings.coverMusicbrainz')}
        </Switch>
      </SettingRow>
      <SettingRow
        label={t('admin.radio.settings.coverItunes')}
        help={t('admin.radio.settings.coverItunesHelp')}
      >
        <Switch isSelected={draft.coverItunes} onChange={(value) => set('coverItunes', value)}>
          {t('admin.radio.settings.coverItunes')}
        </Switch>
      </SettingRow>
      <SettingRow
        label={t('admin.radio.settings.wikidata')}
        help={t('admin.radio.settings.wikidataHelp')}
      >
        <Switch
          isSelected={draft.wikidataEnabled}
          onChange={(value) => set('wikidataEnabled', value)}
        >
          {t('admin.radio.settings.wikidata')}
        </Switch>
      </SettingRow>
      <Text tone="secondary">{t('admin.radio.settings.rights')}</Text>
      <SaveBar
        dirty={dirty}
        saving={save.isPending}
        onSave={() => {
          if (dirty) save.mutate();
        }}
        onDiscard={() => {
          setDraft(draftOf(config));
          void afterRender().then(() => focusTitleIfLost(titleRef));
        }}
      />
      {save.error && <Notice tone="danger">{problemMessage(t, save.error)}</Notice>}
    </div>
  );
}
