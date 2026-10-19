import type { Meta, StoryObj } from '@storybook/react-vite';
import { AriaButton } from './AriaButton';
import { ConfirmDialog, FormDialog } from './Dialog';
import { PinField } from './PinField';

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

/** Dialog around a form, opened by the page: focus sits in the first field. */
export const Form: Story = {
  render: () => (
    <>
      {/* The dialog sits in a portal; this keeps the story root from being empty. */}
      <AriaButton>Profil öffnen</AriaButton>
      <FormDialog
        isOpen
        onOpenChange={() => undefined}
        title="PIN für Lena"
        description="Dieses Profil ist mit einer PIN geschützt."
      >
        <PinField
          name="pin"
          label="PIN"
          description="4 bis 8 Ziffern."
          invalidMessage="Die PIN besteht aus 4 bis 8 Ziffern."
          value=""
          onChange={() => undefined}
          autoFocus
        />
        <AriaButton variant="primary">Öffnen</AriaButton>
      </FormDialog>
    </>
  ),
};
