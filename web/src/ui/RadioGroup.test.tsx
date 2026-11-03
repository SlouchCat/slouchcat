import { render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { describe, expect, it } from 'vitest';
import { RadioGroup } from './RadioGroup';

const OPTIONS = [
  { value: 'daily', label: 'Täglich' },
  { value: 'weekly', label: 'Wöchentlich', isDisabled: true },
  { value: 'monthly', label: 'Monatlich' },
];

describe('RadioGroup vertical (concept 12)', () => {
  it('tells its orientation', () => {
    render(<RadioGroup label="Intervall" orientation="vertical" options={OPTIONS} />);
    expect(screen.getByRole('radiogroup', { name: 'Intervall' })).toHaveAttribute(
      'aria-orientation',
      'vertical',
    );
  });

  it('moves the choice with ↓ and ↑ and skips a disabled option', async () => {
    const user = userEvent.setup();
    render(
      <RadioGroup
        label="Intervall"
        orientation="vertical"
        options={OPTIONS}
        defaultValue="daily"
      />,
    );
    const daily = screen.getByRole('radio', { name: 'Täglich' });
    const weekly = screen.getByRole('radio', { name: 'Wöchentlich' });
    const monthly = screen.getByRole('radio', { name: 'Monatlich' });
    expect(weekly).toBeDisabled();

    await user.tab();
    expect(daily).toHaveFocus();
    await user.keyboard('{ArrowDown}');
    expect(monthly).toHaveFocus();
    expect(monthly).toBeChecked();
    expect(weekly).not.toBeChecked();
    await user.keyboard('{ArrowUp}');
    expect(daily).toHaveFocus();
    expect(daily).toBeChecked();
  });
});
