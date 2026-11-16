import { render, screen, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { describe, expect, it, vi } from 'vitest';
import { LinkRouterProvider } from './AppLink';
import { AreaTabs, type AreaTabsProps } from './AreaTabs';

const props: AreaTabsProps = {
  label: 'Seiten im Bereich Medien',
  tabs: [
    {
      href: '/admin/libraries',
      label: 'Bibliotheken',
      count: '2.318',
      accessibleLabel: 'Bibliotheken, 2.318 Titel',
    },
    { href: '/admin/metadata', label: 'Metadaten', current: true },
    {
      href: '/admin/maintenance',
      label: 'Pflege',
      alarm: { value: '12', tone: 'warning' },
      accessibleLabel: 'Pflege, 12 Titel brauchen Pflege',
    },
  ],
};

const nav = () => screen.getByRole('navigation', { name: 'Seiten im Bereich Medien' });

describe('AreaTabs (build plan 2.5)', () => {
  it('is a navigation of links with the spoken numbers and the current page', () => {
    render(<AreaTabs {...props} />);
    expect(
      within(nav())
        .getAllByRole('link')
        .map((link) => link.getAttribute('aria-label') ?? link.textContent),
    ).toEqual(['Bibliotheken, 2.318 Titel', 'Metadaten', 'Pflege, 12 Titel brauchen Pflege']);
    expect(screen.getByRole('link', { name: 'Metadaten' })).toHaveAttribute('aria-current', 'page');
    expect(screen.getByRole('link', { name: /^Pflege/ })).toHaveTextContent('Pflege12');
    expect(screen.getByRole('link', { name: /^Pflege/ })).toHaveAttribute(
      'href',
      '/admin/maintenance',
    );
  });

  it('is one Tab stop on the current page; the arrows move, wrapping, Home and End', async () => {
    const user = userEvent.setup();
    render(
      <>
        <AreaTabs {...props} />
        <button type="button">Inhalt</button>
      </>,
    );
    await user.tab();
    expect(screen.getByRole('link', { name: 'Metadaten' })).toHaveFocus();
    await user.keyboard('{ArrowRight}');
    expect(screen.getByRole('link', { name: /^Pflege/ })).toHaveFocus();
    await user.keyboard('{ArrowRight}');
    expect(screen.getByRole('link', { name: /^Bibliotheken/ })).toHaveFocus();
    await user.keyboard('{ArrowLeft}');
    expect(screen.getByRole('link', { name: /^Pflege/ })).toHaveFocus();
    await user.keyboard('{Home}');
    expect(screen.getByRole('link', { name: /^Bibliotheken/ })).toHaveFocus();
    await user.keyboard('{End}');
    expect(screen.getByRole('link', { name: /^Pflege/ })).toHaveFocus();
    await user.tab();
    expect(screen.getByRole('button', { name: 'Inhalt' })).toHaveFocus();
    // Leaving put the Tab stop back on the current page.
    await user.tab({ shift: true });
    expect(screen.getByRole('link', { name: 'Metadaten' })).toHaveFocus();
  });

  it('follows the focused tab with Enter', async () => {
    const user = userEvent.setup();
    const navigate = vi.fn();
    render(
      <LinkRouterProvider navigate={navigate}>
        <AreaTabs {...props} />
      </LinkRouterProvider>,
    );
    await user.tab();
    await user.keyboard('{ArrowLeft}{Enter}');
    expect(navigate).toHaveBeenCalledWith('/admin/libraries');
  });

  it('carries the focused tab over to the next page’s copy (focusCarry)', async () => {
    const user = userEvent.setup();
    const focusCarry = { current: null as number | null };
    const { unmount } = render(<AreaTabs {...props} focusCarry={focusCarry} />);
    await user.tab();
    await user.keyboard('{ArrowRight}');
    unmount();
    expect(focusCarry.current).toBe(2);
    render(<AreaTabs {...props} focusCarry={focusCarry} />);
    expect(screen.getByRole('link', { name: /^Pflege/ })).toHaveFocus();
    expect(focusCarry.current).toBeNull();
  });
});
