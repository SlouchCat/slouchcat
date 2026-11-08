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

const ruleOptions = [
  { value: '', label: 'Merkmal wählen …' },
  {
    group: 'Genre',
    options: [
      { value: 'genre:pop', label: 'Genre: Pop' },
      { value: 'genre:oldies', label: 'Genre: Oldies' },
    ],
  },
  {
    group: 'Sendeart',
    options: [
      { value: 'kind:news', label: 'Sendeart: News' },
      { value: 'kind:talk', label: 'Sendeart: Talk' },
    ],
  },
  { value: 'ignore', label: 'Ignorieren (kein Merkmal)' },
];

/**
 * A select in a table row (the worklist of «Einordnung», concept 16 stage 2): the label for
 * screen readers only, the options in groups.
 */
export const GroupedHiddenLabel: Story = {
  render: function Render() {
    const [value, setValue] = useState('');
    return (
      <SelectField
        label="Merkmal für das Schlagwort 80s"
        hideLabel
        value={value}
        options={ruleOptions}
        onChange={setValue}
      />
    );
  },
};
