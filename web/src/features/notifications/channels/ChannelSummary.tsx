import type { ChannelSetting } from '@slouchcat/api-client';
import { useTranslation } from 'react-i18next';
import { FactList } from '@/ui';

/**
 * A channel as a profile other than the account's primary one sees it (BEN-5): events and the
 * channel's own part as facts, without controls (the server refuses changes from such a profile).
 */
export function ChannelSummary({ setting }: { setting: ChannelSetting }) {
  const { t } = useTranslation();
  const { channel, available } = setting;
  const facts: [string, string][] = [
    [
      t('settings.notifications.events.label'),
      setting.events.length > 0
        ? setting.events
            .map((event) => t(`settings.notifications.events.${event}`))
            .join(t('format.separator'))
        : t('settings.notifications.noEvents'),
    ],
  ];
  if (channel === 'email' && setting.email?.address) {
    facts.push([t('settings.notifications.email.address'), setting.email.address]);
  }
  if (channel === 'telegram' && available) {
    facts.push([
      t('settings.notifications.telegram.label'),
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
  return <FactList facts={facts} />;
}
