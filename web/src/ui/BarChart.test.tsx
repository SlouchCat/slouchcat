import { render, screen, within } from '@testing-library/react';
import { describe, expect, it } from 'vitest';
import { BarChart } from './BarChart';

const bars = [
  { key: 'mon', label: 'Montag', tick: 'Mo', value: 30, valueText: '30 min', tooltip: 'Mo' },
  { key: 'tue', label: 'Dienstag', tick: 'Di', value: 90, valueText: '90 min', tooltip: 'Di' },
];

describe('BarChart', () => {
  it.each(['vertical', 'horizontal'] as const)(
    'keeps the numbers in a table for screen readers (%s)',
    (orientation) => {
      render(
        <BarChart
          title="Nach Wochentag"
          bars={bars}
          columns={['Wochentag', 'Minuten']}
          orientation={orientation}
        />,
      );
      const figure = screen.getByRole('figure', { name: 'Nach Wochentag' });
      const table = within(figure).getByRole('table', { name: 'Nach Wochentag' });
      expect(
        within(table)
          .getAllByRole('rowheader')
          .map((cell) => cell.textContent),
      ).toEqual(['Montag', 'Dienstag']);
      expect(
        within(table)
          .getAllByRole('cell')
          .map((cell) => cell.textContent),
      ).toEqual(['30 min', '90 min']);
    },
  );
});
