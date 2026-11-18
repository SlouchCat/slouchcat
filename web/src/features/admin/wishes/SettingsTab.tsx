import { useQueryClient, type UseQueryResult } from '@tanstack/react-query';
import type { TFunction } from 'i18next';
import { useState, type FormEvent } from 'react';
import { useTranslation } from 'react-i18next';
import { formErrors, problemMessage } from '@/api/errors';
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
  TextField,
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
  wishLimitFieldNames,
  WishLimitNumberFields,
  wishLimitsOf,
  wishNumbersOf,
  wishNumbersValid,
  type WishLimitNumbers,
} from '../../shared/WishLimitFields';
import { seerrStateTone } from '../integrations/shared';
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
  | 'syncPassword'
  | 'profileTags'
  | 'showProgress'
  | 'discoverPages';

const ALREADY_REQUESTED = [
  'join',
  'status',
] as const satisfies readonly WishSettings['alreadyRequested'][];
const UNMAPPED = ['fallback', 'deny'] as const satisfies readonly WishSettings['unmapped'][];

/**
 * «Limit ab Werk» (N-31, F-26): two counts and two periods. A field saves when it is left (or with
 * Enter) and all four hold a valid number; a wrong number stays in its field with the reason, and
 * so does one the server refused.
 */
function DefaultLimit({
  limits,
  refusal,
  onSave,
}: {
  limits: WishLimits;
  /** The server's refusal of the last save of the limit, its reasons go to their fields. */
  refusal: unknown;
  onSave: (limits: WishLimits) => void;
}) {
  const { t } = useTranslation();
  const [draft, setDraft] = useState<WishLimitNumbers>(() => wishNumbersOf(limits));
  // The server's value comes back after a save: a field takes it over unless the reader has
  // typed in it since (the next field is typed while the first one saves).
  const [shown, setShown] = useState(limits);
  if (JSON.stringify(shown) !== JSON.stringify(limits)) {
    const before = wishNumbersOf(shown);
    const after = wishNumbersOf(limits);
    setShown(limits);
    setDraft(
      (Object.keys(after) as (keyof WishLimitNumbers)[]).reduce<WishLimitNumbers>(
        (next, key) => ({ ...next, [key]: draft[key] === before[key] ? after[key] : draft[key] }),
        after,
      ),
    );
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
    <Form
      onSubmit={onSubmit}
      className={styles.limits}
      validationErrors={
        refusal
          ? formErrors(t, refusal, wishLimitFieldNames('defaultLimit.', 'defaultLimit')).fields
          : undefined
      }
    >
      <WishLimitNumberFields
        value={draft}
        onChange={setDraft}
        onBlur={commit}
        name="defaultLimit"
      />
    </Form>
  );
}

/** What Radarr takes in a label, 1 to 10 characters (N-21, F-25). */
const TAG_PREFIX = /^[a-z0-9-]{1,10}$/;

/**
 * «Präfix» of the tag names: saved when the field is left (or with Enter) and valid; a wrong one
 * stays in its field with the reason and is not sent, and one the server refused stays with the
 * server's reason.
 */
function TagPrefix({
  prefix,
  isDisabled,
  refusal,
  onSave,
}: {
  prefix: string;
  isDisabled: boolean;
  refusal: unknown;
  onSave: (prefix: string) => void;
}) {
  const { t } = useTranslation();
  const [draft, setDraft] = useState(prefix);
  // The server's value comes back after a save or a refusal: the field takes it over.
  const [shown, setShown] = useState(prefix);
  if (shown !== prefix) {
    setShown(prefix);
    setDraft(prefix);
  }
  const commit = () => {
    const next = draft.trim();
    if (TAG_PREFIX.test(next) && next !== prefix) onSave(next);
  };
  const onSubmit = (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    commit();
  };
  return (
    <Form
      onSubmit={onSubmit}
      validationErrors={refusal ? formErrors(t, refusal).fields : undefined}
    >
      <TextField
        name="tagPrefix"
        label={t('admin.wishes.settings.tagPrefix')}
        hideLabel
        mono
        autoComplete="off"
        spellCheck="false"
        isDisabled={isDisabled}
        value={draft}
        onChange={setDraft}
        onBlur={commit}
        validate={(text) =>
          TAG_PREFIX.test(text.trim()) ? null : t('admin.wishes.settings.tagPrefixInvalid')
        }
      />
    </Form>
  );
}

/** The fields of a refusal the page names at a field of its own, not at the top. */
const FIELD_KEYS: readonly string[] = ['defaultLimit', 'tagPrefix'];

