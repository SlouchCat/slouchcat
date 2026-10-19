import type { Meta, StoryObj } from '@storybook/react-vite';
import { Chip } from './Chip';
import { Stack } from './Stack';

const meta = {
  title: 'UI/Stack',
  component: Stack,
  args: { children: null },
} satisfies Meta<typeof Stack>;

export default meta;
type Story = StoryObj<typeof meta>;

const chips = (
  <>
    <Chip>Eins</Chip>
    <Chip>Zwei</Chip>
    <Chip>Drei</Chip>
  </>
);

export const Column: Story = { args: { gap: 2, children: chips } };

export const Row: Story = { args: { direction: 'row', gap: 4, children: chips } };
