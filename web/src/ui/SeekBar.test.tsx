import { act, fireEvent, render, screen } from '@testing-library/react';
import { describe, expect, it, vi } from 'vitest';
import { chapterIndexAt, KEY_PREVIEW_MS, SeekBar, trickplayTileAt } from './SeekBar';

const chapters = [
  { startMs: 0, title: 'Anfang' },
  { startMs: 600_000 },
  { startMs: 1_200_000, title: 'Ende' },
];

function renderBar(props: Partial<Parameters<typeof SeekBar>[0]> = {}) {
  const onSeek = vi.fn();
  const view = render(
    <SeekBar
      label="Wiedergabeposition"
      valueText="5:00 von 30:00"
      currentMs={300_000}
      durationMs={1_800_000}
      buffered={[]}
      onSeek={onSeek}
      {...props}
    />,
  );
  const slider = screen.getByRole('slider', { name: 'Wiedergabeposition' });
  vi.spyOn(slider, 'getBoundingClientRect').mockReturnValue(new DOMRect(0, 0, 1000, 24));
  slider.setPointerCapture = vi.fn();
  slider.releasePointerCapture = vi.fn();
  return { ...view, slider, onSeek };
}

describe('chapterIndexAt', () => {
  it('finds the chapter that plays at a time, -1 before the first and without chapters', () => {
    expect(chapterIndexAt(chapters, 0)).toBe(0);
    expect(chapterIndexAt(chapters, 599_999)).toBe(0);
    expect(chapterIndexAt(chapters, 600_000)).toBe(1);
    expect(chapterIndexAt(chapters, 9_999_999)).toBe(2);
    expect(chapterIndexAt([{ startMs: 5000 }], 1000)).toBe(-1);
    expect(chapterIndexAt([], 1000)).toBe(-1);
  });
});

