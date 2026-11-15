import { paths } from '@/app/paths';
import type { IconName } from '@/ui';

// Every page of the administration (A-2 "an einem Ort"), in the areas of the admin sidebar
// (AdminShellNav, concept 11; redesign, build plan section 1). Notification channels belong to the
// account; the administration shows the same page under /admin/notifications (concept 15, P-4),
// so the entry stays in the admin area, while /settings/notifications stays for every account.
// The profiles of the own account are part of «Konten» (concept 12, KP-1): the own account's row
// folds out «Meine Profile»; `/settings/profiles` stays for every account.

/**
 * The areas of the administration (redesign, build plan section 1): the sidebar has one entry per
 * area besides the overview, and every page of an area shows the area's pages as tabs above it.
 */
export const adminAreas = ['media', 'people', 'home', 'automation', 'system'] as const;
export type AdminArea = (typeof adminAreas)[number];

/** The icon of an area's sidebar entry. */
export const adminAreaIcons = {
  media: 'folder',
  people: 'users',
  home: 'star',
  automation: 'refresh',
  system: 'server',
} as const satisfies Record<AdminArea, IconName>;

export interface AdminSection {
  /** Key of the text `admin.sections.<key>.title`. */
  key:
    | 'libraries'
    | 'metadata'
    | 'maintenance'
    | 'podcasts'
    | 'radio'
    | 'audiobookSources'
    | 'accounts'
    | 'devices'
    | 'streams'
    | 'homeRows'
    | 'spotlight'
    | 'picks'
    | 'collections'
    | 'myMedia'
    | 'announcements'
    | 'jobs'
    | 'integrations'
    | 'notifications'
    | 'stats'
    | 'backup';
  href: string;
  icon: IconName;
  area: AdminArea;
  /** Further path prefixes that belong to the page (a title's admin page to maintenance). */
  also?: readonly string[];
}

/** Every page in the order of its area's tabs; the areas in the order of `adminAreas`. */
export const adminSections: readonly AdminSection[] = [
  { key: 'libraries', href: paths.adminLibraries, icon: 'folder', area: 'media' },
  { key: 'metadata', href: paths.adminMetadata, icon: 'image', area: 'media' },
  {
    key: 'maintenance',
    href: paths.adminMaintenance,
    icon: 'wrench',
    area: 'media',
    also: ['/admin/items'],
  },
  { key: 'podcasts', href: paths.adminPodcasts, icon: 'mic', area: 'media' },
  // Webradio (concept 16, section 11).
  { key: 'radio', href: paths.adminRadio, icon: 'radio', area: 'media' },
  // A page of its own in the area since the redesign; «Metadaten» keeps its link to it.
  { key: 'audiobookSources', href: paths.adminAudiobookSources, icon: 'book', area: 'media' },
  { key: 'accounts', href: paths.adminAccounts, icon: 'users', area: 'people' },
  { key: 'devices', href: paths.adminDevices, icon: 'monitor', area: 'people' },
  { key: 'streams', href: paths.adminStreams, icon: 'play', area: 'people' },
  { key: 'homeRows', href: paths.adminHomeRows, icon: 'list', area: 'home' },
  { key: 'spotlight', href: paths.adminSpotlight, icon: 'spotlight', area: 'home' },
  { key: 'picks', href: paths.adminPicks, icon: 'star', area: 'home' },
  { key: 'collections', href: paths.adminCollections, icon: 'stack', area: 'home' },
  // Order and hiding of «Meine Medien», directly after «Startseite» (concept 21 P-11).
  { key: 'myMedia', href: paths.adminMyMedia, icon: 'grid', area: 'home' },
  { key: 'announcements', href: paths.adminAnnouncements, icon: 'megaphone', area: 'home' },
  { key: 'jobs', href: paths.adminJobs, icon: 'refresh', area: 'automation' },
  { key: 'integrations', href: paths.adminIntegrations, icon: 'plug', area: 'automation' },
  { key: 'notifications', href: paths.adminNotifications, icon: 'bell', area: 'automation' },
  { key: 'stats', href: paths.adminStats, icon: 'chart', area: 'system' },
  { key: 'backup', href: paths.adminBackup, icon: 'archive', area: 'system' },
];

const under = (pathname: string, prefix: string) =>
  pathname === prefix || pathname.startsWith(`${prefix}/`);

/** The length of the longest of the section's paths that `pathname` lies under, or -1. */
function matchLength(section: AdminSection, pathname: string): number {
  return [section.href, ...(section.also ?? [])]
    .filter((prefix) => under(pathname, prefix))
    .reduce((longest, prefix) => Math.max(longest, prefix.length), -1);
}

/**
 * The page lies under `section`'s path (or one of its further prefixes). Section hrefs are bare
 * paths without `?` or `#` (sections.test.ts), so they compare with the pathname as they are.
 * Several sections can match (`/admin/metadata/audiobooks` lies under «Metadaten» too); which one
 * is current tells `currentSection`.
 */
export const isSectionPath = (section: AdminSection, pathname: string) =>
  matchLength(section, pathname) >= 0;

/**
 * The current page for a path: the section with the longest matching path, so
 * `/admin/metadata/audiobooks` is «Hörbuch-Quellen», not «Metadaten». None on the overview and
 * outside /admin.
 */
export function currentSection(pathname: string): AdminSection | undefined {
  let best: AdminSection | undefined;
  let bestLength = -1;
  for (const section of adminSections) {
    const length = matchLength(section, pathname);
    if (length > bestLength) {
      best = section;
      bestLength = length;
    }
  }
  return best;
}

/** The pages of an area, in the order of its tabs. */
export const sectionsOf = (area: AdminArea): AdminSection[] =>
  adminSections.filter((section) => section.area === area);

/** The first page of an area: where its sidebar entry leads. */
export function firstSectionOf(area: AdminArea): AdminSection {
  const first = sectionsOf(area)[0];
  if (!first) throw new Error(`admin area ${area} has no page`);
  return first;
}
