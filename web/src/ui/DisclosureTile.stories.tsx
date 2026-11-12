import type { Meta, StoryObj } from '@storybook/react-vite';
import { useState } from 'react';
import { DisclosureTile } from './DisclosureTile';

const meta = {
  title: 'UI/DisclosureTile',
  component: DisclosureTile,
  args: {
    title: 'Europa',
    subtitle: '51 Länder · 26.395 Sender',
    isExpanded: false,
    onToggle: () => undefined,
  },
  decorators: [
    (Story) => (
      <div style={{ width: '18rem', padding: 'var(--sc-space-6)' }}>
        <Story />
      </div>
    ),
  ],
} satisfies Meta<typeof DisclosureTile>;

export default meta;
type Story = StoryObj<typeof meta>;

export const Default: Story = {};

/** Chosen countries: the badge at the end, before the chevron. */
export const WithBadge: Story = { args: { badge: '3 gewählt' } };

/** Open: accent border on a tinted ground, the chevron turned. */
export const Expanded: Story = { args: { isExpanded: true, badge: '1 gewählt', controls: 'x' } };

/** Focus: the ring of every control. */
export const Focused: Story = { tags: ['press:Tab'] };

const CONTINENTS = [
  ['Afrika', '58 Länder · 1.322 Sender', undefined],
  ['Asien', '50 Länder · 4.981 Sender', undefined],
  ['Europa', '51 Länder · 26.395 Sender', '1 gewählt'],
  ['Nordamerika', '41 Länder · 9.874 Sender', undefined],
  ['Ozeanien', '25 Länder · 1.203 Sender', '2 gewählt'],
  ['Südamerika', '14 Länder · 3.311 Sender', undefined],
] as const;

/**
 * Six tiles in the grid of the radio offer (auto-fill, at least 250 px): every tile of a row as
 * high as the highest, one open.
 */
export const Grid: Story = {
  decorators: [
    (Story) => (
      <div style={{ width: '52rem', padding: 'var(--sc-space-6)' }}>
        <Story />
      </div>
    ),
  ],
  render: () => <TileGrid />,
};

function TileGrid() {
  const [open, setOpen] = useState<string | null>('Europa');
  return (
    <div
      style={{
        display: 'grid',
        gridTemplateColumns: 'repeat(auto-fill, minmax(250px, 1fr))',
        gap: 'var(--sc-space-2)',
      }}
    >
      {CONTINENTS.map(([name, sub, badge]) => (
        <DisclosureTile
          key={name}
          title={name}
          subtitle={sub}
          badge={badge}
          isExpanded={open === name}
          onToggle={() => setOpen(open === name ? null : name)}
        />
      ))}
    </div>
  );
}
