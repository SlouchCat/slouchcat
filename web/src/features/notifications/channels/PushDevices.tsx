import type { ChannelSetting, PushSubscriptionSummary } from '@slouchcat/api-client';
import type { TFunction } from 'i18next';
import { useId, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { problemMessage } from '@/api/errors';
import { formatDate } from '@/lib/format';
import { useTransientMutation } from '@/lib/useTransientMutation';
import { Chip, Notice, Text } from '@/ui';
import { AriaButton } from '@/ui/aria';
import { PushError, pushSupported, subscribeThisDevice, unsubscribeThisDevice } from '../push';
import { createPushSubscription, deletePushSubscription, vapidKey } from '../queries';
import { afterRender, nextMessage, Status, useRefreshChannels, type Message } from './shared';
import styles from './channels.module.css';

function pushMessage(t: TFunction, error: unknown): string {
  return error instanceof PushError
    ? t(`settings.notifications.push.${error.problem}`)
    : problemMessage(t, error);
}

const subscribe = async () => createPushSubscription(await subscribeThisDevice(vapidKey));

const remove = async (subscription: PushSubscriptionSummary) => {
  if (subscription.thisDevice) await unsubscribeThisDevice();
  await deletePushSubscription(subscription.id);
};

/**
 * Web Push (BEN-3): the subscribed devices with «Dieses Gerät», their date and «Entfernen»; this
 * device's subscription, the iOS hint and the hint for a browser without push. Subscribing and
 * removing run as transient mutations: nothing of a subscription stays in the mutation cache.
 */
export function PushDevices({
  setting,
  focusTitleIfLost,
}: {
  setting: ChannelSetting;
  /** The focus fell to the body (a button went with a change): the h1 takes it. */
  focusTitleIfLost: () => void;
}) {
  const { t, i18n } = useTranslation();
  const refresh = useRefreshChannels();
  const headingId = useId();
  const [message, setMessage] = useState<Message | null>(null);
  const [error, setError] = useState<unknown>(null);
  const subscriptions = setting.push?.subscriptions ?? [];
  const supported = pushSupported();

  /** After a change: the list again, and the h1 for a focus whose button went with it. */
  const done = async (text: string) => {
    setMessage((current) => nextMessage(current, text));
    await refresh();
    await afterRender();
    focusTitleIfLost();
  };
  const add = useTransientMutation(subscribe, () =>
    done(t('settings.notifications.push.subscribed')),
  );
  const drop = useTransientMutation(remove, () => done(t('settings.notifications.push.removed')));
  /** Runs an action; its outcome replaces the last one's (message or error). */
  const run = <T,>(action: (value: T) => Promise<{ ok: boolean; error?: unknown }>, value: T) => {
    setMessage(null);
    setError(null);
    void action(value).then((outcome) => {
      if (!outcome.ok) setError(outcome.error);
    });
  };
  const name = (subscription: PushSubscriptionSummary) =>
    subscription.deviceName ?? t('settings.notifications.push.unknownDevice');

  return (
    <div className={styles.devices}>
      <h3 className={styles.subheading} id={headingId}>
        {t('settings.notifications.push.devices')}
      </h3>
      {!supported && <Notice tone="warning">{t('settings.notifications.push.unsupported')}</Notice>}
      {subscriptions.length === 0 ? (
        <Text tone="secondary">{t('settings.notifications.push.none')}</Text>
      ) : (
        <ul className={styles.deviceList} aria-labelledby={headingId}>
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
                variant="danger"
                aria-label={t('settings.notifications.push.removeLabel', {
                  device: name(subscription),
                })}
                isDisabled={drop.isPending}
                onPress={() => run(drop.run, subscription)}
              >
                {t('settings.notifications.push.remove')}
              </AriaButton>
            </li>
          ))}
        </ul>
      )}
      {error ? <Notice tone="danger">{pushMessage(t, error)}</Notice> : null}
      <Status message={message} />
      <div className={styles.actions}>
        {supported && !subscriptions.some((subscription) => subscription.thisDevice) && (
          <AriaButton isPending={add.isPending} onPress={() => run(add.run, undefined)}>
            {t('settings.notifications.push.subscribe')}
          </AriaButton>
        )}
        <Text tone="muted" variant="caption">
          {t('settings.notifications.push.ios')}
        </Text>
      </div>
    </div>
  );
}
