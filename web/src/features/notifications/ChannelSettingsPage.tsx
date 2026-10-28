import type {
  ChannelSetting,
  ChannelSettings,
  EventType,
  PushSubscriptionSummary,
} from '@slouchcat/api-client';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import type { TFunction } from 'i18next';
import { useState, type FormEvent } from 'react';
import { useTranslation } from 'react-i18next';
import { formErrors, problemMessage } from '@/api/errors';
import { paths } from '@/app/paths';
import { useActiveSession } from '@/app/session';
import { formatDate, formatDateTime } from '@/lib/format';
import { useDocumentTitle } from '@/lib/useDocumentTitle';
import { ButtonLink, Chip, FactList, Notice, Page, Panel, Skeleton, Text, TextLink } from '@/ui';
import { AriaButton, CheckboxGroup, Form, Switch, TextField } from '@/ui/aria';
import { PushError, pushSupported, subscribeThisDevice, unsubscribeThisDevice } from './push';
import {
  channelsQuery,
  createPushSubscription,
  createTelegramLink,
  deletePushSubscription,
  deleteTelegramLink,
  testChannel,
  updateChannel,
  vapidKey,
} from './queries';
import styles from './ChannelSettingsPage.module.css';

/** Events in the order of the checkboxes; `errors` is for the main account only. */
const EVENTS: readonly EventType[] = ['new_content', 'errors', 'invitations', 'announcements'];

/** While a Telegram link code is open, the page asks every few seconds whether the bot got it. */
const LINK_POLL_MS = 3000;

const linkOpen = (data: ChannelSettings | undefined) =>
  data?.channels.some(
    (channel) =>
      channel.channel === 'telegram' && !channel.telegram?.linked && channel.telegram?.linkCode,
  ) ?? false;

interface Draft {
  enabled: boolean;
  events: EventType[];
  address: string;
}

/** Puts a channel the server answered with into the cached list. */
function useStoreChannel() {
  const queryClient = useQueryClient();
  return (next: ChannelSetting) =>
    queryClient.setQueryData<ChannelSettings>(channelsQuery.queryKey, (old) =>
      old
        ? { channels: old.channels.map((entry) => (entry.channel === next.channel ? next : entry)) }
        : old,
    );
}

/** A status line that screen readers announce; empty when there is nothing to say. */
function Status({ message }: { message: string | null }) {
  return (
    <div role="status" className={styles.live}>
      {message && <Notice live={false}>{message}</Notice>}
    </div>
  );
}

function pushMessage(t: TFunction, error: unknown): string {
  return error instanceof PushError
    ? t(`settings.notifications.push.${error.problem}`)
    : problemMessage(t, error);
}

