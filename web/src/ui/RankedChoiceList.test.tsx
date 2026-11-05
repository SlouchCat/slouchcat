import { act, fireEvent, render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { useState } from 'react';
import { describe, expect, it } from 'vitest';
import { chooseAt, RankedChoiceList, type RankedChoice } from './RankedChoiceList';

const choices: RankedChoice[] = [
  { value: '', label: 'Ohne Text' },
  { value: 'de', label: 'Deutsch' },
  { value: 'en', label: 'Englisch' },
  { value: 'fr', label: 'Französisch' },
  { value: 'ja', label: 'Japanisch' },
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
  isAllowed,
  isDisabled,
  allowEmpty,
}: {
  initial: string[];
  isAllowed?: (list: readonly string[]) => boolean;
  isDisabled?: boolean;
  allowEmpty?: boolean;
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
        isAllowed={isAllowed}
        isDisabled={isDisabled}
        allowEmpty={allowEmpty}
      />
    </>
  );
}

const optionsOf = (select: HTMLElement) => Array.from((select as HTMLSelectElement).options);

describe('RankedChoiceList', () => {
  it('shows one labelled select per place with every choice and the hint', () => {
    render(<Places initial={['de', 'en']} />);
    const first = screen.getByRole('combobox', { name: '1. Sprache' });
    expect(first).toHaveValue('de');
    expect(first).toHaveAccessibleDescription(
      'Wählst du einen Eintrag, der auf einem anderen Platz steht, tauschen die beiden die Plätze.',
    );
    expect(screen.getByRole('combobox', { name: '2. Sprache' })).toHaveValue('en');
    expect(optionsOf(first).map((option) => option.textContent)).toEqual([
      'Ohne Text',
      'Deutsch',
      'Englisch',
      'Französisch',
      'Japanisch',
    ]);
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
  });

  it('several arrow steps on a closed select end where a direct pick would', () => {
    render(<Places initial={['de', 'en', 'ja']} />);
    const first = screen.getByRole('combobox', { name: '1. Sprache' });
    act(() => first.focus());
    // ↓ three times from Deutsch: Englisch, Französisch, Japanisch; each a change event.
    for (const step of ['en', 'fr', 'ja']) fireEvent.change(first, { target: { value: step } });
    expect(screen.getByText('ja, en, de')).toBeVisible();
    expect(screen.getByRole('status')).toHaveTextContent(
      'Japanisch und Deutsch haben die Plätze getauscht.',
    );
    // Back to the start leaves the list as it was, without a swap note.
    for (const step of ['fr', 'en', 'de']) fireEvent.change(first, { target: { value: step } });
    expect(screen.getByText('de, en, ja')).toBeVisible();
    expect(screen.getByRole('status')).toBeEmptyDOMElement();
    // A new focus starts from the list as it is then.
    act(() => first.blur());
    act(() => first.focus());
    fireEvent.change(first, { target: { value: 'fr' } });
    expect(screen.getByText('fr, en, ja')).toBeVisible();
  });

  it('appends a free language first, «Ohne Text» last, focuses the new place; «×» removes one', async () => {
    const user = userEvent.setup();
    render(<Places initial={['de', 'en']} />);
    const more = screen.getByRole('button', { name: 'Weitere Sprache' });
    await user.click(more);
    expect(screen.getByText('de, en, fr')).toBeVisible();
    expect(screen.getByRole('combobox', { name: '3. Sprache' })).toHaveFocus();
    await user.click(more);
    await user.click(more);
    expect(screen.getByText('de, en, fr, ja, none')).toBeVisible();
    // Every choice is used: nothing left to add.
    expect(more).toBeDisabled();

    await user.click(screen.getByRole('button', { name: '2. Sprache (Englisch) entfernen' }));
    expect(screen.getByText('de, fr, ja, none')).toBeVisible();
    // The focus goes to the place that moved up.
    expect(screen.getByRole('combobox', { name: '2. Sprache' })).toHaveFocus();
    await user.click(screen.getByRole('button', { name: '4. Sprache (Ohne Text) entfernen' }));
    expect(screen.getByRole('combobox', { name: '3. Sprache' })).toHaveFocus();
  });

  it('keeps the page rule: choices and removals that would break it are off', () => {
    render(<Places initial={['de', 'en']} isAllowed={(list) => list.includes('de')} />);
    expect(screen.getByRole('button', { name: '1. Sprache (Deutsch) entfernen' })).toBeDisabled();
    expect(screen.getByRole('button', { name: '2. Sprache (Englisch) entfernen' })).toBeEnabled();
    const first = screen.getByRole('combobox', { name: '1. Sprache' });
    // Place 1 may only swap Deutsch away to place 2: Englisch; every other choice drops Deutsch.
    expect(
      optionsOf(first)
        .filter((option) => option.disabled)
        .map((option) => option.textContent),
    ).toEqual(['Ohne Text', 'Französisch', 'Japanisch']);
    const second = screen.getByRole('combobox', { name: '2. Sprache' });
    expect(optionsOf(second).filter((option) => option.disabled)).toHaveLength(0);
  });

  it('judges the choices from the list at focus while the arrows step', () => {
    render(<Places initial={['de', 'en']} isAllowed={(list) => list.includes('de')} />);
    const second = screen.getByRole('combobox', { name: '2. Sprache' });
    act(() => second.focus());
    // Deutsch on place 2 swaps; from the list at focus, every choice of place 2 stays allowed.
    fireEvent.change(second, { target: { value: 'de' } });
    expect(screen.getByText('en, de')).toBeVisible();
    expect(optionsOf(second).filter((option) => option.disabled)).toHaveLength(0);
    fireEvent.change(second, { target: { value: 'fr' } });
    expect(screen.getByText('de, fr')).toBeVisible();
  });

  it('never removes the only place', () => {
    render(<Places initial={['de']} />);
    expect(screen.getByRole('button', { name: '1. Sprache (Deutsch) entfernen' })).toBeDisabled();
  });

  it('removes the only place with allowEmpty and moves the focus to the add button', async () => {
    const user = userEvent.setup();
    render(<Places initial={['de']} allowEmpty />);
    await user.click(screen.getByRole('button', { name: '1. Sprache (Deutsch) entfernen' }));
    expect(screen.queryByRole('combobox')).toBeNull();
    const more = screen.getByRole('button', { name: 'Weitere Sprache' });
    expect(more).toHaveFocus();
    await user.click(more);
    expect(screen.getByRole('combobox', { name: '1. Sprache' })).toHaveFocus();
  });

  it('is read-only when disabled: no «×», no add button, no hint', () => {
    render(<Places initial={['de', 'en']} isDisabled />);
    const first = screen.getByRole('combobox', { name: '1. Sprache' });
    expect(first).toBeDisabled();
    expect(first).not.toHaveAccessibleDescription();
    expect(screen.queryByRole('button')).toBeNull();
    expect(screen.queryByRole('status')).toBeNull();
  });
});
