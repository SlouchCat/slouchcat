import type { Meta, StoryObj } from '@storybook/react-vite';
import { CommandPalette, type CommandPaletteItem } from './CommandPalette';

// The search palette of the administration (build plan 2.4): the field with the focus, the
// destinations below it, the first one chosen.

const items: CommandPaletteItem[] = [
  { id: 'overview', title: 'Übersicht', icon: 'grid', href: '#admin' },
  { id: 'media', title: 'Medien', detail: 'Bereich', icon: 'folder', href: '#libraries' },
  { id: 'libraries', title: 'Bibliotheken', detail: 'Medien', icon: 'folder', href: '#libraries' },
  {
    id: 'jobs',
    title: 'Hintergrundjobs',
    detail: 'Automatisierung',
    icon: 'refresh',
    href: '#jobs',
  },
  { id: 'backup', title: 'Sicherung', detail: 'System', icon: 'archive', href: '#backup' },
  { id: 'back', title: 'Zurück zur App', icon: 'back', href: '#home' },
  { id: 'search', title: 'Mediensuche', icon: 'search', href: '#search' },
];

const meta = {
  title: 'UI/CommandPalette',
  component: CommandPalette,
  parameters: { layout: 'fullscreen' },
  args: {
    isOpen: true,
    onOpenChange: () => undefined,
    label: 'Bereich der Verwaltung suchen',
    placeholder: 'Bibliotheken, Jobs, Sicherung …',
    resultsLabel: 'Treffer',
    emptyLabel: 'Nichts gefunden',
    hint: 'Pfeiltasten wählen, Enter öffnet, Esc schliesst',
    items,
  },
  decorators: [
    (Story) => (
      <div style={{ minHeight: '40rem' }}>
        <Story />
      </div>
    ),
  ],
} satisfies Meta<typeof CommandPalette>;

export default meta;
type Story = StoryObj<typeof meta>;

/** Open: every destination, the first one chosen. */
export const Open: Story = {};

/** ArrowDown twice: the third destination is chosen. */
export const Chosen: Story = { tags: ['press:ArrowDown+ArrowDown'] };

/** Nothing matches what was typed. */
export const Empty: Story = { args: { items: [] } };
