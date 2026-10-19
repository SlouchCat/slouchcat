import type { Meta, StoryObj } from '@storybook/react-vite';
import { Text } from './Text';
import { VisuallyHidden } from './VisuallyHidden';

// Screen readers hear the hidden part; the screenshot shows only the visible sentence.
const meta = {
  title: 'UI/VisuallyHidden',
  component: VisuallyHidden,
  args: { children: 'Startseite' },
} satisfies Meta<typeof VisuallyHidden>;

export default meta;
type Story = StoryObj<typeof meta>;

export const NextToVisibleText: Story = {
  render: (args) => (
    <Text>
      <VisuallyHidden {...args} />
      Nur dieser Satz ist sichtbar.
    </Text>
  ),
};
