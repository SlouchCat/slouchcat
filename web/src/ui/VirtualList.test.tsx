import { act, fireEvent, render, screen, waitFor, within } from '@testing-library/react';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { DataTable, type DataColumn } from './DataTable';
import type { PlaceSpan } from './useVirtualPlaces';
import { VirtualList } from './VirtualList';

// The drawn part of a long list (concept 18 C-3), without a browser's layout: every row is 50 px
// high, the window 800 px, the list starts at the top of the document. A virtual list draws the
// rows in view and a screen above and below; Tab past the last drawn entry scrolls on to it.

const ROW = 50;
const VIEW = 800;

let scrollY = 0;
let scrollTo: ReturnType<typeof vi.spyOn>;

beforeEach(() => {
  scrollY = 0;
  vi.spyOn(window, 'innerHeight', 'get').mockReturnValue(VIEW);
  vi.spyOn(window, 'scrollY', 'get').mockImplementation(() => scrollY);
  vi.spyOn(window, 'pageYOffset', 'get').mockImplementation(() => scrollY);
  scrollTo = vi
    .spyOn(window, 'scrollTo')
    .mockImplementation((options?: ScrollToOptions | number) => {
      if (typeof options === 'object' && options.top !== undefined) {
        scrollY = Math.max(0, options.top);
        window.dispatchEvent(new Event('scroll'));
      }
    });
  vi.spyOn(Element.prototype, 'scrollHeight', 'get').mockImplementation(function (this: Element) {
    return this === document.documentElement ? 10_000_000 : 0;
  });
  // The virtualizer measures a drawn row by its offsetHeight.
  vi.spyOn(HTMLElement.prototype, 'offsetHeight', 'get').mockImplementation(function (
    this: HTMLElement,
  ) {
    return this.tagName === 'LI' || this.tagName === 'TR' ? ROW : 0;
  });
});

afterEach(() => vi.restoreAllMocks());

const entries = (count: number, missing: (index: number) => boolean = () => false) =>
  Array.from({ length: count }, (_, index) =>
    missing(index) ? undefined : { id: index, name: `Eintrag ${index}` },
  );

function List({
  slots,
  virtual,
  onRange,
  two = false,
  layout,
  className,
}: {
  slots: readonly ({ id: number; name: string } | undefined)[];
  virtual: boolean;
  onRange?: (span: PlaceSpan) => void;
  /** Two controls per place: the entry and «Ausblenden». */
  two?: boolean;
  layout?: 'list' | 'grid';
  className?: string;
}) {
  return (
    <VirtualList
      slots={slots}
      virtual={virtual}
      layout={layout}
      className={className}
      label="Liste"
      estimate={ROW}
      itemKey={(entry) => entry.id}
      onRange={onRange}
      renderPlaceholder={(index) => <span>Platz {index}</span>}
      renderItem={(entry) => (
        <>
          <button type="button">{entry.name}</button>
          {two && <button type="button">{`Ausblenden ${entry.id}`}</button>}
        </>
      )}
    />
  );
}

/** A stylesheet for the test's lists (jsdom computes what it declares). */
function sheet(css: string) {
  const style = document.createElement('style');
  style.textContent = css;
  document.head.append(style);
  return () => style.remove();
}

const drawn = () =>
  within(screen.getByRole('list', { name: 'Liste' }))
    .getAllByRole('listitem')
    .map((item) => Number(item.dataset.place));

