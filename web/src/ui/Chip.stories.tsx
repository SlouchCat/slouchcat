import type { Meta, StoryObj } from '@storybook/react-vite';
import { Chip } from './Chip';
import { Stack } from './Stack';

const meta = {
  title: 'UI/Chip',
  component: Chip,
  args: { children: 'FSK 16' },
} satisfies Meta<typeof Chip>;

export default meta;
type Story = StoryObj<typeof meta>;

export const Tones: Story = {
  render: () => (
    <Stack direction="row" gap={2}>
      <Chip>2024</Chip>
      <Chip tone="outline">FSK 16</Chip>
      <Chip tone="info" icon="folder">
        Netzfreigabe
      </Chip>
      <Chip tone="success">Bereit</Chip>
      <Chip tone="warning" icon="warning">
        Datei fehlt
      </Chip>
      <Chip tone="danger">Fehler</Chip>
    </Stack>
  ),
};
