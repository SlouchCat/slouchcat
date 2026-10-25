import type { Meta, StoryObj } from '@storybook/react-vite';
import { PathList, type PathItem } from './PathList';

const items: PathItem[] = [
  { id: 'Filme', name: 'Filme', detail: '/media/Filme', note: 'Bibliothek Filme' },
  { id: 'Serien', name: 'Serien', detail: '/media/Serien' },
  { id: 'Neu', name: 'Neu zu sortieren', detail: '/media/Neu zu sortieren' },
  {
    id: 'ablage',
    name: 'ablage',
    detail: '/media/ablage',
    note: 'Verknüpfung, wird nicht durchsucht',
    closed: true,
  },
];

const meta = {
  title: 'UI/PathList',
  component: PathList,
  args: {
    label: 'Ordner',
    items,
    emptyText: 'Keine Unterordner.',
    onAction: () => undefined,
  },
  decorators: [
    (Story) => (
      <div style={{ maxWidth: '40rem' }}>
        <Story />
      </div>
    ),
  ],
} satisfies Meta<typeof PathList>;

export default meta;
type Story = StoryObj<typeof meta>;

export const Default: Story = {};

/**
 * Tab reaches the list as one stop, the arrow keys walk the rows. The rows carry no selection:
 * one click or Enter enters the folder, which is what a folder picker is for.
 */
export const Focused: Story = { tags: ['press:Tab+ArrowDown'] };

/** A folder without subfolders: the dialog says so instead of showing an empty box. */
export const Empty: Story = {
  render: (args) => <PathList {...args} items={[]} />,
};
