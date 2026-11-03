import { render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { describe, expect, it } from 'vitest';
import { Tabs } from './Tabs';
import { useDrafts } from './useDrafts';

describe('Tabs with numbers (concept 12, 11.2)', () => {
  it('reads the number with the label, or its spoken form', () => {
    render(
      <Tabs
        label="Jobarten"
        tabs={[
          { id: 'library', label: 'Bibliothek', count: '6', content: 'a' },
          {
            id: 'failed',
            label: 'Fehlschläge',
            count: '265',
            countLabel: '265 fehlgeschlagen',
            tone: 'danger',
            content: 'b',
          },
        ]}
      />,
    );
    expect(screen.getByRole('tab', { name: 'Bibliothek 6' })).toBeVisible();
    expect(screen.getByRole('tab', { name: 'Fehlschläge 265 fehlgeschlagen' })).toBeVisible();
  });
});

/** A page with two tabs that edit the same job type; the draft lives in the page (S-17). */
function JobPage() {
  const drafts = useDrafts<string>(() => 'alle 6 h');
  const field = (
    <input
      aria-label="Zeitplan"
      value={drafts.valueOf('scan') ?? ''}
      onChange={(event) => drafts.set('scan', event.target.value)}
    />
  );
  return (
    <>
      <p>{drafts.dirty ? 'geändert' : 'unverändert'}</p>
      <Tabs
        label="Gruppen"
        tabs={[
          { id: 'library', label: 'Bibliothek', content: field },
          { id: 'failed', label: 'Fehlschläge', content: 'keine' },
          { id: 'all', label: 'Alle', content: field },
        ]}
      />
    </>
  );
}

describe('a draft and the tabs (S-13, S-17)', () => {
  it('keeps what was typed when the tab changes, though the inactive panel unmounts', async () => {
    const user = userEvent.setup();
    render(<JobPage />);
    const field = screen.getByRole('textbox', { name: 'Zeitplan' });
    await user.clear(field);
    await user.type(field, 'alle 2 h');
    expect(screen.getByText('geändert')).toBeVisible();

    await user.click(screen.getByRole('tab', { name: 'Fehlschläge' }));
    expect(screen.queryByRole('textbox', { name: 'Zeitplan' })).toBeNull();

    await user.click(screen.getByRole('tab', { name: 'Bibliothek' }));
    expect(screen.getByRole('textbox', { name: 'Zeitplan' })).toHaveValue('alle 2 h');
    // The same item in another tab shows the same draft.
    await user.click(screen.getByRole('tab', { name: 'Alle' }));
    expect(screen.getByRole('textbox', { name: 'Zeitplan' })).toHaveValue('alle 2 h');
  });
});
