import type { Meta, StoryObj } from '@storybook/react-vite';
import { InboxEntry } from './InboxEntry';

const meta = {
  title: 'UI/InboxEntry',
  component: InboxEntry,
  args: {
    href: '#',
    title: 'S01E03 · Die lange Nacht',
    context: 'Neue Folge von Die Stunde der Dämmerung',
    time: '21.10.2026, 08:15',
    unread: true,
    label:
      'Ungelesen: S01E03 · Die lange Nacht, Neue Folge von Die Stunde der Dämmerung, 21.10.2026, 08:15',
  },
  decorators: [
    (Story) => (
      <div style={{ maxWidth: '40rem', padding: 'var(--sc-space-6)' }}>
        <Story />
      </div>
    ),
  ],
} satisfies Meta<typeof InboxEntry>;

export default meta;
type Story = StoryObj<typeof meta>;

/** Unread: the accent dot and the bold title. */
export const Unread: Story = {};

export const Read: Story = { args: { unread: false } };

/** Focus ring and the lighter surface. */
export const Focused: Story = { tags: ['press:Tab'] };
