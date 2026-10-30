import { render, screen, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { SideNav, SideNavMenu, type SideNavProps } from './SideNav';

const props = (overrides: Partial<SideNavProps> = {}): SideNavProps => ({
  label: 'Hauptnavigation',
  primary: [
    { href: '/', label: 'Start', current: true },
    { href: '/search', label: 'Suche' },
  ],
  librariesLabel: 'Meine Medien',
  toggleLibrariesLabel: 'Meine Medien ein- oder ausklappen',
  libraries: [
    { href: '/libraries/1', label: 'Filme', count: 12, accessibleLabel: 'Filme, 12 Titel' },
    { href: '/libraries/2', label: 'Serien', count: 3, accessibleLabel: 'Serien, 3 Titel' },
  ],
  picks: { href: '/picks', label: "Cat's Empfehlungen", count: 4 },
  librariesOffline: 'Offline: Bibliotheken nicht verfügbar',
  offlineLink: { href: '/downloads', label: 'Zu den Downloads' },
  footer: [{ href: '/settings/profiles', label: 'Profile' }],
  logoutLabel: 'Abmelden',
  onLogout: () => undefined,
  hideLabel: 'Seitenleiste ausblenden',
  onHide: () => undefined,
  ...overrides,
});

const nav = () => screen.getByRole('navigation', { name: 'Hauptnavigation' });

afterEach(() => localStorage.clear());

describe('side navigation (concept 08)', () => {
  it('lists the entries in order, the libraries with their counts, the current page marked', () => {
    render(<SideNav {...props()} />);
    const names = within(nav())
      .getAllByRole('link')
      .map((link) => link.getAttribute('aria-label') ?? link.textContent);
    expect(names).toEqual([
      'Start',
      'Suche',
      'Filme, 12 Titel',
      'Serien, 3 Titel',
      "Cat's Empfehlungen4",
      'Profile',
    ]);
    expect(screen.getByRole('link', { name: 'Start' })).toHaveAttribute('aria-current', 'page');
    expect(screen.getByRole('button', { name: 'Abmelden' })).toBeVisible();
    expect(screen.getByRole('button', { name: 'Seitenleiste ausblenden' })).toBeVisible();
  });

  it('collapses "Meine Medien" and keeps that on this device', async () => {
    const user = userEvent.setup();
    const { unmount } = render(<SideNav {...props()} />);
    const toggle = screen.getByRole('button', { name: 'Meine Medien ein- oder ausklappen' });
    expect(toggle).toHaveAttribute('aria-expanded', 'true');
    expect(document.getElementById(toggle.getAttribute('aria-controls') ?? '')).not.toBeNull();
    await user.click(toggle);
    expect(toggle).toHaveAttribute('aria-expanded', 'false');
    expect(screen.queryByRole('link', { name: /^Filme/ })).not.toBeInTheDocument();
    expect(localStorage.getItem('slouchcat.sidenav.libraries')).toBe('closed');

    unmount();
    render(<SideNav {...props()} />);
    expect(
      screen.getByRole('button', { name: 'Meine Medien ein- oder ausklappen' }),
    ).toHaveAttribute('aria-expanded', 'false');
  });

  it('says so when the libraries cannot be read, and the rest stands', () => {
    render(<SideNav {...props({ libraries: 'offline', picks: undefined })} />);
    expect(screen.getByText('Offline: Bibliotheken nicht verfügbar')).toBeVisible();
    expect(screen.getByRole('link', { name: 'Zu den Downloads' })).toHaveAttribute(
      'href',
      '/downloads',
    );
    expect(screen.getByRole('link', { name: 'Start' })).toBeVisible();
    expect(screen.getByRole('link', { name: 'Profile' })).toBeVisible();
  });

  it('hides the column and signs out with its buttons', async () => {
    const user = userEvent.setup();
    const onHide = vi.fn();
    const onLogout = vi.fn();
    render(<SideNav {...props({ onHide, onLogout })} />);
    await user.click(screen.getByRole('button', { name: 'Seitenleiste ausblenden' }));
    expect(onHide).toHaveBeenCalledOnce();
    await user.click(screen.getByRole('button', { name: 'Abmelden' }));
    expect(onLogout).toHaveBeenCalledOnce();
  });

  it('the drawer opens on its first entry, offers "einblenden" and closes with Escape', async () => {
    const user = userEvent.setup();
    const onClose = vi.fn();
    const onShow = vi.fn();
    render(
      <SideNavMenu
        {...props()}
        isOpen
        onClose={onClose}
        id="app-menu"
        menuLabel="Menü"
        closeLabel="Menü schliessen"
        show={{ label: 'Seitenleiste einblenden', onPress: onShow }}
      />,
    );
    const dialog = await screen.findByRole('dialog', { name: 'Menü' });
    expect(dialog).toHaveAttribute('id', 'app-menu');
    expect(within(dialog).getByRole('link', { name: 'Start' })).toHaveFocus();
    expect(
      within(dialog).queryByRole('button', { name: 'Seitenleiste ausblenden' }),
    ).not.toBeInTheDocument();
    await user.click(within(dialog).getByRole('button', { name: 'Seitenleiste einblenden' }));
    expect(onShow).toHaveBeenCalledOnce();
    await user.keyboard('{Escape}');
    expect(onClose).toHaveBeenCalled();
  });
});
