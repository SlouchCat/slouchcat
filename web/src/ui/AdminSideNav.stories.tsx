import type { Meta, StoryObj } from '@storybook/react-vite';
import type { CSSProperties } from 'react';
import {
  AdminSideNav,
  type AdminSideNavEntry,
  type AdminSideNavGroup,
  type AdminSideNavProps,
} from './AdminSideNav';

// The administration's sidebar (concept 11, 11.1) at 1280 px: back to the app, the overview and
// the 18 pages in four groups, each group under its band. The sheet on a phone has stories of its
// own (AdminSideNavMenu.stories.tsx).

const groups: AdminSideNavGroup[] = [
  {
    label: 'Bibliothek',
    icon: 'film',
    entries: [
      { href: '#libraries', label: 'Bibliotheken', icon: 'folder' },
      { href: '#podcasts', label: 'Podcasts', icon: 'mic' },
      { href: '#maintenance', label: 'Pflege', icon: 'wrench' },
      { href: '#metadata', label: 'Metadaten', icon: 'image' },
    ],
  },
  {
    label: 'Konten',
    icon: 'users',
    entries: [
      { href: '#accounts', label: 'Konten', icon: 'users' },
      { href: '#profiles', label: 'Profile', icon: 'user' },
      { href: '#devices', label: 'Geräte', icon: 'monitor' },
      { href: '#streams', label: 'Streams', icon: 'play' },
    ],
  },
  {
    label: 'Kuratierung',
    icon: 'star',
    entries: [
      { href: '#spotlight', label: 'Spotlight', icon: 'spotlight' },
      { href: '#picks', label: "Cat's Empfehlungen", icon: 'star' },
      { href: '#collections', label: 'Sammlungen', icon: 'stack' },
      { href: '#home-rows', label: 'Startseite', icon: 'list' },
      { href: '#announcements', label: 'Ankündigungen', icon: 'megaphone' },
    ],
  },
  {
    label: 'System',
    icon: 'server',
    entries: [
      { href: '#jobs', label: 'Hintergrundjobs', icon: 'refresh' },
      { href: '#integrations', label: 'Integrationen', icon: 'plug' },
      { href: '#notifications', label: 'Benachrichtigungen', icon: 'bell' },
      { href: '#stats', label: 'Statistiken', icon: 'chart' },
      { href: '#backup', label: 'Sicherung', icon: 'archive' },
    ],
  },
];

export const adminSideNavArgs: AdminSideNavProps = {
  label: 'Verwaltung',
  top: [
    { href: '#home', label: 'Zurück zur App', icon: 'back' },
    { href: '#admin', label: 'Übersicht', icon: 'grid', current: true },
  ],
  groups,
};

/** Numbers after F-7: few muted counts, coloured pills for what needs a look. */
type Numbers = Pick<AdminSideNavEntry, 'count' | 'alarm' | 'badge'> & { accessibleLabel: string };

const numbers: Record<string, Numbers> = {
  '#admin': { badge: 'Update', accessibleLabel: 'Übersicht, Update auf Version 1.7.0 verfügbar' },
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

const withNumbers = (entry: AdminSideNavEntry): AdminSideNavEntry => {
  const current = entry.href === '#libraries';
  const extra = numbers[entry.href];
  return extra ? { ...entry, ...extra, current } : { ...entry, current };
};

const meta = {
  title: 'UI/AdminSideNav',
  excludeStories: ['adminSideNavArgs'],
  component: AdminSideNav,
  parameters: { layout: 'fullscreen' },
  args: adminSideNavArgs,
} satisfies Meta<typeof AdminSideNav>;

export default meta;
type Story = StoryObj<typeof meta>;

/** The overview is current; no numbers (loading, or the dashboard could not be read). */
export const Default: Story = {};

/** A page is current; counts, alarm pills and the update badge on the overview. */
export const Counts: Story = {
  args: {
    top: adminSideNavArgs.top.map(withNumbers),
    groups: groups.map((group) => ({ ...group, entries: group.entries.map(withNumbers) })),
  },
};

/** Tab lands on the current entry, ArrowDown moves to the next one (across the band). */
export const ArrowKeys: Story = { tags: ['press:Tab+ArrowDown'] };

/** Two profile colours: the band and the current entry's icon follow the accent. */
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
