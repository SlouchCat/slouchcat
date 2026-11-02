import type { Meta, StoryObj } from '@storybook/react-vite';
import { AdminSideNavMenu } from './AdminSideNavMenu';
import { adminSideNavArgs } from './AdminSideNav.stories';

// The administration's sidebar on a phone (concept 11, 11.1): the button at the top of an admin
// page and the sheet it opens, with the same entries as the column.

const meta = {
  title: 'UI/AdminSideNavMenu',
  component: AdminSideNavMenu,
  args: {
    ...adminSideNavArgs,
    top: adminSideNavArgs.top.map((entry) => ({ ...entry, current: false })),
    groups: adminSideNavArgs.groups.map((group) => ({
      ...group,
      entries: group.entries.map((entry) => ({ ...entry, current: entry.href === '#libraries' })),
    })),
    buttonLabel: 'Verwaltung',
    pageLabel: 'Bibliotheken',
    menuLabel: 'Seiten der Verwaltung',
    closeLabel: 'Seiten der Verwaltung schliessen',
  },
  decorators: [
    (Story) => (
      <div style={{ width: '23.4375rem' }}>
        <Story />
      </div>
    ),
  ],
} satisfies Meta<typeof AdminSideNavMenu>;

export default meta;
type Story = StoryObj<typeof meta>;

/** The button on top of the page: the administration and the current page. */
export const Closed: Story = {};

/** Tab to the button, Enter opens the sheet; the current page's entry has the focus. */
export const Phone: Story = { tags: ['press:Tab+Enter'] };
