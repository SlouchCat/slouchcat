import type { Meta, StoryObj } from '@storybook/react-vite';
import { AriaButton } from './AriaButton';
import { Stack } from './Stack';

// The react-aria button for forms, dialogs and the player: the same look as Button.
const meta = {
  title: 'UI/AriaButton',
  component: AriaButton,
  args: { children: 'Speichern', variant: 'primary' },
} satisfies Meta<typeof AriaButton>;

export default meta;
type Story = StoryObj<typeof meta>;

export const Variants: Story = {
  render: () => (
    <Stack direction="row" gap={2}>
      <AriaButton variant="primary">Speichern</AriaButton>
      <AriaButton variant="secondary" icon="plus">
        Bibliothek hinzufügen
      </AriaButton>
      <AriaButton variant="ghost">Abbrechen</AriaButton>
      <AriaButton variant="primary" isDisabled>
        Gesperrt
      </AriaButton>
    </Stack>
  ),
};

export const Focused: Story = { tags: ['press:Tab'] };
