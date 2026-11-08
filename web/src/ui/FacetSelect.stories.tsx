import type { Meta, StoryObj } from '@storybook/react-vite';
import { useState, type ComponentProps } from 'react';
import { FacetSelect } from './FacetSelect';

const options = [
  { value: 'pop', label: 'Pop', count: 1204 },
  { value: 'rock', label: 'Rock', count: 812 },
  { value: 'jazz', label: 'Jazz', count: 233 },
  { value: 'classical', label: 'Klassik', count: 198 },
  { value: 'schlager', label: 'Schlager', count: 0 },
];

function Demo(props: Partial<ComponentProps<typeof FacetSelect>>) {
  const [values, setValues] = useState<string[]>(['jazz']);
  return (
    <FacetSelect
      label="Genre"
      options={options}
      values={values}
      onChange={setValues}
      emptyLabel="Keine Werte zur Auswahl"
      clearLabel="Auswahl löschen"
      doneLabel="Fertig"
      {...props}
    />
  );
}

const meta = {
  title: 'UI/FacetSelect',
  component: FacetSelect,
  args: {
    label: 'Genre',
    options,
    values: [],
    onChange: () => undefined,
    emptyLabel: 'Keine Werte zur Auswahl',
    clearLabel: 'Auswahl löschen',
    doneLabel: 'Fertig',
  },
  render: () => <Demo />,
} satisfies Meta<typeof FacetSelect>;

export default meta;
type Story = StoryObj<typeof meta>;

/** Closed: the button names the filter and how many values are chosen. */
export const Closed: Story = {};

/** Open under its button: the values with their counts; a value without stations recedes. */
export const Open: Story = { tags: ['press:Tab+Enter'] };

/** On a phone the values open in a sheet from below. */
export const Sheet: Story = {
  render: () => (
    <Demo sheet={{ closeLabel: 'Filter schliessen', showLabel: '233 Sender anzeigen' }} />
  ),
  tags: ['press:Tab+Enter'],
};
