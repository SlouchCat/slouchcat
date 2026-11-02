import type { Meta, StoryObj } from '@storybook/react-vite';
import { RadioGroup } from './RadioGroup';
import { SettingRow } from './SettingRow';
import { Switch } from './Switch';
import { TextLink } from './TextLink';

const Rows = () => (
  <div>
    <SettingRow label="Aktiv" help="Die Spotlight-Reihe erscheint auf der Startseite.">
      <Switch isSelected onChange={() => undefined}>
        Aktiv
      </Switch>
    </SettingRow>
    <SettingRow label="Altersfreigabe" help="Nach welchem Land die Freigaben gelten.">
      <RadioGroup
        name="country"
        label="Land"
        value="CH"
        onChange={() => undefined}
        options={[
          { value: 'CH', label: 'Schweiz' },
          { value: 'DE', label: 'Deutschland' },
        ]}
      />
    </SettingRow>
  </div>
);

const meta = {
  title: 'UI/SettingRow',
  component: SettingRow,
  args: { label: 'Aktiv', children: null },
  render: Rows,
  decorators: [
    (Story) => (
      <div style={{ width: '48rem' }}>
        <Story />
      </div>
    ),
  ],
} satisfies Meta<typeof SettingRow>;

export default meta;
type Story = StoryObj<typeof meta>;

/** Label and help left, the control right; one setting per line (S-7). */
export const Default: Story = {};

export const Compact: Story = { globals: { density: 'compact' } };

/** The help may hold a link (Spotlight: the key under «Integrationen»). */
export const LinkInHelp: Story = {
  render: () => (
    <SettingRow
      label="Trakt-Liste"
      help={
        <>
          Öffentliche Listen, kein Trakt-Login.{' '}
          <TextLink href="#">Schlüssel unter Integrationen</TextLink>
        </>
      }
    >
      <RadioGroup
        name="list"
        label="Trakt-Liste"
        hideLabel
        defaultValue="trending"
        options={[
          { value: 'trending', label: 'Trending bei Trakt' },
          { value: 'popular', label: 'Beliebt bei Trakt' },
        ]}
      />
    </SettingRow>
  ),
};
