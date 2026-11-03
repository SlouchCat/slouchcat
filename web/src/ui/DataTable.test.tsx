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

  it('leaves ↑ and ↓ to a select inside a fold-out', async () => {
    const user = userEvent.setup();
    render(
      <DataTable
        caption="Jobarten"
        columns={columns}
        rows={rows}
        rowKey={(row) => row.id}
        expand={{
          render: (row) => (
            <select aria-label={`Takt von ${row.name}`} defaultValue="6">
              <option value="1">1 h</option>
              <option value="6">6 h</option>
              <option value="24">24 h</option>
            </select>
          ),
        }}
      />,
    );
    await user.click(screen.getByRole('button', { name: 'Bibliotheksscan' }));
    const select = screen.getByRole('combobox', { name: 'Takt von Bibliotheksscan' });
    select.focus();
    const event = new KeyboardEvent('keydown', {
      key: 'ArrowDown',
      bubbles: true,
      cancelable: true,
    });
    select.dispatchEvent(event);
    expect(event.defaultPrevented).toBe(false);
    expect(select).toHaveFocus();
  });

  it('leaves ↑ and ↓ to a text field in a row', () => {
    render(
      <DataTable
        caption="Notizen"
        columns={[
          { key: 'name', header: 'Jobart', rowHeader: true, cell: (row: Job) => row.name },
          {
            key: 'note',
            header: 'Notiz',
            cell: (row: Job) => <input aria-label={`Notiz zu ${row.name}`} />,
          },
        ]}
        rows={rows}
        rowKey={(row) => row.id}
      />,
    );
    const field = screen.getByRole('textbox', { name: 'Notiz zu Bibliotheksscan' });
    field.focus();
    const event = new KeyboardEvent('keydown', {
      key: 'ArrowDown',
      bubbles: true,
      cancelable: true,
    });
    field.dispatchEvent(event);
    expect(event.defaultPrevented).toBe(false);
    expect(field).toHaveFocus();
  });
});

describe('DataTable with a fixed layout (K12-9)', () => {
  const sized: DataColumn<Job>[] = [
    { key: 'name', header: 'Jobart', rowHeader: true, width: '40%', cell: (row) => row.name },
    { key: 'state', header: 'Zustand', cell: (row) => row.state },
    {
      key: 'actions',
      header: 'Aktionen',
      hideHeader: true,
      width: '8rem',
      cell: (row) => <button type="button">Starten {row.name}</button>,
    },
  ];
  const widthsOf = (name: string) => {
    const grid = screen.getByRole('table', { name });
    return {
      grid,
      cols: Array.from(grid.querySelectorAll('colgroup > col'), (col) =>
        (col as HTMLTableColElement).style.getPropertyValue('--column-width'),
      ),
    };
  };

  it('takes the column widths from the definitions, not from the content', () => {
    render(
      <>
        <DataTable
          caption="Bibliothek"
          columns={sized}
          rows={rows}
          rowKey={(row) => row.id}
          layout="fixed"
        />
        <DataTable
          caption="Metadaten"
          columns={sized}
          rows={[{ id: 'long', name: 'Ein sehr viel längerer Name einer Jobart', state: '-' }]}
          rowKey={(row) => row.id}
          layout="fixed"
        />
      </>,
    );
    const first = widthsOf('Bibliothek');
    const second = widthsOf('Metadaten');
    expect(first.grid).toHaveClass('fixed');
    // One <col> per column; a column without a width shares what is left.
    expect(first.cols).toEqual(['40%', '', '8rem']);
    // Two groups with the same columns get the same widths, whatever their rows hold.
    expect(second.cols).toEqual(first.cols);
    // The headers and roles stay as they are.
    expect(
      within(first.grid)
        .getAllByRole('columnheader')
        .map((cell) => cell.textContent),
    ).toEqual(['Jobart', 'Zustand', 'Aktionen']);
  });

  it('sizes the columns from their content by default and ignores the widths', () => {
    render(<DataTable caption="Jobarten" columns={sized} rows={rows} rowKey={(row) => row.id} />);
    const { grid, cols } = widthsOf('Jobarten');
    expect(grid).not.toHaveClass('fixed');
    expect(cols).toEqual([]);
  });
});
