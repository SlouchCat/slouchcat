import { useQuery } from '@tanstack/react-query';
import { useEffect, useMemo, useRef, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { useActiveSession } from '@/app/session';
import { intlLocale } from '@/i18n';
import { useDocumentTitle } from '@/lib/useDocumentTitle';
import { Notice } from '@/ui';
import { AdminPage, StatusStrip, Tabs, useSearchSelection } from '@/ui/aria';
import { seerrStatusQuery, seerrUsersQuery, wishMappingsQuery, wishSettingsQuery } from './queries';
import { SeerrUsersTab } from './wishes/SeerrUsersTab';
import { SettingsTab } from './wishes/SettingsTab';

/**
 * The tabs (concept 25, administration 1.2); the ids are part of links and stay stable. The tab
 * «Anfragen» comes with a later build step of the same release.
 */
const TABS = ['seerr-benutzer', 'einstellungen'] as const;

/** How long «Benutzerliste neu laden» reads the lists again, every two seconds, for the job. */
const RELOAD_POLL_MS = 2000;
const RELOAD_WINDOW_MS = 20_000;

/**
 * Medien › Wünsche (concept 25, M-24 and M-25): the Seerr users of the accounts and the settings
 * of the wishes, as tabs (`?tab=`). The counters of the Seerr users stand in the page's head while
 * their tab is open. Both tabs take effect at once, so the page holds no draft.
 */
export function WishesPage() {
  const { t } = useTranslation();
  useDocumentTitle(t('admin.wishes.title'));
  const session = useActiveSession();
  if (session.account.role !== 'admin' || !session.profile.isPrimary) {
    return (
      <AdminPage title={t('admin.wishes.title')}>
        <Notice title={t('admin.access.notAllowed.title')}>
          {t('admin.access.notAllowed.body')}
        </Notice>
      </AdminPage>
    );
  }
  return <WishesContent />;
}

function WishesContent() {
  const { t, i18n } = useTranslation();
  const number = useMemo(() => new Intl.NumberFormat(intlLocale(i18n.language)), [i18n.language]);
  const titleRef = useRef<HTMLHeadingElement>(null);
  const [tab, selectTab] = useSearchSelection('tab', TABS, 'seerr-benutzer', { normalise: true });
  // The rows that are open live in the page, so a change of tab keeps them.
  const [expanded, setExpanded] = useState<ReadonlySet<string>>(() => new Set());
  const [reloadUntil, setReloadUntil] = useState(0);
  const reloading = reloadUntil > 0;
  useEffect(() => {
    if (reloadUntil === 0) return;
    const timer = setTimeout(() => setReloadUntil(0), reloadUntil - Date.now());
    return () => clearTimeout(timer);
  }, [reloadUntil]);
  const poll = reloading ? RELOAD_POLL_MS : false;

  const status = useQuery(seerrStatusQuery);
  const settings = useQuery(wishSettingsQuery);
  const mappings = useQuery({ ...wishMappingsQuery, refetchInterval: poll });
  const users = useQuery({ ...seerrUsersQuery, refetchInterval: poll });

  const accounts = mappings.data?.accounts ?? [];
  const waiting = accounts.filter((mapping) => mapping.state === 'waiting').length;
  const without = accounts.filter((mapping) => mapping.seerrUserId === undefined).length;
  const passwordOpen = accounts.filter((mapping) => mapping.passwordState === 'next_login').length;
  const shown = (value: number) => (mappings.data ? number.format(value) : '–');
  const strip = (
    <StatusStrip
      label={t('admin.wishes.strip.label')}
      tiles={[
        { key: 'accounts', label: t('admin.wishes.strip.accounts'), value: shown(accounts.length) },
        {
          key: 'opened',
          label: t('admin.wishes.strip.opened'),
          value: shown(mappings.data?.opened ?? 0),
          note: t('admin.wishes.strip.openedNote'),
        },
        {
          key: 'without',
          label: t('admin.wishes.strip.without'),
          value: shown(without),
          note: waiting > 0 ? t('admin.wishes.strip.waiting', { count: waiting }) : undefined,
          tone: without > 0 ? 'warning' : 'neutral',
        },
        {
          key: 'password',
          label: t('admin.wishes.strip.password'),
          value: shown(passwordOpen),
          note: t('admin.wishes.strip.passwordNote'),
        },
      ]}
    />
  );

  return (
    <AdminPage
      title={t('admin.wishes.title')}
      help={t('admin.wishes.intro')}
      titleRef={titleRef}
      status={tab === 'seerr-benutzer' ? strip : undefined}
    >
      <Tabs
        label={t('admin.wishes.tabs.label')}
        selectedKey={tab}
        onSelectionChange={selectTab}
        tabs={[
          {
            id: 'seerr-benutzer',
            label: t('admin.wishes.tabs.users'),
            content: (
              <SeerrUsersTab
                mappings={mappings}
                users={users}
                settings={settings.data}
                status={status.data}
                expanded={expanded}
                onExpandedChange={(key, open) =>
                  setExpanded((current) => {
                    const next = new Set(current);
                    if (open) next.add(key);
                    else next.delete(key);
                    return next;
                  })
                }
                onReloaded={() => setReloadUntil(Date.now() + RELOAD_WINDOW_MS)}
              />
            ),
          },
          {
            id: 'einstellungen',
            label: t('admin.wishes.tabs.settings'),
            content: (
              <SettingsTab settings={settings} status={status.data} mappings={mappings.data} />
            ),
          },
        ]}
      />
    </AdminPage>
  );
}
