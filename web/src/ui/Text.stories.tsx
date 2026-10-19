import type { Meta, StoryObj } from '@storybook/react-vite';
import { Stack } from './Stack';
import { Text } from './Text';

const meta = {
  title: 'UI/Text',
  component: Text,
  args: { children: 'Die Stunde der Dämmerung' },
} satisfies Meta<typeof Text>;

export default meta;
type Story = StoryObj<typeof meta>;

export const Variants: Story = {
  render: () => (
    <Stack gap={4}>
      <Text variant="display" as="h1">
        Die Stunde der Dämmerung
      </Text>
      <Text variant="title" as="h2">
        Neu hinzugefügt
      </Text>
      <Text>Ein Leuchtturmwärter zählt die Schiffe, die nicht mehr kommen.</Text>
      <Text variant="caption" tone="secondary">
        2023 · 1 h 52 min
      </Text>
      <Text variant="caption" tone="muted">
        Zuletzt gesehen vor drei Tagen
      </Text>
    </Stack>
  ),
};
