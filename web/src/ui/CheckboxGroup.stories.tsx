import type { Meta, StoryObj } from '@storybook/react-vite';
import { CheckboxGroup } from './CheckboxGroup';

const meta = {
  title: 'UI/CheckboxGroup',
  component: CheckboxGroup,
  args: {
    label: 'Bibliotheken wählen',
    defaultValue: ['1'],
    options: [
      { value: '1', label: 'Filme' },
      { value: '2', label: 'Serien' },
      { value: '3', label: 'Kinder' },
    ],
  },
  decorators: [
    (Story) => (
      <div style={{ maxWidth: 'var(--sc-size-form-max)' }}>
        <Story />
      </div>
    ),
  ],
} satisfies Meta<typeof CheckboxGroup>;

export default meta;
type Story = StoryObj<typeof meta>;

export const Default: Story = {};

/** Tab, then Space toggles the focused box. */
export const Focused: Story = { tags: ['press:Tab+Tab+Space'] };
