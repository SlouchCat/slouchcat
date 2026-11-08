import type { Meta, StoryObj } from '@storybook/react-vite';
import { SideNav, type SideNavEntry, type SideNavProps } from './SideNav';

// The column of the side navigation (concepts 08 and 17) at 1280 px, with Marco's libraries. The
// drawer, its other rendering, has stories of its own (SideNavDrawer.stories.tsx). What a profile
// sees depends on its role (concept 17, 9.2): every profile media and playback, the primary
// profile of an account also the profiles, the server admin also the administration.

const playback: SideNavEntry = { href: '#playback', label: 'Meine Wiedergabe', icon: 'sliders' };
const profiles: SideNavEntry = { href: '#profiles', label: 'Profile', icon: 'users' };
const admin: SideNavEntry = {
  href: '#admin',
  label: 'Verwaltung',
  icon: 'wrench',
  badge: 'Update',
  accessibleLabel: 'Verwaltung, Update auf Version 1.4.0 verfügbar',
};

const profile = (name: string, role: string): SideNavProps['profile'] => ({
  name,
  role,
  href: '#profiles',
  switchLabel: 'Wechseln',
  switchAccessibleLabel: `Profil wechseln, jetzt ${name}`,
});

export const sideNavArgs: SideNavProps = {
  label: 'Hauptnavigation',
  profile: profile('Marco', 'Server-Admin'),
  primary: [
    { href: '#home', label: 'Start', icon: 'home', current: true },
    { href: '#search', label: 'Suche', icon: 'search' },
    { href: '#watchlist', label: 'Merkliste', icon: 'bookmark' },
    { href: '#radio', label: 'Radio', icon: 'radio' },
    { href: '#downloads', label: 'Downloads', icon: 'download' },
  ],
  librariesLabel: 'Meine Medien',
  libraries: [
    {
      href: '#movies',
      label: 'Filme',
      icon: 'film',
      count: 2514,
      accessibleLabel: 'Filme, 2514 Titel',
    },
    {
      href: '#series',
      label: 'Serien',
      icon: 'tv',
      count: 412,
      accessibleLabel: 'Serien, 412 Titel',
    },
    { href: '#docs', label: 'Dokus', icon: 'film', count: 87, accessibleLabel: 'Dokus, 87 Titel' },
    { href: '#kids', label: 'Kids', icon: 'film', count: 143, accessibleLabel: 'Kids, 143 Titel' },
    {
      href: '#books',
      label: 'Hörbücher',
      icon: 'book',
      count: 56,
      accessibleLabel: 'Hörbücher, 56 Titel',
    },
    {
      href: '#podcasts',
      label: 'Podcasts',
      icon: 'mic',
      count: 12,
      accessibleLabel: 'Podcasts, 12 Titel',
    },
  ],
  picks: {
    href: '#picks',
    label: 'User-Empfehlungen',
    icon: 'star',
    count: 24,
    accessibleLabel: 'User-Empfehlungen, 24 Titel',
  },
  librariesOffline: 'Offline: Bibliotheken nicht verfügbar',
  offlineLink: { href: '#downloads', label: 'Zu den Downloads' },
  accountLabel: 'Konto',
  account: [playback, profiles, admin],
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
} satisfies Meta<typeof SideNav>;

export default meta;
type Story = StoryObj<typeof meta>;

/** The server admin: every entry; the start page is current, an update is available. */
export const Bar: Story = {};

/** The primary profile of another account: profiles, no administration. */
export const PrimaryProfile: Story = {
  args: { profile: profile('Lea', 'Hauptprofil'), account: [playback, profiles] },
};

/** Any further profile: media and playback only. */
export const OtherProfile: Story = {
  args: { profile: profile('Lina', 'Weiteres Profil'), account: [playback] },
};

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

/** The libraries could not be read: the band says so and points to the downloads. */
export const Offline: Story = { args: { libraries: 'offline', picks: undefined } };

/** The first stop focused by Tab, the profile card's «Wechseln», with its ring. */
export const Focused: Story = { tags: ['press:Tab'] };
