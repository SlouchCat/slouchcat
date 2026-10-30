import type { Meta, StoryObj } from '@storybook/react-vite';
import type { ReactNode } from 'react';
import { SideNav, type SideNavEntry, type SideNavProps } from './SideNav';

// The column of the side navigation (concept 08) at 1280 px, with Marco's libraries. The drawer,
// its other rendering, has stories of its own (SideNavDrawer.stories.tsx).

/** The libraries group's state on this device, set before the story mounts. */
function GroupState({ open, children }: { open: boolean; children: ReactNode }) {
  try {
    if (open) localStorage.removeItem('slouchcat.sidenav.libraries');
    else localStorage.setItem('slouchcat.sidenav.libraries', 'closed');
  } catch {
    // The story then shows the group open.
  }
  return <>{children}</>;
}

export const sideNavArgs: SideNavProps = {
  label: 'Hauptnavigation',
  primary: [
    { href: '#home', label: 'Start', current: true },
    { href: '#search', label: 'Suche' },
    { href: '#watchlist', label: 'Merkliste' },
    { href: '#downloads', label: 'Downloads' },
  ],
  librariesLabel: 'Meine Medien',
  toggleLibrariesLabel: 'Meine Medien ein- oder ausklappen',
  libraries: [
    { href: '#movies', label: 'Filme', count: 2514, accessibleLabel: 'Filme, 2514 Titel' },
    { href: '#series', label: 'Serien', count: 412, accessibleLabel: 'Serien, 412 Titel' },
    { href: '#docs', label: 'Dokus', count: 87, accessibleLabel: 'Dokus, 87 Titel' },
    { href: '#kids', label: 'Kids', count: 143, accessibleLabel: 'Kids, 143 Titel' },
    { href: '#books', label: 'Hörbücher', count: 56, accessibleLabel: 'Hörbücher, 56 Titel' },
    { href: '#podcasts', label: 'Podcasts', count: 12, accessibleLabel: 'Podcasts, 12 Titel' },
  ],
  picks: {
    href: '#picks',
    label: "Cat's Empfehlungen",
    count: 24,
    accessibleLabel: "Cat's Empfehlungen, 24 Titel",
  },
  librariesOffline: 'Offline: Bibliotheken nicht verfügbar',
  offlineLink: { href: '#downloads', label: 'Zu den Downloads' },
  footer: [
    {
      href: '#admin',
      label: 'Verwaltung',
      badge: 'Update',
      accessibleLabel: 'Verwaltung, Update auf Version 1.4.0 verfügbar',
    },
    { href: '#profiles', label: 'Profile' },
  ],
  logoutLabel: 'Abmelden',
  onLogout: () => undefined,
  hideLabel: 'Seitenleiste ausblenden',
  onHide: () => undefined,
};

const meta = {
  title: 'UI/SideNav',
  component: SideNav,
  parameters: { layout: 'fullscreen' },
  args: sideNavArgs,
  decorators: [
    (Story) => (
      <GroupState open>
        <Story />
      </GroupState>
    ),
  ],
} satisfies Meta<typeof SideNav>;

export default meta;
type Story = StoryObj<typeof meta>;

/** The column with every entry; the start page is current, the main account sees an update. */
export const Bar: Story = {};

/** A library page is current. */
export const LibraryCurrent: Story = {
  args: {
    primary: sideNavArgs.primary.map((entry) => ({ ...entry, current: false })),
    libraries: (sideNavArgs.libraries as SideNavEntry[]).map((entry) => ({
      ...entry,
      current: entry.label === 'Serien',
    })),
  },
};

/** "Meine Medien" collapsed on this device. */
export const CollapsedGroup: Story = {
  decorators: [
    (Story) => (
      <GroupState open={false}>
        <Story />
      </GroupState>
    ),
  ],
};

/** The libraries could not be read: the group says so and points to the downloads. */
export const Offline: Story = { args: { libraries: 'offline', picks: undefined } };

/** The first entry focused by Tab, with its ring. */
export const Focused: Story = { tags: ['press:Tab'] };