/** Telegram: link state, the link code with the instruction for the bot, renew and unlink. */
function TelegramLink({ setting }: { setting: ChannelSetting }) {
  const { t, i18n } = useTranslation();
  const queryClient = useQueryClient();
  const store = useStoreChannel();
  const [message, setMessage] = useState<string | null>(null);
  const telegram = setting.telegram;
  const link = useMutation({ mutationFn: createTelegramLink, onSuccess: store });
  const unlink = useMutation({
    mutationFn: deleteTelegramLink,
    onSuccess: async () => {
      setMessage(t('settings.notifications.telegram.unlinked'));
      await queryClient.invalidateQueries({ queryKey: channelsQuery.queryKey });
    },
  });
  const error = link.error ?? unlink.error;
  /** A new action starts: the outcome of the last one (message or error) goes away. */
  const start = () => {
    setMessage(null);
    link.reset();
    unlink.reset();
  };
  // Empty when the server could not ask Telegram for the bot's name (getMe failed).
  const bot = telegram?.botUsername ?? '';
  const code = telegram?.linkCode;

  return (
    <div className={styles.block}>
      <div role="status" className={styles.state}>
        {telegram?.linked ? (
          <Chip tone="success" icon="check">
            {telegram.chatId
              ? t('settings.notifications.telegram.linkedChat', { chat: telegram.chatId })
              : t('settings.notifications.telegram.linked')}
          </Chip>
        ) : (
          <Chip tone="outline">{t('settings.notifications.telegram.notLinked')}</Chip>
        )}
      </div>
      {!telegram?.linked && code && (
        <>
          <p className={styles.instruction}>
            {bot
              ? t('settings.notifications.telegram.instruction', { bot, code })
              : t('settings.notifications.telegram.instructionNoBot', { code })}
          </p>
          {!bot && (
            <Text tone="secondary" variant="caption">
              {t('settings.notifications.telegram.noBot')}
            </Text>
          )}
          {telegram.linkExpiresAt && (
            <Text tone="muted" variant="caption">
              {t('settings.notifications.telegram.expires', {
                time: formatDateTime(telegram.linkExpiresAt, i18n.language),
              })}
            </Text>
          )}
          {bot && (
            <TextLink
              href={`https://t.me/${encodeURIComponent(bot)}?start=${encodeURIComponent(code)}`}
              external
            >
              {t('settings.notifications.telegram.open')}
            </TextLink>
          )}
        </>
      )}
      {error && <Notice tone="danger">{problemMessage(t, error)}</Notice>}
      <Status message={message} />
      <div className={styles.actions}>
        {!telegram?.linked && (
          <AriaButton
            isPending={link.isPending}
            onPress={() => {
              start();
              link.mutate();
            }}
          >
            {code
              ? t('settings.notifications.telegram.renew')
              : t('settings.notifications.telegram.link')}
          </AriaButton>
        )}
        {(telegram?.linked || code) && (
          <AriaButton
            variant="ghost"
            isPending={unlink.isPending}
            onPress={() => {
              start();
              unlink.mutate();
            }}
          >
            {t('settings.notifications.telegram.unlink')}
          </AriaButton>
        )}
      </div>
    </div>
  );
}

/** Web Push: the subscribed devices, this device's subscription and the iOS hint. */
function PushDevices({ setting }: { setting: ChannelSetting }) {
  const { t, i18n } = useTranslation();
  const queryClient = useQueryClient();
  const [message, setMessage] = useState<string | null>(null);
  const subscriptions = setting.push?.subscriptions ?? [];
  const supported = pushSupported();
  const refresh = () => queryClient.invalidateQueries({ queryKey: channelsQuery.queryKey });
  const subscribe = useMutation({
    mutationFn: async () => createPushSubscription(await subscribeThisDevice(vapidKey)),
    onSuccess: async () => {
      setMessage(t('settings.notifications.push.subscribed'));
      await refresh();
    },
  });
  const remove = useMutation({
    mutationFn: async (subscription: PushSubscriptionSummary) => {
      if (subscription.thisDevice) await unsubscribeThisDevice();
      await deletePushSubscription(subscription.id);
    },
    onSuccess: async () => {
      setMessage(t('settings.notifications.push.removed'));
      await refresh();
    },
  });
  const error = subscribe.error ?? remove.error;
  /** A new action starts: the outcome of the last one (message or error) goes away. */
  const start = () => {
    setMessage(null);
    subscribe.reset();
    remove.reset();
  };
  const name = (subscription: PushSubscriptionSummary) =>
    subscription.deviceName ?? t('settings.notifications.push.unknownDevice');

  return (
    <div className={styles.block}>
      <Text tone="muted" variant="caption">
        {t('settings.notifications.push.ios')}
      </Text>
      {!supported && <Notice tone="warning">{t('settings.notifications.push.unsupported')}</Notice>}
      <h3 className={styles.subheading} id="push-devices">
        {t('settings.notifications.push.devices')}
      </h3>
      {subscriptions.length === 0 ? (
        <Text tone="secondary">{t('settings.notifications.push.none')}</Text>
      ) : (
        <ul className={styles.devices} aria-labelledby="push-devices">
          {subscriptions.map((subscription) => (
            <li key={subscription.id} className={styles.device}>
              <div className={styles.deviceText}>
                <span className={styles.deviceName}>{name(subscription)}</span>
                <Text tone="muted" variant="caption">
                  {t('settings.notifications.push.created', {
                    date: formatDate(subscription.createdAt, i18n.language),
                  })}
                </Text>
              </div>
              {subscription.thisDevice && (
                <Chip tone="info">{t('settings.notifications.push.thisDevice')}</Chip>
              )}
              <AriaButton
                variant="ghost"
                aria-label={t('settings.notifications.push.removeLabel', {
                  device: name(subscription),
                })}
                isDisabled={remove.isPending}
                onPress={() => {
                  start();
                  remove.mutate(subscription);
                }}
              >
                {t('settings.notifications.push.remove')}
              </AriaButton>
            </li>
          ))}
        </ul>
      )}
      {error && <Notice tone="danger">{pushMessage(t, error)}</Notice>}
      <Status message={message} />
      {supported && !subscriptions.some((subscription) => subscription.thisDevice) && (
        <div className={styles.actions}>
          <AriaButton
            isPending={subscribe.isPending}
            onPress={() => {
              start();
              subscribe.mutate();
            }}
          >
            {t('settings.notifications.push.subscribe')}
          </AriaButton>
        </div>
      )}
    </div>
  );
}

