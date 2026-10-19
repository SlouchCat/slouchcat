import type { Meta, StoryObj } from '@storybook/react-vite';
import { useState } from 'react';
import { PinField } from './PinField';

function Controlled({ initial = '' }: { initial?: string }) {
  const [value, setValue] = useState(initial);
  return (
    <PinField
      name="pin"
      label="PIN"
      description="4 bis 8 Ziffern."
      invalidMessage="Die PIN besteht aus 4 bis 8 Ziffern."
      value={value}
      onChange={setValue}
    />
  );
}

const meta = {
  title: 'UI/PinField',
  component: PinField,
  args: {
    name: 'pin',
    label: 'PIN',
    value: '',
    onChange: () => undefined,
    invalidMessage: 'Die PIN besteht aus 4 bis 8 Ziffern.',
  },
  decorators: [
    (Story) => (
      <div style={{ maxWidth: 'var(--sc-size-form-max)' }}>
        <Story />
      </div>
    ),
  ],
} satisfies Meta<typeof PinField>;

export default meta;
type Story = StoryObj<typeof meta>;

export const Empty: Story = { render: () => <Controlled /> };

/** The digits are masked. */
export const Filled: Story = { render: () => <Controlled initial="4821" /> };

export const Focused: Story = { tags: ['press:Tab'], render: () => <Controlled initial="48" /> };
