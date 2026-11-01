import { render, screen, waitFor, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { describe, expect, it } from 'vitest';
import { AdminSideNavMenu, type AdminSideNavMenuProps } from './AdminSideNavMenu';

const props: AdminSideNavMenuProps = {
  label: 'Verwaltung',
  top: [
    { href: '#home', label: 'Zurück zur App', icon: 'back' },
    { href: '#admin', label: 'Übersicht', icon: 'grid' },
  ],
  groups: [
    {
      label: 'Bibliothek',
      icon: 'film',
      entries: [
        { href: '#libraries', label: 'Bibliotheken', icon: 'folder', current: true },
        { href: '#maintenance', label: 'Pflege', icon: 'wrench' },
      ],
    },
  ],
  buttonLabel: 'Verwaltung',
  pageLabel: 'Bibliotheken',
  menuLabel: 'Seiten der Verwaltung',
  closeLabel: 'Seiten der Verwaltung schliessen',
};

// While the sheet is open react-aria hides the rest of the page from the accessibility tree.
const trigger = () =>
  screen.getByRole('button', { name: 'Verwaltung · Bibliotheken', hidden: true });

describe('admin sheet on a phone (concept 11, 11.1)', () => {
  it('opens on the current page, traps the focus, and Escape returns it to the button', async () => {
    const user = userEvent.setup();
    render(
      <>
        <AdminSideNavMenu {...props} />
        <button type="button">Inhalt</button>
      </>,
    );
    expect(screen.queryByRole('dialog')).not.toBeInTheDocument();
    expect(trigger()).toHaveAttribute('aria-expanded', 'false');

    await user.tab();
    expect(trigger()).toHaveFocus();
    await user.keyboard('{Enter}');
    const dialog = await screen.findByRole('dialog', { name: 'Seiten der Verwaltung' });
    expect(trigger()).toHaveAttribute('aria-expanded', 'true');
    expect(trigger()).toHaveAttribute('aria-controls', dialog.id);
    const current = within(dialog).getByRole('link', { name: 'Bibliotheken' });
    expect(current).toHaveFocus();
    expect(current).toHaveAttribute('aria-current', 'page');

    // The same keyboard model as the column; Tab stays inside the sheet.
    await user.keyboard('{ArrowDown}');
    expect(within(dialog).getByRole('link', { name: 'Pflege' })).toHaveFocus();
    const close = within(dialog).getByRole('button', { name: 'Seiten der Verwaltung schliessen' });
    await user.tab();
    expect(close).toHaveFocus();
    await user.tab();
    expect(dialog).toContainElement(document.activeElement as HTMLElement);
    expect(screen.getByRole('button', { name: 'Inhalt', hidden: true })).not.toHaveFocus();

    await user.keyboard('{Escape}');
    await waitFor(() => expect(screen.queryByRole('dialog')).not.toBeInTheDocument());
    expect(trigger()).toHaveFocus();
  });

  it('closes with its close button and after following an entry, focus back on the button', async () => {
    const user = userEvent.setup();
    render(<AdminSideNavMenu {...props} />);

    await user.click(trigger());
    const dialog = await screen.findByRole('dialog');
    await user.click(within(dialog).getByRole('button', { name: /schliessen$/ }));
    await waitFor(() => expect(screen.queryByRole('dialog')).not.toBeInTheDocument());
    expect(trigger()).toHaveFocus();

    await user.click(trigger());
    await user.click(
      within(await screen.findByRole('dialog')).getByRole('link', { name: 'Pflege' }),
    );
    await waitFor(() => expect(screen.queryByRole('dialog')).not.toBeInTheDocument());
    expect(trigger()).toHaveFocus();
  });
});
