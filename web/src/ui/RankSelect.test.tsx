import { render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { useState } from 'react';
import { describe, expect, it } from 'vitest';
import { RankSelect, rankTo, type RankMode } from './RankSelect';

describe('rankTo (concept 12, S-16)', () => {
  const list = ['de', 'en', 'fr', 'ja'];
  it('moves: the entries in between move along by one', () => {
    expect(rankTo(list, 3, 0, 'move')).toEqual(['ja', 'de', 'en', 'fr']);
    expect(rankTo(list, 0, 2, 'move')).toEqual(['en', 'fr', 'de', 'ja']);
  });
  it('swaps: the entry on the chosen position takes the old one', () => {
    expect(rankTo(list, 3, 0, 'swap')).toEqual(['ja', 'en', 'fr', 'de']);
  });
  it('leaves the list as it is for the same or an impossible position', () => {
    expect(rankTo(list, 1, 1, 'move')).toEqual(list);
    expect(rankTo(list, 1, 9, 'swap')).toEqual(list);
  });
});

function Languages({ mode }: { mode: RankMode }) {
  const [order, setOrder] = useState(['Deutsch', 'Englisch', 'Französisch']);
  return (
    <>
      <p>{order.join(', ')}</p>
      {order.map((name, index) => (
        <RankSelect
          key={name}
          name={name}
          position={index}
          count={order.length}
          mode={mode}
          onChange={(to) => setOrder(rankTo(order, index, to, mode))}
        />
      ))}
    </>
  );
}

describe('RankSelect', () => {
  it('chooses the position of an entry; the others move along', async () => {
    const user = userEvent.setup();
    render(<Languages mode="move" />);
    const select = screen.getByRole('combobox', { name: 'Platz für Französisch' });
    expect(select).toHaveValue('2');
    expect(select).toHaveAccessibleDescription('Die anderen Einträge rücken nach.');
    expect(
      screen
        .getAllByRole('option', { name: /^Platz/ })
        .slice(0, 3)
        .map((o) => o.textContent),
    ).toEqual(['Platz 1', 'Platz 2', 'Platz 3']);
    await user.selectOptions(select, 'Platz 1');
    expect(screen.getByText('Französisch, Deutsch, Englisch')).toBeVisible();
    expect(screen.getByRole('combobox', { name: 'Platz für Französisch' })).toHaveValue('0');
  });

  it('swaps with the entry on the chosen position', async () => {
    const user = userEvent.setup();
    render(<Languages mode="swap" />);
    const select = screen.getByRole('combobox', { name: 'Platz für Französisch' });
    expect(select).toHaveAccessibleDescription(
      'Der Eintrag auf dem gewählten Platz tauscht mit diesem.',
    );
    await user.selectOptions(select, 'Platz 1');
    expect(screen.getByText('Französisch, Englisch, Deutsch')).toBeVisible();
  });
});
