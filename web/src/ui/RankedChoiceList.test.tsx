import { render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { useState } from 'react';
import { describe, expect, it } from 'vitest';
import { chooseAt, RankedChoiceList, type RankedChoice } from './RankedChoiceList';

const choices: RankedChoice[] = [
  { value: '', label: 'Ohne Text' },
  { value: 'de', label: 'Deutsch' },
  { value: 'en', label: 'Englisch' },
  { value: 'fr', label: 'Französisch' },
];

describe('chooseAt (concept 12, 10.2)', () => {
  it('puts a free choice on the place', () => {
    expect(chooseAt(['de', 'en'], 1, 'fr')).toEqual(['de', 'fr']);
  });
  it('swaps with the place that held the choice', () => {
    expect(chooseAt(['de', 'en', ''], 0, '')).toEqual(['', 'en', 'de']);
  });
  it('keeps the list for the choice the place already holds', () => {
    expect(chooseAt(['de', 'en'], 0, 'de')).toEqual(['de', 'en']);
  });
});

function Places({
  initial,
  canRemove,
  isDisabled,
}: {
  initial: string[];
  canRemove?: (index: number) => boolean;
  isDisabled?: boolean;
}) {
  const [value, setValue] = useState(initial);
  return (
    <>
      <p>{value.map((code) => code || 'none').join(', ')}</p>
      <RankedChoiceList
        choices={choices}
        value={value}
        onChange={setValue}
        rankLabel={(index) => `${index + 1}. Sprache`}
        addLabel="Weitere Sprache"
        canRemove={canRemove}
        isDisabled={isDisabled}
      />
    </>
  );
}

describe('RankedChoiceList', () => {
  it('shows one labelled select per place with every choice and the hint', () => {
    render(<Places initial={['de', 'en']} />);
    const first = screen.getByRole('combobox', { name: '1. Sprache' });
    expect(first).toHaveValue('de');
    expect(first).toHaveAccessibleDescription(
      'Wählst du einen Eintrag, der auf einem anderen Platz steht, tauschen die beiden die Plätze.',
    );
    expect(screen.getByRole('combobox', { name: '2. Sprache' })).toHaveValue('en');
    expect(
      Array.from((first as HTMLSelectElement).options).map((option) => option.textContent),
    ).toEqual(['Ohne Text', 'Deutsch', 'Englisch', 'Französisch']);
  });

  it('swaps two places, names them and keeps the focus on the select', async () => {
    const user = userEvent.setup();
    render(<Places initial={['de', 'en', '']} />);
    const first = screen.getByRole('combobox', { name: '1. Sprache' });
    first.focus();
    await user.selectOptions(first, 'Ohne Text');
    expect(screen.getByText('none, en, de')).toBeVisible();
    expect(screen.getByRole('status')).toHaveTextContent(
      'Ohne Text und Deutsch haben die Plätze getauscht.',
    );
    expect(first).toHaveFocus();
    // A free choice replaces the place without a swap note.
    await user.selectOptions(first, 'Französisch');
    expect(screen.getByText('fr, en, de')).toBeVisible();
    expect(screen.getByRole('status')).toBeEmptyDOMElement();
  });

  it('appends a place with the first free choice and focuses it; «×» removes one', async () => {
    const user = userEvent.setup();
    render(<Places initial={['de', 'en']} />);
    await user.click(screen.getByRole('button', { name: 'Weitere Sprache' }));
    expect(screen.getByText('de, en, none')).toBeVisible();
    expect(screen.getByRole('combobox', { name: '3. Sprache' })).toHaveFocus();
    await user.click(screen.getByRole('button', { name: 'Weitere Sprache' }));
    expect(screen.getByText('de, en, none, fr')).toBeVisible();
    // Every choice is used: nothing left to add.
    expect(screen.getByRole('button', { name: 'Weitere Sprache' })).toBeDisabled();

    await user.click(screen.getByRole('button', { name: '2. Sprache entfernen' }));
    expect(screen.getByText('de, none, fr')).toBeVisible();
    // The focus goes to the place that moved up.
    expect(screen.getByRole('combobox', { name: '2. Sprache' })).toHaveFocus();
    await user.click(screen.getByRole('button', { name: '3. Sprache entfernen' }));
    expect(screen.getByRole('combobox', { name: '2. Sprache' })).toHaveFocus();
  });

  it('keeps the last place and the places the page protects', () => {
    render(<Places initial={['de', 'en']} canRemove={(index) => index !== 1} />);
    expect(screen.getByRole('button', { name: '1. Sprache entfernen' })).toBeEnabled();
    expect(screen.getByRole('button', { name: '2. Sprache entfernen' })).toBeDisabled();
  });

  it('never removes the only place', () => {
    render(<Places initial={['de']} />);
    expect(screen.getByRole('button', { name: '1. Sprache entfernen' })).toBeDisabled();
  });

  it('is read-only when disabled: no «×», no add button', () => {
    render(<Places initial={['de', 'en']} isDisabled />);
    expect(screen.getByRole('combobox', { name: '1. Sprache' })).toBeDisabled();
    expect(screen.queryByRole('button')).toBeNull();
  });
});
