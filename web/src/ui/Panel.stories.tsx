import type { Meta, StoryObj } from '@storybook/react-vite';
import { Chip } from './Chip';
import { Panel } from './Panel';
import { Text } from './Text';

const meta = {
  title: 'UI/Panel',
  component: Panel,
  args: { children: null },
} satisfies Meta<typeof Panel>;

export default meta;
type Story = StoryObj<typeof meta>;

export const Default: Story = {
  args: {
    children: (
      <>
        <Text as="h2" variant="title">
          Filme
        </Text>
        <Text tone="secondary">/media/Filme · 128 Titel</Text>
        <div>
          <Chip tone="info" icon="folder">
            Netzfreigabe
          </Chip>
        </div>
      </>
    ),
  },
};
