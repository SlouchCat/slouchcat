import type { Meta, StoryObj } from '@storybook/react-vite';
import { useState } from 'react';
import { DataTable, type DataColumn } from './DataTable';
import { SelectCheck } from './SelectCheck';

interface Issue {
  id: string;
  title: string;
  path: string;
}

const issues: Issue[] = [
  { id: 'i1', title: 'Slouchcat Test Broken', path: '/media/Filme/Broken (2024).mkv' },
  { id: 'i2', title: 'Unbekannter Film', path: '/media/Filme/Unbekannt (2019).mkv' },
  { id: 'i3', title: 'Heimvideo', path: '/media/Filme/Heimvideo 2024.mp4' },
];

function Single({ selected = false }: { selected?: boolean }) {
  const [isSelected, setSelected] = useState(selected);
  return (
    <SelectCheck
      label="Unbekannter Film auswählen"
      isSelected={isSelected}
      onChange={setSelected}
    />
  );
}

/** A table of the maintenance page: the box in the first column, ↑/↓ move between the rows. */
function InTable() {
  const [selected, setSelected] = useState<ReadonlySet<string>>(() => new Set(['i2']));
  const columns: DataColumn<Issue>[] = [
    {
      key: 'select',
      header: 'Auswahl',
      hideHeader: true,
      cell: (row) => (
        <SelectCheck
          label={`${row.title} auswählen`}
          isSelected={selected.has(row.id)}
          onChange={(on) =>
            setSelected((current) => {
              const next = new Set(current);
              if (on) next.add(row.id);
              else next.delete(row.id);
              return next;
            })
          }
        />
      ),
    },
    { key: 'title', header: 'Titel', rowHeader: true, cell: (row) => row.title },
    { key: 'path', header: 'Pfad', cell: (row) => row.path },
  ];
  return (
    <DataTable
      caption="Nicht abspielbar in Filme"
      columns={columns}
      rows={issues}
      rowKey={(row) => row.id}
    />
  );
}

const meta = {
  title: 'UI/SelectCheck',
  component: SelectCheck,
  args: { label: 'Unbekannter Film auswählen', isSelected: false, onChange: () => undefined },
  render: () => <Single />,
  decorators: [
    (Story) => (
      <div style={{ width: '40rem' }}>
        <Story />
      </div>
    ),
  ],
} satisfies Meta<typeof SelectCheck>;

export default meta;
type Story = StoryObj<typeof meta>;

export const Default: Story = {};

export const Checked: Story = { render: () => <Single selected /> };

/** Tab to the box: the ring. */
export const Focused: Story = { tags: ['press:Tab'] };

/** Tab to the first row's box, ↓ to the second (checked) row's box. */
export const Table: Story = { render: () => <InTable />, tags: ['press:Tab+ArrowDown'] };

// The press tags are written out: the story index reads them from the source, not from a spread.
export const DefaultCompact: Story = { globals: { density: 'compact' } };

export const CheckedCompact: Story = {
  render: () => <Single selected />,
  globals: { density: 'compact' },
};

export const FocusedCompact: Story = { globals: { density: 'compact' }, tags: ['press:Tab'] };

export const TableCompact: Story = {
  render: () => <InTable />,
  globals: { density: 'compact' },
  tags: ['press:Tab+ArrowDown'],
};
