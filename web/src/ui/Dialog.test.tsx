import { render, screen, waitFor, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { useState } from 'react';
import { describe, expect, it, vi } from 'vitest';
import { AriaButton } from './AriaButton';
import { ConfirmDialog, FormDialog } from './Dialog';
import { RadioGroup } from './RadioGroup';
import { TextField } from './TextField';

const texts = {
  title: 'Entfernen?',
  body: 'Die Bibliothek wird entfernt.',
  confirmLabel: 'Entfernen',
  cancelLabel: 'Abbrechen',
};

describe('ConfirmDialog', () => {
  it('opens from its trigger and returns the focus there (unchanged)', async () => {
    const user = userEvent.setup();
    const onConfirm = vi.fn();
    render(
      <ConfirmDialog {...texts} trigger={<AriaButton>Öffnen</AriaButton>} onConfirm={onConfirm} />,
    );
    await user.click(screen.getByRole('button', { name: 'Öffnen' }));
    expect(screen.getByRole('alertdialog', { name: 'Entfernen?' })).toBeVisible();
    expect(screen.getByRole('button', { name: 'Entfernen' })).toHaveFocus();
    await user.keyboard('{Escape}');
    await waitFor(() => expect(screen.getByRole('button', { name: 'Öffnen' })).toHaveFocus());
    expect(onConfirm).not.toHaveBeenCalled();
  });

  it('is opened and closed by the page with isOpen and onOpenChange', async () => {
    const user = userEvent.setup();
    const onConfirm = vi.fn();
    function Page() {
      const [open, setOpen] = useState(false);
      return (
        <>
          <button type="button" onClick={() => setOpen(true)}>
            Aus dem Menü
          </button>
          <p>{open ? 'offen' : 'zu'}</p>
          <ConfirmDialog {...texts} isOpen={open} onOpenChange={setOpen} onConfirm={onConfirm} />
        </>
      );
    }
    render(<Page />);
    expect(screen.queryByRole('alertdialog')).toBeNull();
    await user.click(screen.getByRole('button', { name: 'Aus dem Menü' }));
    expect(screen.getByRole('alertdialog', { name: 'Entfernen?' })).toBeVisible();
    await user.click(screen.getByRole('button', { name: 'Entfernen' }));
    expect(onConfirm).toHaveBeenCalledOnce();
    await waitFor(() => expect(screen.queryByRole('alertdialog')).toBeNull());
    expect(screen.getByText('zu')).toBeVisible();
    await waitFor(() => expect(screen.getByRole('button', { name: 'Aus dem Menü' })).toHaveFocus());
  });

  it('closes the controlled dialog with Escape', async () => {
    const user = userEvent.setup();
    const onOpenChange = vi.fn();
    render(
      <ConfirmDialog {...texts} isOpen onOpenChange={onOpenChange} onConfirm={() => undefined} />,
    );
    expect(screen.getByRole('alertdialog')).toBeVisible();
    await user.keyboard('{Escape}');
    expect(onOpenChange).toHaveBeenCalledWith(false);
  });

  it('starts on Cancel when asked to (defaultFocus)', async () => {
    render(
      <ConfirmDialog
        {...texts}
        isOpen
        onOpenChange={() => undefined}
        onConfirm={() => undefined}
        defaultFocus="cancel"
      />,
    );
    // focusOnMount moves the focus right after the commit, not during the render.
    await waitFor(() => expect(screen.getByRole('button', { name: 'Abbrechen' })).toHaveFocus());
  });
});

/** A form dialog with a button that opens a second form dialog on top of it. */
function NestedForms() {
  const [outer, setOuter] = useState(false);
  const [inner, setInner] = useState(false);
  return (
    <>
      <AriaButton onPress={() => setOuter(true)}>Aussen öffnen</AriaButton>
      <FormDialog isOpen={outer} onOpenChange={setOuter} title="Aussen">
        <TextField label="Name" />
        <AriaButton onPress={() => setInner(true)}>Innen öffnen</AriaButton>
        <FormDialog isOpen={inner} onOpenChange={setInner} title="Innen">
          <RadioGroup
            name="content"
            label="Inhalt"
            value="movie"
            onChange={() => undefined}
            options={[
              { value: 'movie', label: 'Filme' },
              { value: 'series', label: 'Serien' },
            ]}
          />
          <TextField label="Pfad" />
        </FormDialog>
      </FormDialog>
    </>
  );
}

describe('FormDialog', () => {
  it('focuses the first field of a dialog opened from inside another one', async () => {
    const user = userEvent.setup();
    render(<NestedForms />);
    await user.click(screen.getByRole('button', { name: 'Aussen öffnen' }));
    const outer = await screen.findByRole('dialog', { name: 'Aussen' });
    await waitFor(() => expect(within(outer).getByLabelText('Name')).toHaveFocus());

    // Opened by keyboard, the way the folders dialog opens «Ändern».
    await user.tab();
    expect(within(outer).getByRole('button', { name: 'Innen öffnen' })).toHaveFocus();
    await user.keyboard('{Enter}');
    const inner = await screen.findByRole('dialog', { name: 'Innen' });
    // The first field of the inner dialog, the checked radio like in «Ordner ändern»; the outer
    // dialog's focus scope must not pull the focus back.
    await waitFor(() => expect(within(inner).getByRole('radio', { name: 'Filme' })).toHaveFocus());
  });
});
