import type { Meta, StoryObj } from '@storybook/react-vite';
import type { CSSProperties } from 'react';
import {
  AdminSideNav,
  type AdminSideNavBranch,
  type AdminSideNavEntry,
  type AdminSideNavGroup,
  type AdminSideNavProps,
} from './AdminSideNav';

// The administration's sidebar (concept 11, 11.1; redesign, build plan 2.2) at 1280 px: back to
// the app, the heading «Verwaltung» with the overview and the five areas as a tree (Explorer), each
// folding its pages in and out, the active profile at the foot. The sheet on a phone, with every
// page of every area, has stories of its own (AdminSideNavMenu.stories.tsx).

/** The sheet's entries: back, the overview, the five areas with all their pages. */
const groups: AdminSideNavGroup[] = [
  {
    label: 'Medien',
    icon: 'folder',
    entries: [
      { href: '#libraries', label: 'Bibliotheken', icon: 'folder' },
      { href: '#metadata', label: 'Metadaten', icon: 'image' },
      { href: '#maintenance', label: 'Pflege', icon: 'wrench' },
      { href: '#podcasts', label: 'Podcasts', icon: 'mic' },
      { href: '#radio', label: 'Radio', icon: 'radio' },
      { href: '#audiobooks', label: 'Hörbuch-Quellen', icon: 'book' },
    ],
  },
  {
    label: 'Nutzer & Geräte',
    icon: 'users',
    entries: [
      { href: '#accounts', label: 'Konten', icon: 'users' },
      { href: '#devices', label: 'Geräte', icon: 'monitor' },
      { href: '#streams', label: 'Streams', icon: 'play' },
    ],
  },
  {
    label: 'Startseite',
    icon: 'star',
    entries: [
      { href: '#home-rows', label: 'Startseite', icon: 'list' },
      { href: '#spotlight', label: 'Spotlight', icon: 'spotlight' },
      { href: '#picks', label: 'User-Empfehlungen', icon: 'star' },
      { href: '#collections', label: 'Sammlungen', icon: 'stack' },
      { href: '#my-media', label: 'Meine Medien', icon: 'grid' },
      { href: '#announcements', label: 'Ankündigungen', icon: 'megaphone' },
    ],
  },
  {
    label: 'Automatisierung',
    icon: 'refresh',
    entries: [
      { href: '#jobs', label: 'Hintergrundjobs', icon: 'refresh' },
      { href: '#integrations', label: 'Integrationen', icon: 'plug' },
      { href: '#notifications', label: 'Benachrichtigungen', icon: 'bell' },
    ],
  },
  {
    label: 'System',
    icon: 'server',
    entries: [
      { href: '#stats', label: 'Statistiken', icon: 'chart' },
      { href: '#backup', label: 'Sicherung', icon: 'archive' },
    ],
  },
];

/** Numbers after F-7: few muted counts, coloured pills for what needs a look. */
type Numbers = Pick<AdminSideNavEntry, 'count' | 'alarm' | 'badge'> & { accessibleLabel: string };

const numbers: Record<string, Numbers> = {
  '#admin': { badge: 'Update', accessibleLabel: 'Übersicht, Update auf Version 1.7.0 verfügbar' },
  '#media': {
    alarm: { value: '12', tone: 'warning' },
    accessibleLabel: 'Medien, 12 Titel brauchen Pflege',
  },
  '#people': {
    alarm: { value: '2', tone: 'accent' },
    accessibleLabel: 'Nutzer & Geräte, 2 laufen',
  },
  '#automation': {
    alarm: { value: '265', tone: 'danger' },
    accessibleLabel: 'Automatisierung, 265 fehlgeschlagen',
  },
  '#libraries': { count: '2.318', accessibleLabel: 'Bibliotheken, 2.318 Titel' },
  '#maintenance': {
    alarm: { value: '12', tone: 'warning' },
    accessibleLabel: 'Pflege, 12 Titel brauchen Pflege',
  },
  '#accounts': { count: '3', accessibleLabel: 'Konten, 3 Konten' },
  '#streams': { alarm: { value: '2', tone: 'accent' }, accessibleLabel: 'Streams, 2 laufen' },
  '#jobs': {
    alarm: { value: '265', tone: 'danger' },
    accessibleLabel: 'Hintergrundjobs, 265 fehlgeschlagen',
  },
  '#integrations': {
    alarm: { value: '1', tone: 'warning' },
    accessibleLabel: 'Integrationen, 1 Webhook still',
  },
};

