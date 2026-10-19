import type { Meta, StoryObj } from '@storybook/react-vite';
import { Stack } from './Stack';
import { StatusDot, type StatusDotState } from './StatusDot';
import { Text } from './Text';

const states: StatusDotState[] = ['pending', 'active', 'success', 'warning', 'danger'];

const meta = {
  title: 'UI/StatusDot',
  component: StatusDot,
  args: { state: 'active' },
} satisfies Meta<typeof StatusDot>;

export default meta;
type Story = StoryObj<typeof meta>;

export const States: Story = {
  render: () => (
    <Stack gap={2}>
      {states.map((state) => (
        <Stack key={state} direction="row" gap={2}>
          <StatusDot state={state} />
          <Text variant="caption" as="span">
            {state}
          </Text>
        </Stack>
      ))}
    </Stack>
  ),
};
