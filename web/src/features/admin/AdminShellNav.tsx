import type { Dashboard } from '@slouchcat/api-client';
import { useQuery } from '@tanstack/react-query';
import { useLocation } from '@tanstack/react-router';
import type { RefObject } from 'react';
import { useTranslation } from 'react-i18next';
import { paths } from '@/app/paths';
import { useActiveSession } from '@/app/session';
import {
  AdminSideNav,
  AdminSideNavMenu,
  type AdminSideNavEntry,
  type AdminSideNavGroup,
  type AdminSideNavProps,
  type AdminSideNavTone,
  type AreaTabsProps,
} from '@/ui/aria';
import { dashboardQuery } from './queries';
import {
  adminAreaIcons,
  adminAreas,
  currentSection,
  firstSectionOf,
  sectionsOf,
  type AdminArea,
  type AdminSection,
} from './sections';
import { intlLocale } from '@/i18n';

/** Poll interval of the sidebar's numbers on the admin pages but the overview, which asks itself. */
const SHELL_POLL_MS = 30_000;

type Figure = Pick<AdminSideNavEntry, 'count' | 'alarm'> & { accessibleLabel: string };

/** The numbers with a spoken name of their own (`admin.nav.counts.*`). */
type SpokenCount =
  'titles' | 'maintenance' | 'accounts' | 'streams' | 'jobsRunning' | 'jobsFailed' | 'silent';

/** What the admin frame needs: the column, the sheet, the area's tabs and where the page stands. */
interface AdminNav {
  /** The column from 768 px: back, then the overview and the five areas, the profile at the foot. */
  column: Omit<AdminSideNavProps, 'focusCarry'>;
  /** The sheet on a phone: back, the overview and the areas with all their pages. */
  sheet: Omit<AdminSideNavProps, 'focusCarry'>;
  /** The tabs of the current page's area; none on the overview. */
  tabs?: Omit<AreaTabsProps, 'focusCarry'>;
  /** Where the page stands: "Medien / Bibliotheken", or "Übersicht". */
  trail: string;
}

/**
 * The navigation of the administration (concept 11, 11.1; redesign, build plan 1 and 2): the
 * overview with the update badge (A-5), the five areas and every page in its area. The numbers
 * come from the dashboard after F-7 (few muted counts, coloured pills for what needs a look) and
 * stand on the pages' tabs and the sheet's entries; an area's entry shows only what needs a look
 * (build plan 1). Without the dashboard (loading, an error) the entries stand without numbers.
 */
export function useAdminNav(): AdminNav {
  const { t, i18n } = useTranslation();
  const session = useActiveSession();
  const { pathname } = useLocation();
  // The overview asks every 5 s itself (and not at all while the server restarts).
  const dashboard = useQuery({
    ...dashboardQuery,
    refetchInterval: pathname === paths.admin ? false : SHELL_POLL_MS,
  });
  const number = new Intl.NumberFormat(intlLocale(i18n.language));

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

  /** An area's entry shows only what needs a look (build plan 1). */
  const areaFigure = (area: AdminArea, name: string, data: Dashboard): Figure | undefined => {
    switch (area) {
      case 'media':
        return data.counts.maintenanceOpen > 0
          ? pill('maintenance', name, data.counts.maintenanceOpen, 'warning')
          : undefined;
      case 'people':
        return data.streams.length > 0
          ? pill('streams', name, data.streams.length, 'accent')
          : undefined;
      case 'automation':
        // A failed job before a silent webhook.
        if (data.jobs.failed > 0) return pill('jobsFailed', name, data.jobs.failed, 'danger');
        return data.webhooks.silent.length > 0
          ? pill('silent', name, data.webhooks.silent.length, 'warning')
          : undefined;
      default:
        return undefined;
    }
  };

  const current = currentSection(pathname);
  const areaName = (area: AdminArea) => t(`admin.nav.areas.${area}`);
  const pageName = (section: AdminSection) => t(`admin.sections.${section.key}.title`);
  const pageEntry = (section: AdminSection): AdminSideNavEntry => {
    const label = pageName(section);
    const link = { href: section.href, label, icon: section.icon, current: section === current };
    const numbers = dashboard.data && figure(section.key, label, dashboard.data);
    return numbers ? { ...link, ...numbers } : link;
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
  const back: AdminSideNavEntry = { href: paths.home, label: t('admin.nav.back'), icon: 'back' };
  const areas = adminAreas.map((area): AdminSideNavEntry => {
    const label = areaName(area);
    const link = {
      href: firstSectionOf(area).href,
      label,
      icon: adminAreaIcons[area],
      current: current?.area === area,
    };
    const numbers = dashboard.data && areaFigure(area, label, dashboard.data);
    return numbers ? { ...link, ...numbers } : link;
  });
  const groups: AdminSideNavGroup[] = adminAreas.map((area) => ({
    label: areaName(area),
    icon: adminAreaIcons[area],
    entries: sectionsOf(area).map(pageEntry),
  }));
  const label = t('admin.nav.label');
  return {
    column: {
      label,
      top: [back],
      heading: label,
      entries: [overview, ...areas],
      // Only an administrator's primary profile has the admin frame (AdminLayout).
      profile: { name: session.profile.name, role: t('nav.profileRole.admin') },
    },
    sheet: { label, top: [back, overview], groups },
    tabs: current && {
      label: t('admin.nav.tabs', { area: areaName(current.area) }),
      tabs: sectionsOf(current.area).map(pageEntry),
    },
    trail: current
      ? t('admin.nav.trail', { area: areaName(current.area), page: pageName(current) })
      : t('admin.nav.overview'),
  };
}

/**
 * The administration's sidebar in the shell's column (AppLayout, from 768 px). The shell keeps it
 * between the admin pages; `focusCarry` brings the focused entry back when it mounts again.
 */
export function AdminShellNav({ focusCarry }: { focusCarry: RefObject<number | null> }) {
  const { column } = useAdminNav();
  return <AdminSideNav {...column} focusCarry={focusCarry} />;
}

/**
 * On a phone: the button «Verwaltung › area / page» with the sheet of every page (AdminLayout).
 */
export function AdminShellMenu({ nav }: { nav: AdminNav }) {
  const { t } = useTranslation();
  return (
    <AdminSideNavMenu
      {...nav.sheet}
      buttonLabel={t('admin.nav.label')}
      pageLabel={nav.trail}
      menuLabel={t('admin.nav.sheet.label')}
      closeLabel={t('admin.nav.sheet.close')}
    />
  );
}
