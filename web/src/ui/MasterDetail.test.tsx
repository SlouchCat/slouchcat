import { render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { useState } from 'react';
import { describe, expect, it } from 'vitest';
import { MasterDetail } from './MasterDetail';

function Areas() {
  const [selected, setSelected] = useState('languages');
  return (
    <>
      <MasterDetail
        label="Bereiche"
        items={[
          { id: 'languages', label: 'Sprachen', description: 'Deutsch, Englisch' },
          { id: 'images', label: 'Bilder' },
          { id: 'quality', label: 'Güte' },
        ]}
        selectedId={selected}
        onSelect={setSelected}
      >
        <p>Detail {selected}</p>
        <button type="button">Im Detail</button>
      </MasterDetail>
    </>
  );
}

describe('MasterDetail (concept 12, 11.2)', () => {
  it('is one Tab stop; the arrows move and Enter selects', async () => {
    const user = userEvent.setup();
    render(<Areas />);
    expect(screen.getByRole('listbox', { name: 'Bereiche' })).toBeVisible();
    // Label and description slots: the second line is the option's description, not its name.
    expect(screen.getByRole('option', { name: 'Sprachen' })).toHaveAccessibleDescription(
      'Deutsch, Englisch',
    );
    await user.tab();
    expect(screen.getByRole('option', { name: 'Sprachen' })).toHaveFocus();
    await user.keyboard('{ArrowDown}');
    expect(screen.getByRole('option', { name: 'Bilder' })).toHaveFocus();
    // Moving does not select yet.
    expect(screen.getByText('Detail languages')).toBeVisible();
    await user.keyboard('{Enter}');
    expect(screen.getByRole('option', { name: 'Bilder' })).toHaveAttribute('aria-selected', 'true');
    expect(screen.getByText('Detail images')).toBeVisible();
    await user.tab();
    expect(screen.getByRole('button', { name: 'Im Detail' })).toHaveFocus();
    await user.tab({ shift: true });
    expect(screen.getByRole('option', { name: 'Bilder' })).toHaveFocus();
  });
});
