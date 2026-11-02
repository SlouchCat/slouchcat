import { render, screen, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { describe, expect, it } from 'vitest';
import { DataTable, type DataColumn } from './DataTable';

interface Job {
  id: string;
  name: string;
  state: string;
}

const rows: Job[] = [
  { id: 'scan', name: 'Bibliotheksscan', state: 'Bereit' },
  { id: 'fill', name: 'Metadaten auffüllen', state: '12 wartend' },
];

const columns: DataColumn<Job>[] = [
  { key: 'name', header: 'Jobart', rowHeader: true, cell: (row) => row.name },
  { key: 'state', header: 'Zustand', cell: (row) => row.state },
  {
    key: 'actions',
    header: 'Aktionen',
    hideHeader: true,
    cell: (row) => <button type="button">Starten {row.name}</button>,
  },
];

const table = () =>
  render(
    <DataTable
      caption="Jobarten"
      columns={columns}
      rows={rows}
      rowKey={(row) => row.id}
      expand={{ render: (row) => <p>Zeitplan von {row.name}</p> }}
    />,
  );

describe('DataTable with rows that fold out (concept 12, 11.2)', () => {
  it('keeps the table, its headers and the roles row and cell', () => {
    table();
    const grid = screen.getByRole('table', { name: 'Jobarten' });
    expect(
      within(grid)
        .getAllByRole('columnheader')
        .map((cell) => cell.textContent),
    ).toEqual(['Jobart', 'Zustand', 'Aktionen']);
    // Header row and two rows; the closed fold-outs are hidden rows.
    expect(within(grid).getAllByRole('row')).toHaveLength(3);
    expect(within(grid).getAllByRole('rowheader')).toHaveLength(2);
    expect(within(grid).getAllByRole('cell')).toHaveLength(4);
  });

  it('opens a row with Enter and Space into a row of its own below it', async () => {
    const user = userEvent.setup();
    table();
    const toggle = screen.getByRole('button', { name: 'Bibliotheksscan' });
    expect(toggle).toHaveAttribute('aria-expanded', 'false');
    expect(toggle.closest('th')).toHaveAttribute('scope', 'row');

    toggle.focus();
    await user.keyboard('{Enter}');
    expect(toggle).toHaveAttribute('aria-expanded', 'true');
    const detail = screen.getByText('Zeitplan von Bibliotheksscan');
    const cell = detail.closest('td');
    expect(toggle).toHaveAttribute('aria-controls', cell?.id);
    expect(cell).toHaveAttribute('colspan', '3');
    // The fold-out is the next row of the table.
    expect(cell?.parentElement?.previousElementSibling).toBe(toggle.closest('tr'));
    expect(screen.getAllByRole('row')).toHaveLength(4);

    await user.keyboard(' ');
    expect(toggle).toHaveAttribute('aria-expanded', 'false');
    expect(screen.queryByText('Zeitplan von Bibliotheksscan')).toBeNull();
  });

  it('moves with ↓ past a closed fold-out to the same control of the next row', async () => {
    const user = userEvent.setup();
    table();
    screen.getByRole('button', { name: 'Starten Bibliotheksscan' }).focus();
    await user.keyboard('{ArrowDown}');
    expect(screen.getByRole('button', { name: 'Starten Metadaten auffüllen' })).toHaveFocus();
    await user.keyboard('{ArrowUp}');
    expect(screen.getByRole('button', { name: 'Starten Bibliotheksscan' })).toHaveFocus();
  });

  it('follows the open rows of the page when it holds them', async () => {
    const user = userEvent.setup();
    const changes: [string, boolean][] = [];
    render(
      <DataTable
        caption="Jobarten"
        columns={columns}
        rows={rows}
        rowKey={(row) => row.id}
        expand={{
          render: (row) => row.state,
          expandedKeys: new Set(['fill']),
          onExpandedChange: (key, open) => changes.push([key, open]),
        }}
      />,
    );
    expect(screen.getByRole('button', { name: 'Metadaten auffüllen' })).toHaveAttribute(
      'aria-expanded',
      'true',
    );
    await user.click(screen.getByRole('button', { name: 'Bibliotheksscan' }));
    expect(changes).toEqual([['scan', true]]);
  });
});
