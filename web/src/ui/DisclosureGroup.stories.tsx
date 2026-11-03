import type { Meta, StoryObj } from '@storybook/react-vite';
import { DataTable, type DataColumn } from './DataTable';
import { DisclosureGroup } from './DisclosureGroup';
import { Stack } from './Stack';

interface Issue {
  id: string;
  title: string;
  path: string;
}

const issues: Issue[] = [
  { id: 'i1', title: 'Unbekannter Film', path: '/media/Filme/Unbekannt (2019).mkv' },
  { id: 'i2', title: 'Heimvideo', path: '/media/Filme/Heimvideo 2024.mp4' },
];

const columns: DataColumn<Issue>[] = [
  { key: 'title', header: 'Titel', rowHeader: true, cell: (row) => row.title },
  { key: 'path', header: 'Pfad', cell: (row) => row.path },
];

const table = (caption: string) => (
  <DataTable caption={caption} columns={columns} rows={issues} rowKey={(row) => row.id} />
);

const meta = {
  title: 'UI/DisclosureGroup',
  component: DisclosureGroup,
  args: { title: 'Filme', count: '2', countLabel: '2 Titel', children: table('Filme') },
  render: (args) => (
    <Stack gap={4}>
      <DisclosureGroup {...args} />
      <DisclosureGroup title="Serien" count="2" countLabel="2 Titel">
        {table('Serien')}
      </DisclosureGroup>
    </Stack>
  ),
  decorators: [
    (Story) => (
      <div style={{ width: '48rem' }}>
        <Story />
      </div>
    ),
  ],
} satisfies Meta<typeof DisclosureGroup>;

export default meta;
type Story = StoryObj<typeof meta>;

/** One section per library, each with its own table; open at first. */
export const Default: Story = {};

/** Folded with the keyboard: Tab to the first heading, Enter. */
export const Collapsed: Story = { tags: ['press:Tab+Enter'] };

export const Compact: Story = { globals: { density: 'compact' } };
