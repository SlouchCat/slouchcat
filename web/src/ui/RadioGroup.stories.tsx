import type { Meta, StoryObj } from '@storybook/react-vite';
import { RadioGroup } from './RadioGroup';

const meta = {
  title: 'UI/RadioGroup',
  component: RadioGroup,
  args: {
    label: 'Sprache',
    defaultValue: 'de',
    options: [
      { value: 'de', label: 'Deutsch' },
      { value: 'en', label: 'English' },
    ],
  },
} satisfies Meta<typeof RadioGroup>;

export default meta;
type Story = StoryObj<typeof meta>;

export const Default: Story = {};

/** Arrow keys move focus and selection together (focus duration). */
export const Focused: Story = { tags: ['press:Tab+ArrowRight'] };
