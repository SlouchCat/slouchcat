/// <reference types="node" />
import { readFileSync } from 'node:fs';
import { join } from 'node:path';
import { accentNames, tokens, type AccentName } from '@slouchcat/design-tokens';
import { render, screen, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { describe, expect, it, vi } from 'vitest';
import { AdminSideNav, type AdminSideNavEntry, type AdminSideNavProps } from './AdminSideNav';
import { LinkRouterProvider } from './AppLink';

const props = (overrides: Partial<AdminSideNavProps> = {}): AdminSideNavProps => ({
  label: 'Verwaltung',
  top: [
    { href: '/', label: 'Zurück zur App', icon: 'back' },
    { href: '/admin', label: 'Übersicht', icon: 'grid' },
  ],
  groups: [
    {
      label: 'Bibliothek',
      icon: 'film',
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
        { href: '/admin/jobs', label: 'Hintergrundjobs', icon: 'refresh' },
        { href: '/admin/backup', label: 'Sicherung', icon: 'archive' },
      ],
    },
  ],
  ...overrides,
});

const nav = () => screen.getByRole('navigation', { name: 'Verwaltung' });
const links = () => within(nav()).getAllByRole('link');

describe('admin sidebar (concept 11, 11.6)', () => {
  it('names the groups by their bands, without headings, and marks the current page', () => {
    render(<AdminSideNav {...props()} />);
    expect(within(nav()).queryAllByRole('heading')).toEqual([]);
    const library = within(nav()).getByRole('list', { name: 'Bibliothek' });
    const band = document.getElementById(library.getAttribute('aria-labelledby') ?? '');
    expect(band?.tagName).toBe('SPAN');
    expect(
      within(library)
        .getAllByRole('link')
        .map((link) => link.textContent),
    ).toEqual(['Bibliotheken2.318', 'Pflege12']);
    expect(within(nav()).getByRole('list', { name: 'System' })).toBeVisible();

    expect(links().map((link) => link.getAttribute('aria-label') ?? link.textContent)).toEqual([
      'Zurück zur App',
      'Übersicht',
      'Bibliotheken, 2.318 Titel',
      'Pflege, 12 Titel brauchen Pflege',
      'Hintergrundjobs',
      'Sicherung',
    ]);
    expect(screen.getByRole('link', { name: 'Pflege, 12 Titel brauchen Pflege' })).toHaveAttribute(
      'aria-current',
      'page',
    );
    expect(links().filter((link) => link.hasAttribute('aria-current'))).toHaveLength(1);
  });

  it('is a single Tab stop on the current page; Tab leaves the column', async () => {
    const user = userEvent.setup();
    render(
      <>
        <AdminSideNav {...props()} />
        <button type="button">Inhalt</button>
      </>,
    );
    expect(links().filter((link) => link.tabIndex === 0)).toEqual([
      screen.getByRole('link', { name: /^Pflege/ }),
    ]);
    await user.tab();
    expect(screen.getByRole('link', { name: /^Pflege/ })).toHaveFocus();
    await user.tab();
    expect(screen.getByRole('button', { name: 'Inhalt' })).toHaveFocus();
    await user.tab({ shift: true });
    expect(screen.getByRole('link', { name: /^Pflege/ })).toHaveFocus();
  });

  it('moves with ArrowDown and ArrowUp across the bands, wrapping, and with Home and End', async () => {
    const user = userEvent.setup();
    render(<AdminSideNav {...props()} />);
    await user.tab();
    await user.keyboard('{ArrowDown}');
    expect(screen.getByRole('link', { name: 'Hintergrundjobs' })).toHaveFocus();
    await user.keyboard('{ArrowUp}{ArrowUp}');
    expect(screen.getByRole('link', { name: /^Bibliotheken/ })).toHaveFocus();
    await user.keyboard('{Home}');
    expect(screen.getByRole('link', { name: 'Zurück zur App' })).toHaveFocus();
    await user.keyboard('{ArrowUp}');
    expect(screen.getByRole('link', { name: 'Sicherung' })).toHaveFocus();
    await user.keyboard('{ArrowDown}');
    expect(screen.getByRole('link', { name: 'Zurück zur App' })).toHaveFocus();
    await user.keyboard('{End}');
    expect(screen.getByRole('link', { name: 'Sicherung' })).toHaveFocus();
    // The moved focus carries the Tab stop; leaving the column puts it back on the current page.
    expect(links().filter((link) => link.tabIndex === 0)).toHaveLength(1);
    await user.tab();
    expect(screen.getByRole('link', { name: /^Pflege/ })).toHaveAttribute('tabindex', '0');
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
    expect(focusCarry.current).toBe(4);
    render(<AdminSideNav {...props({ focusCarry })} />);
    expect(screen.getByRole('link', { name: 'Hintergrundjobs' })).toHaveFocus();
    expect(focusCarry.current).toBeNull();
  });

  it('scrolls the current entry into view in the column', () => {
    const scroll = vi.spyOn(Element.prototype, 'scrollIntoView');
    render(<AdminSideNav {...props()} />);
    expect(scroll).toHaveBeenCalledOnce();
    expect(scroll.mock.contexts[0]).toBe(screen.getByRole('link', { name: /^Pflege/ }));
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
    render(<AdminSideNav {...props({ groups: [] })} />);
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

/**
 * Accents whose own colour is too weak on its band: the band would take text-primary for them.
 * None today (lowest: ember, 5.04:1 at 16 %). Adding an accent here is not enough on its own: CSS
 * only sees `--sc-accent`, not its name, so the band also needs a hook for it (e.g. a
 * `data-accent` attribute set by the session); otherwise the UI does not change.
 */
const PRIMARY_TEXT: readonly AccentName[] = [];

const MIN_RATIO = 4.5;

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

describe('band and pill contrast for every profile accent (F-9, F-7)', () => {
  it('reads the tints from the stylesheet', () => {
    expect(BAND_TINT).toBe(16);
    expect(PILL_TINT).toBe(16);
  });

  it.each(cases)('%s meets 4.5:1 over bg-elevated', (_, text, tone, percent) => {
    const ratio = contrast(text, tint(tone, percent, ground));
    expect(ratio, `${ratio.toFixed(2)}:1`).toBeGreaterThanOrEqual(MIN_RATIO);
  });
});
