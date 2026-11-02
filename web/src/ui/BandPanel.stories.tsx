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

/** The three forms of the band: plain, folding away, with a tab bar. */
function Example({ form }: { form: 'plain' | 'collapsible' | 'tabs' }) {
  if (form === 'tabs') {
    return (
      <BandPanel
        title="Geräte"
        icon="monitor"
        tabs={{
          label: 'Ansicht',
          tabs: [
            { id: 'devices', label: 'Geräte', count: '4', content: body },
            {
              id: 'caps',
              label: 'Fähigkeiten',
              content: <Text tone="secondary">H.264, HEVC</Text>,
            },
          ],
        }}
      />
    );
  }
  return (
    <BandPanel
      title="E-Mail-Server"
      icon="bell"
      meta={<Chip tone="success">eingerichtet</Chip>}
      collapsible={form === 'collapsible'}
    >
      {body}
    </BandPanel>
  );
}

const meta = {
  title: 'UI/BandPanel',
  component: Example,
  args: { form: 'plain' },
  decorators: [
    (Story) => (
      <div style={{ width: '48rem' }}>
        <Story />
      </div>
    ),
  ],
} satisfies Meta<typeof Example>;

export default meta;
type Story = StoryObj<typeof meta>;

/** The tinted band (the profile's accent at 12 %) with title, icon and state. */
export const Default: Story = {};

/** The title is a button that folds the body away. */
export const Collapsible: Story = { args: { form: 'collapsible' } };

/** Folded with the keyboard: Tab to the title, Enter. */
export const Collapsed: Story = { args: { form: 'collapsible' }, tags: ['press:Tab+Enter'] };

/** A tab bar in the band (devices, podcasts, backup). */
export const WithTabs: Story = { args: { form: 'tabs' } };

export const Compact: Story = { args: { form: 'collapsible' }, globals: { density: 'compact' } };

export const CompactWithTabs: Story = { ...WithTabs, globals: { density: 'compact' } };
