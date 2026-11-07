import type { Meta, StoryObj } from '@storybook/react-vite';
import { ActiveFilters } from './ActiveFilters';

const noop = () => undefined;

const meta = {
  title: 'UI/ActiveFilters',
  component: ActiveFilters,
  args: {
    label: 'Aktive Filter',
    filters: [
      {
        key: 'country-CH',
        label: 'Schweiz',
        removeLabel: 'Filter Schweiz entfernen',
        onRemove: noop,
      },
      { key: 'genre-jazz', label: 'Jazz', removeLabel: 'Filter Jazz entfernen', onRemove: noop },
      {
        key: 'titles',
        label: 'Nur mit Titelanzeige',
        removeLabel: 'Filter Nur mit Titelanzeige entfernen',
        onRemove: noop,
      },
      { key: 'q', label: '«Swiss»', removeLabel: 'Filter «Swiss» entfernen', onRemove: noop },
    ],
    resetLabel: 'Alle Filter zurücksetzen',
    onReset: noop,
  },
} satisfies Meta<typeof ActiveFilters>;

export default meta;
type Story = StoryObj<typeof meta>;

export const Default: Story = {};

/** The first chip focused. */
export const Focused: Story = { tags: ['press:Tab'] };
