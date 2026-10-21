import type { Meta, StoryObj } from '@storybook/react-vite';
import { useState } from 'react';
import { SearchField } from './SearchField';
import { SpatialPage } from './spatial';

function Field({ initial = '' }: { initial?: string }) {
  const [value, setValue] = useState(initial);
  return (
    <SpatialPage defaultFocusKey="search">
      <div style={{ maxWidth: '40rem', padding: 'var(--sc-space-6)' }}>
        <SearchField
          label="Suche"
          placeholder="Titel, Personen, Sammlungen"
          value={value}
          onChange={setValue}
          focusKey="search"
        />
      </div>
    </SpatialPage>
  );
}

const meta = {
  title: 'UI/SearchField',
  component: SearchField,
  args: { label: 'Suche', value: '', onChange: () => undefined },
  render: () => <Field />,
} satisfies Meta<typeof SearchField>;

export default meta;
type Story = StoryObj<typeof meta>;

export const Empty: Story = {};

/** Typed text with the focus ring (Tab into the field). */
export const Focused: Story = { tags: ['press:Tab'], render: () => <Field initial="Dämm" /> };

/** The short year field of the search filters, with its label. */
export const Year: Story = {
  render: () => (
    <div style={{ padding: 'var(--sc-space-6)' }}>
      <SearchField label="Jahr von" variant="year" value="1990" onChange={() => undefined} />
    </div>
  ),
};
