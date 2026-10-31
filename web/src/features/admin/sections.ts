import { paths } from '@/app/paths';
import type { IconName } from '@/ui';

// Every page of the administration (A-2 "an einem Ort"), in the groups of the admin navigation and
// of the overview's cards. Pages outside /admin (profiles and notification channels belong to the
// account) appear as cards only: the admin navigation lives in the /admin layout.

export const adminGroups = ['library', 'accounts', 'curation', 'system'] as const;
export type AdminGroup = (typeof adminGroups)[number];

export interface AdminSection {
  /** Key of the texts `admin.sections.<key>.title` and `.description`. */
  key:
    | 'libraries'
    | 'podcasts'
    | 'maintenance'
    | 'accounts'
    | 'profiles'
    | 'devices'
    | 'streams'
    | 'spotlight'
    | 'picks'
    | 'collections'
    | 'homeRows'
    | 'announcements'
    | 'jobs'
    | 'integrations'
    | 'notifications'
    | 'stats'
    | 'backup';
  href: string;
  icon: IconName;
  group: AdminGroup;
  /** Further path prefixes that belong to the page (a title's admin page to maintenance). */
  also?: readonly string[];
}

export const adminSections: readonly AdminSection[] = [
  { key: 'libraries', href: paths.adminLibraries, icon: 'folder', group: 'library' },
  { key: 'podcasts', href: paths.adminPodcasts, icon: 'mic', group: 'library' },
  {
    key: 'maintenance',
    href: paths.adminMaintenance,
    icon: 'wrench',
    group: 'library',
    also: ['/admin/items'],
  },
  { key: 'accounts', href: paths.adminAccounts, icon: 'users', group: 'accounts' },
  { key: 'profiles', href: paths.profileSettings, icon: 'user', group: 'accounts' },
  { key: 'devices', href: paths.adminDevices, icon: 'monitor', group: 'accounts' },
  { key: 'streams', href: paths.adminStreams, icon: 'play', group: 'accounts' },
  { key: 'spotlight', href: paths.adminSpotlight, icon: 'spotlight', group: 'curation' },
  { key: 'picks', href: paths.adminPicks, icon: 'star', group: 'curation' },
  { key: 'collections', href: paths.adminCollections, icon: 'stack', group: 'curation' },
  { key: 'homeRows', href: paths.adminHomeRows, icon: 'list', group: 'curation' },
  { key: 'announcements', href: paths.adminAnnouncements, icon: 'megaphone', group: 'curation' },
  { key: 'jobs', href: paths.adminJobs, icon: 'refresh', group: 'system' },
  { key: 'integrations', href: paths.adminIntegrations, icon: 'plug', group: 'system' },
  { key: 'notifications', href: paths.notificationSettings, icon: 'bell', group: 'system' },
  { key: 'stats', href: paths.adminStats, icon: 'chart', group: 'system' },
  { key: 'backup', href: paths.adminBackup, icon: 'archive', group: 'system' },
];

const under = (pathname: string, prefix: string) =>
  pathname === prefix || pathname.startsWith(`${prefix}/`);

/** The page is `section`'s (or one of its sub-pages). */
export const isSectionPath = (section: AdminSection, pathname: string) =>
  under(pathname, section.href) || (section.also ?? []).some((prefix) => under(pathname, prefix));