interface ChannelCardProps {
  setting: ChannelSetting;
  isAdmin: boolean;
  accountEmail: string;
}

/**
 * One channel: switch, events, the channel's own part (address, link, devices), "Speichern"
 * (PUT /me/channels/{channel}) and "Testnachricht". A channel the server lacks is shown disabled
 * with the hint where the main user sets it up.
 */
function ChannelCard({ setting, isAdmin, accountEmail }: ChannelCardProps) {
  const { t } = useTranslation();
  const store = useStoreChannel();
  const [draft, setDraft] = useState<Draft | null>(null);
  const [message, setMessage] = useState<string | null>(null);
  const { channel, available } = setting;
  const current: Draft = draft ?? {
    enabled: setting.enabled,
    events: setting.events,
    address: setting.email?.address ?? accountEmail,
  };
  const save = useMutation({
    mutationFn: (values: Draft) => {
      const address = values.address.trim();
      return updateChannel(channel, {
        enabled: values.enabled,
        events: values.events,
        ...(channel === 'email' && address ? { email: { address } } : {}),
      });
    },
    onSuccess: (saved) => {
      store(saved);
      setDraft(null);
      setMessage(t('settings.notifications.saved'));
    },
  });
  const test = useMutation({
    mutationFn: () => testChannel(channel),
    onSuccess: () => setMessage(t('settings.notifications.testSent')),
  });
  const errors = save.error ? formErrors(t, save.error) : null;
  const change = (next: Partial<Draft>) => {
    setDraft({ ...current, ...next });
    setMessage(null);
    if (save.error) save.reset();
  };
  const onSubmit = (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    test.reset();
    save.mutate(current);
  };

  const headingId = `channel-${channel}`;
  const events = EVENTS.filter((event) => isAdmin || event !== 'errors');
  const state = !available ? 'unavailable' : setting.enabled ? 'on' : 'off';

  return (
    <Panel as="article" aria-labelledby={headingId}>
      <div className={styles.cardHead}>
        <h2 id={headingId} className={styles.name}>
          {t(`settings.notifications.channel.${channel}`)}
        </h2>
        <Chip tone={state === 'on' ? 'success' : 'outline'}>
          {t(`settings.notifications.state.${state}`)}
        </Chip>
      </div>
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
      {available && channel === 'telegram' && <TelegramLink setting={setting} />}
      {available && channel === 'push' && <PushDevices setting={setting} />}
      <Form className={styles.form} onSubmit={onSubmit} validationErrors={errors?.fields}>
        <Switch
          isSelected={current.enabled}
          isDisabled={!available}
          onChange={(enabled) => change({ enabled })}
        >
          {t('settings.notifications.enabled')}
        </Switch>
        {channel === 'email' && (
          <TextField
            name="email.address"
            type="email"
            label={t('settings.notifications.email.address')}
            description={t('settings.notifications.email.description')}
            value={current.address}
            isDisabled={!available}
            onChange={(address) => change({ address })}
          />
        )}
        <CheckboxGroup
          label={t('settings.notifications.events.label')}
          options={events.map((event) => ({
            value: event,
            label: t(`settings.notifications.events.${event}`),
          }))}
          value={current.events}
          isDisabled={!available}
          onChange={(values) =>
            change({ events: EVENTS.filter((event) => values.includes(event)) })
          }
        />
        {errors?.form && <Notice tone="danger">{errors.form}</Notice>}
        {test.isError && <Notice tone="danger">{problemMessage(t, test.error)}</Notice>}
        <Status message={message} />
        <div className={styles.actions}>
          <AriaButton
            type="submit"
            variant="primary"
            isDisabled={!available}
            isPending={save.isPending}
          >
            {save.isPending ? t('common.saving') : t('common.save')}
          </AriaButton>
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
        </div>
      </Form>
    </Panel>
  );
}

