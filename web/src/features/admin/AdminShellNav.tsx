import type { Dashboard } from '@slouchcat/api-client';
import { useQuery } from '@tanstack/react-query';
import { useLocation } from '@tanstack/react-router';
import type { RefObject } from 'react';
import { useTranslation } from 'react-i18next';
import { paths } from '@/app/paths';
import { useActiveSession } from '@/app/session';
import type { IconName } from '@/ui';
import {
  AdminSideNav,
  AdminSideNavMenu,
  type AdminSideNavEntry,
  type AdminSideNavGroup,
  type AdminSideNavProps,
  type AdminSideNavTone,
} from '@/ui/aria';
import { dashboardQuery } from './queries';
import { adminGroups, adminSections, isSectionPath, type AdminGroup } from './sections';

/** Poll interval of the sidebar's numbers on the admin pages but the overview, which asks itself. */
const SHELL_POLL_MS = 30_000;

const groupIcons = {
  library: 'film',
  accounts: 'users',
  curation: 'star',
  system: 'server',
} as const satisfies Record<AdminGroup, IconName>;

type Figure = Pick<AdminSideNavEntry, 'count' | 'alarm'> & { accessibleLabel: string };

/** The numbers with a spoken name of their own (`admin.nav.counts.*`). */
type SpokenCount =
  'titles' | 'maintenance' | 'accounts' | 'streams' | 'jobsRunning' | 'jobsFailed' | 'silent';

/**
 * The entries of the sidebar (concept 11, 11.1): back to the app, the overview with the update
 * badge (A-5), then every admin page in its group. The numbers come from the dashboard after F-7
 * (few muted counts, coloured pills for what needs a look); without it (loading, an error) the
 * entries stand without numbers.
 */
function useAdminNav(): { nav: Omit<AdminSideNavProps, 'focusCarry'>; pageLabel: string } {
  const { t, i18n } = useTranslation();
  const session = useActiveSession();
  const { pathname } = useLocation();
  // The overview asks every 5 s itself (and not at all while the server restarts).
  const dashboard = useQuery({
    ...dashboardQuery,
    refetchInterval: pathname === paths.admin ? false : SHELL_POLL_MS,
  });
  const number = new Intl.NumberFormat(i18n.language);

  /** The spoken name of an entry with its number ("Pflege, 12 Titel brauchen Pflege"). */
  const spoken = (kind: SpokenCount, name: string, count: number) =>
    t(`admin.nav.counts.${kind}`, { name, count, formatted: number.format(count) });
  const muted = (kind: SpokenCount, name: string, count: number): Figure => ({
    count: number.format(count),
    accessibleLabel: spoken(kind, name, count),
  });
  const pill = (
    kind: SpokenCount,
    name: string,
    count: number,
    tone: AdminSideNavTone,
  ): Figure => ({
    alarm: { value: number.format(count), tone },
    accessibleLabel: spoken(kind, name, count),
  });

  const figure = (key: string, name: string, data: Dashboard): Figure | undefined => {
    const { counts, jobs } = data;
    switch (key) {
      case 'libraries':
        return muted('titles', name, counts.titles);
      case 'maintenance':
        return counts.maintenanceOpen > 0
          ? pill('maintenance', name, counts.maintenanceOpen, 'warning')
          : undefined;
      case 'accounts':
        return muted('accounts', name, counts.accounts);
      case 'streams':
        return data.streams.length > 0
          ? pill('streams', name, data.streams.length, 'accent')
          : undefined;
      case 'jobs':
        // A failure replaces the number of running jobs.
        if (jobs.failed > 0) return pill('jobsFailed', name, jobs.failed, 'danger');
        return jobs.running > 0 ? muted('jobsRunning', name, jobs.running) : undefined;
      case 'integrations': {
        // A silent webhook replaces «set up / total».
        const silent = data.webhooks.silent.length;
        if (silent > 0) return pill('silent', name, silent, 'warning');
        const configured = data.integrations.filter(
          (integration) => integration.state === 'configured',
        ).length;
        const total = data.integrations.length;
        return {
          count: t('admin.nav.counts.ofTotal', {
            count: number.format(configured),
            total: number.format(total),
          }),
          accessibleLabel: t('admin.nav.counts.integrations', { name, configured, total }),
        };
      }
      default:
        return undefined;
    }
  };

  const update = session.updateAvailable;
  const overviewLink = {
    href: paths.admin,
    label: t('admin.nav.overview'),
    icon: 'grid',
    current: pathname === paths.admin,
  } as const;
  const overview: AdminSideNavEntry = update
    ? {
        ...overviewLink,
        badge: t('nav.update'),
        accessibleLabel: t('admin.nav.updateLabel', { version: update.version }),
      }
    : overviewLink;
  const top: AdminSideNavEntry[] = [
    { href: paths.home, label: t('admin.nav.back'), icon: 'back' },
    overview,
  ];
  const groups: AdminSideNavGroup[] = adminGroups.map((group) => ({
    label: t(`admin.nav.groups.${group}`),
    icon: groupIcons[group],
    entries: adminSections
      .filter((section) => section.group === group)
      .map((section): AdminSideNavEntry => {
        const label = t(`admin.sections.${section.key}.title`);
        const link = {
          href: section.href,
          label,
          icon: section.icon,
          current: isSectionPath(section, pathname),
        };
        const numbers = dashboard.data && figure(section.key, label, dashboard.data);
        return numbers ? { ...link, ...numbers } : link;
      }),
  }));
  const current = [...top, ...groups.flatMap((group) => group.entries)].find(
    (entry) => entry.current,
  );
  return {
    nav: { label: t('admin.nav.label'), top, groups },
    pageLabel: current?.label ?? t('admin.nav.overview'),
  };
}

/**
 * The administration's sidebar in the shell's column (AppLayout, from 768 px). The shell keeps it
 * between the admin pages; `focusCarry` brings the focused entry back when it mounts again.
 */
export function AdminShellNav({ focusCarry }: { focusCarry: RefObject<number | null> }) {
  const { nav } = useAdminNav();
  return <AdminSideNav {...nav} focusCarry={focusCarry} />;
}

/** The same entries on a phone: the button «Verwaltung · page» with the sheet (AdminLayout). */
export function AdminShellMenu() {
  const { t } = useTranslation();
  const { nav, pageLabel } = useAdminNav();
  return (
    <AdminSideNavMenu
      {...nav}
      buttonLabel={t('admin.nav.label')}
      pageLabel={pageLabel}
      menuLabel={t('admin.nav.sheet.label')}
      closeLabel={t('admin.nav.sheet.close')}
    />
  );
}