/**
 * Medien › Wünsche › Einstellungen (concept 25, M-25, administration 7): what may be requested,
 * from where and by which rules. Every switch and choice takes effect at once and says so in the
 * outcome line; there is no «Speichern» (useImmediateSave). «Wünsche aktiv» goes on only after a
 * passed test of the saved connection (T-1): before, the switch is locked with the reason and a
 * link to the connection. Every band folds away with the summary of its values in the head
 * (concept 07 §7, design rule 1.3); «Grundsatz» opens itself while the wishes are off, because
 * that is the thing to do.
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
      // Only the fields of this change: the whole answer may predate the change of another
      // control that is still on its way.
      const touched = Object.fromEntries(
        Object.keys(patch).map((field) => [field, next[field as keyof WishSettings]]),
      );
      queryClient.setQueryData(wishSettingsQuery.queryKey, (current: WishSettings | undefined) =>
        current ? { ...current, ...touched } : next,
      );
      return savedText(t, key, patch);
    },
  });

  const basicsCare = settings.data ? !settings.data.enabled : false;
  const [basicsOpen, setBasicsOpen] = useState(basicsCare);
  const [basicsCared, setBasicsCared] = useState(basicsCare);
  if (basicsCare !== basicsCared) {
    setBasicsCared(basicsCare);
    if (basicsCare) setBasicsOpen(true);
  }

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
  const onOff = (value: boolean) =>
    t(value ? 'admin.wishes.settings.on' : 'admin.wishes.settings.off');
  const refusalOf = (key: string) => (save.failure?.key === key ? save.failure.error : null);
  // A refusal with reasons for a field stands at that field.
  const atField =
    save.failure !== null &&
    FIELD_KEYS.includes(save.failure.key) &&
    Object.keys(formErrors(t, save.failure.error).fields).length > 0;
  const linksMeta = [
    status &&
      t('admin.wishes.settings.linksConnection', {
        state: t(`admin.integrations.seerr.state.${status.state}`),
      }),
    mappings &&
      t('admin.wishes.settings.usersCount', { opened: mappings.opened, count: mappings.total }),
  ]
    .filter(Boolean)
    .join(t('admin.wishes.separator'));

  return (
    <div className={styles.tabBody}>
      <VisuallyHidden as="h2">{t('admin.wishes.tabs.settings')}</VisuallyHidden>
      {save.failure && !atField && (
        <Notice tone="danger">
          {t('admin.wishes.settings.refused', { reason: problemMessage(t, save.failure.error) })}
        </Notice>
      )}
      <StatusRegion message={save.failure ? null : save.message} />
      <Text tone="secondary">{t('admin.wishes.settings.atOnce')}</Text>

      <BandPanel
        title={t('admin.wishes.settings.basics')}
        icon="heart"
        collapsible
        isExpanded={basicsOpen}
        onExpandedChange={setBasicsOpen}
        meta={
          <span>
            {t(
              locked
                ? 'admin.wishes.settings.basicsSummaryLocked'
                : 'admin.wishes.settings.basicsSummary',
              {
                enabled: onOff(shown.enabled),
                movies: onOff(shown.allowMovies),
                shows: onOff(shown.allowShows),
              },
            )}
          </span>
        }
      >
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

      <BandPanel
        title={t('admin.wishes.settings.where')}
        icon="search"
        collapsible
        meta={
          <span>
            {t('admin.wishes.settings.whereSummary', {
              search: onOff(shown.fromSearch),
              detail: onOff(shown.fromDetail),
              discover: onOff(shown.discoverPages),
            })}
          </span>
        }
      >
        {row('fromSearch')}
        {row('fromDetail')}
        {row('discoverPages')}
      </BandPanel>

      <BandPanel
        title={t('admin.wishes.settings.rules')}
        icon="settings"
        collapsible
        meta={
          <span>
            {t('admin.wishes.settings.rulesSummary', {
              already: t(`admin.wishes.settings.alreadyRequestedOptions.${shown.alreadyRequested}`),
              unmapped: t(`admin.wishes.settings.unmappedOptions.${shown.unmapped}`),
              withdraw: onOff(shown.allowWithdraw),
            })}
          </span>
        }
      >
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
            limits={settings.data.defaultLimit}
            refusal={refusalOf('defaultLimit')}
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

      <BandPanel
        title={t('admin.wishes.settings.arr')}
        icon="bookmark"
        collapsible
        meta={
          <span>
            {t('admin.wishes.settings.arrSummary', {
              tags: onOff(shown.profileTags),
              prefix: shown.tagPrefix,
              progress: onOff(shown.showProgress),
            })}
          </span>
        }
      >
        {row('profileTags')}
        <SettingRow
          label={t('admin.wishes.settings.tagPrefix')}
          help={t('admin.wishes.settings.tagPrefixHelp')}
        >
          <TagPrefix
            prefix={settings.data.tagPrefix}
            refusal={refusalOf('tagPrefix')}
            isDisabled={!shown.profileTags}
            onSave={(tagPrefix) => save.change('tagPrefix', { tagPrefix })}
          />
        </SettingRow>
        {row('showProgress')}
        <SettingRow
          label={t('admin.wishes.settings.arrLinks')}
          help={t('admin.wishes.settings.arrLinksHelp')}
        >
          <span className={styles.chips}>
            <TextLink href={connection}>{t('admin.wishes.settings.toArr')}</TextLink>
            <TextLink href={`${paths.adminWishes}?tab=seerr-benutzer`}>
              {t('admin.wishes.settings.toUsers')}
            </TextLink>
          </span>
        </SettingRow>
      </BandPanel>

      <BandPanel
        title={t('admin.wishes.settings.links')}
        icon="external"
        collapsible
        meta={linksMeta ? <span>{linksMeta}</span> : undefined}
      >
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
              <Chip tone={seerrStateTone[status.state]}>
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
                  count: mappings.total,
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

/** The outcome line of a change: «"Filme erlauben" ist ausgeschaltet. Gilt sofort.» and the like. */
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
  if (key === 'tagPrefix' && patch.tagPrefix) {
    return t('admin.wishes.settings.tagPrefixSaved', { prefix: patch.tagPrefix });
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
