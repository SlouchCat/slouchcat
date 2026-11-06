import type { Meta, StoryObj } from '@storybook/react-vite';
import { useState } from 'react';
import { LockToggle } from './LockToggle';
import { Stack } from './Stack';

const meta = {
  title: 'UI/LockToggle',
  component: LockToggle,
  args: {
    label: 'Sperre Titel',
    isLocked: true,
    lockedText: 'gesperrt',
    freeText: 'frei',
    onChange: () => undefined,
  },
} satisfies Meta<typeof LockToggle>;

export default meta;
type Story = StoryObj<typeof meta>;

/** Locked, free and free without anything to toggle. */
export const States: Story = {
  render: (args) => (
    <Stack gap={4}>
      <LockToggle {...args} />
      <LockToggle {...args} label="Sperre Beschreibung" isLocked={false} />
      <LockToggle {...args} label="Sperre Jahr" isLocked={false} isDisabled />
    </Stack>
  ),
};

function ToggleDemo(args: Story['args']) {
  const [locked, setLocked] = useState(true);
  return (
    <LockToggle
      label="Sperre Titel"
      lockedText="gesperrt"
      freeText="frei"
      {...args}
      isLocked={locked}
      onChange={() => setLocked(!locked)}
    />
  );
}

/** Space or Enter toggles. */
export const Toggling: Story = {
  render: (args) => <ToggleDemo {...args} />,
};
