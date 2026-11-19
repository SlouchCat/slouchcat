/// <reference types="node" />
import { readFileSync } from 'node:fs';
import { join } from 'node:path';
import { accentNames, tokens, type AccentName } from '@slouchcat/design-tokens';
import { act, render, screen, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { describe, expect, it, vi } from 'vitest';
import {
  AdminSideNav,
  type AdminSideNavBranch,
  type AdminSideNavEntry,
  type AdminSideNavProps,
} from './AdminSideNav';
import { LinkRouterProvider } from './AppLink';

/**
 * The column (build plan 2.2; Explorer tree): back, the heading «Verwaltung» with the overview and
 * the areas as branches: «Medien» open with the current page, «Automatisierung» closed, «System»
 * open.
 */
const props = (overrides: Partial<AdminSideNavProps> = {}): AdminSideNavProps => ({
  label: 'Verwaltung',
  top: [{ href: '/', label: 'Zurück zur App', icon: 'back' }],
  heading: 'Verwaltung',
  entries: [{ href: '/admin', label: 'Übersicht', icon: 'grid' }],
  branches: [
    {
      id: 'media',
      label: 'Medien',
      icon: 'folder',
      current: true,
      expanded: true,
      alarm: { value: '12', tone: 'warning' },
      accessibleLabel: 'Medien, 12 Titel brauchen Pflege',
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
      id: 'automation',
      label: 'Automatisierung',
      icon: 'refresh',
      expanded: false,
      entries: [
        { href: '/admin/jobs', label: 'Hintergrundjobs', icon: 'refresh' },
        { href: '/admin/integrations', label: 'Integrationen', icon: 'plug' },
      ],
    },
    {
      id: 'system',
      label: 'System',
      icon: 'server',
      expanded: true,
      entries: [
        { href: '/admin/stats', label: 'Statistiken', icon: 'chart' },
        { href: '/admin/backup', label: 'Sicherung', icon: 'archive' },
      ],
    },
  ],
  profile: { name: 'Marco', role: 'Server-Admin' },
  ...overrides,
});

/** The column's branches with «Medien» closed: its current page is hidden. */
const mediaClosed = (): Partial<AdminSideNavProps> => ({
  branches: props().branches?.map((branch) =>
    branch.id === 'media' ? { ...branch, expanded: false } : branch,
  ),
});

/** The sheet's shape: the areas as bands with their pages. */
const grouped: Partial<AdminSideNavProps> = {
  top: [
    { href: '/', label: 'Zurück zur App', icon: 'back' },
    { href: '/admin', label: 'Übersicht', icon: 'grid' },
  ],
  heading: undefined,
  entries: undefined,
  branches: undefined,
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
const branch = (name: RegExp | string) => within(nav()).getByRole('button', { name });
/** Every link and branch button of the column in DOM order, hidden ones too. */
const items = () => [...nav().querySelectorAll<HTMLElement>('a, button')];

describe('admin sidebar (concept 11, 11.6; build plan 2.2)', () => {
  it('lists back, then the overview and the areas as a tree under the heading, without headings', () => {
    render(<AdminSideNav {...props()} />);
    expect(within(nav()).queryAllByRole('heading')).toEqual([]);
    const main = within(nav()).getByRole('list', { name: 'Verwaltung' });
    const heading = document.getElementById(main.getAttribute('aria-labelledby') ?? '');
    expect(heading?.tagName).toBe('SPAN');
    // The overview, then one branch per area, each a button followed by the list of its pages.
    const rows = [...main.children].map((row) =>
      [...row.children].map((child) => child.tagName).join(' '),
    );
    expect(rows).toEqual(['A', 'BUTTON UL', 'BUTTON UL', 'BUTTON UL']);
    expect(
      within(main)
        .getAllByRole('button')
        .map((button) => button.getAttribute('aria-label') ?? button.textContent),
    ).toEqual(['Medien, 12 Titel brauchen Pflege', 'Automatisierung', 'System']);
    // The links of the closed «Automatisierung» are hidden.
    expect(links().map((link) => link.getAttribute('aria-label') ?? link.textContent)).toEqual([
      'Zurück zur App',
      'Übersicht',
      'Bibliotheken, 2.318 Titel',
      'Pflege, 12 Titel brauchen Pflege',
      'Statistiken',
      'Sicherung',
    ]);
    expect(
      within(main)
        .getAllByRole('link')
        .map((link) => link.textContent),
    ).toEqual(['Übersicht', 'Bibliotheken2.318', 'Pflege12', 'Statistiken', 'Sicherung']);
    expect(screen.getByRole('link', { name: 'Pflege, 12 Titel brauchen Pflege' })).toHaveAttribute(
      'aria-current',
      'page',
    );
    expect(nav().querySelectorAll('[aria-current="page"]')).toHaveLength(1);
  });

  it('gives each branch aria-expanded and aria-controls on the list of its pages, named by the area', () => {
    render(<AdminSideNav {...props()} />);
    for (const [name, list, expanded] of [
      [/^Medien/, 'Medien', true],
      ['Automatisierung', 'Automatisierung', false],
      ['System', 'System', true],
    ] as const) {
      const button = branch(name);
      expect(button).toHaveAttribute('type', 'button');
      expect(button).toHaveAttribute('aria-expanded', String(expanded));
      const pages = document.getElementById(button.getAttribute('aria-controls') ?? '');
      expect(pages?.tagName).toBe('UL');
      expect(pages).toHaveAttribute('aria-label', list);
      expect(pages?.hidden).toBe(!expanded);
      expect(button.nextElementSibling).toBe(pages);
    }
    expect(within(nav()).getByRole('list', { name: 'Medien' })).toBeVisible();
    expect(within(nav()).queryByRole('list', { name: 'Automatisierung' })).toBeNull();
    expect(within(nav()).queryByRole('link', { name: 'Hintergrundjobs' })).toBeNull();
    // The pill stays short on screen; the button's name spells it out.
    expect(branch(/^Medien/)).toHaveTextContent('Medien12');
    expect(within(branch(/^Medien/)).getByText('12')).toHaveAttribute('aria-hidden', 'true');
  });

  it('marks the current page’s branch, open or closed, and keeps exactly one current page', () => {
    const { rerender } = render(<AdminSideNav {...props()} />);
    expect(branch(/^Medien/)).toHaveAttribute('aria-current', 'true');
    expect(branch('Automatisierung')).not.toHaveAttribute('aria-current');
    expect(branch('System')).not.toHaveAttribute('aria-current');
    rerender(<AdminSideNav {...props(mediaClosed())} />);
    expect(branch(/^Medien/)).toHaveAttribute('aria-current', 'true');
    expect(within(nav()).queryByRole('link', { name: /^Pflege/ })).toBeNull();
    const current = nav().querySelectorAll('[aria-current="page"]');
    expect(current).toHaveLength(1);
    expect(current[0]).toHaveAttribute('href', '/admin/maintenance');
  });

  it('opens and closes a branch with a click, Enter and Space, without following a link', async () => {
    const user = userEvent.setup();
    const navigate = vi.fn();
    const onToggleBranch = vi.fn();
    render(
      <LinkRouterProvider navigate={navigate}>
        <AdminSideNav {...props({ onToggleBranch })} />
      </LinkRouterProvider>,
    );
    await user.click(branch('Automatisierung'));
    expect(onToggleBranch).toHaveBeenLastCalledWith('automation');
    expect(branch('Automatisierung')).toHaveFocus();
    await user.keyboard('{Enter}');
    expect(onToggleBranch).toHaveBeenLastCalledWith('automation');
    await user.keyboard('{ArrowDown} ');
    expect(branch('System')).toHaveFocus();
    expect(onToggleBranch).toHaveBeenLastCalledWith('system');
    expect(onToggleBranch).toHaveBeenCalledTimes(3);
    expect(navigate).not.toHaveBeenCalled();
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

  it('draws the current page’s arrow in the sheet but not in the tree', () => {
    const { unmount } = render(<AdminSideNav {...props()} />);
    // Only the page's icon: an arrow at its end would read as a disclosure arrow.
    expect(screen.getByRole('link', { name: /^Pflege/ }).querySelectorAll('svg')).toHaveLength(1);
    // Each branch: its icon and the disclosure arrow.
    expect(branch(/^Medien/).querySelectorAll('svg')).toHaveLength(2);
    unmount();
    render(<AdminSideNav {...props(grouped)} />);
    expect(screen.getByRole('link', { name: /^Pflege/ }).querySelectorAll('svg')).toHaveLength(2);
  });

  it('is a single Tab stop on the current entry; Tab leaves the column', async () => {
    const user = userEvent.setup();
    render(
      <>
        <AdminSideNav {...props()} />
        <button type="button">Inhalt</button>
      </>,
    );
    expect(items().filter((item) => item.tabIndex === 0)).toEqual([
      screen.getByRole('link', { name: /^Pflege/ }),
    ]);
    await user.tab();
    expect(screen.getByRole('link', { name: /^Pflege/ })).toHaveFocus();
    await user.tab();
    expect(screen.getByRole('button', { name: 'Inhalt' })).toHaveFocus();
    await user.tab({ shift: true });
    expect(screen.getByRole('link', { name: /^Pflege/ })).toHaveFocus();
  });

  it('puts the Tab stop on the branch’s button while the current page is folded away', async () => {
    const user = userEvent.setup();
    const { rerender } = render(<AdminSideNav {...props(mediaClosed())} />);
    expect(items().filter((item) => item.tabIndex === 0)).toEqual([branch(/^Medien/)]);
    await user.tab();
    expect(branch(/^Medien/)).toHaveFocus();
    // Open again, the page takes the Tab stop back once the focus left the column.
    rerender(<AdminSideNav {...props()} />);
    act(() => (document.activeElement as HTMLElement).blur());
    expect(items().filter((item) => item.tabIndex === 0)).toEqual([
      screen.getByRole('link', { name: /^Pflege/ }),
    ]);
    // Closed while a page of it holds the Tab stop: the stop falls back to the branch's button.
    await user.tab();
    await user.keyboard('{ArrowUp}');
    expect(screen.getByRole('link', { name: /^Bibliotheken/ })).toHaveFocus();
    rerender(<AdminSideNav {...props(mediaClosed())} />);
    expect(items().filter((item) => item.tabIndex === 0)).toEqual([branch(/^Medien/)]);
  });

  it('moves with ArrowDown and ArrowUp over the shown items, wrapping, and with Home and End', async () => {
    const user = userEvent.setup();
    render(<AdminSideNav {...props()} />);
    await user.tab();
    // The pages of the closed «Automatisierung» are skipped, those of the open «System» not.
    await user.keyboard('{ArrowDown}');
    expect(branch('Automatisierung')).toHaveFocus();
    await user.keyboard('{ArrowDown}');
    expect(branch('System')).toHaveFocus();
    await user.keyboard('{ArrowDown}');
    expect(screen.getByRole('link', { name: 'Statistiken' })).toHaveFocus();
    await user.keyboard('{ArrowDown}{ArrowDown}');
    expect(screen.getByRole('link', { name: 'Zurück zur App' })).toHaveFocus();
    await user.keyboard('{ArrowUp}');
    expect(screen.getByRole('link', { name: 'Sicherung' })).toHaveFocus();
    await user.keyboard('{Home}');
    expect(screen.getByRole('link', { name: 'Zurück zur App' })).toHaveFocus();
    await user.keyboard('{ArrowDown}{ArrowDown}');
    expect(branch(/^Medien/)).toHaveFocus();
    await user.keyboard('{End}');
    expect(screen.getByRole('link', { name: 'Sicherung' })).toHaveFocus();
    // The moved focus carries the Tab stop; leaving the column puts it back on the current page.
    expect(items().filter((item) => item.tabIndex === 0)).toHaveLength(1);
    await user.tab();
    expect(screen.getByRole('link', { name: /^Pflege/ })).toHaveAttribute('tabindex', '0');
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
    await user.keyboard('{ArrowUp}{Enter}');
    expect(navigate).toHaveBeenCalledWith('/admin/libraries');
  });

  it('carries the focused entry over to the next page’s copy (focusCarry)', async () => {
    const user = userEvent.setup();
    const focusCarry = { current: null as number | null };
    const { unmount } = render(<AdminSideNav {...props({ focusCarry })} />);
    await user.tab();
    await user.keyboard('{ArrowDown}');
    unmount();
    // Every item counts, hidden ones too: back, overview, Medien and its two pages, Automatisierung.
    expect(focusCarry.current).toBe(5);
    render(<AdminSideNav {...props({ focusCarry })} />);
    expect(branch('Automatisierung')).toHaveFocus();
    expect(focusCarry.current).toBeNull();
  });

  it('carries the focus to the overview at index 1 and to a hidden page’s branch', () => {
    const focusCarry = { current: 1 as number | null };
    const { unmount } = render(<AdminSideNav {...props({ focusCarry })} />);
    expect(screen.getByRole('link', { name: 'Übersicht' })).toHaveFocus();
    unmount();
    // «Hintergrundjobs» in the closed «Automatisierung».
    focusCarry.current = 6;
    render(<AdminSideNav {...props({ focusCarry })} />);
    expect(items()[6]).toHaveAttribute('href', '/admin/jobs');
    expect(branch('Automatisierung')).toHaveFocus();
  });

  it('scrolls the current entry into view in the column', () => {
    const scroll = vi.spyOn(Element.prototype, 'scrollIntoView');
    render(<AdminSideNav {...props()} />);
    expect(scroll).toHaveBeenCalledOnce();
    expect(scroll.mock.contexts[0]).toBe(screen.getByRole('link', { name: /^Pflege/ }));
    expect(scroll).toHaveBeenCalledWith({ block: 'nearest' });
    scroll.mockRestore();
  });

  it('scrolls nothing while the current page is folded away', () => {
    const scroll = vi.spyOn(Element.prototype, 'scrollIntoView');
    render(<AdminSideNav {...props(mediaClosed())} />);
    expect(scroll).not.toHaveBeenCalled();
    scroll.mockRestore();
  });

  it('requires a spoken name for a number, pill or badge (type check)', () => {
    const entry = (value: AdminSideNavEntry) => value;
    // @ts-expect-error A count without accessibleLabel would vanish for screen readers.
    entry({ href: '/a', label: 'A', icon: 'grid', count: '3' });
    // @ts-expect-error The same for an alarm pill.
    entry({ href: '/a', label: 'A', icon: 'grid', alarm: { value: '3', tone: 'danger' } });
    expect(entry({ href: '/a', label: 'A', icon: 'grid' }).label).toBe('A');
    const area = (value: AdminSideNavBranch) => value;
    const base = { id: 'a', label: 'A', icon: 'grid', entries: [], expanded: false } as const;
    // @ts-expect-error A branch's pill needs a spoken name as well.
    area({ ...base, alarm: { value: '3', tone: 'danger' } });
    // @ts-expect-error And its badge.
    area({ ...base, badge: 'Update' });
    expect(area(base).label).toBe('A');
  });

  it('puts the Tab stop on the first entry when no page is current', () => {
    render(
      <AdminSideNav
        {...props({
          entries: [{ href: '/admin', label: 'Übersicht', icon: 'grid' }],
          branches: [],
        })}
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

/** The current page's area in the tree: its name in the accent, straight on the column. */
const branchCases: [string, readonly number[], readonly number[], number][] = accentNames.map(
  (name): [string, number[], number[], number] => {
    const accent = channels(tokens.color.accent[name].base);
    return [`current branch ${name}`, accent, accent, 0];
  },
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

  it.each([...cases, ...currentCases, ...branchCases, ...avatarCases])(
    '%s meets 4.5:1 over bg-elevated',
    (_, text, tone, percent) => {
      const ratio = contrast(text, tint(tone, percent, ground));
      expect(ratio, `${ratio.toFixed(2)}:1`).toBeGreaterThanOrEqual(MIN_RATIO);
    },
  );

  // Hovered, the current area's button keeps the accent over the entry's hover surface.
  it.each(accentNames)('the current branch in %s meets 4.5:1 over its hover surface', (name) => {
    const accent = channels(tokens.color.accent[name].base);
    const ratio = contrast(accent, channels(tokens.color.surface[2]));
    expect(ratio, `${ratio.toFixed(2)}:1`).toBeGreaterThanOrEqual(MIN_RATIO);
  });

  it.each(accentNames)('the current entry’s icon in %s meets 3:1 on its tint', (name) => {
    const accent = channels(tokens.color.accent[name].base);
    const ratio = contrast(accent, tint(accent, CURRENT_TINT, ground));
    expect(ratio, `${ratio.toFixed(2)}:1`).toBeGreaterThanOrEqual(MIN_ICON_RATIO);
  });
});