describe('VirtualList', () => {
  it('draws every entry of a list that is not virtual, an empty place as its placeholder', () => {
    render(<List slots={entries(30, (index) => index === 7)} virtual={false} />);
    expect(drawn()).toHaveLength(30);
    expect(screen.getByText('Platz 7')).toBeInTheDocument();
    expect(screen.getAllByRole('listitem')[0]).toHaveAttribute('aria-setsize', '30');
  });

  it('draws only the rows near the view of a virtual list and asks for half a screen more', async () => {
    const onRange = vi.fn();
    render(<List slots={entries(2_000)} virtual onRange={onRange} />);
    // 16 rows in view, 16 drawn below (a screen), none above the top.
    await waitFor(() => expect(drawn().at(-1)).toBeGreaterThan(16));
    expect(drawn().length).toBeLessThan(60);
    expect(drawn()[0]).toBe(0);
    // The places to read: the rows in view and half a screen (8 rows) below: 0 to 23.
    await waitFor(() => expect(onRange).toHaveBeenLastCalledWith({ start: 0, end: 23 }));

    act(() => {
      scrollY = 1_000 * ROW;
      window.dispatchEvent(new Event('scroll'));
    });
    await waitFor(() => expect(drawn()).toContain(1_000));
    expect(drawn()).not.toContain(0);
    expect(drawn().length).toBeLessThan(60);
  });

  it('keeps the focused entry drawn at its place when the list scrolls far away', async () => {
    const onRange = vi.fn<(span: PlaceSpan) => void>();
    render(<List slots={entries(2_000)} virtual onRange={onRange} />);
    await waitFor(() => expect(drawn()).toContain(10));
    screen.getByRole('button', { name: 'Eintrag 10' }).focus();
    act(() => {
      scrollY = 1_000 * ROW;
      window.dispatchEvent(new Event('scroll'));
    });
    await waitFor(() => expect(drawn()).toContain(1_000));
    expect(drawn()).toContain(10);
    // Its entry stays read with the places in view (else its page would leave the range).
    await waitFor(() =>
      expect(onRange).toHaveBeenLastCalledWith(
        expect.objectContaining({ keep: { start: 10, end: 10 } }),
      ),
    );
    expect(onRange.mock.lastCall![0].start).toBeGreaterThan(900);
    // Room for the rows between them: the focused row does not move up to the others.
    const list = screen.getByRole('list', { name: 'Liste' });
    const focused = list.querySelector('[data-place="10"]')!;
    const room = focused.nextElementSibling as HTMLElement;
    expect(room).toHaveAttribute('role', 'presentation');
    expect(parseFloat(room.style.height)).toBeGreaterThan(900 * ROW);
  });

  it('Tab past the last drawn entry scrolls on and focuses the next one', async () => {
    render(<List slots={entries(2_000)} virtual />);
    await waitFor(() => expect(drawn().at(-1)).toBeGreaterThan(16));
    const last = drawn().at(-1)!;
    const button = screen.getByRole('button', { name: `Eintrag ${last}` });
    button.focus();
    fireEvent.keyDown(button, { key: 'Tab' });
    await waitFor(() =>
      expect(screen.getByRole('button', { name: `Eintrag ${last + 1}` })).toHaveFocus(),
    );
    expect(scrollTo).toHaveBeenCalled();
  });

  it('an arrow onto an empty place waits for its entry, then focuses it', async () => {
    const { rerender } = render(<List slots={entries(2_000, (index) => index === 3)} virtual />);
    await waitFor(() => expect(screen.getByText('Platz 3')).toBeInTheDocument());
    const button = screen.getByRole('button', { name: 'Eintrag 2' });
    button.focus();
    fireEvent.keyDown(button, { key: 'ArrowDown' });
    expect(button).toHaveFocus();
    rerender(<List slots={entries(2_000)} virtual />);
    await waitFor(() => expect(screen.getByRole('button', { name: 'Eintrag 3' })).toHaveFocus());
  });

  it('an arrow onto a place that is not drawn focuses the same control of it', async () => {
    render(<List slots={entries(2_000)} virtual two />);
    await waitFor(() => expect(drawn().at(-1)).toBeGreaterThan(16));
    const last = drawn().at(-1)!;
    const hide = screen.getByRole('button', { name: `Ausblenden ${last}` });
    hide.focus();
    fireEvent.keyDown(hide, { key: 'ArrowDown' });
    await waitFor(() =>
      expect(screen.getByRole('button', { name: `Ausblenden ${last + 1}` })).toHaveFocus(),
    );
    // And back up onto a drawn place: the same control again.
    const next = screen.getByRole('button', { name: `Ausblenden ${last + 1}` });
    fireEvent.keyDown(next, { key: 'ArrowUp' });
    expect(screen.getByRole('button', { name: `Ausblenden ${last}` })).toHaveFocus();
  });

  it('↑ onto a row above the drawn ones scrolls it to just below the fixed header', async () => {
    const HEADER = 64;
    // The app's header token, measured by a probe of its height.
    vi.spyOn(Element.prototype, 'getBoundingClientRect').mockImplementation(function (
      this: Element,
    ) {
      return this instanceof HTMLElement && this.style.height.includes('--sc-size-header-height')
        ? new DOMRect(0, 0, 0, HEADER)
        : new DOMRect();
    });
    render(<List slots={entries(2_000)} virtual />);
    await waitFor(() => expect(drawn().at(-1)).toBeGreaterThan(16));
    act(() => {
      scrollY = 1_000 * ROW;
      window.dispatchEvent(new Event('scroll'));
    });
    await waitFor(() => expect(drawn()).toContain(1_000));
    const top = drawn()[0]!;
    const button = screen.getByRole('button', { name: `Eintrag ${top}` });
    button.focus();
    fireEvent.keyDown(button, { key: 'ArrowUp' });
    await waitFor(() =>
      expect(screen.getByRole('button', { name: `Eintrag ${top - 1}` })).toHaveFocus(),
    );
    expect(scrollTo).toHaveBeenLastCalledWith(
      expect.objectContaining({ top: (top - 1) * ROW - HEADER }),
    );
  });

  it('the room for the rows not drawn counts each gap once', async () => {
    const GAP = 10;
    const unstyle = sheet(`ul.gapped { row-gap: ${GAP}px; }`);
    try {
      render(<List slots={entries(2_000)} virtual className="gapped" />);
      await waitFor(() => expect(drawn().at(-1)).toBeGreaterThan(16));
      const last = drawn().at(-1)!;
      const list = screen.getByRole('list', { name: 'Liste' });
      const room = list.lastElementChild as HTMLElement;
      expect(room).toHaveAttribute('role', 'presentation');
      // The rows after the last drawn one, each a row and a gap high, less the gap before them.
      const rest = 2_000 - last - 1;
      expect(parseFloat(room.style.height)).toBeCloseTo(rest * (ROW + GAP) - GAP, -1);
    } finally {
      unstyle();
    }
  });

  it('in a grid ← and → go through the controls of a place before they leave it', async () => {
    const unstyle = sheet('ul.tiles { display: grid; grid-template-columns: 100px 100px 100px; }');
    try {
      render(<List slots={entries(2_000)} virtual two layout="grid" className="tiles" />);
      await waitFor(() => expect(drawn().at(-1)).toBeGreaterThan(16));
      const tile = screen.getByRole('button', { name: 'Eintrag 0' });
      tile.focus();
      fireEvent.keyDown(tile, { key: 'ArrowRight' });
      const heart = screen.getByRole('button', { name: 'Ausblenden 0' });
      expect(heart).toHaveFocus();
      fireEvent.keyDown(heart, { key: 'ArrowRight' });
      expect(screen.getByRole('button', { name: 'Eintrag 1' })).toHaveFocus();
      // ← enters the place before at its last control, then goes on to its first.
      fireEvent.keyDown(screen.getByRole('button', { name: 'Eintrag 1' }), { key: 'ArrowLeft' });
      expect(heart).toHaveFocus();
      fireEvent.keyDown(heart, { key: 'ArrowLeft' });
      expect(tile).toHaveFocus();
      // ↓ keeps the control: the heart of the tile below.
      fireEvent.keyDown(tile, { key: 'ArrowRight' });
      fireEvent.keyDown(heart, { key: 'ArrowDown' });
      expect(screen.getByRole('button', { name: 'Ausblenden 3' })).toHaveFocus();
    } finally {
      unstyle();
    }
  });
});

