import type { Meta, StoryObj } from '@storybook/react-vite';
import { SaveBar } from './SaveBar';

const meta = {
  title: 'UI/SaveBar',
  component: SaveBar,
  args: { dirty: true, saving: false, onSave: () => undefined, onDiscard: () => undefined },
  decorators: [
    (Story) => (
      <div style={{ width: '48rem' }}>
        <Story />
      </div>
    ),
  ],
} satisfies Meta<typeof SaveBar>;

export default meta;
type Story = StoryObj<typeof meta>;

/** Changed: «Verwerfen» and «Speichern» are active. */
export const Dirty: Story = {};

/** Nothing changed: «Gespeichert», both buttons wait. */
export const Clean: Story = { args: { dirty: false } };

export const Compact: Story = { globals: { density: 'compact' } };
