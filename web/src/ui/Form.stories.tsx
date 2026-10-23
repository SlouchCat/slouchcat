import type { Meta, StoryObj } from '@storybook/react-vite';
import { AriaButton } from './AriaButton';
import { Form } from './Form';
import { Stack } from './Stack';
import { TextField } from './TextField';

const meta = {
  title: 'UI/Form',
  component: Form,
  decorators: [
    (Story) => (
      <div style={{ maxWidth: 'var(--sc-size-form-max)' }}>
        <Story />
      </div>
    ),
  ],
} satisfies Meta<typeof Form>;

export default meta;
type Story = StoryObj<typeof meta>;

/** What a form looks like after the server refused one of its fields. */
export const ServerAnswer: Story = {
  render: () => (
    <Form validationErrors={{ host: 'Dieser Wert ist ungültig.' }}>
      <Stack gap={6}>
        <TextField name="host" label="Server" mono defaultValue="smtp.beispiel.ch" />
        <TextField name="port" label="Port" mono defaultValue="587" />
        <AriaButton type="submit" variant="primary">
          Speichern
        </AriaButton>
      </Stack>
    </Form>
  ),
};
