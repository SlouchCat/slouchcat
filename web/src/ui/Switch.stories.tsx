import type { Meta, StoryObj } from '@storybook/react-vite';
import { Switch } from './Switch';

const meta = {
  title: 'UI/Switch',
  component: Switch,
  args: { children: 'Aktiv' },
} satisfies Meta<typeof Switch>;

export default meta;
type Story = StoryObj<typeof meta>;

export const Off: Story = {};

export const On: Story = { args: { defaultSelected: true } };

export const Disabled: Story = { args: { isDisabled: true } };

/** Tab, then Space switches it on. */
export const Focused: Story = { tags: ['press:Tab+Space'] };
