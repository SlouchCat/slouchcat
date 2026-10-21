import type { Meta, StoryObj } from '@storybook/react-vite';
import { useState } from 'react';
import { FilterChips } from './FilterChips';
import { SpatialPage } from './spatial';

const types = [
  { value: 'movie', label: 'Filme' },
  { value: 'series', label: 'Serien' },
  { value: 'audiobook', label: 'Hörbücher' },
  { value: 'podcast', label: 'Podcasts' },
];

function Types() {
  const [values, setValues] = useState<string[]>(['movie']);
  return (
    <SpatialPage defaultFocusKey="type-movie">
      <FilterChips
        label="Medientyp"
        options={types}
        values={values}
        onChange={setValues}
        focusKey="type"
      />
    </SpatialPage>
  );
}

const meta = {
  title: 'UI/FilterChips',
  component: FilterChips,
  args: { label: 'Medientyp', options: types, values: ['movie'], onChange: () => undefined },
  render: () => <Types />,
} satisfies Meta<typeof FilterChips>;

export default meta;
type Story = StoryObj<typeof meta>;

export const Default: Story = {};

/** Arrow keys move between the chips, Enter toggles the focused one (here: Serien on). */
export const ArrowAndEnter: Story = { tags: ['press:ArrowDown+ArrowRight+Enter'] };
