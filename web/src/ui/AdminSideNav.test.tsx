/// <reference types="node" />
import { readFileSync } from 'node:fs';
import { join } from 'node:path';
import { accentNames, tokens, type AccentName } from '@slouchcat/design-tokens';
import { render, screen, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { describe, expect, it, vi } from 'vitest';
import { AdminSideNav, type AdminSideNavEntry, type AdminSideNavProps } from './AdminSideNav';
import { LinkRouterProvider } from './AppLink';

/** The column (build plan 2.2): back, the heading «Verwaltung» with the overview and the areas. */
const props = (overrides: Partial<AdminSideNavProps> = {}): AdminSideNavProps => ({
  label: 'Verwaltung',
  top: [{ href: '/', label: 'Zurück zur App', icon: 'back' }],
  heading: 'Verwaltung',
  entries: [
    { href: '/admin', label: 'Übersicht', icon: 'grid' },
    {
      href: '/admin/libraries',
      label: 'Medien',
      icon: 'folder',
      current: true,
      alarm: { value: '12', tone: 'warning' },
      accessibleLabel: 'Medien, 12 Titel brauchen Pflege',
    },
    { href: '/admin/jobs', label: 'Automatisierung', icon: 'refresh' },
    { href: '/admin/stats', label: 'System', icon: 'server' },
  ],
  profile: { name: 'Marco', role: 'Server-Admin' },
  ...overrides,
});

/** The sheet's shape: the areas as bands with their pages. */
const grouped: Partial<AdminSideNavProps> = {
  top: [
    { href: '/', label: 'Zurück zur App', icon: 'back' },
    { href: '/admin', label: 'Übersicht', icon: 'grid' },
  ],
  heading: undefined,
  entries: undefined,
  groups: [
    {
      label: 'Medien',
      icon: 'folder',
      entries: [
        {
          href: '/admin/libraries',
          label: 'Bibliotheken',
          icon: 'folder',
          count: '2.318',
          accessibleLabel: 'Bibliotheken, 2.318 Titel',
        },
        {
          href: '/admin/maintenance',
          label: 'Pflege',
          icon: 'wrench',
          current: true,
          alarm: { value: '12', tone: 'warning' },
          accessibleLabel: 'Pflege, 12 Titel brauchen Pflege',
        },
      ],
    },
    {
      label: 'System',
      icon: 'server',
      entries: [
        { href: '/admin/stats', label: 'Statistiken', icon: 'chart' },
        { href: '/admin/backup', label: 'Sicherung', icon: 'archive' },
      ],
    },
  ],
};

const nav = () => screen.getByRole('navigation', { name: 'Verwaltung' });
const links = () => within(nav()).getAllByRole('link');

describe('admin sidebar (concept 11, 11.6; build plan 2.2)', () => {
  it('lists back, then the overview and the areas under the heading, without headings', () => {
    render(<AdminSideNav {...props()} />);
    expect(within(nav()).queryAllByRole('heading')).toEqual([]);
    const main = within(nav()).getByRole('list', { name: 'Verwaltung' });
    const heading = document.getElementById(main.getAttribute('aria-labelledby') ?? '');
    expect(heading?.tagName).toBe('SPAN');
    expect(
      within(main)
        .getAllByRole('link')
        .map((link) => link.textContent),
    ).toEqual(['Übersicht', 'Medien12', 'Automatisierung', 'System']);
    expect(links().map((link) => link.getAttribute('aria-label') ?? link.textContent)).toEqual([
      'Zurück zur App',
      'Übersicht',
      'Medien, 12 Titel brauchen Pflege',
      'Automatisierung',
      'System',
    ]);
    expect(screen.getByRole('link', { name: 'Medien, 12 Titel brauchen Pflege' })).toHaveAttribute(
      'aria-current',
      'page',
    );
    expect(links().filter((link) => link.hasAttribute('aria-current'))).toHaveLength(1);
  });

  it('shows the active profile with its role at the foot, outside the navigation', () => {
    render(<AdminSideNav {...props()} />);
    expect(screen.getByText('Marco')).toBeVisible();
    expect(screen.getByText('Server-Admin')).toBeVisible();
    expect(nav()).not.toContainElement(screen.getByText('Marco'));
  });

  it('names the groups of the sheet by their bands and marks the current page', () => {
    render(<AdminSideNav {...props(grouped)} />);
    expect(within(nav()).queryAllByRole('heading')).toEqual([]);
    const media = within(nav()).getByRole('list', { name: 'Medien' });
    const band = document.getElementById(media.getAttribute('aria-labelledby') ?? '');
    expect(band?.tagName).toBe('SPAN');
    expect(
      within(media)
        .getAllByRole('link')
        .map((link) => link.textContent),
    ).toEqual(['Bibliotheken2.318', 'Pflege12']);
    expect(within(nav()).getByRole('list', { name: 'System' })).toBeVisible();
    expect(screen.getByRole('link', { name: 'Pflege, 12 Titel brauchen Pflege' })).toHaveAttribute(
      'aria-current',
      'page',
    );
  });

  it('is a single Tab stop on the current entry; Tab leaves the column', async () => {
    const user = userEvent.setup();
    render(
      <>
        <AdminSideNav {...props()} />
        <button type="button">Inhalt</button>
      </>,
    );
    expect(links().filter((link) => link.tabIndex === 0)).toEqual([
      screen.getByRole('link', { name: /^Medien/ }),
    ]);
    await user.tab();
    expect(screen.getByRole('link', { name: /^Medien/ })).toHaveFocus();
    await user.tab();
    expect(screen.getByRole('button', { name: 'Inhalt' })).toHaveFocus();
    await user.tab({ shift: true });
    expect(screen.getByRole('link', { name: /^Medien/ })).toHaveFocus();
  });

  it('moves with ArrowDown and ArrowUp across the heading, wrapping, and with Home and End', async () => {
    const user = userEvent.setup();
    render(<AdminSideNav {...props()} />);
    await user.tab();
    await user.keyboard('{ArrowDown}');
    expect(screen.getByRole('link', { name: 'Automatisierung' })).toHaveFocus();
    await user.keyboard('{ArrowUp}{ArrowUp}{ArrowUp}');
    expect(screen.getByRole('link', { name: 'Zurück zur App' })).toHaveFocus();
    await user.keyboard('{ArrowUp}');
    expect(screen.getByRole('link', { name: 'System' })).toHaveFocus();
    await user.keyboard('{ArrowDown}');
    expect(screen.getByRole('link', { name: 'Zurück zur App' })).toHaveFocus();
    await user.keyboard('{End}');
    expect(screen.getByRole('link', { name: 'System' })).toHaveFocus();
    await user.keyboard('{Home}');
    expect(screen.getByRole('link', { name: 'Zurück zur App' })).toHaveFocus();
    // The moved focus carries the Tab stop; leaving the column puts it back on the current page.
    expect(links().filter((link) => link.tabIndex === 0)).toHaveLength(1);
    await user.tab();
    expect(screen.getByRole('link', { name: /^Medien/ })).toHaveAttribute('tabindex', '0');
  });

  it('moves across the bands of the sheet in the same way', async () => {
    const user = userEvent.setup();
    render(<AdminSideNav {...props(grouped)} />);
    await user.tab();
    expect(screen.getByRole('link', { name: /^Pflege/ })).toHaveFocus();
    await user.keyboard('{ArrowDown}');
    expect(screen.getByRole('link', { name: 'Statistiken' })).toHaveFocus();
    await user.keyboard('{ArrowUp}{ArrowUp}{ArrowUp}');
    expect(screen.getByRole('link', { name: 'Übersicht' })).toHaveFocus();
  });

  it('follows the focused link with Enter', async () => {
    const user = userEvent.setup();
    const navigate = vi.fn();
    render(
      <LinkRouterProvider navigate={navigate}>
        <AdminSideNav {...props()} />
      </LinkRouterProvider>,
    );
    await user.tab();
    await user.keyboard('{ArrowDown}{Enter}');
    expect(navigate).toHaveBeenCalledWith('/admin/jobs');
  });

  it('carries the focused entry over to the next page’s copy (focusCarry)', async () => {
    const user = userEvent.setup();
    const focusCarry = { current: null as number | null };
    const { unmount } = render(<AdminSideNav {...props({ focusCarry })} />);
    await user.tab();
    await user.keyboard('{ArrowDown}');
    unmount();
    expect(focusCarry.current).toBe(3);
    render(<AdminSideNav {...props({ focusCarry })} />);
    expect(screen.getByRole('link', { name: 'Automatisierung' })).toHaveFocus();
    expect(focusCarry.current).toBeNull();
  });

  it('scrolls the current entry into view in the column', () => {
    const scroll = vi.spyOn(Element.prototype, 'scrollIntoView');
    render(<AdminSideNav {...props()} />);
    expect(scroll).toHaveBeenCalledOnce();
    expect(scroll.mock.contexts[0]).toBe(screen.getByRole('link', { name: /^Medien/ }));
    expect(scroll).toHaveBeenCalledWith({ block: 'nearest' });
    scroll.mockRestore();
  });

  it('requires a spoken name for a number, pill or badge (type check)', () => {
    const entry = (value: AdminSideNavEntry) => value;
    // @ts-expect-error A count without accessibleLabel would vanish for screen readers.
    entry({ href: '/a', label: 'A', icon: 'grid', count: '3' });
    // @ts-expect-error The same for an alarm pill.
    entry({ href: '/a', label: 'A', icon: 'grid', alarm: { value: '3', tone: 'danger' } });
    expect(entry({ href: '/a', label: 'A', icon: 'grid' }).label).toBe('A');
  });

  it('puts the Tab stop on the first entry when no page is current', () => {
    render(
      <AdminSideNav
        {...props({ entries: [{ href: '/admin', label: 'Übersicht', icon: 'grid' }] })}
      />,
    );
    expect(screen.getByRole('link', { name: 'Zurück zur App' })).toHaveAttribute('tabindex', '0');
    expect(screen.getByRole('link', { name: 'Übersicht' })).toHaveAttribute('tabindex', '-1');
  });
});

// WCAG 2.x relative luminance and contrast ratio (as in packages/design-tokens/test).
function channels(hex: string): [number, number, number] {
  const match = /^#([0-9a-f]{2})([0-9a-f]{2})([0-9a-f]{2})$/i.exec(hex);
  if (!match) throw new Error(`expected an opaque #rrggbb colour, got ${hex}`);
  return match.slice(1).map((part) => parseInt(part, 16)) as [number, number, number];
}

function luminance(rgb: readonly number[]): number {
  const [r = 0, g = 0, b = 0] = rgb.map((value) => {
    const channel = value / 255;
    return channel <= 0.04045 ? channel / 12.92 : ((channel + 0.055) / 1.055) ** 2.4;
  });
  return 0.2126 * r + 0.7152 * g + 0.0722 * b;
}

function contrast(foreground: readonly number[], background: readonly number[]): number {
  const [light = 0, dark = 0] = [luminance(foreground), luminance(background)].sort(
    (a, b) => b - a,
  );
  return (light + 0.05) / (dark + 0.05);
}

/** `color-mix(in srgb, accent p%, transparent)` painted over `ground`. */
const tint = (accent: readonly number[], percent: number, ground: readonly number[]) =>
  accent.map((value, index) => (value * percent + (ground[index] ?? 0) * (100 - percent)) / 100);

// Vitest hands CSS modules over as class names, so the stylesheet is read from disk.
const css = readFileSync(join(import.meta.dirname, 'AdminSideNav.module.css'), 'utf8');

/** A tint as the stylesheet defines it: `<variable>: color-mix(in srgb, var(<tone>) p%, …)`. */
const tintOf = (variable: string, tone: string) =>
  Number(
    new RegExp(
      `${variable}:\\s*color-mix\\(in srgb, var\\(${tone}\\) (\\d+)%, transparent\\)`,
    ).exec(css)?.[1],
  );

const BAND_TINT = tintOf('--sc-band-tint', '--sc-accent');
const PILL_TINT = tintOf('--sc-pill-tint', '--sc-pill-tone');
const CURRENT_TINT = tintOf('--sc-current-tint', '--sc-accent');
const AVATAR_TINT = tintOf('--sc-avatar-tint', '--sc-accent');

/**
 * Accents whose own colour is too weak on its band: the band would take text-primary for them.
 * None today (lowest: ember, 5.04:1 at 16 %). Adding an accent here is not enough on its own: CSS
 * only sees `--sc-accent`, not its name, so the band also needs a hook for it (e.g. a
 * `data-accent` attribute set by the session); otherwise the UI does not change.
 */
const PRIMARY_TEXT: readonly AccentName[] = [];

const MIN_RATIO = 4.5;
/** Non-text contrast (WCAG 1.4.11): the current entry's icon and arrow in the accent. */
const MIN_ICON_RATIO = 3;

const ground = channels(tokens.color.bg.elevated);

/** Text in `tone` on `tone` at `percent` over the column (bg-elevated). */
const cases: [string, readonly number[], readonly number[], number][] = [
  ...accentNames.map((name): [string, number[], number[], number] => {
    const accent = channels(tokens.color.accent[name].base);
    const text = PRIMARY_TEXT.includes(name) ? channels(tokens.color.text.primary) : accent;
    return [`band ${name}`, text, accent, BAND_TINT];
  }),
  ...accentNames.map((name): [string, number[], number[], number] => {
    const accent = channels(tokens.color.accent[name].base);
    return [`accent pill ${name}`, accent, accent, PILL_TINT];
  }),
  ...(['warning', 'danger'] as const).map((tone): [string, number[], number[], number] => {
    const colour = channels(tokens.color.state[tone]);
    return [`${tone} pill`, colour, colour, PILL_TINT];
  }),
];

/** The current entry (build plan 2.2): primary text on the accent's tint over the column. */
const currentCases: [string, readonly number[], readonly number[], number][] = accentNames.map(
  (name): [string, number[], number[], number] => [
    `current entry ${name}`,
    channels(tokens.color.text.primary),
    channels(tokens.color.accent[name].base),
    CURRENT_TINT,
  ],
);

/** The initial of the profile at the foot: the accent on its tint. */
const avatarCases: [string, readonly number[], readonly number[], number][] = accentNames.map(
  (name): [string, number[], number[], number] => {
    const accent = channels(tokens.color.accent[name].base);
    return [`profile initial ${name}`, accent, accent, AVATAR_TINT];
  },
);

describe('band, pill and current entry contrast for every profile accent (F-9, F-7)', () => {
  it('reads the tints from the stylesheet', () => {
    expect(BAND_TINT).toBe(16);
    expect(PILL_TINT).toBe(16);
    expect(CURRENT_TINT).toBe(12);
    expect(AVATAR_TINT).toBe(16);
  });

  it.each([...cases, ...currentCases, ...avatarCases])(
    '%s meets 4.5:1 over bg-elevated',
    (_, text, tone, percent) => {
      const ratio = contrast(text, tint(tone, percent, ground));
      expect(ratio, `${ratio.toFixed(2)}:1`).toBeGreaterThanOrEqual(MIN_RATIO);
    },
  );

  it.each(accentNames)('the current entry’s icon in %s meets 3:1 on its tint', (name) => {
    const accent = channels(tokens.color.accent[name].base);
    const ratio = contrast(accent, tint(accent, CURRENT_TINT, ground));
    expect(ratio, `${ratio.toFixed(2)}:1`).toBeGreaterThanOrEqual(MIN_ICON_RATIO);
  });
});