describe('DataTable (virtual)', () => {
  const columns: DataColumn<{ id: number; name: string }>[] = [
    { key: 'name', header: 'Name', rowHeader: true, cell: (row) => row.name },
    {
      key: 'action',
      header: 'Aktion',
      cell: (row) => <button type="button">{`Bearbeiten ${row.id}`}</button>,
    },
  ];

  it('draws only the rows near the view, with room for the others', async () => {
    render(
      <DataTable
        caption="Sender"
        columns={columns}
        rows={entries(40_000)}
        rowKey={(row) => String(row.id)}
        layout="fixed"
        virtual
      />,
    );
    const table = screen.getByRole('table', { name: 'Sender' });
    await waitFor(() => expect(within(table).getAllByRole('row').length).toBeGreaterThan(17));
    // The header row, the drawn rows and the room below.
    expect(within(table).getAllByRole('row').length).toBeLessThan(60);
    const spacer = table.querySelector<HTMLElement>('tbody tr[aria-hidden="true"]');
    expect(parseFloat(spacer?.style.height ?? '0')).toBeGreaterThan(1_000_000);
    // Screen readers learn the whole count and where each drawn row stands (the header first).
    expect(table).toHaveAttribute('aria-rowcount', '40001');
    expect(table.querySelector('thead tr')).toHaveAttribute('aria-rowindex', '1');
    expect(table.querySelector('tr[data-place="5"]')).toHaveAttribute('aria-rowindex', '7');
  });

  it('↓ from the last drawn row scrolls on and focuses the same control of the next row', async () => {
    render(
      <DataTable
        caption="Sender"
        columns={columns}
        rows={entries(40_000)}
        rowKey={(row) => String(row.id)}
        layout="fixed"
        virtual
      />,
    );
    const table = screen.getByRole('table', { name: 'Sender' });
    await waitFor(() => expect(within(table).getAllByRole('row').length).toBeGreaterThan(17));
    const places = [...table.querySelectorAll<HTMLElement>('tr[data-place]')].map((row) =>
      Number(row.dataset.place),
    );
    const last = Math.max(...places);
    const button = screen.getByRole('button', { name: `Bearbeiten ${last}` });
    button.focus();
    fireEvent.keyDown(button, { key: 'ArrowDown' });
    await waitFor(() =>
      expect(screen.getByRole('button', { name: `Bearbeiten ${last + 1}` })).toHaveFocus(),
    );
  });

  it('↓ onto a row not drawn keeps the column of the control, not the first one', async () => {
    const two: DataColumn<{ id: number; name: string }>[] = [
      ...columns,
      {
        key: 'hide',
        header: 'Ausblenden',
        cell: (row) => <button type="button">{`Ausblenden ${row.id}`}</button>,
      },
    ];
    render(
      <DataTable
        caption="Sender"
        columns={two}
        rows={entries(40_000)}
        rowKey={(row) => String(row.id)}
        layout="fixed"
        virtual
      />,
    );
    const table = screen.getByRole('table', { name: 'Sender' });
    await waitFor(() => expect(within(table).getAllByRole('row').length).toBeGreaterThan(17));
    const places = [...table.querySelectorAll<HTMLElement>('tr[data-place]')].map((row) =>
      Number(row.dataset.place),
    );
    const last = Math.max(...places);
    const hide = screen.getByRole('button', { name: `Ausblenden ${last}` });
    hide.focus();
    fireEvent.keyDown(hide, { key: 'ArrowDown' });
    await waitFor(() =>
      expect(screen.getByRole('button', { name: `Ausblenden ${last + 1}` })).toHaveFocus(),
    );
  });
});
