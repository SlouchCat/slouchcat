import { useQuery } from '@tanstack/react-query';
import { useCallback, useRef, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { problemMessage } from '@/api/errors';
import { useActiveSession } from '@/app/session';
import { useDocumentTitle } from '@/lib/useDocumentTitle';
import { Notice, Skeleton, Text } from '@/ui';
import {
  AdminPage,
  AriaButton,
  BandPanel,
  ConfirmDialog,
  useDrafts,
  useUnsavedChanges,
} from '@/ui/aria';
import { EmailForm } from './integrations/EmailForm';
import {
  PROVIDER_KEYS,
  PROVIDERS,
  ProviderCard,
  providerSource,
} from './integrations/ProviderCard';
import { PushBlock } from './integrations/PushBlock';
import { ServerBlock } from './integrations/ServerBlock';
import {
  BandMeta,
  EMAIL_DRAFT,
  emailValues,
  keyDraft,
  PASSWORD_DRAFT,
  PUBLIC_URL_DRAFT,
  TELEGRAM_DRAFT,
  type EmailValues,
} from './integrations/shared';
import { TelegramForm } from './integrations/TelegramForm';
import { integrationsQuery, settingsQuery } from './queries';
import { WebhooksSection, type PathMapDraft } from './WebhooksSection';
import styles from './integrations/Integrations.module.css';

/**
 * Integrations (A-2, M7's key page; concept 12, variant A): six blocks that fold away by their
 * band (provider keys, the incoming webhooks of Radarr and Sonarr, the e-mail server with a test
 * mail, the Telegram bot, the Web Push keys, and the server's update check and public address).
 * Every block starts open; unsaved input lives in the page (useDrafts), so folding a block or a
 * webhook away keeps it, and leaving the page with it asks first (S-13, S-17). Secrets are
 * write-only: a stored key shows its last four characters at most; a value from deploy/.env
 * cannot be changed here.
 */
export function IntegrationsPage() {
  const { t } = useTranslation();
  useDocumentTitle(t('admin.integrations.title'));
  const session = useActiveSession();
  const allowed = session.account.role === 'admin' && session.profile.isPrimary;
  const settings = useQuery({ ...settingsQuery, enabled: allowed });
  const integrations = useQuery({ ...integrationsQuery, enabled: allowed });
  const titleRef = useRef<HTMLHeadingElement>(null);

  // The originals of the drafts: secrets are write-only, so their original is empty.
  const data = settings.data;
  const fields = useDrafts<string>(
    useCallback(
      (id: string) => (data ? (id === PUBLIC_URL_DRAFT ? (data.publicUrl ?? '') : '') : undefined),
      [data],
    ),
  );
  const email = useDrafts<EmailValues>(
    useCallback(
      (id: string) => (data && id === EMAIL_DRAFT ? emailValues(data.email) : undefined),
      [data],
    ),
  );
  const list = integrations.data;
  const pathMaps = useDrafts<PathMapDraft>(
    useCallback(
      (id: string) => {
        const integration = list?.find((entry) => String(entry.id) === id);
        return integration && { base: integration.pathMap, rows: integration.pathMap };
      },
      [list],
    ),
  );
  // The name of a new webhook integration: not a draft of an existing item, but typed input all
  // the same, so leaving the page asks for it too (S-13).
  const [newName, setNewName] = useState('');
  const webhooksDirty = pathMaps.dirty || newName.trim() !== '';
  const unsaved = useUnsavedChanges(fields.dirty || email.dirty || webhooksDirty);
  const keysDirty = PROVIDER_KEYS.some((keyName) => fields.isDirty(keyDraft(keyName)));
  // A provider counts as set up by its main key (concept 20, P-12).
  const keysSet = data
    ? PROVIDERS.filter((provider) => providerSource(provider, data.keys) !== 'none').length
    : 0;

  return (
    <AdminPage
      title={t('admin.integrations.title')}
      help={t('admin.integrations.intro')}
      titleRef={titleRef}
    >
      {!allowed ? (
        <Notice title={t('admin.access.notAllowed.title')}>
          {t('admin.access.notAllowed.body')}
        </Notice>
      ) : (
        <div className={styles.blocks} aria-busy={settings.isPending}>
          {settings.isPending && <Skeleton shape="block" />}
          {settings.isError && (
            <Notice
              tone="danger"
              title={problemMessage(t, settings.error)}
              actions={
                <AriaButton icon="refresh" onPress={() => void settings.refetch()}>
                  {t('common.retry')}
                </AriaButton>
              }
            />
          )}
          {data && (
            <>
              <BandPanel
                collapsible
                icon="lock"
                title={t('admin.integrations.keys.title')}
                meta={
                  <BandMeta
                    unsaved={keysDirty}
                    count={t('admin.integrations.keys.count', {
                      set: keysSet,
                      total: PROVIDERS.length,
                    })}
                  />
                }
              >
                <Text tone="secondary">{t('admin.integrations.keys.intro')}</Text>
                {/* The first work row of the page (docs/perf/verwaltungsseiten.md). */}
                <div data-perf="first-work-row" className={styles.providers}>
                  {PROVIDERS.map((provider) => (
                    <ProviderCard
                      key={provider.id}
                      provider={provider}
                      keys={data.keys}
                      drafts={fields}
                    />
                  ))}
                </div>
              </BandPanel>
              <BandPanel
                collapsible
                icon="plug"
                title={t('admin.integrations.webhooks.title')}
                meta={
                  <BandMeta
                    unsaved={webhooksDirty}
                    count={
                      list
                        ? t('admin.integrations.webhooks.count', { count: list.length })
                        : undefined
                    }
                  />
                }
              >
                <WebhooksSection
                  drafts={pathMaps}
                  newName={newName}
                  onNewNameChange={setNewName}
                  onRemoved={() => titleRef.current?.focus()}
                />
              </BandPanel>
              <BandPanel
                collapsible
                icon="megaphone"
                title={t('admin.integrations.email.title')}
                meta={
                  <BandMeta
                    source={data.email.source}
                    unsaved={email.dirty || fields.isDirty(PASSWORD_DRAFT)}
                  />
                }
              >
                <EmailForm email={data.email} drafts={email} fields={fields} />
              </BandPanel>
              <BandPanel
                collapsible
                icon="external"
                title={t('admin.integrations.telegram.title')}
                meta={
                  <BandMeta
                    source={data.telegram.source}
                    unsaved={fields.isDirty(TELEGRAM_DRAFT)}
                  />
                }
              >
                <TelegramForm telegram={data.telegram} fields={fields} />
              </BandPanel>
              <BandPanel collapsible icon="bell" title={t('admin.integrations.push.title')}>
                <PushBlock push={data.push} />
              </BandPanel>
              <BandPanel
                collapsible
                icon="server"
                title={t('admin.integrations.server.title')}
                meta={<BandMeta unsaved={fields.isDirty(PUBLIC_URL_DRAFT)} />}
              >
                <ServerBlock settings={data} fields={fields} />
              </BandPanel>
            </>
          )}
        </div>
      )}
      <ConfirmDialog {...unsaved.dialog} />
    </AdminPage>
  );
}
