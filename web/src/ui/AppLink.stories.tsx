import type { Meta, StoryObj } from '@storybook/react-vite';
import { AppLink } from './AppLink';

// The plain link under ButtonLink, PosterCard and TextLink; unstyled on its own.
const meta = {
  title: 'UI/AppLink',
  component: AppLink,
  args: { href: '#', children: 'Zur Startseite' },
} satisfies Meta<typeof AppLink>;

export default meta;
type Story = StoryObj<typeof meta>;

export const Default: Story = {};

export const Focused: Story = { tags: ['press:Tab'] };
