import { render, screen, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { describe, expect, it, vi } from 'vitest';
import { SideNav, SideNavMenu, type SideNavProps } from './SideNav';

const props = (overrides: Partial<SideNavProps> = {}): SideNavProps => ({
  label: 'Hauptnavigation',
  profile: {
    name: 'Marco',
    role: 'Hauptprofil',
    href: '/profiles',
    switchLabel: 'Wechseln',
    switchAccessibleLabel: 'Profil wechseln, jetzt Marco',
  },
  primary: [
    { href: '/', label: 'Start', icon: 'home', current: true },
    { href: '/search', label: 'Suche', icon: 'search' },
  ],
  librariesLabel: 'Meine Medien',
  libraries: [
    {
      href: '/libraries/1',
      label: 'Filme',
      icon: 'film',
      count: 12,
      accessibleLabel: 'Filme, 12 Titel',
    },
    {
      href: '/libraries/2',
      label: 'Serien',
      icon: 'tv',
      count: 3,
      accessibleLabel: 'Serien, 3 Titel',
    },
  ],
  picks: { href: '/picks', label: 'User-Empfehlungen', icon: 'star', count: 4 },
  librariesOffline: 'Offline: Bibliotheken nicht verfügbar',
  offlineLink: { href: '/downloads', label: 'Zu den Downloads' },
  accountLabel: 'Konto',
  account: [
    { href: '/settings/playback', label: 'Meine Wiedergabe', icon: 'sliders' },
    { href: '/settings/profiles', label: 'Profile', icon: 'users' },
  ],
  logoutLabel: 'Abmelden',
  onLogout: () => undefined,
  hideLabel: 'Seitenleiste ausblenden',
  onHide: () => undefined,
  ...overrides,
});

const nav = () => screen.getByRole('navigation', { name: 'Hauptnavigation' });

describe('side navigation (concepts 08 and 17)', () => {
  it('lists the entries in order, the libraries with their counts, the current page marked', () => {
    render(<SideNav {...props()} />);
    const names = within(nav())
      .getAllByRole('link')
      .map((link) => link.getAttribute('aria-label') ?? link.textContent);
    expect(names).toEqual([
      'Profil wechseln, jetzt Marco',
      'Start',
      'Suche',
      'Filme, 12 Titel',
      'Serien, 3 Titel',
      'User-Empfehlungen4',
      'Meine Wiedergabe',
      'Profile',
    ]);
    expect(screen.getByRole('link', { name: 'Start' })).toHaveAttribute('aria-current', 'page');
    expect(screen.getByRole('button', { name: 'Abmelden' })).toBeVisible();
    expect(screen.getByRole('button', { name: 'Seitenleiste ausblenden' })).toBeVisible();
    // Every entry has its icon, hidden from screen readers.
    for (const link of within(nav()).getAllByRole('link').slice(1)) {
      expect(link.querySelector('svg[aria-hidden="true"]')).not.toBeNull();
    }
  });

  it('shows the active profile on top, with its role and the way to the profile picker', () => {
    render(<SideNav {...props()} />);
    expect(within(nav()).getByText('Marco')).toBeVisible();
    expect(within(nav()).getByText('Hauptprofil')).toBeVisible();
    const change = screen.getByRole('link', { name: 'Profil wechseln, jetzt Marco' });
    expect(change).toHaveAttribute('href', '/profiles');
    expect(change).toHaveTextContent('Wechseln');
  });

  it('names both lists by their bands, «Meine Medien» always open, without a toggle', () => {
    render(<SideNav {...props()} />);
    const media = within(nav()).getByRole('list', { name: 'Meine Medien' });
    expect(
      within(media)
        .getAllByRole('link')
        .map((link) => link.textContent),
    ).toEqual(['Filme12', 'Serien3', 'User-Empfehlungen4']);
    const account = within(nav()).getByRole('list', { name: 'Konto' });
    expect(
      within(account)
        .getAllByRole('link')
        .map((link) => link.textContent),
    ).toEqual(['Meine Wiedergabe', 'Profile']);
    expect(within(nav()).queryByRole('button', { name: /Meine Medien/ })).not.toBeInTheDocument();
    expect(nav().querySelector('[aria-expanded]')).toBeNull();
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
      within(dialog).getByRole('link', { name: 'Profil wechseln, jetzt Marco' }),
    ).toHaveAttribute('href', '/profiles');
    expect(
      within(dialog).queryByRole('button', { name: 'Seitenleiste ausblenden' }),
    ).not.toBeInTheDocument();
    await user.click(within(dialog).getByRole('button', { name: 'Seitenleiste einblenden' }));
    expect(onShow).toHaveBeenCalledOnce();
    await user.keyboard('{Escape}');
    expect(onClose).toHaveBeenCalled();
  });
});
