import type { Meta, StoryObj } from '@storybook/react-vite';
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
