import type { AdminSettings } from '@slouchcat/api-client';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { useNavigate } from '@tanstack/react-router';
import { useCallback, useId, useState, type ReactNode } from 'react';
import { useTranslation } from 'react-i18next';
import { problemMessage } from '@/api/errors';
import { Chip, Notice, Skeleton, Text, TextLink } from '@/ui';
import { ConfirmDialog, useDrafts } from '@/ui/aria';
import { EmailForm } from '../admin/integrations/EmailForm';
import { PROVIDERS, ProviderCard, type Provider } from '../admin/integrations/ProviderCard';
import {
  EMAIL_DRAFT,
  emailValues,
  type EmailValues,
  type KeyName,
} from '../admin/integrations/shared';
import { TelegramForm } from '../admin/integrations/TelegramForm';
import { settingsQuery, updateSettings } from '../admin/queries';
import { PushDevices } from '../notifications/channels/PushDevices';
import { channelsQuery } from '../notifications/queries';
import { pushSupported } from '../notifications/push';
import { setupPath, setupProgressQuery, updateSetupProgress } from './setupState';
import { SetupLayout } from './SetupLayout';
import styles from './SetupLayout.module.css';

/** Where the Telegram bot token comes from (F-6): BotFather, Telegram's own bot. */
const BOTFATHER_URL = 'https://t.me/BotFather';

/** TMDB is recommended (F-13); the other providers are optional. */
const recommended = (provider: Provider) => provider.id === 'tmdb';

/** «Empfohlen» or «Optional» and the link where the key is requested, visible while folded. */
function Badge({ recommend, link }: { recommend: boolean; link?: ReactNode }) {
  const { t } = useTranslation();
  return (
    <span className={styles.row}>
      <Chip tone={recommend ? 'success' : 'outline'}>
        {t(recommend ? 'setup.services.recommended' : 'setup.services.optional')}
      </Chip>
      {link}
    </span>
  );
}

/** A service without a provider card (e-mail, Web Push, Telegram): its name, badge and purpose. */
function ServiceBlock({
  name,
  purpose,
  recommend,
  link,
  children,
}: {
  name: string;
  purpose: string;
  recommend: boolean;
  link?: ReactNode;
  children: ReactNode;
}) {
  const id = useId();
  return (
    <section className={styles.section} aria-labelledby={id}>
      <h3 id={id} className={styles.sectionTitle}>
        {name}
      </h3>
      <Text tone="secondary">{purpose}</Text>
      <Badge recommend={recommend} link={link} />
      {children}
    </section>
  );
}

/** Web Push of this device (F-13): the subscription of the owner's browser, as in the settings. */
function PushService() {
  const { t } = useTranslation();
  const channels = useQuery(channelsQuery);
  const push = channels.data?.channels.find((setting) => setting.channel === 'push');
  return (
    <ServiceBlock
      name={t('setup.services.push.name')}
      purpose={t('setup.services.push.purpose')}
      recommend={false}
    >
      {!pushSupported() && window.isSecureContext !== true ? (
        <Notice live={false}>{t('setup.services.push.insecure')}</Notice>
      ) : push ? (
        <PushDevices setting={push} focusTitleIfLost={() => undefined} />
      ) : channels.isError ? (
        <Notice tone="danger">{problemMessage(t, channels.error)}</Notice>
      ) : (
        <Skeleton shape="line" />
      )}
    </ServiceBlock>
  );
}

