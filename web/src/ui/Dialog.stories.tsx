import type { Meta, StoryObj } from '@storybook/react-vite';
import { useState } from 'react';
import { AriaButton } from './AriaButton';
import { ConfirmDialog, ConfirmInputDialog, FormDialog } from './Dialog';
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

/**
 * More than the question (concept 15 P-8): the release notes of an update in a box of their own,
 * scrolled with the keyboard, between the sentence and the buttons.
 */
export const WithDetails: Story = {
  tags: ['press:Tab+Enter'],
  args: {
    trigger: <AriaButton>Update auf 1.16.0 installieren</AriaButton>,
    title: 'Update auf 1.16.0 installieren?',
    body: 'Der Server startet neu; laufende Wiedergaben brechen ab. Vor einer Datenbankänderung sichert er die Datenbank.',
    confirmLabel: 'Installieren',
    tone: 'primary',
    children: (
      <div
        role="region"
        aria-label="Release-Notizen"
        // eslint-disable-next-line jsx-a11y/no-noninteractive-tabindex
        tabIndex={0}
        style={{ maxHeight: '8rem', overflowY: 'auto', whiteSpace: 'pre-wrap' }}
      >
        {
          'Neu: Update per Knopf mit dem Updater-Begleiter.\n\nBehoben: Die Update-Prüfung versucht es nach einem Aussetzer nach 15 Minuten erneut.\n\n- Kleinere Korrekturen in der Verwaltung\n- Neue Texte für die Hintergrundjobs\n- Schnellerer Start der Wiedergabe'
        }
      </div>
    ),
  },
};

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

/**
 * Confirmation that has to be typed out (idea 02, E12): the button stays disabled until the
 * field repeats the path, and the text says what the removal really does.
 */
export const TypedConfirmation: Story = {
  render: () => (
    <>
      {/* The dialog sits in a portal; this keeps the story root from being empty. */}
      <AriaButton variant="danger">Ordner entfernen</AriaButton>
      <ConfirmInputDialog
        isOpen
        onOpenChange={() => undefined}
        title="Ordner wirklich entfernen?"
        expected="/media/Filme-2"
        inputLabel="Tippe den Pfad, um zu bestätigen"
        mismatchText="Tippe /media/Filme-2 genau so ab, dann wird der Knopf aktiv."
        confirmLabel="Ordner entfernen"
        cancelLabel="Abbrechen"
        onConfirm={() => undefined}
      >
        <p>
          Du entfernst <code>/media/Filme-2</code> aus der Bibliothek Filme. Bis zu 3 Titel
          verschwinden damit aus allen Listen.
        </p>
        <ul>
          <li>Fortschritt, Merkliste und Picks bleiben an den Titeln hängen.</li>
          <li>Wirklich weg sind die Offline-Downloads auf den Geräten.</li>
        </ul>
      </ConfirmInputDialog>
    </>
  ),
};

/** Opened by the page (from a menu or a row), without a trigger of its own. */
export const Controlled: Story = {
  render: (args) => (
    <>
      {/* The dialog sits in a portal; this keeps the story root from being empty. */}
      <AriaButton variant="danger">Sicherung löschen</AriaButton>
      <ConfirmDialog {...args} trigger={undefined} isOpen onOpenChange={() => undefined} />
    </>
  ),
};

/** The same in the compact density: AdminDensityScope marks the dialog's portal. */
export const ControlledCompact: Story = { ...Controlled, globals: { density: 'compact' } };

/**
 * The typed confirmation opened by Enter on a plain `<button>` (a "virtual" click for react-aria):
 * the focus still starts in the field (e2e-storybook/dialog-focus.spec.ts).
 */
function TypedConfirmationFromButton() {
  const [isOpen, setOpen] = useState(false);
  return (
    <>
      <button type="button" onClick={() => setOpen(true)}>
        Ordner entfernen
      </button>
      <ConfirmInputDialog
        isOpen={isOpen}
        onOpenChange={setOpen}
        title="Ordner wirklich entfernen?"
        expected="/media/Filme-2"
        inputLabel="Tippe den Pfad, um zu bestätigen"
        mismatchText="Tippe /media/Filme-2 genau so ab, dann wird der Knopf aktiv."
        confirmLabel="Ordner entfernen"
        cancelLabel="Abbrechen"
        onConfirm={() => setOpen(false)}
      >
        <p>
          Du entfernst <code>/media/Filme-2</code> aus der Bibliothek Filme.
        </p>
      </ConfirmInputDialog>
    </>
  );
}

export const TypedConfirmationOpenedByButton: Story = {
  render: () => <TypedConfirmationFromButton />,
};
