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
    navLabel: 'Hauptnavigation',
    nav: [
      { href: '#', label: 'Start', current: true },
      { href: '#libraries', label: 'Bibliotheken' },
      { href: '#access', label: 'Zugriff' },
      { href: '#profiles', label: 'Profile' },
    ],
    profileName: 'Marco',
    profileHref: '#profiles',
    profileLabel: 'Profil wechseln, aktuell Marco',
    logoutLabel: 'Abmelden',
    onLogout: () => undefined,
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

export const Default: Story = {};

/** Second Tab stop: the first navigation link. */
export const NavFocused: Story = { tags: ['press:Tab+Tab'] };

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
