import type { Meta, StoryObj } from '@storybook/react-vite';
import { AriaButton } from './AriaButton';
import { ConfirmDialog } from './Dialog';

const meta = {
  title: 'UI/ConfirmDialog',
  component: ConfirmDialog,
  args: {
    trigger: <AriaButton variant="danger">Bibliothek entfernen</AriaButton>,
    title: 'Bibliothek entfernen?',
    body: 'Die Dateien bleiben auf der Platte; Slouchcat vergisst nur die Titel dieser Bibliothek.',
    confirmLabel: 'Entfernen',
    cancelLabel: 'Abbrechen',
    tone: 'danger',
    onConfirm: () => undefined,
  },
} satisfies Meta<typeof ConfirmDialog>;

export default meta;
type Story = StoryObj<typeof meta>;

export const Closed: Story = {};

/** Opened from the keyboard: focus sits on the confirm button. */
export const Open: Story = { tags: ['press:Tab+Enter'] };
