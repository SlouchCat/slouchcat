import type { Meta, StoryObj } from '@storybook/react-vite';
import { TitleOrLogo } from './TitleOrLogo';
import { art } from './stories/fixtures';

const meta = {
  title: 'UI/TitleOrLogo',
  component: TitleOrLogo,
  args: { title: 'Glut' },
  render: (args) => (
    <h1 style={{ maxWidth: '24rem', fontSize: 'var(--sc-font-size-2xl)' }}>
      <TitleOrLogo {...args} />
    </h1>
  ),
} satisfies Meta<typeof TitleOrLogo>;

export default meta;
type Story = StoryObj<typeof meta>;

/** Without a logo the heading carries the text title. */
export const Text: Story = {};

/** With a logo the heading carries the image; `alt` keeps the title for screen readers. */
export const Logo: Story = { args: { logoUrl: art.logoEmber } };

/** A logo that does not load (offline) falls back to the text. */
export const BrokenLogo: Story = { args: { logoUrl: 'missing-logo.png' } };
