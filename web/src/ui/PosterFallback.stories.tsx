import type { Meta, StoryObj } from '@storybook/react-vite';
import { PosterFallback } from './PosterFallback';

const meta = {
  title: 'UI/PosterFallback',
  component: PosterFallback,
  decorators: [
    (Story) => (
      <div style={{ width: 'var(--sc-size-poster-lg)' }}>
        <Story />
      </div>
    ),
  ],
} satisfies Meta<typeof PosterFallback>;

export default meta;
type Story = StoryObj<typeof meta>;

export const Default: Story = {};
