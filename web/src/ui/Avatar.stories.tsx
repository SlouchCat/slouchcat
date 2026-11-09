import type { Meta, StoryObj } from '@storybook/react-vite';
import { Avatar } from './Avatar';
import { art } from './stories/fixtures';

const meta = {
  title: 'UI/Avatar',
  component: Avatar,
  args: { name: 'Sigourney Weaver', imageUrl: art.posterDusk },
  decorators: [
    (Story) => (
      <div style={{ padding: 'var(--sc-space-6)' }}>
        <Story />
      </div>
    ),
  ],
} satisfies Meta<typeof Avatar>;

export default meta;
type Story = StoryObj<typeof meta>;

export const WithImage: Story = {};

/** No portrait (yet): the first letter of the name. */
export const Letter: Story = { args: { imageUrl: undefined } };

/** The portrait fails to load: the letter takes its place. */
export const BrokenImage: Story = { args: { imageUrl: 'does-not-exist.jpg' } };

/** The large size, in the head of the person page. */
export const Large: Story = { args: { size: 'lg' } };
