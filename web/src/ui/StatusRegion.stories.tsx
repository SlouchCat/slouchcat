import type { Meta, StoryObj } from '@storybook/react-vite';
import { Notice } from './Notice';
import { Stack } from './Stack';
import { StatusRegion } from './StatusRegion';
import { Text } from './Text';

const meta = {
  title: 'UI/StatusRegion',
  component: StatusRegion,
  args: { message: { text: 'Gespeichert.', key: 1 } },
  decorators: [
    (Story) => (
      <div style={{ width: '40rem' }}>
        <Story />
      </div>
    ),
  ],
} satisfies Meta<typeof StatusRegion>;

export default meta;
type Story = StoryObj<typeof meta>;

/** The outcome of the last action, between the page's other blocks. */
export const Default: Story = {
  render: (args) => (
    <Stack gap={4}>
      <Text>Block davor</Text>
      <StatusRegion {...args} />
      <Text>Block danach</Text>
    </Stack>
  ),
};

export const Compact: Story = { ...Default, globals: { density: 'compact' } };

/** Empty, the region takes no room and no gap: the two blocks stand as without it. */
export const Empty: Story = { ...Default, args: { message: null } };

/** The alert variant with a danger message. */
export const Alert: Story = {
  ...Default,
  args: { role: 'alert', message: { text: 'Speichern fehlgeschlagen.', tone: 'danger', key: 1 } },
};

/** Several messages at once (one per saved scope, Metadaten). */
export const Several: Story = {
  ...Default,
  args: {
    message: null,
    children: (
      <>
        <Notice live={false}>Standard gespeichert.</Notice>
        <Notice live={false}>Filme gespeichert.</Notice>
      </>
    ),
  },
};
