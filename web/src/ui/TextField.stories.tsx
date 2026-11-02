import type { Meta, StoryObj } from '@storybook/react-vite';
import { SettingRow } from './SettingRow';
import { Stack } from './Stack';
import { TextField } from './TextField';

const meta = {
  title: 'UI/TextField',
  component: TextField,
  args: { label: 'E-Mail', placeholder: 'name@beispiel.ch' },
  decorators: [
    (Story) => (
      <div style={{ maxWidth: 'var(--sc-size-form-max)' }}>
        <Story />
      </div>
    ),
  ],
} satisfies Meta<typeof TextField>;

export default meta;
type Story = StoryObj<typeof meta>;

export const States: Story = {
  render: () => (
    <Stack gap={6}>
      <TextField label="E-Mail" placeholder="name@beispiel.ch" />
      <TextField
        label="Ordner"
        mono
        defaultValue="/media/Filme"
        description="Pfad im Container, nur lesend eingebunden."
      />
      <TextField label="Passwort" type="password" isInvalid defaultValue="kurz" />
      <TextField
        label="Text"
        rows={4}
        description="Höchstens 4000 Zeichen."
        defaultValue={'Am Samstag ist der Server von 8 bis 10 Uhr nicht erreichbar.\nDanke!'}
      />
    </Stack>
  ),
};

export const Focused: Story = { tags: ['press:Tab'], args: { defaultValue: 'marco@' } };

/**
 * In a SettingRow the row shows the label and the help on the left; `hideLabel` keeps the label
 * as the field's name for screen readers only, so it does not show twice.
 */
const InSettingRow = () => (
  <SettingRow label="Höchstens" help="Titel in der Reihe, je Profil (5–20)">
    <div style={{ width: '8rem' }}>
      <TextField label="Höchstens" hideLabel type="number" defaultValue="12" />
    </div>
  </SettingRow>
);

export const HiddenLabel: Story = { render: InSettingRow };

export const HiddenLabelCompact: Story = { render: InSettingRow, globals: { density: 'compact' } };
