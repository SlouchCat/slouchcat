import type { Meta, StoryObj } from '@storybook/react-vite';
import { BandPanel } from './BandPanel';
import { Chip } from './Chip';
import { SettingRow } from './SettingRow';
import { Switch } from './Switch';
import { Text } from './Text';

const body = (
  <>
    <SettingRow label="E-Mail-Versand" help="Mitteilungen per E-Mail an die Profile.">
      <Switch isSelected onChange={() => undefined}>
        Aktiv
      </Switch>
    </SettingRow>
    <SettingRow label="Absender" help="So erscheint Slouchcat im Posteingang.">
      <Text tone="secondary">slouchcat@example.test</Text>
    </SettingRow>
  </>
);

const meta = {
  title: 'UI/BandPanel',
  component: BandPanel,
  args: {
    title: 'E-Mail-Server',
    icon: 'bell',
    meta: <Chip tone="success">eingerichtet</Chip>,
    children: body,
  },
  decorators: [
    (Story) => (
      <div style={{ width: '48rem' }}>
        <Story />
      </div>
    ),
  ],
} satisfies Meta<typeof BandPanel>;

export default meta;
type Story = StoryObj<typeof meta>;

/** The tinted band (the profile's accent at 12 %) with title, icon and state. */
export const Default: Story = {};

/** The title is a button that folds the body away. */
export const Collapsible: Story = { args: { collapsible: true } };

/** Folded with the keyboard: Tab to the title, Enter. */
export const Collapsed: Story = { args: { collapsible: true }, tags: ['press:Tab+Enter'] };

/** A tab bar in the band (devices, podcasts, backup). */
export const WithTabs: Story = {
  args: {
    title: 'Geräte',
    icon: 'monitor',
    meta: undefined,
    tabs: {
      label: 'Ansicht',
      tabs: [
        { id: 'devices', label: 'Geräte', count: '4', content: body },
        { id: 'caps', label: 'Fähigkeiten', content: <Text tone="secondary">H.264, HEVC</Text> },
      ],
    },
  },
};

export const Compact: Story = { args: { collapsible: true }, globals: { density: 'compact' } };

export const CompactWithTabs: Story = { ...WithTabs, globals: { density: 'compact' } };
