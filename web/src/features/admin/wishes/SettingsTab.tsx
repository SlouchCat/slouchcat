import { useQueryClient, type UseQueryResult } from '@tanstack/react-query';
import type { TFunction } from 'i18next';
import { useState, type FormEvent } from 'react';
import { useTranslation } from 'react-i18next';
import { problemMessage } from '@/api/errors';
import { paths } from '@/app/paths';
import { Chip, Icon, Notice, Skeleton, Text, TextLink, VisuallyHidden } from '@/ui';
import {
  AriaButton,
  BandPanel,
  Form,
  RadioGroup,
  SettingRow,
  StatusRegion,
  Switch,
} from '@/ui/aria';
import {
  setWishSettings,
  wishSettingsQuery,
  type SeerrStatus,
  type WishLimits,
  type WishMappings,
  type WishSettings,
  type WishSettingsUpdate,
} from '../queries';
import {
  limitSummary,
  WishLimitNumberFields,
  wishLimitsOf,
  wishNumbersOf,
  wishNumbersValid,
  type WishLimitNumbers,
} from '../../shared/WishLimitFields';
import { useImmediateSave } from './useImmediateSave';
import styles from './Wishes.module.css';

type Toggle =
  | 'enabled'
  | 'allowMovies'
  | 'allowShows'
  | 'fromSearch'
  | 'fromDetail'
  | 'newAccountsCanRequest'
  | 'allowWithdraw'
  | 'pinExternalHideTitle'
  | 'provisionAccounts'
  | 'syncPassword';

const ALREADY_REQUESTED = [
  'join',
  'status',
] as const satisfies readonly WishSettings['alreadyRequested'][];
const UNMAPPED = ['fallback', 'deny'] as const satisfies readonly WishSettings['unmapped'][];

/**
 * «Limit ab Werk» (N-31, F-26): two counts and two periods. A field saves when it is left (or with
 * Enter) and all four hold a valid number; a wrong number stays in its field with the reason.
 */
function DefaultLimit({
  limits,
  onSave,
}: {
  limits: WishLimits;
  onSave: (limits: WishLimits) => void;
}) {
  const [draft, setDraft] = useState<WishLimitNumbers>(() => wishNumbersOf(limits));
  // The server's value comes back after a save or a refusal: the fields take it over.
  const [shown, setShown] = useState(limits);
  if (JSON.stringify(shown) !== JSON.stringify(limits)) {
    setShown(limits);
    setDraft(wishNumbersOf(limits));
  }
  const commit = () => {
    if (!wishNumbersValid(draft)) return;
    const next = wishLimitsOf(draft);
    if (JSON.stringify(next) !== JSON.stringify(limits)) onSave(next);
  };
  const onSubmit = (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    commit();
  };
  return (
    <Form onSubmit={onSubmit} className={styles.limits}>
      <WishLimitNumberFields
        value={draft}
        onChange={setDraft}
        onBlur={commit}
        name="defaultLimit"
      />
    </Form>
  );
}

/**
 * Medien › Wünsche › Einstellungen (concept 25, M-25, administration 7): what may be requested,
 * from where and by which rules. Every switch and choice takes effect at once and says so in the
 * outcome line; there is no «Speichern» (useImmediateSave). «Wünsche aktiv» goes on only after a
 * passed test of the saved connection (T-1): before, the switch is locked with the reason and a
 * link to the connection. The three groups of rules stand open as cards; «Limit ab Werk» and
 * «Konten in Seerr» fold away with their summary in the head (design rule 1.3).
 */
