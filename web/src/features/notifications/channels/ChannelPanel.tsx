import type { ChannelSetting } from '@slouchcat/api-client';
import { useMutation } from '@tanstack/react-query';
import { type FormEvent } from 'react';
import { useTranslation } from 'react-i18next';
import { formErrors, problemMessage } from '@/api/errors';
import { paths } from '@/app/paths';
import { ButtonLink, Notice, VisuallyHidden } from '@/ui';
import {
  AriaButton,
  CheckboxGroup,
  Form,
  SettingRow,
  StatusRegion,
  Switch,
  TextField,
  useStatusMessage,
} from '@/ui/aria';
import { testChannel, updateChannel } from '../queries';
import { PushDevices } from './PushDevices';
import { EVENTS, useStoreChannel, type Ask, type ChannelDraft } from './shared';
import { TelegramLink } from './TelegramLink';
import styles from './channels.module.css';

interface ChannelPanelProps {
  setting: ChannelSetting;
  isAdmin: boolean;
  /** The channel's values: the page's draft, else what the server holds. */
  values: ChannelDraft;
  onChange: (next: ChannelDraft) => void;
  /** Stored: the page drops the draft. */
  onSaved: () => void;
  ask: Ask;
  /** The page asks for an open Telegram code every few seconds (false while it holds). */
  polling: boolean;
  focusTitleIfLost: () => void;
  focusTitle: () => void;
}

/**
 * The body of one channel (BEN-1 to BEN-4): the switch, the address (e-mail), the link
 * (Telegram) or the devices (Web Push), the events, then «Speichern» (PUT /me/channels/{channel})
 * and «Testnachricht». A channel the server lacks stays disabled, with the hint where the main user
 * sets it up and, for the main user, the link to the integrations (which link back here).
 */
export function ChannelPanel({
  setting,
  isAdmin,
  values,
  onChange,
  onSaved,
  ask,
  polling,
  focusTitleIfLost,
  focusTitle,
}: ChannelPanelProps) {
  const { t } = useTranslation();
  const store = useStoreChannel();
  const [message, setMessage] = useStatusMessage();
  const { channel, available } = setting;
  const say = (text: string) => setMessage(text);
  const save = useMutation({
    mutationFn: (draft: ChannelDraft) => {
      const address = draft.address.trim();
      return updateChannel(channel, {
        enabled: draft.enabled,
        events: draft.events,
        ...(channel === 'email' && address ? { email: { address } } : {}),
      });
    },
    onSuccess: (saved) => {
      store(saved);
      onSaved();
      say(t('settings.notifications.saved'));
    },
  });
  const test = useMutation({
    mutationFn: () => testChannel(channel),
    onSuccess: () => say(t('settings.notifications.testSent')),
  });
  const errors = save.error ? formErrors(t, save.error) : null;
  const change = (next: Partial<ChannelDraft>) => {
    onChange({ ...values, ...next });
    setMessage(null);
    if (save.error) save.reset();
  };
  const onSubmit = (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    setMessage(null);
    test.reset();
    save.mutate(values);
  };
  const events = EVENTS.filter((event) => isAdmin || event !== 'errors');

  return (
    <>
      {!available && (
        <Notice
          actions={
            isAdmin && (
              <ButtonLink href={paths.adminIntegrations} variant="secondary">
                {t('settings.notifications.unavailable.action')}
              </ButtonLink>
            )
          }
        >
          {t('settings.notifications.unavailable.body')}
        </Notice>
      )}
      <Form className={styles.form} onSubmit={onSubmit} validationErrors={errors?.fields}>
        <SettingRow
          label={t('settings.notifications.enabled')}
          help={t('settings.notifications.enabledHelp')}
        >
          <Switch
            isSelected={values.enabled}
            isDisabled={!available}
            onChange={(enabled) => change({ enabled })}
          >
            <VisuallyHidden>{t('settings.notifications.enabled')}</VisuallyHidden>
          </Switch>
        </SettingRow>
        {available && channel === 'telegram' && (
          <TelegramLink
            setting={setting}
            ask={ask}
            polling={polling}
            focusTitleIfLost={focusTitleIfLost}
            focusTitle={focusTitle}
          />
        )}
        {channel === 'email' && (
          <TextField
            name="email.address"
            type="email"
            label={t('settings.notifications.email.address')}
            description={t('settings.notifications.email.description')}
            value={values.address}
            isDisabled={!available}
            onChange={(address) => change({ address })}
          />
        )}
        {available && channel === 'push' && (
          <PushDevices setting={setting} focusTitleIfLost={focusTitleIfLost} />
        )}
        <CheckboxGroup
          label={t('settings.notifications.events.label')}
          description={
            isAdmin
              ? t('settings.notifications.events.helpAdmin')
              : t('settings.notifications.events.help')
          }
          options={events.map((event) => ({
            value: event,
            label: t(`settings.notifications.events.${event}`),
          }))}
          value={values.events}
          isDisabled={!available}
          onChange={(chosen) =>
            change({ events: EVENTS.filter((event) => chosen.includes(event)) })
          }
        />
        {errors?.form && <Notice tone="danger">{errors.form}</Notice>}
        {test.isError && <Notice tone="danger">{problemMessage(t, test.error)}</Notice>}
        <StatusRegion message={message} />
        {/* As in the mockup: the test first, the main action «Speichern» last. */}
        <div className={styles.actions}>
          <AriaButton
            isDisabled={!available}
            isPending={test.isPending}
            onPress={() => {
              setMessage(null);
              save.reset();
              test.mutate();
            }}
          >
            {t('settings.notifications.test')}
          </AriaButton>
          <AriaButton
            type="submit"
            variant="primary"
            isDisabled={!available}
            isPending={save.isPending}
          >
            {save.isPending ? t('common.saving') : t('common.save')}
          </AriaButton>
        </div>
      </Form>
    </>
  );
}
