import type { Meta, StoryObj } from '@storybook/react-vite';
import { AreaTabs } from './AreaTabs';

// The pages of an administration area as tabs above the page (build plan 2.5): the current page
// raised, the numbers and pills the sidebar's entries used to carry.

const meta = {
  title: 'UI/AreaTabs',
  component: AreaTabs,
  args: {
    label: 'Seiten im Bereich Medien',
    tabs: [
      {
        href: '#libraries',
        label: 'Bibliotheken',
        current: true,
        count: '2.318',
        accessibleLabel: 'Bibliotheken, 2.318 Titel',
      },
      { href: '#metadata', label: 'Metadaten' },
      {
        href: '#maintenance',
        label: 'Pflege',
        alarm: { value: '12', tone: 'warning' },
        accessibleLabel: 'Pflege, 12 Titel brauchen Pflege',
      },
      { href: '#podcasts', label: 'Podcasts' },
      { href: '#radio', label: 'Radio' },
      { href: '#audiobooks', label: 'Hörbuch-Quellen' },
    ],
  },
} satisfies Meta<typeof AreaTabs>;

export default meta;
type Story = StoryObj<typeof meta>;

export const Default: Story = {};

/** Tab lands on the current page, ArrowRight moves to the next one. */
export const ArrowKeys: Story = { tags: ['press:Tab+ArrowRight'] };

/** A phone: the tabs wrap onto a second line. */
export const Narrow: Story = {
  decorators: [
    (Story) => (
      <div style={{ width: '22rem' }}>
        <Story />
      </div>
    ),
  ],
};

/** The administration's compact density. */
export const Compact: Story = { globals: { density: 'compact' } };