function ServiceCards({ settings }: { settings: AdminSettings }) {
  const { t } = useTranslation();
  const queryClient = useQueryClient();
  const navigate = useNavigate();
  const [asking, setAsking] = useState(false);
  const fields = useDrafts<string>(useCallback(() => '', []));
  const email = useDrafts<EmailValues>(
    useCallback(
      (id: string) => (id === EMAIL_DRAFT ? emailValues(settings.email) : undefined),
      [settings.email],
    ),
  );
  // A key the provider refused (KeyState.rejected) is not left behind unnoticed (3.3).
  const rejected = PROVIDERS.flatMap((provider) =>
    settings.keys[provider.keys[0]].rejected === true &&
    settings.keys[provider.keys[0]].source === 'stored'
      ? [provider]
      : [],
  );
  const advance = useMutation({
    mutationFn: async (state: 'done' | 'skipped') => {
      if (state === 'done' && rejected.length > 0) {
        const clear: KeyName[] = rejected.flatMap((provider) => [...provider.keys]);
        queryClient.setQueryData(settingsQuery.queryKey, await updateSettings({ clear }));
      }
      queryClient.setQueryData(
        setupProgressQuery.queryKey,
        await updateSetupProgress({ step: 'services', state }),
      );
    },
    onSuccess: () => navigate({ to: setupPath('playback') }),
  });
  const link = (url: string, name: string) => (
    <TextLink href={url} external>
      {t('admin.integrations.providers.link', { name })}
    </TextLink>
  );
  const card = (provider: Provider) => {
    const name = t(`admin.integrations.providers.${provider.id}.name`);
    return (
      <ProviderCard
        key={provider.id}
        provider={provider}
        keys={settings.keys}
        drafts={fields}
        extra={<Badge recommend={recommended(provider)} link={link(provider.url, name)} />}
      />
    );
  };

  return (
    <SetupLayout
      step="services"
      intro={t('setup.steps.services.intro')}
      footer={{
        onNext: () => (rejected.length > 0 ? setAsking(true) : advance.mutate('done')),
        isPending: advance.isPending,
        onSkip: () => advance.mutate('skipped'),
      }}
      error={advance.error ? problemMessage(t, advance.error) : undefined}
    >
      <section className={styles.section} aria-labelledby="setup-services-recommended">
        <h2 id="setup-services-recommended" className={styles.sectionTitle}>
          {t('setup.services.recommendedTitle')}
        </h2>
        {PROVIDERS.filter(recommended).map(card)}
        <ServiceBlock
          name={t('admin.integrations.email.title')}
          purpose={t('setup.services.email.purpose')}
          recommend
        >
          <EmailForm email={settings.email} drafts={email} fields={fields} />
        </ServiceBlock>
      </section>
      <section className={styles.section} aria-labelledby="setup-services-optional">
        <h2 id="setup-services-optional" className={styles.sectionTitle}>
          {t('setup.services.optionalTitle')}
        </h2>
        {PROVIDERS.filter((provider) => !recommended(provider)).map(card)}
        <PushService />
        <ServiceBlock
          name={t('admin.integrations.telegram.title')}
          purpose={t('setup.services.telegram.purpose')}
          recommend={false}
          link={link(BOTFATHER_URL, t('admin.integrations.telegram.name'))}
        >
          <TelegramForm telegram={settings.telegram} fields={fields} />
        </ServiceBlock>
      </section>
      <ConfirmDialog
        isOpen={asking}
        onOpenChange={setAsking}
        title={t('setup.services.rejected.title')}
        body={t('setup.services.rejected.body', {
          names: rejected
            .map((provider) => t(`admin.integrations.providers.${provider.id}.name`))
            .join(t('format.separator')),
        })}
        confirmLabel={t('setup.services.rejected.confirm')}
        cancelLabel={t('common.cancel')}
        tone="danger"
        onConfirm={() => advance.mutate('done')}
      />
    </SetupLayout>
  );
}

/**
 * Step 4 (concept 22, 3.3, F-6, F-13): every service as in Verwaltung › Integrationen, the same
 * cards: TMDB and e-mail recommended, TheTVDB, Fanart.tv, YouTube, Trakt, Google Books, Web Push
 * and Telegram optional, each with its badge and the link where the key is requested. Saving
 * stores, «Prüfen» asks the provider; «Weiter» offers to remove a key the provider refused.
 */
export function ServicesStep() {
  const { t } = useTranslation();
  const settings = useQuery(settingsQuery);
  if (settings.data) return <ServiceCards settings={settings.data} />;
  return (
    <SetupLayout step="services" intro={t('setup.steps.services.intro')}>
      {settings.isError ? (
        <Notice tone="danger">{problemMessage(t, settings.error)}</Notice>
      ) : (
        <Skeleton shape="block" />
      )}
    </SetupLayout>
  );
}