describe('SeekBar (concept 13, V-8)', () => {
  it('is a slider in seconds with the page’s value text', () => {
    const { slider } = renderBar();
    expect(slider).toHaveAttribute('aria-valuemax', '1800');
    expect(slider).toHaveAttribute('aria-valuenow', '300');
    expect(slider).toHaveAttribute('aria-valuetext', '5:00 von 30:00');
  });

  it('marks every chapter start inside the title, not the first one at 0', () => {
    const { container } = renderBar({ chapters });
    const marks = container.querySelectorAll<HTMLElement>('[data-chapter-mark]');
    expect([...marks].map((mark) => mark.style.left)).toEqual([
      `${(600_000 / 1_800_000) * 100}%`,
      `${(1_200_000 / 1_800_000) * 100}%`,
    ]);
  });

  it('draws intro and outro as stretches of the track', () => {
    const { container } = renderBar({
      segments: {
        intro: { startMs: 0, endMs: 180_000 },
        outro: { startMs: 1_620_000, endMs: 1_800_000 },
      },
    });
    const box = (segment: string) => {
      const style = container.querySelector<HTMLElement>(`[data-segment="${segment}"]`)!.style;
      return [parseFloat(style.left), parseFloat(style.width)];
    };
    const [introLeft, introWidth] = box('intro');
    const [outroLeft, outroWidth] = box('outro');
    expect(introLeft).toBe(0);
    expect(introWidth).toBeCloseTo(10);
    expect(outroLeft).toBeCloseTo(90);
    expect(outroWidth).toBeCloseTo(10);
  });

  it('shows time and chapter under the pointer; an untitled chapter is numbered by position', () => {
    const { slider, container } = renderBar({ chapters });
    // 0.5 of 30 minutes: 15:00, in the second chapter, which has no title.
    fireEvent.pointerMove(slider, { clientX: 500 });
    const preview = container.querySelector('[aria-hidden="true"]');
    expect(preview).toHaveTextContent('15:00');
    expect(preview).toHaveTextContent('Kapitel 2');
    fireEvent.pointerMove(slider, { clientX: 900 });
    expect(container.querySelector('[aria-hidden="true"]')).toHaveTextContent('27:00Ende');
    fireEvent.pointerLeave(slider);
    expect(container.querySelector('[aria-hidden="true"]')).toBeNull();
  });

  it('shows the image of the chapter under the pointer above time and name, only that one', () => {
    const { slider, container } = renderBar({
      chapters: [
        { startMs: 0, title: 'Anfang', imageUrl: '/c/0.webp' },
        { startMs: 600_000 },
        { startMs: 1_200_000, title: 'Ende', imageUrl: '/c/2.webp' },
      ],
    });
    fireEvent.pointerMove(slider, { clientX: 100 });
    const preview = () => container.querySelector('[aria-hidden="true"]')!;
    expect(
      [...preview().querySelectorAll('img')].map((image) => image.getAttribute('src')),
    ).toEqual(['/c/0.webp']);
    expect(preview().firstElementChild?.tagName).toBe('IMG');
    expect(preview().querySelector('img')).toHaveAttribute('alt', '');
    expect(preview()).toHaveTextContent('3:00Anfang');
    expect(preview().querySelector('[data-image-box]')).toBeNull();
    // The second chapter has no image yet: an empty 16:9 box keeps the field's height.
    fireEvent.pointerMove(slider, { clientX: 500 });
    expect(preview().querySelector('img')).toBeNull();
    expect(preview().firstElementChild).toHaveAttribute('data-image-box');
    expect(preview()).toHaveTextContent('15:00Kapitel 2');
    fireEvent.pointerMove(slider, { clientX: 900 });
    expect(preview().querySelector('img')).toHaveAttribute('src', '/c/2.webp');
  });

  it('drops an image that fails to load and keeps time and name', () => {
    const { slider, container } = renderBar({
      chapters: [{ startMs: 0, title: 'Anfang', imageUrl: '/c/missing.webp' }],
    });
    fireEvent.pointerMove(slider, { clientX: 100 });
    fireEvent.error(container.querySelector('img')!);
    expect(container.querySelector('img')).toBeNull();
    expect(container.querySelector('[data-image-box]')).not.toBeNull();
    fireEvent.pointerMove(slider, { clientX: 200 });
    expect(container.querySelector('img')).toBeNull();
    expect(container.querySelector('[aria-hidden="true"]')).toHaveTextContent('6:00Anfang');
  });

  it('keeps no image box when no chapter has an image', () => {
    const { slider, container } = renderBar({ chapters });
    fireEvent.pointerMove(slider, { clientX: 500 });
    expect(container.querySelector('[data-image-box]')).toBeNull();
  });

  it('shows the time alone without chapters', () => {
    const { slider, container } = renderBar();
    fireEvent.pointerMove(slider, { clientX: 100 });
    expect(container.querySelector('[aria-hidden="true"]')?.textContent).toBe('3:00');
  });

  it('seeks on press and while dragging, not on a mere hover', () => {
    const { slider, onSeek } = renderBar();
    fireEvent.pointerMove(slider, { clientX: 300 });
    expect(onSeek).not.toHaveBeenCalled();
    fireEvent.pointerDown(slider, { clientX: 250, pointerId: 1 });
    expect(onSeek).toHaveBeenLastCalledWith(450_000);
    fireEvent.pointerMove(slider, { clientX: 500, pointerId: 1 });
    expect(onSeek).toHaveBeenLastCalledWith(900_000);
    fireEvent.pointerUp(slider, { clientX: 500, pointerId: 1 });
    fireEvent.pointerMove(slider, { clientX: 700 });
    expect(onSeek).toHaveBeenCalledTimes(2);
  });

  it('takes Shift+←/→ for the chapters and leaves plain arrows to the player', () => {
    const onPreviousChapter = vi.fn();
    const onNextChapter = vi.fn();
    const outside = vi.fn();
    window.addEventListener('keydown', outside);
    try {
      const { slider } = renderBar({ chapters, onPreviousChapter, onNextChapter });
      fireEvent.keyDown(slider, { key: 'ArrowRight', shiftKey: true });
      expect(onNextChapter).toHaveBeenCalledOnce();
      fireEvent.keyDown(slider, { key: 'ArrowLeft', shiftKey: true });
      expect(onPreviousChapter).toHaveBeenCalledOnce();
      // Neither reached the player's key map.
      expect(outside).not.toHaveBeenCalled();
      fireEvent.keyDown(slider, { key: 'ArrowRight' });
      expect(onNextChapter).toHaveBeenCalledOnce();
      expect(outside).toHaveBeenCalledOnce();
    } finally {
      window.removeEventListener('keydown', outside);
    }
  });

  it('lets Shift+arrows through when the page passes no chapter handlers', () => {
    const outside = vi.fn();
    window.addEventListener('keydown', outside);
    try {
      const { slider } = renderBar();
      fireEvent.keyDown(slider, { key: 'ArrowRight', shiftKey: true });
      expect(outside).toHaveBeenCalledOnce();
    } finally {
      window.removeEventListener('keydown', outside);
    }
  });

  it('drops the field when a drag is let go outside the bar, keeps it inside', () => {
    const { slider, container } = renderBar({ chapters });
    const preview = () => container.querySelector('[aria-hidden="true"]');
    fireEvent.pointerDown(slider, { clientX: 250, clientY: 10, pointerId: 1 });
    fireEvent.pointerMove(slider, { clientX: 500, clientY: 10, pointerId: 1 });
    // The pointer leaves while the capture holds it: the field stays during the drag.
    fireEvent.pointerLeave(slider, { clientX: 500, clientY: 200 });
    expect(preview()).not.toBeNull();
    fireEvent.lostPointerCapture(slider, { clientX: 500, clientY: 200, pointerId: 1 });
    expect(preview()).toBeNull();
    // Let go over the bar: the field stays where the pointer is.
    fireEvent.pointerDown(slider, { clientX: 250, clientY: 10, pointerId: 2 });
    fireEvent.lostPointerCapture(slider, { clientX: 250, clientY: 10, pointerId: 2 });
    expect(preview()).toHaveTextContent('7:30');
  });

  it('keys the marks of two chapters at the same time apart', () => {
    const error = vi.spyOn(console, 'error').mockImplementation(() => undefined);
    try {
      const { container } = renderBar({
        chapters: [{ startMs: 0 }, { startMs: 600_000 }, { startMs: 600_000 }],
      });
      expect(container.querySelectorAll('[data-chapter-mark]')).toHaveLength(2);
      expect(error).not.toHaveBeenCalled();
    } finally {
      error.mockRestore();
    }
  });
});

