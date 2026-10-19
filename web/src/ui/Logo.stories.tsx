import type { Meta, StoryObj } from '@storybook/react-vite';
import { Logo } from './Logo';

const meta = {
  title: 'UI/Logo',
  component: Logo,
  args: { name: 'Slouchcat' },
} satisfies Meta<typeof Logo>;

export default meta;
type Story = StoryObj<typeof meta>;

export const Default: Story = {};
