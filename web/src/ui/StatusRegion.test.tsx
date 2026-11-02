import { act, render, renderHook, screen } from '@testing-library/react';
import { describe, expect, it } from 'vitest';
import { StatusRegion, useStatusMessage } from './StatusRegion';

describe('StatusRegion', () => {
  it('stays in the accessibility tree while empty', () => {
    render(<StatusRegion message={null} />);
    const region = screen.getByRole('status');
    expect(region).toBeEmptyDOMElement();
    expect(region).toBeVisible();
  });

  it('shows the message inside the region, not as a live region of its own', () => {
    render(<StatusRegion message={{ text: 'Gespeichert.', key: 1 }} />);
    expect(screen.getByRole('status')).toHaveTextContent('Gespeichert.');
    expect(screen.getAllByRole('status')).toHaveLength(1);
  });

  it('has an alert variant', () => {
    render(<StatusRegion role="alert" message={{ text: 'Fehler.', tone: 'danger', key: 1 }} />);
    expect(screen.getByRole('alert')).toHaveTextContent('Fehler.');
  });

  it('renders the same text again as a new node, so it is announced again', () => {
    const { result } = renderHook(() => useStatusMessage());
    act(() => result.current[1]('Gespeichert.'));
    const first = result.current[0];
    const { rerender } = render(<StatusRegion message={first} />);
    const before = screen.getByText('Gespeichert.');
    act(() => result.current[1]('Gespeichert.'));
    expect(result.current[0]?.key).toBe((first?.key ?? 0) + 1);
    rerender(<StatusRegion message={result.current[0]} />);
    expect(screen.getByText('Gespeichert.')).not.toBe(before);
    expect(before).not.toBeInTheDocument();
  });

  it('empties the region with say(null)', () => {
    const { result } = renderHook(() => useStatusMessage());
    act(() => result.current[1]('Gelöscht.', 'warning'));
    expect(result.current[0]).toMatchObject({ text: 'Gelöscht.', tone: 'warning' });
    act(() => result.current[1](null));
    expect(result.current[0]).toBeNull();
  });
});
