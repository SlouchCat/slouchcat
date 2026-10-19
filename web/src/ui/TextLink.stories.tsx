import type { Meta, StoryObj } from '@storybook/react-vite';
import { Text } from './Text';
import { TextLink } from './TextLink';

const meta = {
  title: 'UI/TextLink',
  component: TextLink,
  args: { href: '#', children: 'TMDB-Schlüssel anlegen' },
} satisfies Meta<typeof TextLink>;

export default meta;
type Story = StoryObj<typeof meta>;

export const InText: Story = {
  render: (args) => (
    <Text tone="secondary">
      Ohne Schlüssel bleiben die Titel ohne Poster. <TextLink {...args} />
    </Text>
  ),
};

export const External: Story = { args: { external: true } };

export const Focused: Story = { tags: ['press:Tab'] };
