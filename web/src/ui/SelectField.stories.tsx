import type { Meta, StoryObj } from '@storybook/react-vite';
import { useState } from 'react';
import { SelectField } from './SelectField';
import { SpatialPage } from './spatial';

const sorts = [
  { value: 'title', label: 'Titel' },
  { value: 'added', label: 'Zuletzt hinzugefügt' },
  { value: 'released', label: 'Erscheinungsjahr' },
  { value: 'vote', label: 'Bewertung' },
];

function Sort() {
  const [value, setValue] = useState('title');
  return (
    <SpatialPage defaultFocusKey="sort">
      <SelectField
        label="Sortieren"
        value={value}
        options={sorts}
        onChange={setValue}
        focusKey="sort"
      />
    </SpatialPage>
  );
}

const meta = {
  title: 'UI/SelectField',
  component: SelectField,
  args: { label: 'Sortieren', value: 'title', options: sorts, onChange: () => undefined },
  render: () => <Sort />,
} satisfies Meta<typeof SelectField>;

export default meta;
type Story = StoryObj<typeof meta>;

export const Default: Story = {};

/** Focused by Tab: the ring sits on the select. */
export const Focused: Story = { tags: ['press:Tab'] };
