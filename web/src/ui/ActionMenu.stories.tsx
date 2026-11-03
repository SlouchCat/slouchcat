import type { Meta, StoryObj } from '@storybook/react-vite';
import { useState } from 'react';
import { ActionMenu } from './ActionMenu';
import { AriaButton } from './AriaButton';
import { ConfirmDialog } from './Dialog';
import { Stack } from './Stack';

/** A row of the backup page: «Herunterladen» visible, the rest in the menu (S-6). */
function BackupRow() {
  const [confirm, setConfirm] = useState(false);
  return (
    <Stack direction="row" gap={2}>
      <AriaButton icon="download">Herunterladen</AriaButton>
      <ActionMenu
        rowName="Sicherung vom 1. November 2026"
        items={[
          { id: 'restore', label: 'Wiederherstellen', icon: 'replay' },
          { id: 'delete', label: 'Löschen', icon: 'trash', danger: true },
        ]}
        onAction={(id) => setConfirm(id === 'delete')}
      />
      <ConfirmDialog
        isOpen={confirm}
        onOpenChange={setConfirm}
        title="Sicherung löschen?"
        body="Die Datei wird vom Server entfernt."
        confirmLabel="Löschen"
        cancelLabel="Abbrechen"
        tone="danger"
        onConfirm={() => undefined}
      />
    </Stack>
  );
}

const meta = {
  title: 'UI/ActionMenu',
  component: BackupRow,
  decorators: [
    (Story) => (
      <div style={{ paddingBottom: '12rem' }}>
        <Story />
      </div>
    ),
  ],
} satisfies Meta<typeof BackupRow>;

export default meta;
type Story = StoryObj<typeof meta>;

export const Closed: Story = {};

/** Opened from the keyboard: the first action has the focus, «Gefahr» at the bottom. */
export const Open: Story = { tags: ['press:Tab+Tab+Enter'] };

export const Compact: Story = { globals: { density: 'compact' }, tags: ['press:Tab+Tab+Enter'] };
