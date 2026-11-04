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

/** The gear panel with a page for the quality: a level down and back up. */
function GearPlayer() {
  const [open, setOpen] = useState(false);
  const [page, setPage] = useState<'main' | 'quality'>('main');
  return (
    <div>
      <button type="button" onClick={() => setOpen(true)}>
        Einstellungen öffnen
      </button>
      {open && (
        <PlayerPanel
          title={page === 'main' ? 'Einstellungen' : 'Qualität'}
          onClose={() => setOpen(false)}
          onBack={page === 'quality' ? () => setPage('main') : undefined}
        >
          {page === 'main' ? (
            <>
              <button type="button" onClick={() => setPage('quality')}>
                Qualität
              </button>
              <button type="button" role="switch" aria-checked="true">
                Nachtmodus
              </button>
            </>
          ) : (
            <OptionList label="Qualität" selectedId="1080" onSelect={() => undefined}>
              <OptionRow id="auto" label="Automatisch" />
              <OptionRow id="1080" label="1080p" />
            </OptionList>
          )}
        </PlayerPanel>
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

  it('keeps the arrow keys in the panel, also at the end of the list', async () => {
    const outside = vi.fn();
    window.addEventListener('keydown', outside);
    try {
      const { user } = await openPanel();
      await waitFor(() => expect(screen.getByRole('option', { name: 'Englisch' })).toHaveFocus());
      // Down to the last row, then once more: the list stops there, the player sees nothing.
      await user.keyboard('{ArrowDown}{ArrowDown}');
      expect(screen.getByRole('option', { name: 'Französisch' })).toHaveFocus();
      await user.keyboard('{ArrowLeft}{ArrowRight}{Home}{End}{PageUp}{PageDown}');
      expect(outside).not.toHaveBeenCalled();
      // With Shift too: Shift+← would jump a chapter in the player. Shift alone may pass.
      await user.keyboard('{Shift>}{ArrowLeft}{ArrowRight}{ArrowUp}{ArrowDown}{/Shift}');
      const keys = outside.mock.calls.map(([event]) => (event as KeyboardEvent).key);
      expect(keys.filter((name) => name !== 'Shift')).toEqual([]);
    } finally {
      window.removeEventListener('keydown', outside);
    }
  });

  it('lets the player’s letter keys through', async () => {
    const outside = vi.fn();
    window.addEventListener('keydown', outside);
    try {
      const { user } = await openPanel();
      await waitFor(() => expect(screen.getByRole('option', { name: 'Englisch' })).toHaveFocus());
      await user.keyboard('m');
      expect(outside).toHaveBeenCalledWith(expect.objectContaining({ key: 'm' }));
    } finally {
      window.removeEventListener('keydown', outside);
    }
  });

  it('gives the focus back when the page closes the panel while the focus is in it', async () => {
    function ChoosingPlayer() {
      const [open, setOpen] = useState(false);
      return (
        <div>
          <button type="button" onClick={() => setOpen(true)}>
            Ton öffnen
          </button>
          {open && (
            <PlayerPanel title="Ton" onClose={() => setOpen(false)}>
              <OptionList label="Ton" selectedId="en" onSelect={() => setOpen(false)}>
                <OptionRow id="de" label="Deutsch" />
                <OptionRow id="en" label="Englisch" />
              </OptionList>
            </PlayerPanel>
          )}
        </div>
      );
    }
    const user = userEvent.setup();
    render(<ChoosingPlayer />);
    const opener = screen.getByRole('button', { name: 'Ton öffnen' });
    await user.click(opener);
    await waitFor(() => expect(screen.getByRole('option', { name: 'Englisch' })).toHaveFocus());
    await user.keyboard('{ArrowUp}{Enter}');
    expect(screen.queryByRole('dialog')).toBeNull();
    await waitFor(() => expect(opener).toHaveFocus());
  });

  it('a page inside the panel: back button and Backspace go up, Escape closes', async () => {
    const user = userEvent.setup();
    render(<GearPlayer />);
    const opener = screen.getByRole('button', { name: 'Einstellungen öffnen' });
    await user.click(opener);
    // A checked switch is no chosen row: the first control gets the focus.
    const quality = () => screen.getByRole('button', { name: 'Qualität' });
    await waitFor(() => expect(quality()).toHaveFocus());
    expect(screen.queryByRole('button', { name: 'Zurück' })).toBeNull();

    await user.keyboard('{Enter}');
    expect(screen.getByRole('dialog', { name: 'Qualität' })).toBeInTheDocument();
    act(() => screen.getByRole('option', { name: '1080p' }).focus());
    // Backspace goes up a level, and the focus lands on the new level.
    await user.keyboard('{Backspace}');
    expect(screen.getByRole('dialog', { name: 'Einstellungen' })).toBeInTheDocument();
    await waitFor(() => expect(quality()).toHaveFocus());

    // The back button does the same; the chosen row of the page has the focus first.
    await user.keyboard('{Enter}');
    await waitFor(() => expect(screen.getByRole('option', { name: '1080p' })).toHaveFocus());
    await user.click(screen.getByRole('button', { name: 'Zurück' }));
    await waitFor(() => expect(quality()).toHaveFocus());

    // Escape closes the whole panel, from any level.
    await user.keyboard('{Enter}');
    await waitFor(() => expect(screen.getByRole('option', { name: '1080p' })).toHaveFocus());
    await user.keyboard('{Escape}');
    expect(screen.queryByRole('dialog')).toBeNull();
    expect(opener).toHaveFocus();
  });
});
