import type { Meta, StoryObj } from '@storybook/react-vite';
import { AdminNav } from './AdminNav';

const meta = {
  title: 'UI/AdminNav',
  component: AdminNav,
  args: {
    label: 'Verwaltung',
    groups: [
      { links: [{ href: '#admin', label: 'Übersicht' }] },
      {
        label: 'Bibliothek',
        links: [
          { href: '#libraries', label: 'Bibliotheken', current: true },
          { href: '#podcasts', label: 'Podcasts' },
          { href: '#maintenance', label: 'Pflege' },
        ],
      },
      {
        label: 'Konten',
        links: [
          { href: '#accounts', label: 'Konten' },
          { href: '#access', label: 'Zugriff' },
          { href: '#devices', label: 'Geräte' },
          { href: '#streams', label: 'Streams' },
        ],
      },
      {
        label: 'Kuratierung',
        links: [
          { href: '#picks', label: "Cat's Empfehlungen" },
          { href: '#collections', label: 'Sammlungen' },
          { href: '#home-rows', label: 'Startseite' },
          { href: '#announcements', label: 'Ankündigungen' },
        ],
      },
      {
        label: 'System',
        links: [
          { href: '#integrations', label: 'Integrationen' },
          { href: '#stats', label: 'Statistiken' },
          { href: '#backup', label: 'Sicherung' },
        ],
      },
    ],
  },
  decorators: [
    (Story) => (
      <div style={{ padding: 'var(--sc-space-6)' }}>
        <Story />
      </div>
    ),
  ],
} satisfies Meta<typeof AdminNav>;

export default meta;
type Story = StoryObj<typeof meta>;

export const Default: Story = {};

/** Tab lands on the current page's link, ArrowRight moves to the next one. */
export const ArrowKeys: Story = { tags: ['press:Tab+ArrowRight'] };

/** On a phone the groups wrap onto several lines. */
export const Narrow: Story = {
  decorators: [
    (Story) => (
      <div style={{ width: '22rem' }}>
        <Story />
      </div>
    ),
  ],
};
