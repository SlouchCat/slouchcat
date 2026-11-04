import { act, render, screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { useEffect, useState, type ReactNode } from 'react';
import { describe, expect, it, vi } from 'vitest';
import { OptionList, OptionRow } from './OptionRow';
import { PlayerPanel } from './PlayerPanel';

/** A player with a button that opens the panel and a control after it. */
function Player({ children }: { children?: ReactNode }) {
  const [open, setOpen] = useState(false);
  return (
    <div>
      <button type="button" onClick={() => setOpen(true)}>
        Ton öffnen
      </button>
      {open && (
        <PlayerPanel title="Ton" onClose={() => setOpen(false)}>
          {children ?? (
            <OptionList label="Ton" selectedId="en" onSelect={() => undefined}>
              <OptionRow id="de" label="Deutsch" />
              <OptionRow id="en" label="Englisch" />
              <OptionRow id="fr" label="Französisch" />
            </OptionList>
          )}
        </PlayerPanel>
      )}
      <button type="button">Danach</button>
    </div>
  );
}

/** A list whose rows come after the first render, like a react-aria collection in a browser. */
function LateList() {
  const [ready, setReady] = useState(false);
  useEffect(() => {
    const timer = setTimeout(() => setReady(true), 10);
    return () => clearTimeout(timer);
  }, []);
  return (
    <div role="listbox" aria-label="Ton" tabIndex={0}>
      {ready && (
        <>
          <div role="option" aria-selected="false" tabIndex={-1}>
            Deutsch
          </div>
          <div role="option" aria-selected="true" tabIndex={-1}>
            Englisch
          </div>
        </>
      )}
    </div>
  );
}

async function openPanel(children?: ReactNode) {
  const user = userEvent.setup();
  render(<Player>{children}</Player>);
  const opener = screen.getByRole('button', { name: 'Ton öffnen' });
  await user.click(opener);
  const panel = screen.getByRole('dialog', { name: 'Ton' });
  return { user, opener, panel };
}

describe('PlayerPanel (concept 13, 11.2)', () => {
  it('moves the focus to the chosen row when it opens', async () => {
    await openPanel();
    await waitFor(() => expect(screen.getByRole('option', { name: 'Englisch' })).toHaveFocus());
  });

  it('waits for rows that render after the panel and focuses the chosen one', async () => {
    await openPanel(<LateList />);
    expect(screen.getByRole('listbox', { name: 'Ton' })).not.toHaveFocus();
    await waitFor(() => expect(screen.getByRole('option', { name: 'Englisch' })).toHaveFocus());
  });

  it('focuses the first control when nothing is chosen', async () => {
    await openPanel(
      <>
        <button type="button">Qualität</button>
        <button type="button">Geschwindigkeit</button>
      </>,
    );
    await waitFor(() => expect(screen.getByRole('button', { name: 'Qualität' })).toHaveFocus());
  });

  it('closes on Escape, gives the focus back and keeps the key from the page', async () => {
    const outside = vi.fn();
    window.addEventListener('keydown', outside);
    try {
      const { user, opener } = await openPanel();
      await waitFor(() => expect(screen.getByRole('option', { name: 'Englisch' })).toHaveFocus());
      await user.keyboard('{Escape}');
      expect(screen.queryByRole('dialog')).toBeNull();
      expect(opener).toHaveFocus();
      expect(outside).not.toHaveBeenCalled();
    } finally {
      window.removeEventListener('keydown', outside);
    }
  });

  it('closes on Backspace (the remote’s back key), but not while typing in a field', async () => {
    const { user, opener } = await openPanel(
      <>
        <input aria-label="Versatz" />
        <button type="button">Zurücksetzen</button>
      </>,
    );
    const field = screen.getByRole('textbox', { name: 'Versatz' });
    await waitFor(() => expect(field).toHaveFocus());
    await user.keyboard('12{Backspace}');
    expect(field).toHaveValue('1');
    expect(screen.getByRole('dialog')).toBeInTheDocument();
    act(() => screen.getByRole('button', { name: 'Zurücksetzen' }).focus());
    await user.keyboard('{Backspace}');
    expect(screen.queryByRole('dialog')).toBeNull();
    expect(opener).toHaveFocus();
  });

  it('closes with its button and gives the focus back', async () => {
    const { user, opener } = await openPanel();
    await user.click(screen.getByRole('button', { name: 'Schliessen' }));
    expect(screen.queryByRole('dialog')).toBeNull();
    expect(opener).toHaveFocus();
  });

  it('traps nothing: Tab leaves the panel for the next control', async () => {
    const { user } = await openPanel();
    await waitFor(() => expect(screen.getByRole('option', { name: 'Englisch' })).toHaveFocus());
    // The list is one Tab stop; the next one is the control after the panel.
    await user.tab();
    expect(screen.getByRole('button', { name: 'Danach' })).toHaveFocus();
    expect(screen.getByRole('dialog')).toBeInTheDocument();
    await user.tab({ shift: true });
    await user.tab({ shift: true });
    expect(screen.getByRole('button', { name: 'Schliessen' })).toHaveFocus();
  });
});