// A set of 30 minutes, a point every 10 s at the keyframe 2 s after the section start: 180 tiles on
// two sheets of 10 x 10, the second with 8 rows.
const pointsMs = Array.from({ length: 180 }, (_, k) => k * 10_000 + 2000);
const trickplay = {
  tileWidth: 320,
  tileHeight: 180,
  columns: 10,
  rows: 10,
  pointsMs,
  sheets: [
    { index: 0, url: '/t/0.webp' },
    { index: 1, url: '/t/1.webp' },
  ],
};

describe('trickplayTileAt (concept 18, 7)', () => {
  it('takes the last point at or before the position, null before the first', () => {
    expect(trickplayTileAt(trickplay, 0)).toBeNull();
    expect(trickplayTileAt(trickplay, 1999)).toBeNull();
    expect(trickplayTileAt(trickplay, 2000)).toMatchObject({
      index: 0,
      sheet: 0,
      column: 0,
      row: 0,
    });
    expect(trickplayTileAt(trickplay, 11_999)).toMatchObject({ index: 0 });
    expect(trickplayTileAt(trickplay, 12_000)).toMatchObject({ index: 1, column: 1, row: 0 });
    expect(trickplayTileAt({ ...trickplay, pointsMs: [] }, 5000)).toBeNull();
  });

  it('finds sheet, row and column, and the rows of the last sheet', () => {
    // Tile 123: sheet 1, tile 23 there, row 2, column 3; the second sheet has 80 tiles, 8 rows.
    expect(trickplayTileAt(trickplay, 1_232_000)).toEqual({
      index: 123,
      sheet: 1,
      column: 3,
      row: 2,
      sheetRows: 8,
    });
    expect(trickplayTileAt(trickplay, 99 * 10_000 + 2000)).toMatchObject({
      index: 99,
      sheet: 0,
      column: 9,
      row: 9,
      sheetRows: 10,
    });
    // Past the end: the last point.
    expect(trickplayTileAt(trickplay, 99_999_999)).toMatchObject({ index: 179, row: 7 });
  });
});

