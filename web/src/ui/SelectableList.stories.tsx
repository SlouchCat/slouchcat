import type { Meta, StoryObj } from '@storybook/react-vite';
import { useState } from 'react';
import { SelectableList, type SelectableItem } from './SelectableList';

const items: SelectableItem[] = [
  {
    id: 'a',
    title: 'Slouchcat Test Broken',
    href: '#',
    meta: <code>Slouchcat Test Broken (2024)/Slouchcat Test Broken.mkv</code>,
    selectLabel: 'Slouchcat Test Broken auswählen',
  },
  {
    id: 'b',
    title: 'Die Stunde der Dämmerung',
    href: '#',
    meta: 'Film · 2023',
    selectLabel: 'Die Stunde der Dämmerung auswählen',
  },
];

function Selectable() {
  const [value, setValue] = useState<string[]>(['b']);
  return <SelectableList label="Auswahl" items={items} value={value} onChange={setValue} />;
}

const meta = {
  title: 'UI/SelectableList',
  component: SelectableList,
  args: { label: 'Auswahl', items },
  render: () => <Selectable />,
  decorators: [
    (Story) => (
      <div style={{ maxWidth: '40rem' }}>
        <Story />
      </div>
    ),
  ],
} satisfies Meta<typeof SelectableList>;

export default meta;
type Story = StoryObj<typeof meta>;

export const Default: Story = {};

/** Tab to the first checkbox, Space selects it. */
export const Focused: Story = { tags: ['press:Tab+Space'] };

/** Display only (rating mismatch): no checkboxes. */
export const ReadOnly: Story = { render: (args) => <SelectableList {...args} /> };
