import { render, screen, waitFor, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { useState } from 'react';
import { describe, expect, it, vi } from 'vitest';
import { ActionMenu } from './ActionMenu';
import { ConfirmDialog } from './Dialog';

function Row({ onDelete }: { onDelete: () => void }) {
  const [confirm, setConfirm] = useState(false);
  const [restored, setRestored] = useState(false);
  return (
    <>
      <ActionMenu
        rowName="Sicherung 1"
        items={[
          { id: 'delete', label: 'Löschen', danger: true },
          { id: 'restore', label: 'Wiederherstellen' },
        ]}
        onAction={(id) => (id === 'delete' ? setConfirm(true) : setRestored(true))}
      />
      {restored && <p>wiederhergestellt</p>}
      <ConfirmDialog
        isOpen={confirm}
        onOpenChange={setConfirm}
        title="Sicherung löschen?"
        body="Die Datei wird entfernt."
        confirmLabel="Löschen"
        cancelLabel="Abbrechen"
        tone="danger"
        onConfirm={onDelete}
      />
    </>
  );
}

describe('ActionMenu (concept 12, S-6)', () => {
  it('names the row, lists danger last under «Gefahr» and returns the focus', async () => {
    const user = userEvent.setup();
    render(<Row onDelete={() => undefined} />);
    const button = screen.getByRole('button', { name: 'Weitere Aktionen für Sicherung 1' });
    await user.click(button);
    const menu = screen.getByRole('menu');
    expect(
      within(menu)
        .getAllByRole('menuitem')
        .map((item) => item.textContent),
    ).toEqual(['Wiederherstellen', 'Löschen']);
    expect(within(menu).getByRole('group', { name: 'Gefahr' })).toContainElement(
      within(menu).getByRole('menuitem', { name: 'Löschen' }),
    );
    await user.keyboard('{Escape}');
    expect(screen.queryByRole('menu')).toBeNull();
    await waitFor(() => expect(button).toHaveFocus());

    await user.keyboard('{Enter}');
    await user.keyboard('{Enter}');
    expect(await screen.findByText('wiederhergestellt')).toBeVisible();
    await waitFor(() => expect(button).toHaveFocus());
  });

  it('opens the confirmation of a dangerous action; cancelling returns to the button', async () => {
    const user = userEvent.setup();
    const onDelete = vi.fn();
    render(<Row onDelete={onDelete} />);
    const button = screen.getByRole('button', { name: 'Weitere Aktionen für Sicherung 1' });
    await user.click(button);
    await user.click(screen.getByRole('menuitem', { name: 'Löschen' }));
    const dialog = await screen.findByRole('alertdialog', { name: 'Sicherung löschen?' });
    await user.click(within(dialog).getByRole('button', { name: 'Abbrechen' }));
    await waitFor(() => expect(screen.queryByRole('alertdialog')).toBeNull());
    await waitFor(() => expect(button).toHaveFocus());
    expect(onDelete).not.toHaveBeenCalled();

    await user.click(button);
    await user.click(screen.getByRole('menuitem', { name: 'Löschen' }));
    await user.click(
      within(await screen.findByRole('alertdialog')).getByRole('button', { name: 'Löschen' }),
    );
    expect(onDelete).toHaveBeenCalledOnce();
    await waitFor(() => expect(button).toHaveFocus());
  });
});
