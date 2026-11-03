/// <reference types="node" />
import { readFileSync } from 'node:fs';
import { join } from 'node:path';
import { render, screen, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { createRef } from 'react';
import { describe, expect, it } from 'vitest';
import { ActionMenu } from './ActionMenu';
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
