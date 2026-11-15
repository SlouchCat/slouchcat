/// <reference types="node" />
import { readFileSync } from 'node:fs';
import { join } from 'node:path';
import { tokens } from '@slouchcat/design-tokens';
import { render, screen, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { createRef } from 'react';
import { describe, expect, it } from 'vitest';
import { ActionMenu } from './ActionMenu';
import { AdminFrame } from './adminFrame';
import { AdminPage } from './AdminPage';
import { ConfirmDialog, FormDialog } from './Dialog';

describe('AdminPage (concept 12, S-2)', () => {
  it('has a focusable h1, the intro behind the help button and the live indicator', async () => {
    const user = userEvent.setup();
    const title = createRef<HTMLHeadingElement>();
    render(
      <AdminPage
        title="Hintergrundjobs"
        help="Alles, was der Server im Hintergrund tut."
        liveSeconds={5}
        titleRef={title}
        actions={<button type="button">Erneut versuchen</button>}
      >
        <p>Inhalt</p>
      </AdminPage>,
    );
    const heading = screen.getByRole('heading', { level: 1, name: 'Hintergrundjobs' });
    expect(heading).toHaveAttribute('tabindex', '-1');
    expect(title.current).toBe(heading);
    title.current?.focus();
    expect(heading).toHaveFocus();

    expect(screen.getByText('Live, alle 5 Sekunden')).toBeVisible();
    // No intro sentence in the flow of the page until it is asked for.
    expect(screen.queryByText('Alles, was der Server im Hintergrund tut.')).not.toBeVisible();
    const help = screen.getByRole('button', { name: 'Hilfe zu dieser Seite' });
    expect(help).toHaveAttribute('aria-expanded', 'false');
    await user.click(help);
    expect(help).toHaveAttribute('aria-expanded', 'true');
    const intro = screen.getByText('Alles, was der Server im Hintergrund tut.');
    expect(intro).toBeVisible();
    expect(help).toHaveAttribute('aria-controls', intro.id);
    await user.keyboard('{Enter}');
    expect(intro).not.toBeVisible();
    expect(screen.getByRole('main')).toContainElement(heading);
  });

  it('shows no help button and no live indicator without them', () => {
    render(<AdminPage title="Sicherung">Inhalt</AdminPage>);
    expect(screen.queryByRole('button', { name: 'Hilfe zu dieser Seite' })).toBeNull();
    expect(screen.queryByText(/^Live/)).toBeNull();
  });
});

describe('AdminPage inside the administration’s frame (build plan 2.5, 2.6)', () => {
  const frame = {
    eyebrow: 'Automatisierung / Hintergrundjobs',
    tabs: {
      label: 'Seiten im Bereich Automatisierung',
      tabs: [
        { href: '/admin/jobs', label: 'Hintergrundjobs', current: true },
        { href: '/admin/integrations', label: 'Integrationen' },
      ],
    },
  };

  it('shows the area’s tabs, the line over the title and the intro open; no help button', () => {
    const title = createRef<HTMLHeadingElement>();
    render(
      <AdminFrame value={frame}>
        <AdminPage
          title="Hintergrundjobs"
          help="Alles, was der Server im Hintergrund tut."
          liveSeconds={5}
          titleRef={title}
          actions={<button type="button">Erneut versuchen</button>}
        >
          <p>Inhalt</p>
        </AdminPage>
      </AdminFrame>,
    );
    const tabs = screen.getByRole('navigation', { name: 'Seiten im Bereich Automatisierung' });
    expect(within(tabs).getByRole('link', { name: 'Hintergrundjobs' })).toHaveAttribute(
      'aria-current',
      'page',
    );
    const heading = screen.getByRole('heading', { level: 1, name: 'Hintergrundjobs' });
    expect(heading).toHaveAttribute('tabindex', '-1');
    expect(title.current).toBe(heading);
    // The tabs come before the header, the line over the title before the h1.
    expect(tabs.compareDocumentPosition(heading) & Node.DOCUMENT_POSITION_FOLLOWING).toBeTruthy();
    const eyebrow = screen.getByText('Automatisierung / Hintergrundjobs');
    expect(
      eyebrow.compareDocumentPosition(heading) & Node.DOCUMENT_POSITION_FOLLOWING,
    ).toBeTruthy();
    expect(screen.getByText('Alles, was der Server im Hintergrund tut.')).toBeVisible();
    expect(screen.queryByRole('button', { name: 'Hilfe zu dieser Seite' })).toBeNull();
    expect(screen.getByText('Live, alle 5 Sekunden')).toBeVisible();
    expect(screen.getByRole('button', { name: 'Erneut versuchen' })).toBeVisible();
  });

  it('shows no tabs where the frame has none', () => {
    render(
      <AdminFrame value={{ eyebrow: 'Übersicht' }}>
        <AdminPage title="Sicherung">Inhalt</AdminPage>
      </AdminFrame>,
    );
    expect(screen.queryByRole('navigation')).toBeNull();
    expect(screen.getByText('Übersicht')).toBeVisible();
  });
});

describe('AdminDensityScope (ADR 0020)', () => {
  // One modal at a time: a second one hides the first from the accessibility tree.
  const dialogs = {
    confirm: (
      <ConfirmDialog
        isOpen
        onOpenChange={() => undefined}
        title="Löschen?"
        body="Weg ist weg."
        confirmLabel="Löschen"
        cancelLabel="Abbrechen"
        onConfirm={() => undefined}
      />
    ),
    form: (
      <FormDialog isOpen onOpenChange={() => undefined} title="Formular">
        <input aria-label="Feld" />
      </FormDialog>
    ),
  };
  const opened = () => screen.queryByRole('alertdialog') ?? screen.getByRole('dialog');

  it.each(Object.entries(dialogs))(
    'marks the portal of a %s dialog opened from an administration page as compact',
    (_, dialog) => {
      render(<AdminPage title="Sicherung">{dialog}</AdminPage>);
      const overlay = opened().closest<HTMLElement>('[data-density]');
      expect(overlay).toHaveAttribute('data-density', 'compact');
      // The attribute sits on the dialog's own portal, outside the page's main.
      expect(screen.getByRole('main', { hidden: true })).not.toContainElement(overlay);
    },
  );

  it.each(Object.entries(dialogs))(
    "leaves a %s dialog outside an administration page in today's density",
    (_, dialog) => {
      render(dialog);
      expect(opened().closest('[data-density]')).toBeNull();
    },
  );

  it('marks the popover of an ActionMenu as well', async () => {
    const user = userEvent.setup();
    render(
      <AdminPage title="Sicherung">
        <ActionMenu rowName="Sicherung 1" items={[{ id: 'a', label: 'A' }]} onAction={() => {}} />
      </AdminPage>,
    );
    await user.click(screen.getByRole('button', { name: 'Weitere Aktionen für Sicherung 1' }));
    const menu = screen.getByRole('menu');
    expect(menu.closest('[data-density]')).toHaveAttribute('data-density', 'compact');
    expect(within(menu).getByRole('menuitem', { name: 'A' })).toBeVisible();
  });
});

describe('density.css (ADR 0020, concept 12 S-1)', () => {
  // Vitest hands CSS over as class names; the rule is read from disk. The browser check of the
  // cascade is e2e-storybook/admin-blocks.spec.ts.
  const css = readFileSync(join(import.meta.dirname, 'density.css'), 'utf8');

  it('applies only under the admin shell, with a mouse from 768 px', () => {
    const rules = [...css.matchAll(/^\s*([^@/*\s][^{]*)\{/gm)].map((match) => match[1]?.trim());
    expect(rules).toEqual([":root[data-shell='admin'] :is(#content, [data-density='compact'])"]);
    expect(css).toContain('@media (width >= 48rem) and (pointer: fine)');
  });
});

describe('admin-look.css (build plan 2.7)', () => {
  const css = readFileSync(join(import.meta.dirname, 'admin-look.css'), 'utf8');

  it('applies only under the admin shell, at every width', () => {
    const rules = [...css.matchAll(/^\s*([^@/*\s-][^{]*)\{/gm)].map((match) => match[1]?.trim());
    expect(rules).toEqual([":root[data-shell='admin'] :is(#content, [data-density='compact'])"]);
    expect(css).not.toContain('@media');
    expect(css).not.toMatch(/#[0-9a-f]{3,8}\b/i);
  });

  it('sets only tokens that a building block reads, with today’s value as its fallback', () => {
    const names = [...css.matchAll(/^\s*(--sc-[\w-]+):/gm)].map((match) => match[1]);
    expect(names.length).toBeGreaterThan(0);
    const modules = [
      'BandPanel',
      'Panel',
      'DataTable',
      'StatTile',
      'Chip',
      'Notice',
      'Button',
      'Field',
      'SelectField',
    ].map((name) => readFileSync(join(import.meta.dirname, `${name}.module.css`), 'utf8'));
    for (const name of names) {
      expect(
        modules.some((module) => module.includes(`var(${name},`)),
        `${name} is read with a fallback`,
      ).toBe(true);
    }
  });

  it.each(['warning', 'danger', 'success'] as const)(
    'a %s badge reads its tone on its tint at 4.5:1',
    (tone) => {
      const percent = Number(
        new RegExp(
          `--sc-chip-${tone}-background: color-mix\\(\\s*in srgb,\\s*var\\(--sc-color-state-${tone}\\) (\\d+)%,\\s*var\\(--sc-color-bg-elevated\\)`,
        ).exec(css)?.[1],
      );
      expect(percent).toBe(16);
      const rgb = (hex: string) => [1, 3, 5].map((at) => parseInt(hex.slice(at, at + 2), 16));
      const luminance = (channels: number[]) => {
        const [r = 0, g = 0, b = 0] = channels.map((value) => {
          const c = value / 255;
          return c <= 0.04045 ? c / 12.92 : ((c + 0.055) / 1.055) ** 2.4;
        });
        return 0.2126 * r + 0.7152 * g + 0.0722 * b;
      };
      const text = rgb(tokens.color.state[tone]);
      const ground = rgb(tokens.color.bg.elevated);
      const tint = text.map(
        (value, i) => (value * percent + (ground[i] ?? 0) * (100 - percent)) / 100,
      );
      const [light = 0, dark = 0] = [luminance(text), luminance(tint)].sort((a, b) => b - a);
      expect((light + 0.05) / (dark + 0.05)).toBeGreaterThanOrEqual(4.5);
    },
  );
});
