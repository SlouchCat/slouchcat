import { act, fireEvent, render, screen } from '@testing-library/react';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { TimeShiftSlider } from './TimeShiftSlider';

const props = {
  label: 'Zeitversatz',
  maxSeconds: 1800,
  offset: 0,
  valueText: (offset: number) => `${offset} s zurück`,
  shownText: (offset: number) => `−${offset}`,
  startLabel: '−30:00',
  endLabel: 'Live',
};

describe('TimeShiftSlider', () => {
  afterEach(() => vi.useRealTimers());

  it('seeks once the slider rests, with the offset of its last position', () => {
    vi.useFakeTimers();
    const onSeek = vi.fn();
    render(<TimeShiftSlider {...props} onSeek={onSeek} />);
    const slider = screen.getByRole('slider', { name: 'Zeitversatz' });
    fireEvent.change(slider, { target: { value: '-5' } });
    fireEvent.change(slider, { target: { value: '-10' } });
    expect(slider).toHaveAttribute('aria-valuetext', '10 s zurück');
    expect(onSeek).not.toHaveBeenCalled();
    act(() => {
      vi.advanceTimersByTime(250);
    });
    expect(onSeek).toHaveBeenCalledTimes(1);
    expect(onSeek).toHaveBeenCalledWith(10);
  });

  it('shows the player offset, never more than the window', () => {
    render(<TimeShiftSlider {...props} offset={2400} onSeek={vi.fn()} />);
    expect(screen.getByRole('slider')).toHaveValue('-1800');
  });
});
