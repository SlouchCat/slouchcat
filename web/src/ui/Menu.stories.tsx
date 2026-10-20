import type { Meta, StoryObj } from '@storybook/react-vite';
import { useState } from 'react';
import { MenuButton } from './Menu';

const versions = [
  { id: 'mf-1', label: '1080p' },
  { id: 'mf-2', label: '720p' },
];

function Versions() {
  const [selected, setSelected] = useState('mf-1');
  return (
    <div style={{ paddingTop: '8rem' }}>
      <MenuButton
        aria-label="Version"
        buttonLabel="Version wählen"
        label={versions.find((version) => version.id === selected)?.label ?? ''}
        options={versions}
        selectedKey={selected}
        onSelect={setSelected}
      />
    </div>
  );
}

const meta = {
  title: 'UI/MenuButton',
  component: MenuButton,
  args: {
    'aria-label': 'Version',
    buttonLabel: 'Version wählen',
    label: '1080p',
    options: versions,
    selectedKey: 'mf-1',
    onSelect: () => undefined,
  },
  render: () => <Versions />,
} satisfies Meta<typeof MenuButton>;

export default meta;
type Story = StoryObj<typeof meta>;

export const Closed: Story = {};

/** Opened from the keyboard: the current choice has the tick and the focus. */
export const Open: Story = { tags: ['press:Tab+Enter'] };