/** The entry with its numbers; `current` marks the open page (or area). */
export const withNumbers =
  (current: string) =>
  (entry: AdminSideNavEntry): AdminSideNavEntry => {
    const extra = numbers[entry.href];
    const link = { ...entry, current: entry.href === current };
    return extra ? { ...link, ...extra } : link;
  };

/** The sheet's props: back and the overview on top, the areas as bands. */
export const adminSheetArgs: AdminSideNavProps = {
  label: 'Verwaltung',
  top: [
    { href: '#home', label: 'Zurück zur App', icon: 'back' },
    { href: '#admin', label: 'Übersicht', icon: 'grid' },
  ],
  groups,
};

export { groups as adminSheetGroups };

/** The ids of the areas, also the keys of their numbers. */
const areaIds = ['#media', '#people', '#home-page', '#automation', '#system'];

/**
 * The column's tree: the sheet's areas as branches, those in `open` expanded; `current` is the
 * open page, `numbered` adds the numbers.
 */
const tree = (current: string, open: readonly string[], numbered: boolean): AdminSideNavBranch[] =>
  groups.map((group, index) => {
    const id = areaIds[index] ?? group.label;
    const entries = group.entries.map((entry) =>
      numbered ? withNumbers(current)(entry) : { ...entry, current: entry.href === current },
    );
    const branch = {
      ...group,
      id,
      entries,
      expanded: open.includes(id),
      current: entries.some((entry) => entry.current),
    };
    const extra = numbered ? numbers[id] : undefined;
    return extra ? { ...branch, ...extra } : branch;
  });

export const adminSideNavArgs: AdminSideNavProps = {
  label: 'Verwaltung',
  top: [{ href: '#home', label: 'Zurück zur App', icon: 'back' }],
  heading: 'Verwaltung',
  entries: [{ href: '#admin', label: 'Übersicht', icon: 'grid', current: true }],
  branches: tree('#admin', [], false),
  profile: { name: 'Marco', role: 'Server-Admin' },
};

const meta = {
  title: 'UI/AdminSideNav',
  excludeStories: ['adminSideNavArgs', 'adminSheetArgs', 'adminSheetGroups', 'withNumbers'],
  component: AdminSideNav,
  parameters: { layout: 'fullscreen' },
  args: adminSideNavArgs,
} satisfies Meta<typeof AdminSideNav>;

export default meta;
type Story = StoryObj<typeof meta>;

/**
 * The overview is current, every area closed; no numbers (loading, or the dashboard could not be
 * read).
 */
export const Default: Story = {};

/**
 * «Hintergrundjobs» is current in the open «Automatisierung», «Medien» is open too: the areas'
 * pills, the pages' numbers and the update badge on the overview.
 */
export const Counts: Story = {
  args: {
    entries: adminSideNavArgs.entries?.map(withNumbers('#jobs')),
    branches: tree('#jobs', ['#media', '#automation'], true),
  },
};

/** Tab lands on the current entry, ArrowDown moves to the next one (the first area). */
export const ArrowKeys: Story = { tags: ['press:Tab+ArrowDown'] };

/**
 * Two profile colours: the current entry's tint, stroke and icon and the current area's name
 * follow the accent.
 */
export const Accents: Story = {
  args: { ...Counts.args },
  render: (args) => (
    <div style={{ display: 'flex', height: '100vh' }}>
      {(['teal', 'violet'] as const).map((accent) => (
        // A transformed frame holds the fixed column, so the two stand side by side.
        <div
          key={accent}
          style={
            {
              width: 'var(--sc-size-sidenav-width)',
              transform: 'translateX(0)',
              '--sc-accent': `var(--sc-color-accent-${accent}-base)`,
              '--sc-on-accent': `var(--sc-color-accent-${accent}-on)`,
            } as CSSProperties
          }
        >
          <AdminSideNav {...args} />
        </div>
      ))}
    </div>
  ),
};
