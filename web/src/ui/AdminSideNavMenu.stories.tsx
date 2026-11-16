import type { Meta, StoryObj } from '@storybook/react-vite';
import { AdminSideNavMenu } from './AdminSideNavMenu';
import { adminSheetArgs, adminSheetGroups, withNumbers } from './AdminSideNav.stories';

// The administration's sidebar on a phone (concept 11, 11.1; build plan 2.3): the button in the
// look of the breadcrumb line at the top of an admin page and the sheet it opens, with every page
// of every area and their numbers.

const meta = {
  title: 'UI/AdminSideNavMenu',
  component: AdminSideNavMenu,
  args: {
    ...adminSheetArgs,
    top: adminSheetArgs.top.map(withNumbers('#libraries')),
    groups: adminSheetGroups.map((group) => ({
      ...group,
      entries: group.entries.map(withNumbers('#libraries')),
    })),
    buttonLabel: 'Verwaltung',
    pageLabel: 'Medien / Bibliotheken',
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

/** The button on top of the page: the administration, the area and the current page. */
export const Closed: Story = {};

/** Tab to the button, Enter opens the sheet; the current page's entry has the focus. */
export const Phone: Story = { tags: ['press:Tab+Enter'] };