describe('SeekBar with preview tiles (concept 18, 9)', () => {
  const preview = (container: HTMLElement) =>
    container.querySelector<HTMLElement>('[data-seek-preview]');
  const tile = (container: HTMLElement) =>
    container.querySelector<HTMLElement>('[data-trickplay-tile]');

  it('shows the tile under the pointer as a cut-out of its sheet, in the tile’s ratio', () => {
    const { slider, container } = renderBar({ trickplay, chapters });
    // 0.7 of 30 minutes: 21:00 = 1 260 000 ms, point 125 (at 1 252 000), sheet 1 row 2 column 5.
    fireEvent.pointerMove(slider, { clientX: 700 });
    const shown = tile(container)!;
    expect(shown).toHaveAttribute('data-trickplay-tile', '125');
    expect(shown.style.backgroundImage).toBe('url("/t/1.webp")');
    expect(shown.style.backgroundSize).toBe('1000% 800%');
    expect(shown.style.backgroundPosition).toBe(`${(5 / 9) * 100}% ${(2 / 7) * 100}%`);
    expect(shown.style.aspectRatio).toBe('320 / 180');
    expect(preview(container)).toHaveTextContent('21:00Ende');
    expect(container.querySelector('img')).toBeNull();
  });

  it('shows the chapter image, or the empty box in the tile’s ratio, where the sheet is missing', () => {
    const partial = { ...trickplay, tileHeight: 134, sheets: [{ index: 1, url: '/t/1.webp' }] };
    const { slider, container } = renderBar({
      trickplay: partial,
      chapters: [{ startMs: 0, title: 'Anfang', imageUrl: '/c/0.webp' }, { startMs: 600_000 }],
    });
    fireEvent.pointerMove(slider, { clientX: 100 });
    expect(tile(container)).toBeNull();
    expect(container.querySelector('img')).toHaveAttribute('src', '/c/0.webp');
    expect(container.querySelector('img')!.style.aspectRatio).toBe('320 / 134');
    // Chapter 2 has no image: the box keeps the height of the tiles.
    fireEvent.pointerMove(slider, { clientX: 500 });
    const box = container.querySelector<HTMLElement>('[data-image-box]')!;
    expect(box.style.aspectRatio).toBe('320 / 134');
    // On sheet 1 the tile is there.
    fireEvent.pointerMove(slider, { clientX: 600 });
    expect(tile(container)).not.toBeNull();
  });

  it('loads the next sheet ahead of the pointer, not the others', () => {
    const sources: string[] = [];
    const original = window.Image;
    class RecordingImage {
      set src(value: string) {
        sources.push(value);
      }
    }
    window.Image = RecordingImage as unknown as typeof Image;
    try {
      const { slider } = renderBar({ trickplay });
      fireEvent.pointerMove(slider, { clientX: 100 });
      expect(sources).toEqual(['/t/1.webp']);
      fireEvent.pointerMove(slider, { clientX: 700 });
      expect(sources).toEqual(['/t/1.webp']);
    } finally {
      window.Image = original;
    }
  });

  it('shows the field over the new position after a seek by key, for a moment', () => {
    vi.useFakeTimers();
    try {
      const { container, rerender } = renderBar({ trickplay });
      expect(preview(container)).toBeNull();
      const bar = (currentMs: number, keySeeks: number) => (
        <SeekBar
          label="Wiedergabeposition"
          valueText=""
          currentMs={currentMs}
          durationMs={1_800_000}
          buffered={[]}
          onSeek={() => undefined}
          trickplay={trickplay}
          keySeeks={keySeeks}
        />
      );
      rerender(bar(310_000, 1));
      expect(preview(container)).toHaveTextContent('5:10');
      expect(tile(container)).toHaveAttribute('data-trickplay-tile', '30');
      // A held key seeks again before the time is up: the field stays.
      act(() => {
        vi.advanceTimersByTime(KEY_PREVIEW_MS - 100);
      });
      rerender(bar(320_000, 2));
      act(() => {
        vi.advanceTimersByTime(KEY_PREVIEW_MS - 100);
      });
      expect(preview(container)).toHaveTextContent('5:20');
      act(() => {
        vi.advanceTimersByTime(200);
      });
      expect(preview(container)).toBeNull();
    } finally {
      vi.useRealTimers();
    }
  });

  it('lets a finger drag the preview and seeks only where it lets go', () => {
    const { slider, onSeek, container } = renderBar({ trickplay });
    fireEvent.pointerDown(slider, { clientX: 250, pointerId: 1, pointerType: 'touch' });
    expect(onSeek).not.toHaveBeenCalled();
    expect(preview(container)).toHaveTextContent('7:30');
    fireEvent.pointerMove(slider, { clientX: 500, pointerId: 1, pointerType: 'touch' });
    expect(onSeek).not.toHaveBeenCalled();
    expect(preview(container)).toHaveTextContent('15:00');
    fireEvent.pointerUp(slider, { clientX: 600, pointerId: 1, pointerType: 'touch' });
    expect(onSeek).toHaveBeenCalledOnce();
    expect(onSeek).toHaveBeenLastCalledWith(1_080_000);
  });

  it('seeks nowhere when the system cancels a finger drag', () => {
    const { slider, onSeek, container } = renderBar({ trickplay });
    fireEvent.pointerDown(slider, { clientX: 250, pointerId: 1, pointerType: 'touch' });
    fireEvent.pointerCancel(slider, { pointerId: 1, pointerType: 'touch' });
    expect(onSeek).not.toHaveBeenCalled();
    expect(preview(container)).toBeNull();
  });
});
