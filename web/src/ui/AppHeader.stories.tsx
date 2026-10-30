import type { Meta, StoryObj } from '@storybook/react-vite';
import type { CSSProperties } from 'react';
import { AppHeader } from './AppHeader';
import { Text } from './Text';

const meta = {
  title: 'UI/AppHeader',
  component: AppHeader,
  parameters: { layout: 'fullscreen' },
  args: {
    appName: 'Slouchcat',
    homeHref: '#',
    search: { href: '#search', label: 'Suche', accessibleLabel: 'Suche öffnen' },
    profileName: 'Marco',
    profileHref: '#profiles',
    profileLabel: 'Profil wechseln, aktuell Marco',
  },
  decorators: [
    (Story) => (
      <div style={{ minHeight: '24rem', paddingTop: 'var(--sc-space-24)' }}>
        <Story />
        <Text tone="secondary">Seiteninhalt unter der Kopfzeile.</Text>
      </div>
    ),
  ],
} satisfies Meta<typeof AppHeader>;

export default meta;
type Story = StoryObj<typeof meta>;

/** From 768 px with the side navigation's column shown: no menu button (concept 08). */
export const Default: Story = {};

/** Second Tab stop: the search link. */
export const SearchFocused: Story = { tags: ['press:Tab+Tab'] };

/** Below 768 px, or with the column hidden: the menu button opens the drawer. */
export const WithMenu: Story = {
  args: {
    menu: { label: 'Menü', expanded: false, controls: 'app-menu', onPress: () => undefined },
  },
};

/** The avatar and the primary fills follow the profile accent (here teal, set by the session). */
export const TealProfile: Story = {
  decorators: [
    (Story) => (
      <div
        style={
          {
            '--sc-accent': 'var(--sc-color-accent-teal-base)',
            '--sc-on-accent': 'var(--sc-color-accent-teal-on)',
          } as CSSProperties
        }
      >
        <Story />
      </div>
    ),
  ],
};

/** With the inbox link and three unread notifications (M7). */
export const WithNotifications: Story = {
  args: {
    notifications: {
      href: '#notifications',
      label: 'Benachrichtigungen',
      accessibleLabel: 'Benachrichtigungen, 3 ungelesen',
      unread: 3,
      overflowLabel: '99+',
    },
  },
};
