import { render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { describe, expect, it } from 'vitest';
import { DisclosureGroup } from './DisclosureGroup';

describe('DisclosureGroup (concept 12, 11.2)', () => {
  it('is a heading with a button that folds the group away, keeping its content', async () => {
    const user = userEvent.setup();
    render(
      <DisclosureGroup title="Filme" count="2" countLabel="2 Titel">
        <input aria-label="Filter" />
      </DisclosureGroup>,
    );
    const heading = screen.getByRole('heading', { level: 3 });
    const toggle = screen.getByRole('button', { name: 'Filme 2 Titel' });
    expect(heading).toContainElement(toggle);
    expect(toggle).toHaveAttribute('aria-expanded', 'true');
    await user.type(screen.getByRole('textbox', { name: 'Filter' }), 'Dämmerung');

    toggle.focus();
    await user.keyboard('{Enter}');
    expect(toggle).toHaveAttribute('aria-expanded', 'false');
    expect(screen.queryByRole('textbox', { name: 'Filter' })).toBeNull();
    await user.keyboard(' ');
    expect(screen.getByRole('textbox', { name: 'Filter' })).toHaveValue('Dämmerung');
  });
});