/**
 * A channel as a profile other than the account's primary one sees it: state, events and the
 * channel's own part as facts, without controls (the server refuses changes from such a profile).
 */
function ChannelSummary({ setting }: { setting: ChannelSetting }) {
  const { t } = useTranslation();
  const { channel, available } = setting;
  const headingId = `channel-${channel}`;
  const state = !available ? 'unavailable' : setting.enabled ? 'on' : 'off';
  const facts: [string, string][] = [
    [
      t('settings.notifications.events.label'),
      setting.events.length > 0
        ? setting.events
            .map((event) => t(`settings.notifications.events.${event}`))
            .join(t('format.separator'))
        : t('settings.notifications.readOnly.noEvents'),
    ],
  ];
  if (channel === 'email' && setting.email?.address) {
    facts.push([t('settings.notifications.email.address'), setting.email.address]);
  }
  if (channel === 'telegram' && available) {
    facts.push([
      t('settings.notifications.readOnly.link'),
      setting.telegram?.linked
        ? t('settings.notifications.telegram.linked')
        : t('settings.notifications.telegram.notLinked'),
    ]);
  }
  if (channel === 'push' && available) {
    facts.push([
      t('settings.notifications.push.devices'),
      String(setting.push?.subscriptions.length ?? 0),
    ]);
  }
  return (
    <Panel as="article" aria-labelledby={headingId}>
      <div className={styles.cardHead}>
        <h2 id={headingId} className={styles.name}>
          {t(`settings.notifications.channel.${channel}`)}
        </h2>
        <Chip tone={state === 'on' ? 'success' : 'outline'}>
          {t(`settings.notifications.state.${state}`)}
        </Chip>
      </div>
      <FactList facts={facts} />
    </Panel>
  );
}

/**
 * Notification channels of the account (A-6, E-8): e-mail, Web Push and Telegram, each with its
 * switch and events. Any profile of the account sees the same settings; the in-app inbox is always
 * on and not listed. The server decides what may be chosen (`errors` for the main account only).
 * Only the account's primary profile changes them: another profile sees them without controls.
 */
export function ChannelSettingsPage() {
  const { t } = useTranslation();
  useDocumentTitle(t('settings.notifications.title'));
  const session = useActiveSession();
  const channels = useQuery({
    ...channelsQuery,
    refetchInterval: (query) => (linkOpen(query.state.data) ? LINK_POLL_MS : false),
  });
  const isAdmin = session.account.role === 'admin';
  const isPrimary = session.profile.isPrimary;

  return (
    <Page>
      <div className={styles.header}>
        <Text as="h1" variant="display">
          {t('settings.notifications.title')}
        </Text>
        <Text tone="secondary">{t('settings.notifications.intro')}</Text>
      </div>
      {!isPrimary && (
        <div className={styles.notice}>
          <Notice
            title={t('settings.notifications.onlyPrimary.title')}
            actions={
              <ButtonLink href={paths.profiles} variant="secondary">
                {t('settings.notifications.onlyPrimary.action')}
              </ButtonLink>
            }
          >
            {t('settings.notifications.onlyPrimary.body')}
          </Notice>
        </div>
      )}
      {channels.isPending && <Skeleton shape="block" />}
      {channels.isError && !channels.data && (
        <Notice
          tone="danger"
          title={problemMessage(t, channels.error)}
          actions={
            <AriaButton icon="refresh" onPress={() => void channels.refetch()}>
              {t('common.retry')}
            </AriaButton>
          }
        />
      )}
      {channels.data && (
        <ul className={styles.list} aria-label={t('settings.notifications.list')}>
          {channels.data.channels.map((setting) => (
            <li key={setting.channel}>
              {isPrimary ? (
                <ChannelCard
                  setting={setting}
                  isAdmin={isAdmin}
                  accountEmail={session.account.email}
                />
              ) : (
                <ChannelSummary setting={setting} />
              )}
            </li>
          ))}
        </ul>
      )}
    </Page>
  );
}
