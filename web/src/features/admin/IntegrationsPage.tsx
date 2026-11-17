import { useQuery } from '@tanstack/react-query';
import { useCallback, useEffect, useRef, useState, type ReactNode } from 'react';
import { useTranslation } from 'react-i18next';
import { problemMessage } from '@/api/errors';
import { paths } from '@/app/paths';
import { useActiveSession } from '@/app/session';
import { useDocumentTitle } from '@/lib/useDocumentTitle';
import { Notice, Skeleton, Text, TextLink } from '@/ui';
import {
  AdminPage,
  AriaButton,
  BandPanel,
  ConfirmDialog,
  useDrafts,
  useSearchSelection,
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
import { ownAddress } from './integrations/SeerrArrBand';
import { SeerrView } from './integrations/SeerrView';
import { ServerBlock } from './integrations/ServerBlock';
import {
  SERVICE_ICON,
  SERVICES,
  ServiceStatus,
  ServiceTile,
  type Service,
} from './integrations/services';
import {
  BandMeta,
  EMAIL_DRAFT,
  emailValues,
  keyDraft,
  PASSWORD_DRAFT,
  PUBLIC_URL_DRAFT,
  SEERR_DRAFT,
  SEERR_KEY_DRAFT,
  seerrValues,
  TELEGRAM_DRAFT,
  type EmailValues,
  type SeerrValues,
} from './integrations/shared';
import { TelegramForm } from './integrations/TelegramForm';
import {
  dashboardQuery,
  integrationsQuery,
  seerrSettingsQuery,
  seerrStatusQuery,
  arrStatusQuery,
  settingsQuery,
  systemQuery,
  wishMappingsQuery,
} from './queries';
import { WebhooksSection, type PathMapDraft } from './WebhooksSection';
import styles from './integrations/Integrations.module.css';

/**
 * Integrations (A-2, M7's key page; concept 12, variant A; admin redesign): seven services
 * (provider keys, the incoming webhooks of Radarr and Sonarr, Seerr for the wishes (concept 25),
 * the e-mail server with a test mail,
 * the Telegram bot, the Web Push keys, and the server's update check, public address and
 * graphics chip). Without `?item=` the page shows a tile per service with what is set up and its
 * state from the dashboard (build plan 7.2); `?item=<service>` shows that service's view. All
 * views stay mounted (hidden but the open one), so a status line, a pending save or a webhook
 * address shown once outlive a change of view, and unsaved input lives in the page (useDrafts):
 * switching views keeps it, leaving the page with it asks first (S-13, S-17). Secrets are
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
  const system = useQuery({ ...systemQuery, enabled: allowed });
  const seerr = useQuery({ ...seerrSettingsQuery, enabled: allowed });
  const seerrStatus = useQuery({ ...seerrStatusQuery, enabled: allowed });
  const seerrConfigured = seerr.data !== undefined && seerr.data.urlSource !== 'none';
  const mappings = useQuery({ ...wishMappingsQuery, enabled: allowed && seerrConfigured });
  const arrStatus = useQuery({ ...arrStatusQuery, enabled: allowed && seerrConfigured });
  // The states of the integrations and webhooks (build plan 7.2): the dashboard the admin shell
  // asks every 30 s; this page reads it along without a poll of its own. Without it the page
  // works all the same and shows no states.
  const dashboard = useQuery({ ...dashboardQuery, enabled: allowed, refetchInterval: false }).data;
  const [item] = useSearchSelection('item', SERVICES, '', { normalise: true });
  const open = item === '' ? undefined : (item as Service);
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
  const seerrData = seerr.data;
  const seerrDrafts = useDrafts<SeerrValues>(
    useCallback(
      (id: string) => (seerrData && id === SEERR_DRAFT ? seerrValues(seerrData) : undefined),
      [seerrData],
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
  // The own addresses of Radarr and Sonarr (stage 4), drafted against the stored ones.
  const arrData = arrStatus.data;
  const arrDrafts = useDrafts<string>(
    useCallback(
      (id: string) =>
        arrData && (id === 'radarr' || id === 'sonarr') ? ownAddress(arrData[id]) : undefined,
      [arrData],
    ),
  );
  const seerrDirty = seerrDrafts.dirty || fields.isDirty(SEERR_KEY_DRAFT) || arrDrafts.dirty;
  const unsaved = useUnsavedChanges(fields.dirty || email.dirty || webhooksDirty || seerrDirty);
  const keysDirty = PROVIDER_KEYS.some((keyName) => fields.isDirty(keyDraft(keyName)));
  // A provider counts as set up by its main key (concept 20, P-12).
  const keysSet = data
    ? PROVIDERS.filter((provider) => providerSource(provider, data.keys) !== 'none').length
    : 0;
  const keysCount = t('admin.integrations.keys.count', { set: keysSet, total: PROVIDERS.length });
  const webhooksCount = list
    ? t('admin.integrations.webhooks.count', { count: list.length })
    : undefined;
  const seerrCount = mappings.data
    ? t('admin.integrations.seerr.accounts', {
        opened: mappings.data.opened,
        total: mappings.data.total,
      })
    : undefined;
  /** What a tile says is set up: the same as its view's band. */
  const tileMeta = (service: Service) => {
    if (service === 'keys') return { count: keysCount };
    if (service === 'webhooks') return { count: webhooksCount };
    if (service === 'seerr') return { count: seerrCount, source: seerrData?.urlSource };
    if (service === 'email') return { source: data?.email.source };
    if (service === 'telegram') return { source: data?.telegram.source };
    if (service === 'push' && data?.push.configured) {
      return { count: t('admin.integrations.push.configured') };
    }
    return {};
  };

  const unsavedOf: Record<Service, boolean> = {
    keys: keysDirty,
    webhooks: webhooksDirty,
    seerr: seerrDirty,
    email: email.dirty || fields.isDirty(PASSWORD_DRAFT),
    telegram: fields.isDirty(TELEGRAM_DRAFT),
    push: false,
    server: fields.isDirty(PUBLIC_URL_DRAFT),
  };

  // The focus follows a change of view: into the view's heading, and back to the tile of the
  // service that was open. Arriving with `?item=` leaves it where the page puts it. It moves in a
  // microtask after the commit, as focusOnMount does: the blur of a field must not run inside it.
  const views = useRef<Partial<Record<Service, HTMLDivElement | null>>>({});
  const tiles = useRef<Partial<Record<Service, HTMLAnchorElement | null>>>({});
  const shown = useRef(open);
  useEffect(() => {
    const before = shown.current;
    shown.current = open;
    if (before === open) return;
    const target = open
      ? views.current[open]?.querySelector<HTMLElement>('h2')
      : before && tiles.current[before];
    if (!target) return;
    if (open) target.tabIndex = -1;
    queueMicrotask(() => {
      if (target.isConnected) target.focus();
    });
  }, [open]);

  const configured = dashboard?.integrations.filter((entry) => entry.state === 'configured').length;

  /** A service's view: its band with the dashboard's state on top; hidden unless it is open. */
  const view = (service: Service, title: string, meta: ReactNode, children: ReactNode) => (
    <div
      key={service}
      ref={(element) => {
        views.current[service] = element;
      }}
      hidden={open !== service}
    >
      <BandPanel icon={SERVICE_ICON[service]} title={title} meta={meta}>
        {dashboard && <ServiceStatus service={service} dashboard={dashboard} />}
        {children}
      </BandPanel>
    </div>
  );

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
              {open ? (
                <div className={styles.back}>
                  <TextLink href={paths.adminIntegrations}>
                    {t('admin.integrations.services.back')}
                  </TextLink>
                </div>
              ) : (
                <>
                  {dashboard && configured !== undefined && (
                    <Text tone="secondary">
                      {t('admin.home.integrations.count', {
                        count: configured,
                        total: dashboard.integrations.length,
                      })}
                    </Text>
                  )}
                  {/* The first work row of the page (docs/perf/verwaltungsseiten.md). */}
                  <ul
                    data-perf="first-work-row"
                    className={styles.tiles}
                    aria-label={t('admin.integrations.services.label')}
                  >
                    {SERVICES.map((service) => (
                      <ServiceTile
                        key={service}
                        service={service}
                        title={t(`admin.integrations.${service}.title`)}
                        description={t(`admin.integrations.services.${service}`)}
                        meta={<BandMeta unsaved={unsavedOf[service]} {...tileMeta(service)} />}
                        status={
                          dashboard && <ServiceStatus service={service} dashboard={dashboard} />
                        }
                        linkRef={(element) => {
                          tiles.current[service] = element;
                        }}
                      />
                    ))}
                  </ul>
                </>
              )}
              {view(
                'keys',
                t('admin.integrations.keys.title'),
                <BandMeta unsaved={keysDirty} count={keysCount} />,
                <>
                  <Text tone="secondary">{t('admin.integrations.keys.intro')}</Text>
                  <div className={styles.providers}>
                    {PROVIDERS.map((provider) => (
                      <ProviderCard
                        key={provider.id}
                        provider={provider}
                        keys={data.keys}
                        drafts={fields}
                      />
                    ))}
                  </div>
                </>,
              )}
              {view(
                'webhooks',
                t('admin.integrations.webhooks.title'),
                <BandMeta unsaved={webhooksDirty} count={webhooksCount} />,
                <WebhooksSection
                  drafts={pathMaps}
                  newName={newName}
                  onNewNameChange={setNewName}
                  onRemoved={() => titleRef.current?.focus()}
                />,
              )}
              {view(
                'seerr',
                t('admin.integrations.seerr.title'),
                <BandMeta unsaved={seerrDirty} count={seerrCount} source={seerrData?.urlSource} />,
                seerr.isError ? (
                  <Notice
                    tone="danger"
                    title={problemMessage(t, seerr.error)}
                    actions={
                      <AriaButton icon="refresh" onPress={() => void seerr.refetch()}>
                        {t('common.retry')}
                      </AriaButton>
                    }
                  />
                ) : seerrData ? (
                  <SeerrView
                    settings={seerrData}
                    status={seerrStatus}
                    drafts={seerrDrafts}
                    fields={fields}
                    arrDrafts={arrDrafts}
                    onFocusLost={() => titleRef.current?.focus()}
                  />
                ) : (
                  <Skeleton shape="block" />
                ),
              )}
              {view(
                'email',
                t('admin.integrations.email.title'),
                <BandMeta source={data.email.source} unsaved={unsavedOf.email} />,
                <EmailForm email={data.email} drafts={email} fields={fields} />,
              )}
              {view(
                'telegram',
                t('admin.integrations.telegram.title'),
                <BandMeta source={data.telegram.source} unsaved={unsavedOf.telegram} />,
                <TelegramForm telegram={data.telegram} fields={fields} />,
              )}
              {view(
                'push',
                t('admin.integrations.push.title'),
                null,
                <PushBlock push={data.push} />,
              )}
              {view(
                'server',
                t('admin.integrations.server.title'),
                <BandMeta unsaved={unsavedOf.server} />,
                <ServerBlock settings={data} fields={fields} system={system.data} />,
              )}
            </>
          )}
        </div>
      )}
      <ConfirmDialog {...unsaved.dialog} />
    </AdminPage>
  );
}
