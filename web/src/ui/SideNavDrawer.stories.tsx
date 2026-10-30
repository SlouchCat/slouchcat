import type { Meta, StoryObj } from '@storybook/react-vite';
import { sideNavArgs } from './SideNav.stories';
import { SideNavDrawer } from './SideNavDrawer';

// The side navigation as the drawer (concept 08): below 768 px, and from 768 px once the viewer has
// hidden the column. It opens on the first entry.

const meta = {
  title: 'UI/SideNavDrawer',
  component: SideNavDrawer,
  parameters: { layout: 'fullscreen' },
  args: {
    ...sideNavArgs,
    isOpen: true,
    onClose: () => undefined,
    id: 'app-menu',
    menuLabel: 'Menü',
    closeLabel: 'Menü schliessen',
  },
} satisfies Meta<typeof SideNavDrawer>;

export default meta;
type Story = StoryObj<typeof meta>;

/** On a phone: the same entries; no "Seitenleiste ausblenden", there is no column to hide. */
export const Phone: Story = {};

/** From 768 px with the column hidden: the drawer's last entry brings it back. */
export const HiddenBar: Story = {
  args: { show: { label: 'Seitenleiste einblenden', onPress: () => undefined } },
};
