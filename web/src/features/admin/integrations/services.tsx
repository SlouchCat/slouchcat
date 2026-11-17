import type { Dashboard, IntegrationStatus } from '@slouchcat/api-client';
import type { ReactNode, Ref } from 'react';
import { useTranslation } from 'react-i18next';
import { paths } from '@/app/paths';
import { AppLink, Icon, StatusDot, Text, type IconName, type StatusDotState } from '@/ui';
import styles from './Integrations.module.css';

/**
 * The services of the integrations page (mockup: one tile each, each with a view of its own),
 * in their order. `?item=` names the open one; without it the page shows the tiles.
 */
export const SERVICES = [
  'keys',
  'webhooks',
  'seerr',
  'email',
  'telegram',
  'push',
  'server',
] as const;
export type Service = (typeof SERVICES)[number];

/** The address of a service's view: the deep link of its tile. */
export const serviceHref = (service: Service) => `${paths.adminIntegrations}?item=${service}`;

export const SERVICE_ICON = {
  keys: 'lock',
  webhooks: 'plug',
  seerr: 'heart',
  email: 'megaphone',
  telegram: 'external',
  push: 'bell',
  server: 'server',
} as const satisfies Record<Service, IconName>;

/**
 * The dashboard's integration keys each service stands for (`getDashboard`, eight keys). Seerr has
 * none yet: its tile and view read getSeerrStatus (SeerrStateLine); with `seerr` in
 * `IntegrationStatus.key` it goes here and SeerrStateLine goes.
 */
const STATUS_KEYS = {
  keys: ['tmdb', 'tvdb', 'youtube', 'trakt'],
  webhooks: [],
  seerr: [],
  email: ['email'],
  telegram: ['telegram'],
  push: ['push'],
  server: ['updateCheck'],
} as const satisfies Record<Service, readonly IntegrationStatus['key'][]>;

/**
 * The address of the view that holds a dashboard integration (the overview's error notice links
 * it); an integration no service names leads to the tiles.
 */
export const integrationHref = (key: IntegrationStatus['key']) => {
  const service = SERVICES.find((entry) => (STATUS_KEYS[entry] as readonly string[]).includes(key));
  return service ? serviceHref(service) : paths.adminIntegrations;
};

/** The dot of an integration's state; the text next to it carries the meaning. */
const integrationDot = {
  configured: 'success',
  missing: 'pending',
  error: 'danger',
  disabled: 'pending',
} as const satisfies Record<IntegrationStatus['state'], StatusDotState>;

/**
 * The state of a service as the dashboard knows it (build plan 7.2): each of its integrations
 * with its state and the reason of an error, and for the webhooks the reports of the last
 * 24 hours and the ones gone quiet. The same texts as the overview's cards before.
 */
export function ServiceStatus({ service, dashboard }: { service: Service; dashboard: Dashboard }) {
  const { t } = useTranslation();
  if (service === 'webhooks') {
    const { webhooks } = dashboard;
    if (webhooks.total === 0) {
      return (
        <Text as="p" variant="caption" tone="secondary">
          {t('admin.home.webhooks.none')}
        </Text>
      );
    }
    return (
      <ul className={styles.status}>
        <li>
          {t('admin.home.webhooks.count', {
            count: webhooks.eventsLast24h,
            total: webhooks.total,
          })}
        </li>
        {webhooks.silent.length > 0 && (
          <li className={styles.statusEntry}>
            <StatusDot state="warning" />
            {t('admin.home.webhooks.silent', {
              names: webhooks.silent.join(t('format.listSeparator')),
            })}
          </li>
        )}
      </ul>
    );
  }
  const keys: readonly string[] = STATUS_KEYS[service];
  const entries = dashboard.integrations.filter((entry) => keys.includes(entry.key));
  if (entries.length === 0) return null;
  return (
    <ul className={styles.status}>
      {entries.map((integration) => (
        <li key={integration.key}>
          <span className={styles.statusEntry}>
            <StatusDot state={integrationDot[integration.state]} />
            {t('admin.home.integrations.entry', {
              name: t(`admin.integrations.names.${integration.key}`),
              state: t(`admin.home.integrations.state.${integration.state}`),
            })}
          </span>
          {integration.detail && <span className={styles.statusDetail}>{integration.detail}</span>}
        </li>
      ))}
    </ul>
  );
}

/**
 * A service's tile: its name as the link into its view (the whole tile answers the pointer),
 * what it is for, what is set up (and «ungespeichert»), and its state from the dashboard.
 */
export function ServiceTile({
  service,
  title,
  description,
  meta,
  status,
  linkRef,
}: {
  service: Service;
  title: string;
  description: string;
  meta: ReactNode;
  status: ReactNode;
  linkRef: Ref<HTMLAnchorElement>;
}) {
  return (
    <li className={styles.tile}>
      <span className={styles.tileIcon}>
        <Icon name={SERVICE_ICON[service]} size="sm" />
      </span>
      <div className={styles.tileText}>
        <h2 className={styles.tileTitle}>
          <AppLink ref={linkRef} href={serviceHref(service)} className={styles.tileLink}>
            {title}
          </AppLink>
        </h2>
        <Text as="p" variant="caption" tone="secondary">
          {description}
        </Text>
        {meta && <div className={styles.tileMeta}>{meta}</div>}
        {status}
      </div>
      <span className={styles.tileArrow}>
        <Icon name="chevronRight" size="sm" />
      </span>
    </li>
  );
}
