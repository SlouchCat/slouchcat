import type { Meta, StoryObj } from '@storybook/react-vite';
import { GenreChip } from './GenreChip';
import { Row } from './Row';

const meta = {
  title: 'UI/GenreChip',
  component: GenreChip,
  args: { href: '#', name: 'Drama', count: 42 },
  decorators: [
    (Story) => (
      <div style={{ padding: 'var(--sc-space-6)' }}>
        <Story />
      </div>
    ),
  ],
} satisfies Meta<typeof GenreChip>;

export default meta;
type Story = StoryObj<typeof meta>;

export const Default: Story = {};

/** Focus: ring, a lighter fill and the focus scale. */
export const Focused: Story = { tags: ['press:Tab'] };

/** The start page row "Genres": chips as wide as their names. */
export const InARow: Story = {
  parameters: { layout: 'fullscreen' },
  render: () => (
    <Row title="Genres" variant="chips">
      {[
        ['Drama', 42],
        ['Komödie', 31],
        ['Science-Fiction', 17],
        ['Dokumentarfilm', 9],
        ['Horror', 4],
      ].map(([name, count]) => (
        <GenreChip key={name} href="#" name={String(name)} count={Number(count)} />
      ))}
    </Row>
  ),
};
