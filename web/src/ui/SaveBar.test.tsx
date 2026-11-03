import { render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { describe, expect, it, vi } from 'vitest';
import { SaveBar } from './SaveBar';

describe('SaveBar (concept 12, S-7)', () => {
  it('says whether something changed and offers discard and save', async () => {
    const user = userEvent.setup();
    const onSave = vi.fn();
    const onDiscard = vi.fn();
    const { rerender } = render(
      <SaveBar dirty saving={false} onSave={onSave} onDiscard={onDiscard} />,
    );
    expect(screen.getByRole('region', { name: 'Speichern' })).toHaveTextContent(
      'Ungespeicherte Änderungen',
    );
    await user.click(screen.getByRole('button', { name: 'Verwerfen' }));
    await user.click(screen.getByRole('button', { name: 'Speichern' }));
    expect(onDiscard).toHaveBeenCalledOnce();
    expect(onSave).toHaveBeenCalledOnce();

    rerender(<SaveBar dirty={false} saving={false} onSave={onSave} onDiscard={onDiscard} />);
    expect(screen.getByRole('region', { name: 'Speichern' })).toHaveTextContent('Gespeichert');
    // Nothing changed: «Verwerfen» is gone, «Speichern» stays and sends nothing (decision 18).
    expect(screen.queryByRole('button', { name: 'Verwerfen' })).toBeNull();
    const save = screen.getByRole('button', { name: 'Speichern' });
    expect(save).toBeEnabled();
    await user.click(save);
    expect(save).toHaveFocus();
    expect(onSave).toHaveBeenCalledOnce();
  });

  it('waits while a save runs: «Verwerfen» is off, «Speichern» shows the progress', () => {
    render(<SaveBar dirty saving onSave={vi.fn()} onDiscard={vi.fn()} />);
    expect(screen.getByRole('button', { name: 'Verwerfen' })).toBeDisabled();
    expect(screen.getByRole('button', { name: /Wird gespeichert/ })).toBeInTheDocument();
  });
});
