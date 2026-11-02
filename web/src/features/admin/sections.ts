import { paths } from '@/app/paths';
import type { IconName } from '@/ui';

// Every page of the administration (A-2 "an einem Ort"), in the groups of the admin sidebar
// (AdminShellNav, concept 11). Notification channels belong to the account and live under
// /settings: that entry leaves the administration, where the app's column stands again. The
// profiles of the own account are part of «Konten» (concept 12, KP-1): their entry opens the own
// account there; `/settings/profiles` stays for every account.

export const adminGroups = ['library', 'accounts', 'curation', 'system'] as const;
export type AdminGroup = (typeof adminGroups)[number];

export interface AdminSection {
  /** Key of the text `admin.sections.<key>.title`. */
  key:
    | 'libraries'
    | 'podcasts'
    | 'maintenance'
    | 'metadata'
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
  /**
   * The entry is current only with this `?item=` on its page («Profile» is `/admin/accounts` with
   * `item=me`); the other entry of that page is current without it.
   */
  item?: string;
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
  { key: 'metadata', href: paths.adminMetadata, icon: 'image', group: 'library' },
  { key: 'accounts', href: paths.adminAccounts, icon: 'users', group: 'accounts' },
  { key: 'profiles', href: paths.adminMyProfiles, icon: 'user', group: 'accounts', item: 'me' },
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

/** The path of an entry's link, without its search and fragment. */
const pathOf = (href: string) => href.split(/[?#]/)[0] ?? href;

const onPage = (section: AdminSection, pathname: string) =>
  under(pathname, pathOf(section.href)) ||
  (section.also ?? []).some((prefix) => under(pathname, prefix));

/**
 * The page is `section`'s (or one of its sub-pages). `item` is the page's `?item=`: an entry with
 * an item of its own is current only with it, and takes the page from the entry without one.
 */
export const isSectionPath = (section: AdminSection, pathname: string, item?: unknown) => {
  if (!onPage(section, pathname)) return false;
  if (section.item !== undefined) return item === section.item;
  return !adminSections.some(
    (other) => other.item !== undefined && other.item === item && onPage(other, pathname),
  );
};
