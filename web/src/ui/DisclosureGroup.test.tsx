import { render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { describe, expect, it } from 'vitest';
import { DisclosureGroup } from './DisclosureGroup';

describe('DisclosureGroup (concept 12, 11.2; concept 24, P-3)', () => {
  it('starts closed; Enter and Space fold it, the focus stays, the content is kept', async () => {
    const user = userEvent.setup();
    render(
      <DisclosureGroup title="Filme" count="2" countLabel="2 Titel">
        <input aria-label="Filter" />
      </DisclosureGroup>,
    );
    const heading = screen.getByRole('heading', { level: 3 });
    const toggle = screen.getByRole('button', { name: 'Filme 2 Titel' });
    expect(heading).toContainElement(toggle);
    expect(toggle).toHaveAttribute('aria-expanded', 'false');
    expect(screen.queryByRole('textbox', { name: 'Filter' })).toBeNull();

    toggle.focus();
    await user.keyboard('{Enter}');
    expect(toggle).toHaveAttribute('aria-expanded', 'true');
    expect(toggle).toHaveFocus();
    await user.type(screen.getByRole('textbox', { name: 'Filter' }), 'Dämmerung');

    toggle.focus();
    await user.keyboard(' ');
    expect(toggle).toHaveAttribute('aria-expanded', 'false');
    expect(toggle).toHaveFocus();
    expect(screen.queryByRole('textbox', { name: 'Filter' })).toBeNull();
    await user.keyboard('{Enter}');
    expect(screen.getByRole('textbox', { name: 'Filter' })).toHaveValue('Dämmerung');
  });

  it('starts open only when the page asks for it (exception 4)', () => {
    render(
      <DisclosureGroup title="Laufend" defaultExpanded>
        <p>Inhalt</p>
      </DisclosureGroup>,
    );
    expect(screen.getByRole('button', { name: 'Laufend' })).toHaveAttribute(
      'aria-expanded',
      'true',
    );
    expect(screen.getByText('Inhalt')).toBeVisible();
  });

  it('describes its button by the summary and «geändert» beside it', () => {
    render(
      <DisclosureGroup
        level={2}
        title="Sprachen"
        meta={
          <>
            <span>Deutsch › Englisch</span>
            <span>geändert</span>
          </>
        }
      >
        <p>Inhalt</p>
      </DisclosureGroup>,
    );
    const toggle = screen.getByRole('button', { name: 'Sprachen' });
    expect(toggle).toHaveAccessibleDescription('Deutsch › Englischgeändert');
  });

  it('has no description without a meta', () => {
    render(
      <DisclosureGroup title="Filme">
        <p>Inhalt</p>
      </DisclosureGroup>,
    );
    expect(screen.getByRole('button', { name: 'Filme' })).not.toHaveAttribute('aria-describedby');
  });
});
