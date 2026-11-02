import { render, screen, within } from '@testing-library/react';
import { describe, expect, it } from 'vitest';
import { HeatGrid } from './HeatGrid';

describe('HeatGrid (concept 12, «Wann geschaut wird»)', () => {
  it('reads as a table of weekdays and hours, Monday 00:00 first', () => {
    const values = Array.from({ length: 168 }, (_, index) => index);
    render(<HeatGrid title="Wann geschaut wird" values={values} formatValue={(v) => `${v} min`} />);
    const table = screen.getByRole('table', { name: 'Wann geschaut wird' });
    const headers = within(table).getAllByRole('columnheader');
    expect(headers).toHaveLength(25);
    expect(headers[0]).toHaveTextContent('Wochentag');
    expect(headers[1]).toHaveTextContent('0 Uhr');
    expect(headers[24]).toHaveTextContent('23 Uhr');
    const days = within(table)
      .getAllByRole('rowheader')
      .map((cell) => cell.textContent);
    expect(days).toEqual([
      'Montag',
      'Dienstag',
      'Mittwoch',
      'Donnerstag',
      'Freitag',
      'Samstag',
      'Sonntag',
    ]);
    const rows = within(table).getAllByRole('row');
    expect(within(rows[1]!).getAllByRole('cell')[0]).toHaveTextContent('0 min');
    expect(within(rows[7]!).getAllByRole('cell')[23]).toHaveTextContent('167 min');
  });
});
