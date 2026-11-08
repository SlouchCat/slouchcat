import type { Meta, StoryObj } from '@storybook/react-vite';
import { AriaButton } from './AriaButton';
import { FacetOptions } from './FacetSelect';
import { FilterSheet } from './FilterSheet';

const meta = {
  title: 'UI/FilterSheet',
  component: FilterSheet,
  args: {
    isOpen: true,
    onOpenChange: () => undefined,
    title: 'Stimmung',
    closeLabel: 'Filter schliessen',
    children: null,
  },
  render: (args) => (
    <>
      {/* The sheet sits in a portal; this keeps the story root from being empty. */}
      <AriaButton>Stimmung</AriaButton>
      <FilterSheet
        {...args}
        footer={
          <>
            <AriaButton variant="ghost">Auswahl löschen</AriaButton>
            <AriaButton variant="primary">48 Sender anzeigen</AriaButton>
          </>
        }
      >
        <FacetOptions
          label="Stimmung"
          options={[
            { value: 'chill', label: 'Chill', count: 48 },
            { value: 'party', label: 'Party', count: 31 },
            { value: 'focus', label: 'Fokus', count: 12 },
            { value: 'sleep', label: 'Schlafen', count: 4 },
          ]}
          values={['chill']}
          onChange={() => undefined}
          emptyLabel="Keine Werte zur Auswahl"
        />
      </FilterSheet>
    </>
  ),
} satisfies Meta<typeof FilterSheet>;

export default meta;
type Story = StoryObj<typeof meta>;

export const Open: Story = {};
