import { fireEvent, render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { createRef } from 'react';
import { describe, expect, it, vi } from 'vitest';
import { clampVolume, VolumeControl } from './VolumeControl';

function renderControl(props: Partial<Parameters<typeof VolumeControl>[0]> = {}) {
  const onVolumeChange = vi.fn();
  const onToggleMute = vi.fn();
  render(
    <VolumeControl
      volume={0.5}
      muted={false}
      onVolumeChange={onVolumeChange}
      onToggleMute={onToggleMute}
      {...props}
    />,
  );
  return { onVolumeChange, onToggleMute };
}

const slider = () => screen.getByRole('slider', { name: 'Lautstärke' });

describe('clampVolume', () => {
  it('keeps a volume in 0 … max, rounded to hundredths', () => {
    expect(clampVolume(0.1 + 0.05, 1)).toBe(0.15);
    expect(clampVolume(-0.2, 1)).toBe(0);
    expect(clampVolume(1.2, 1)).toBe(1);
    expect(clampVolume(1.7, 1.5)).toBe(1.5);
    expect(clampVolume(Number.NaN, 1)).toBe(0);
  });
});

describe('VolumeControl (concept 13, V-3)', () => {
  it('mutes with the button, which says whether it is pressed', async () => {
    const { onToggleMute } = renderControl({ muted: true });
    const button = screen.getByRole('button', { name: 'Ton aus' });
    expect(button).toHaveAttribute('aria-pressed', 'true');
    await userEvent.click(button);
    expect(onToggleMute).toHaveBeenCalledOnce();
  });

  it('is a slider in percent; muted it stands at 0', () => {
    renderControl({ volume: 0.8, muted: true });
    expect(slider()).toHaveAttribute('aria-valuenow', '0');
    expect(slider()).toHaveAttribute('aria-valuemax', '100');
    expect(slider()).toHaveAttribute('aria-valuetext', 'Lautstärke 0 %');
  });

  it('moves 5 % with ←/→, jumps with Home and End, and keeps those keys', () => {
    const outside = vi.fn();
    window.addEventListener('keydown', outside);
    try {
      const { onVolumeChange } = renderControl({ volume: 0.5 });
      fireEvent.keyDown(slider(), { key: 'ArrowRight' });
      expect(onVolumeChange).toHaveBeenLastCalledWith(0.55);
      fireEvent.keyDown(slider(), { key: 'ArrowLeft' });
      expect(onVolumeChange).toHaveBeenLastCalledWith(0.5);
      fireEvent.keyDown(slider(), { key: 'PageDown' });
      expect(onVolumeChange).toHaveBeenLastCalledWith(0.4);
      fireEvent.keyDown(slider(), { key: 'Home' });
      expect(onVolumeChange).toHaveBeenLastCalledWith(0);
      fireEvent.keyDown(slider(), { key: 'End' });
      expect(onVolumeChange).toHaveBeenLastCalledWith(1);
      // The page's key map (←/→ seek) never saw them.
      expect(outside).not.toHaveBeenCalled();
      // Shift+arrows stay with the page (chapters).
      fireEvent.keyDown(slider(), { key: 'ArrowRight', shiftKey: true });
      expect(onVolumeChange).toHaveBeenCalledTimes(5);
      expect(outside).toHaveBeenCalledOnce();
    } finally {
      window.removeEventListener('keydown', outside);
    }
  });

  it('lets ↑ and ↓ go on to the page: they change the zone in the button row (11.2)', () => {
    const parent = vi.fn<(key: string) => void>();
    const onVolumeChange = vi.fn();
    render(
      // eslint-disable-next-line jsx-a11y/no-static-element-interactions
      <div onKeyDown={(event) => parent(event.key)}>
        <VolumeControl
          volume={0.5}
          muted={false}
          onVolumeChange={onVolumeChange}
          onToggleMute={vi.fn()}
        />
      </div>,
    );
    fireEvent.keyDown(slider(), { key: 'ArrowUp' });
    fireEvent.keyDown(slider(), { key: 'ArrowDown' });
    expect(parent.mock.calls).toEqual([['ArrowUp'], ['ArrowDown']]);
    expect(onVolumeChange).not.toHaveBeenCalled();
  });

  it('takes part in a roving tab index through the mute button', () => {
    const button = createRef<HTMLButtonElement>();
    const sliderElement = createRef<HTMLDivElement>();
    const props = {
      volume: 0.5,
      muted: false,
      onVolumeChange: vi.fn(),
      onToggleMute: vi.fn(),
      buttonRef: button,
      sliderRef: sliderElement,
    };
    const { rerender } = render(<VolumeControl {...props} tabIndex={-1} />);
    expect(button.current).toBe(screen.getByRole('button', { name: 'Ton aus' }));
    expect(sliderElement.current).toBe(slider());
    // Not the row's stop: neither is in the Tab order.
    expect(button.current).toHaveAttribute('tabindex', '-1');
    expect(slider()).toHaveAttribute('tabindex', '-1');
    // The row's stop: the button, and the slider right after it.
    rerender(<VolumeControl {...props} tabIndex={0} />);
    expect(button.current).not.toHaveAttribute('tabindex', '-1');
    expect(slider()).toHaveAttribute('tabindex', '0');
  });

  it('steps on from the last value asked for until the page shows it', () => {
    const onVolumeChange = vi.fn();
    const props = { muted: false, onVolumeChange, onToggleMute: vi.fn() };
    const { rerender } = render(<VolumeControl volume={1} {...props} />);
    // Two quick presses before the media element reports the first one.
    fireEvent.keyDown(slider(), { key: 'ArrowLeft' });
    fireEvent.keyDown(slider(), { key: 'ArrowLeft' });
    expect(onVolumeChange.mock.calls).toEqual([[0.95], [0.9]]);
    // The first one arrives; the second is still on its way.
    rerender(<VolumeControl volume={0.95} {...props} />);
    fireEvent.keyDown(slider(), { key: 'ArrowLeft' });
    expect(onVolumeChange).toHaveBeenLastCalledWith(0.85);
    // The second arrives, the third is on its way.
    rerender(<VolumeControl volume={0.9} {...props} />);
    fireEvent.keyDown(slider(), { key: 'ArrowLeft' });
    expect(onVolumeChange).toHaveBeenLastCalledWith(0.8);
    // The page caught up, then set the volume itself (its own key): the next step starts there.
    rerender(<VolumeControl volume={0.8} {...props} />);
    rerender(<VolumeControl volume={0.5} {...props} />);
    fireEvent.keyDown(slider(), { key: 'ArrowLeft' });
    expect(onVolumeChange).toHaveBeenLastCalledWith(0.45);
  });

  it('forgets unanswered values when the slider loses the focus or after a second', () => {
    vi.useFakeTimers();
    try {
      const { onVolumeChange } = renderControl({ volume: 0.5 });
      fireEvent.keyDown(slider(), { key: 'ArrowRight' });
      expect(onVolumeChange).toHaveBeenLastCalledWith(0.55);
      // The page never answers: after a blur the next step starts from the shown 50 %.
      fireEvent.blur(slider());
      fireEvent.keyDown(slider(), { key: 'ArrowRight' });
      expect(onVolumeChange).toHaveBeenLastCalledWith(0.55);
      // Still no answer, and more than a second later: again from 50 %.
      vi.advanceTimersByTime(1001);
      fireEvent.keyDown(slider(), { key: 'ArrowRight' });
      expect(onVolumeChange).toHaveBeenLastCalledWith(0.55);
      // Within the second the step goes on from the value asked for.
      fireEvent.keyDown(slider(), { key: 'ArrowRight' });
      expect(onVolumeChange).toHaveBeenLastCalledWith(0.6);
    } finally {
      vi.useRealTimers();
    }
  });

  it('stops at 100 % without boost', () => {
    const plain = renderControl({ volume: 1 });
    fireEvent.keyDown(slider(), { key: 'ArrowRight' });
    expect(plain.onVolumeChange).toHaveBeenLastCalledWith(1);
  });

  it('with boost goes to 150 % and warns above 100 %', () => {
    const { onVolumeChange } = renderControl({ volume: 1.3, boost: true });
    expect(slider()).toHaveAttribute('aria-valuemax', '150');
    expect(slider()).toHaveAttribute('aria-valuenow', '130');
    expect(slider()).toHaveAttribute(
      'aria-valuetext',
      'Lautstärke 130 %, verstärkt, kann verzerren',
    );
    expect(screen.getByText('130 %')).toHaveAttribute('data-boosted');
    fireEvent.keyDown(slider(), { key: 'End' });
    expect(onVolumeChange).toHaveBeenLastCalledWith(1.5);
  });

  it('no warning at or below 100 %, even with boost', () => {
    renderControl({ volume: 1, boost: true });
    expect(slider()).toHaveAttribute('aria-valuetext', 'Lautstärke 100 %');
    expect(screen.getByText('100 %')).not.toHaveAttribute('data-boosted');
  });

  it('sets the volume where the pointer presses and drags, measured on the track', () => {
    const { onVolumeChange } = renderControl({ boost: true });
    const element = slider();
    // The slider is wider than its track (a margin at both ends); only the track counts.
    vi.spyOn(element, 'getBoundingClientRect').mockReturnValue(new DOMRect(-8, 0, 166, 24));
    vi.spyOn(element.firstElementChild!, 'getBoundingClientRect').mockReturnValue(
      new DOMRect(0, 0, 150, 4),
    );
    element.setPointerCapture = vi.fn();
    element.releasePointerCapture = vi.fn();
    fireEvent.pointerDown(element, { clientX: 30, pointerId: 1 });
    expect(onVolumeChange).toHaveBeenLastCalledWith(0.3);
    fireEvent.pointerMove(element, { clientX: 120, pointerId: 1 });
    expect(onVolumeChange).toHaveBeenLastCalledWith(1.2);
    fireEvent.pointerUp(element, { clientX: 120, pointerId: 1 });
    fireEvent.pointerMove(element, { clientX: 60 });
    expect(onVolumeChange).toHaveBeenCalledTimes(2);
  });

  it('shows only the mute button for a device that cannot set the volume', () => {
    renderControl({ muteOnly: true });
    expect(screen.getByRole('button', { name: 'Ton aus' })).toBeInTheDocument();
    expect(screen.queryByRole('slider')).toBeNull();
  });
});
