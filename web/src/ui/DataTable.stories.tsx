import type { Meta, StoryObj } from '@storybook/react-vite';
import { Button } from './Button';
import { Chip } from './Chip';
import { DataTable, type DataColumn } from './DataTable';

interface Stream {
  id: string;
  profile: string;
  device: string;
  title: string;
  quality: string;
  mode: string;
}

const rows: Stream[] = [
  {
    id: 'ps-1',
    profile: 'Marco',
    device: 'Chrome 156 · Linux',
    title: 'Slouchcat Test Multi Audio',
    quality: '1080p · 8 Mbit/s',
    mode: 'Transcode (HW)',
  },
  {
    id: 'ps-2',
    profile: 'Lena',
    device: 'Safari 19 · iOS',
    title: 'Slouchcat Test Show – S01E02',
    quality: 'Original',
    mode: 'Direct',
  },
];

const columns: DataColumn<Stream>[] = [
  { key: 'profile', header: 'Profil', rowHeader: true, cell: (row) => row.profile },
  { key: 'device', header: 'Gerät', cell: (row) => row.device },
  { key: 'title', header: 'Titel', cell: (row) => row.title },
  { key: 'quality', header: 'Qualität', cell: (row) => row.quality },
  { key: 'mode', header: 'Modus', cell: (row) => <Chip tone="outline">{row.mode}</Chip> },
  {
    key: 'actions',
    header: 'Aktionen',
    hideHeader: true,
    align: 'end',
    cell: () => <Button variant="secondary">Beenden</Button>,
  },
];

const meta = {
  title: 'UI/DataTable',
  component: DataTable<Stream>,
  args: { caption: 'Laufende Streams', columns, rows, rowKey: (row: Stream) => row.id },
  decorators: [
    (Story) => (
      <div style={{ width: '60rem', padding: 'var(--sc-space-6)' }}>
        <Story />
      </div>
    ),
  ],
} satisfies Meta<typeof DataTable<Stream>>;

export default meta;
type Story = StoryObj<typeof meta>;

export const Rows: Story = {};

/** Tab to the first row's button; ↓ moves to the same button in the next row. */
export const RowFocus: Story = { tags: ['press:Tab+ArrowDown'] };
