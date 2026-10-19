import type { Meta, StoryObj } from '@storybook/react-vite';
import { useState } from 'react';
import { ChoiceChips } from './ChoiceChips';
import { SpatialPage } from './spatial';

const seasons = [
  { value: '1', label: 'Staffel 1' },
  { value: '2', label: 'Staffel 2' },
  { value: '0', label: 'Specials' },
];

function Seasons() {
  const [value, setValue] = useState('1');
  return (
    <SpatialPage defaultFocusKey="season-1">
      <ChoiceChips
        label="Staffeln"
        options={seasons}
        value={value}
        onChange={setValue}
        focusKey="season"
      />
    </SpatialPage>
  );
}

const meta = {
  title: 'UI/ChoiceChips',
  component: ChoiceChips,
  args: { label: 'Staffeln', options: seasons, value: '1', onChange: () => undefined },
  render: () => <Seasons />,
} satisfies Meta<typeof ChoiceChips>;

export default meta;
type Story = StoryObj<typeof meta>;

export const Default: Story = {};

/** Arrow keys move between the chips, Enter picks the focused one. */
export const ArrowAndEnter: Story = { tags: ['press:ArrowDown+ArrowRight+Enter'] };
