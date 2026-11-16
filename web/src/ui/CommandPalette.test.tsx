import { render, screen, waitFor, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { useState } from 'react';
import { describe, expect, it, vi } from 'vitest';
import { LinkRouterProvider } from './AppLink';
import {
  CommandPalette,
  filterCommandItems,
  type CommandPaletteItem,
  type CommandPaletteProps,
} from './CommandPalette';

const items: CommandPaletteItem[] = [
  { id: 'overview', title: 'Übersicht', icon: 'grid', href: '/admin' },
  { id: 'media', title: 'Medien', detail: 'Bereich', icon: 'folder', href: '/admin/libraries' },
  {
    id: 'libraries',
    title: 'Bibliotheken',
    detail: 'Medien',
    icon: 'folder',
    href: '/admin/libraries',
  },
  {
    id: 'jobs',
    title: 'Hintergrundjobs',
    detail: 'Automatisierung',
    icon: 'refresh',
    href: '/admin/jobs',
  },
  { id: 'search', title: 'Mediensuche', icon: 'search', href: '/search' },
];

const texts: Omit<CommandPaletteProps, 'isOpen' | 'onOpenChange' | 'items'> = {
  label: 'Bereich der Verwaltung suchen',
  placeholder: 'Bibliotheken, Jobs …',
  resultsLabel: 'Treffer',
  emptyLabel: 'Nichts gefunden',
  hint: 'Pfeiltasten wählen, Enter öffnet, Esc schliesst',
};

/** A button that opens the palette, as the header's «Bereich suchen». */
function Harness({ navigate }: { navigate: (href: string) => void }) {
  const [open, setOpen] = useState(false);
  return (
    <LinkRouterProvider navigate={navigate}>
      <button type="button" onClick={() => setOpen(true)}>
        Bereich suchen
      </button>
      <CommandPalette {...texts} items={items} isOpen={open} onOpenChange={setOpen} />
    </LinkRouterProvider>
  );
}

const field = () => screen.getByRole('combobox', { name: 'Bereich der Verwaltung suchen' });
const options = () =>
  within(screen.getByRole('listbox', { name: 'Treffer' }))
    .queryAllByRole('option')
    .map((option) => option.textContent);

describe('filterCommandItems', () => {
  it('matches every typed word in title or detail, ignoring case', () => {
    expect(filterCommandItems(items, 'MEDIEN').map((item) => item.id)).toEqual([
      'media',
      'libraries',
      'search',
    ]);
    expect(filterCommandItems(items, 'bib med').map((item) => item.id)).toEqual(['libraries']);
    expect(filterCommandItems(items, '  ')).toHaveLength(items.length);
  });
});

describe('CommandPalette (build plan 2.4)', () => {
  it('opens with the focus in the field; typing filters, the arrows choose, Enter follows', async () => {
    const user = userEvent.setup();
    const navigate = vi.fn();
    render(<Harness navigate={navigate} />);
    await user.click(screen.getByRole('button', { name: 'Bereich suchen' }));
    await screen.findByRole('dialog', { name: 'Bereich der Verwaltung suchen' });
    await waitFor(() => expect(field()).toHaveFocus());
    expect(options()).toHaveLength(items.length);

    await user.keyboard('job');
    expect(options()).toEqual(['HintergrundjobsAutomatisierung']);
    await user.clear(field());
    await user.keyboard('medien');
    const chosen = () => screen.getByRole('option', { selected: true });
    expect(chosen()).toHaveTextContent('Medien');
    expect(field()).toHaveAttribute('aria-activedescendant', chosen().id);
    await user.keyboard('{ArrowDown}');
    expect(chosen()).toHaveTextContent('Bibliotheken');
    await user.keyboard('{ArrowDown}{ArrowDown}');
    // Wrapping from the last back to the first.
    expect(chosen()).toHaveTextContent('Medien');
    await user.keyboard('{ArrowUp}');
    expect(chosen()).toHaveTextContent('Mediensuche');
    await user.keyboard('{Enter}');
    expect(navigate).toHaveBeenCalledWith('/search');
    await waitFor(() => expect(screen.queryByRole('dialog')).not.toBeInTheDocument());
  });

  it('says so when nothing matches', async () => {
    const user = userEvent.setup();
    render(<Harness navigate={vi.fn()} />);
    await user.click(screen.getByRole('button', { name: 'Bereich suchen' }));
    await waitFor(() => expect(field()).toHaveFocus());
    await user.keyboard('xyz');
    expect(options()).toEqual([]);
    expect(screen.getByRole('status')).toHaveTextContent('Nichts gefunden');
    expect(field()).not.toHaveAttribute('aria-activedescendant');
  });

  it('traps the focus, closes with Escape and returns the focus to the button', async () => {
    const user = userEvent.setup();
    render(<Harness navigate={vi.fn()} />);
    const button = screen.getByRole('button', { name: 'Bereich suchen' });
    await user.click(button);
    await waitFor(() => expect(field()).toHaveFocus());
    await user.tab();
    expect(screen.getByRole('dialog')).toContainElement(document.activeElement as HTMLElement);
    await user.keyboard('{Escape}');
    await waitFor(() => expect(screen.queryByRole('dialog')).not.toBeInTheDocument());
    expect(screen.getByRole('button', { name: 'Bereich suchen' })).toHaveFocus();
  });

  it('follows a clicked destination', async () => {
    const user = userEvent.setup();
    const navigate = vi.fn();
    render(<Harness navigate={navigate} />);
    await user.click(screen.getByRole('button', { name: 'Bereich suchen' }));
    await user.click(await screen.findByRole('option', { name: /^Hintergrundjobs/ }));
    expect(navigate).toHaveBeenCalledWith('/admin/jobs');
  });
});