export function SettingsTab({
  settings,
  status,
  mappings,
}: {
  settings: UseQueryResult<WishSettings>;
  status: SeerrStatus | undefined;
  mappings: WishMappings | undefined;
}) {
  const { t } = useTranslation();
  const queryClient = useQueryClient();
  const save = useImmediateSave<WishSettingsUpdate, WishSettings>({
    save: setWishSettings,
    saved: (next, patch, key) => {
      queryClient.setQueryData(wishSettingsQuery.queryKey, next);
      return savedText(t, key, patch);
    },
  });

  if (settings.isPending) return <Skeleton shape="block" />;
  if (settings.isError) {
    return (
      <Notice
        tone="danger"
        title={problemMessage(t, settings.error)}
        actions={
          <AriaButton icon="refresh" onPress={() => void settings.refetch()}>
            {t('common.retry')}
          </AriaButton>
        }
      />
    );
  }
  const shown = [...save.pending.values()].reduce<WishSettings>(
    (all, patch) => ({ ...all, ...patch }),
    settings.data,
  );

  const toggle = (key: Toggle, isDisabled = false) => (
    <Switch
      isSelected={shown[key]}
      isDisabled={isDisabled}
      onChange={(value) => save.change(key, { [key]: value })}
    >
      <VisuallyHidden>{t(`admin.wishes.settings.${key}`)}</VisuallyHidden>
    </Switch>
  );
  const row = (key: Toggle) => (
    <SettingRow
      label={t(`admin.wishes.settings.${key}`)}
      help={t(`admin.wishes.settings.${key}Help`)}
    >
      {toggle(key)}
    </SettingRow>
  );
  // T-1: «Wünsche aktiv» goes on only after a passed test; going off is always possible.
  const locked = !shown.enabled && status !== undefined && !status.tested;
  const connection = `${paths.adminIntegrations}?item=seerr`;

  return (
    <div className={styles.tabBody}>
      <VisuallyHidden as="h2">{t('admin.wishes.tabs.settings')}</VisuallyHidden>
      {save.failure && (
        <Notice tone="danger">
          {t('admin.wishes.settings.refused', { reason: problemMessage(t, save.failure.error) })}
        </Notice>
      )}
      <StatusRegion message={save.failure ? null : save.message} />
      <Text tone="secondary">{t('admin.wishes.settings.atOnce')}</Text>

      <BandPanel title={t('admin.wishes.settings.basics')} icon="heart">
        <SettingRow
          label={t('admin.wishes.settings.enabled')}
          help={
            locked ? (
              <span className={styles.locked}>
                <Icon name="lock" size="sm" />
                <span>
                  {t('admin.wishes.settings.enabledLocked')}{' '}
                  <TextLink href={connection}>{t('admin.wishes.settings.toConnection')}</TextLink>
                </span>
              </span>
            ) : (
              t('admin.wishes.settings.enabledHelp')
            )
          }
        >
          {toggle('enabled', locked)}
        </SettingRow>
        {row('allowMovies')}
        {row('allowShows')}
      </BandPanel>

      <BandPanel title={t('admin.wishes.settings.where')} icon="search">
        {row('fromSearch')}
        {row('fromDetail')}
      </BandPanel>

      <BandPanel title={t('admin.wishes.settings.rules')} icon="settings">
        <SettingRow
          label={t('admin.wishes.settings.alreadyRequested')}
          help={t(`admin.wishes.settings.alreadyRequestedHelp.${shown.alreadyRequested}`)}
        >
          <RadioGroup
            label={t('admin.wishes.settings.alreadyRequested')}
            hideLabel
            value={shown.alreadyRequested}
            onChange={(value) =>
              save.change('alreadyRequested', {
                alreadyRequested: ALREADY_REQUESTED.find((option) => option === value) ?? 'join',
              })
            }
            options={ALREADY_REQUESTED.map((option) => ({
              value: option,
              label: t(`admin.wishes.settings.alreadyRequestedOptions.${option}`),
            }))}
          />
        </SettingRow>
        <SettingRow
          label={t('admin.wishes.settings.unmapped')}
          help={t(`admin.wishes.settings.unmappedHelp.${shown.unmapped}`)}
        >
          <RadioGroup
            label={t('admin.wishes.settings.unmapped')}
            hideLabel
            value={shown.unmapped}
            onChange={(value) =>
              save.change('unmapped', {
                unmapped: UNMAPPED.find((option) => option === value) ?? 'fallback',
              })
            }
            options={UNMAPPED.map((option) => ({
              value: option,
              label: t(`admin.wishes.settings.unmappedOptions.${option}`),
            }))}
          />
        </SettingRow>
        {row('newAccountsCanRequest')}
        {row('allowWithdraw')}
        {row('pinExternalHideTitle')}
      </BandPanel>

      <BandPanel
        title={t('admin.wishes.settings.defaultLimit')}
        icon="chart"
        collapsible
        meta={<span>{limitSummary(t, shown.defaultLimit)}</span>}
      >
        <SettingRow
          label={t('admin.wishes.settings.limitPerAccount')}
          help={t('admin.wishes.settings.limitPerAccountHelp')}
        >
          <DefaultLimit
            // A refused limit starts the fields afresh from what the server holds.
            key={save.failure?.key === 'defaultLimit' ? save.failure.id : 0}
            limits={settings.data.defaultLimit}
            onSave={(defaultLimit) => save.change('defaultLimit', { defaultLimit })}
          />
        </SettingRow>
        <SettingRow
          label={t('admin.wishes.settings.perAccount')}
          help={t('admin.wishes.settings.perAccountHelp')}
        >
          <TextLink href={paths.adminAccounts}>{t('admin.wishes.settings.toAccounts')}</TextLink>
        </SettingRow>
      </BandPanel>

      <BandPanel
        title={t('admin.wishes.settings.accountsInSeerr')}
        icon="users"
        collapsible
        meta={
          <span>
            {t(
              shown.provisionAccounts
                ? 'admin.wishes.settings.provisionOn'
                : 'admin.wishes.settings.provisionOff',
            )}
            {t('admin.wishes.separator')}
            {t(
              shown.syncPassword ? 'admin.wishes.settings.syncOn' : 'admin.wishes.settings.syncOff',
            )}
          </span>
        }
      >
        {row('provisionAccounts')}
        {row('syncPassword')}
        <Notice tone="info" live={false}>
          {t('admin.wishes.settings.visibleInSeerr')}
        </Notice>
        <SettingRow
          label={t('admin.wishes.settings.existing')}
          help={t('admin.wishes.settings.existingHelp')}
        >
          <TextLink href={`${paths.adminWishes}?tab=seerr-benutzer`}>
            {t('admin.wishes.settings.toUsers')}
          </TextLink>
        </SettingRow>
      </BandPanel>

      <BandPanel title={t('admin.wishes.settings.links')} icon="external">
        <SettingRow
          label={t('admin.wishes.settings.who')}
          help={t('admin.wishes.settings.whoHelp')}
        >
          <TextLink href={paths.adminAccounts}>{t('admin.wishes.settings.toAccounts')}</TextLink>
        </SettingRow>
        <SettingRow
          label={t('admin.wishes.settings.connection')}
          help={t('admin.wishes.settings.connectionHelp')}
        >
          <span className={styles.chips}>
            {status && (
              <Chip tone={status.state === 'connected' ? 'success' : 'outline'}>
                {t(`admin.integrations.seerr.state.${status.state}`)}
              </Chip>
            )}
            <TextLink href={connection}>{t('admin.wishes.settings.toConnection')}</TextLink>
          </span>
        </SettingRow>
        <SettingRow
          label={t('admin.wishes.settings.users')}
          help={t('admin.wishes.settings.usersHelp')}
        >
          <span className={styles.chips}>
            {mappings && (
              <Chip tone={mappings.opened < mappings.total ? 'warning' : 'success'}>
                {t('admin.wishes.settings.usersCount', {
                  opened: mappings.opened,
                  total: mappings.total,
                })}
              </Chip>
            )}
            <TextLink href={`${paths.adminWishes}?tab=seerr-benutzer`}>
              {t('admin.wishes.settings.toUsers')}
            </TextLink>
          </span>
        </SettingRow>
      </BandPanel>
    </div>
  );
}

/** «Filme erlauben» ist ausgeschaltet. Gilt sofort. */
function savedText(t: TFunction, key: string, patch: WishSettingsUpdate): string {
  if (key === 'defaultLimit' && patch.defaultLimit) {
    return t('admin.wishes.settings.limitSaved', { limit: limitSummary(t, patch.defaultLimit) });
  }
  if (key === 'alreadyRequested' && patch.alreadyRequested) {
    return t('admin.wishes.settings.choiceSaved', {
      name: t('admin.wishes.settings.alreadyRequested'),
      value: t(`admin.wishes.settings.alreadyRequestedOptions.${patch.alreadyRequested}`),
    });
  }
  if (key === 'unmapped' && patch.unmapped) {
    return t('admin.wishes.settings.choiceSaved', {
      name: t('admin.wishes.settings.unmapped'),
      value: t(`admin.wishes.settings.unmappedOptions.${patch.unmapped}`),
    });
  }
  const toggle = key as Toggle;
  return t(
    patch[toggle] ? 'admin.wishes.settings.switchedOn' : 'admin.wishes.settings.switchedOff',
    {
      name: t(`admin.wishes.settings.${toggle}`),
    },
  );
}
