import type { Meta, StoryObj } from '@storybook/react-vite';
import { useState } from 'react';
import { MasterDetail, type MasterDetailItem } from './MasterDetail';
import { SectionHeader } from './SectionHeader';
import { Text } from './Text';

const areas: MasterDetailItem[] = [
  { id: 'languages', label: 'Sprachen', description: 'Deutsch, Englisch' },
  { id: 'images', label: 'Bilder' },
  { id: 'quality', label: 'Güte' },
  { id: 'sources', label: 'Quellen' },
  { id: 'rating', label: 'Altersfreigabe' },
  { id: 'apply', label: 'Anwenden' },
];

const announcements: MasterDetailItem[] = [
  { id: 'a1', label: 'Wartung am Samstag', description: 'Entwurf', section: 'Entwürfe' },
  { id: 'a2', label: 'Neue Filme', description: '3 von 5 Profilen', section: 'Veröffentlicht' },
  { id: 'a3', label: 'Willkommen', description: '5 von 5 Profilen', section: 'Veröffentlicht' },
];

function Areas({ items, initial }: { items: MasterDetailItem[]; initial: string }) {
  const [selected, setSelected] = useState(initial);
  const item = items.find((entry) => entry.id === selected);
  return (
    <MasterDetail label="Bereiche" items={items} selectedId={selected} onSelect={setSelected}>
      <SectionHeader title={item?.label ?? ''} />
      <Text tone="secondary">Die Einstellungen dieses Bereichs.</Text>
    </MasterDetail>
  );
}

const meta = {
  title: 'UI/MasterDetail',
  component: Areas,
  args: { items: areas, initial: 'languages' },
  decorators: [
    (Story) => (
      <div style={{ width: '56rem' }}>
        <Story />
      </div>
    ),
  ],
} satisfies Meta<typeof Areas>;

export default meta;
type Story = StoryObj<typeof meta>;

/** The areas of the metadata page (Met-B): the list left, the selected area right. */
export const Default: Story = {};

/** One Tab stop: Tab into the list, ↓ moves the focus, Enter selects «Bilder». */
export const ArrowKeys: Story = { tags: ['press:Tab+ArrowDown+Enter'] };

/** Sections in the list (announcements, Ank-B). */
export const Sections: Story = { args: { items: announcements, initial: 'a2' } };

export const Compact: Story = { globals: { density: 'compact' } };
