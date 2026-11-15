import { act, fireEvent, render, screen } from '@testing-library/react';
import { useState } from 'react';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { PlayerPanel } from '@/ui/aria';
import { DOUBLE_TAP_MS, usePhoneTap } from './usePhoneTap';

interface Spies {
  togglePlay: () => void;
  toggleFullscreen: () => void;
  seekBy: (seconds: number) => void;
  hide: () => void;
}

/** The picture with its pointer handlers and a panel beside it that a press outside closes. */
function Picture({ phone, spies }: { phone: boolean; spies: Spies }) {
  const [open, setOpen] = useState(true);
  const taps = usePhoneTap({
    phone,
    hidden: false,
    showOsd: () => undefined,
    wantsPlaying: () => true,
    ...spies,
  });
  return (
    <div>
      {/* eslint-disable-next-line jsx-a11y/media-has-caption */}
      <video data-testid="picture" {...taps} />
      {open && (
        <PlayerPanel title="Ton" onClose={() => setOpen(false)}>
          <button type="button">Deutsch</button>
        </PlayerPanel>
      )}
    </div>
  );
}

function renderPicture(phone = false) {
  const spies = {
    togglePlay: vi.fn(),
    toggleFullscreen: vi.fn(),
    seekBy: vi.fn(),
    hide: vi.fn(),
  };
  render(<Picture phone={phone} spies={spies} />);
  return { spies, picture: screen.getByTestId('picture') };
}

/** A press of the mouse: pointerdown, mousedown, pointerup; returns the mousedown. */
function click(picture: HTMLElement, detail = 1) {
  const down = new MouseEvent('mousedown', { bubbles: true, cancelable: true, detail });
  act(() => {
    fireEvent.pointerDown(picture, { button: 0, pointerType: 'mouse' });
    fireEvent(picture, down);
    fireEvent.pointerUp(picture, { button: 0, pointerType: 'mouse' });
  });
  return down;
}

describe('usePhoneTap: a press that closes a panel only closes it (concept 23 P-2)', () => {
  beforeEach(() => vi.useFakeTimers());
  afterEach(() => vi.useRealTimers());

  it('a click on the picture closes the panel without play or pause, and keeps the focus', () => {
    const { spies, picture } = renderPicture();
    const down = click(picture);
    expect(screen.queryByRole('dialog')).toBeNull();
    expect(spies.togglePlay).not.toHaveBeenCalled();
    // The default of the mousedown would move the focus to the body.
    expect(down.defaultPrevented).toBe(true);

    // The panel is gone: the next click plays or pauses, and keeps its default.
    const next = click(picture);
    expect(spies.togglePlay).toHaveBeenCalledOnce();
    expect(next.defaultPrevented).toBe(false);
  });

  it('a double click whose first click closed the panel neither plays nor goes full screen', () => {
    const { spies, picture } = renderPicture();
    click(picture, 1);
    click(picture, 2);
    act(() => {
      fireEvent.doubleClick(picture);
    });
    expect(spies.togglePlay).not.toHaveBeenCalled();
    expect(spies.toggleFullscreen).not.toHaveBeenCalled();

    // A double click of its own goes full screen as before.
    click(picture, 1);
    click(picture, 2);
    act(() => {
      fireEvent.doubleClick(picture);
    });
    expect(spies.toggleFullscreen).toHaveBeenCalledOnce();
  });

  it('a tap of a finger in the phone layout that closes the panel neither hides nor seeks', () => {
    const { spies, picture } = renderPicture(true);
    const tap = () =>
      act(() => {
        fireEvent.pointerDown(picture, { button: 0, pointerType: 'touch', clientX: 1 });
        fireEvent.pointerUp(picture, { button: 0, pointerType: 'touch', clientX: 1 });
      });
    tap();
    expect(screen.queryByRole('dialog')).toBeNull();
    act(() => {
      vi.advanceTimersByTime(DOUBLE_TAP_MS + 10);
    });
    expect(spies.hide).not.toHaveBeenCalled();
    expect(spies.togglePlay).not.toHaveBeenCalled();

    // The closing tap does not count as the first of two: one more tap is a single tap.
    tap();
    act(() => {
      vi.advanceTimersByTime(DOUBLE_TAP_MS + 10);
    });
    expect(spies.seekBy).not.toHaveBeenCalled();
    expect(spies.hide).toHaveBeenCalledOnce();
  });
});
