import type { Meta, StoryObj } from '@storybook/react-vite';
import { Tabs } from './Tabs';
import { Text } from './Text';

const meta = {
  title: 'UI/Tabs',
  component: Tabs,
  args: {
    label: 'Problemarten',
    tabs: [
      {
        id: 'unmatched',
        label: 'Nicht zugeordnet (3)',
        content: <Text tone="secondary">Drei Titel ohne Zuordnung.</Text>,
      },
      {
        id: 'unplayable',
        label: 'Nicht abspielbar (1)',
        content: <Text tone="secondary">Eine Datei lässt sich nicht lesen.</Text>,
      },
      { id: 'duplicates', label: 'Duplikate', content: <Text tone="secondary">Keine.</Text> },
    ],
  },
} satisfies Meta<typeof Tabs>;

export default meta;
type Story = StoryObj<typeof meta>;

export const Default: Story = {};

/** Tab into the list, ArrowRight selects the next tab. */
export const ArrowKeys: Story = { tags: ['press:Tab+ArrowRight'] };
