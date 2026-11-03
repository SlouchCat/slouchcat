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

/** A number per tab, failures in the danger tone (concept 12, JOB-8). */
export const Counts: Story = {
  args: {
    label: 'Jobarten',
    tabs: [
      {
        id: 'library',
        label: 'Bibliothek',
        count: '6',
        content: <Text tone="secondary">Sechs Jobarten.</Text>,
      },
      { id: 'metadata', label: 'Metadaten', count: '7', content: <Text>Sieben Jobarten.</Text> },
      { id: 'delivery', label: 'Zustellung', count: '2', content: <Text>Zwei Jobarten.</Text> },
      {
        id: 'failed',
        label: 'Fehlschläge',
        count: '265',
        countLabel: '265 fehlgeschlagen',
        tone: 'danger',
        content: <Text>Die zehn jüngsten Fehlschläge.</Text>,
      },
    ],
  },
};

export const CountsCompact: Story = { ...Counts, globals: { density: 'compact' } };

/** The whole tab bar locked while a restore runs (Sicherung, concept 12, K12-7). */
export const Disabled: Story = { args: { isDisabled: true } };

export const DisabledCompact: Story = { ...Disabled, globals: { density: 'compact' } };
