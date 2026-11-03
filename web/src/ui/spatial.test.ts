import { afterEach, describe, expect, it, vi } from 'vitest';
import { guardAutoRestore, loadSpatialNavigation, nextInOrder } from './spatial';

// A row that wraps: play, back and download on the first line, manage under play. By geometry ↓
// from play would land on manage; in order → walks the row and ↑/↓ leave it.
const row = () => {
  const container = document.createElement('div');
  const siblings = ['play', 'back', 'download', 'manage'].map((focusKey) => {
    const node = document.createElement('button');
    container.append(node);
    return { focusKey, node };
  });
  // Registered out of DOM order (a lazily loaded button registers last).
  return [3, 0, 2, 1].flatMap((index) => siblings[index] ?? []);
};

describe('nextInOrder', () => {
  it('walks the siblings in DOM order with ← and →', () => {
    const siblings = row();
    expect(nextInOrder('right', 'play', siblings)?.focusKey).toBe('back');
    expect(nextInOrder('right', 'download', siblings)?.focusKey).toBe('manage');
    expect(nextInOrder('left', 'manage', siblings)?.focusKey).toBe('download');
    expect(nextInOrder('left', 'back', siblings)?.focusKey).toBe('play');
  });

  it('leaves the row at its ends and on ↑ and ↓', () => {
    const siblings = row();
    expect(nextInOrder('left', 'play', siblings)).toBeNull();
    expect(nextInOrder('right', 'manage', siblings)).toBeNull();
    expect(nextInOrder('down', 'play', siblings)).toBeNull();
    expect(nextInOrder('up', 'manage', siblings)).toBeNull();
  });

  it('leaves the row when the current child is not registered in it', () => {
    expect(nextInOrder('right', 'elsewhere', row())).toBeNull();
  });
});

// norigin's restore of a group's focus, 300 ms after its focused child went away, acts only when
// the focus is really lost (spatial.tsx guardAutoRestore).
describe('guardAutoRestore', () => {
  afterEach(() => {
    vi.useRealTimers();
    document.body.replaceChildren();
  });

  type Restore = ((focusKey: string) => void) & { cancel(): void };

  const service = () => {
    const setFocus = vi.fn((focusKey: string) => Promise.resolve(void focusKey));
    const navigation = {
      setFocus,
      setFocusDebounced: Object.assign(vi.fn(), { cancel: vi.fn() }) as unknown,
    };
    expect(guardAutoRestore(navigation)).toBe(true);
    return { setFocus, restore: navigation.setFocusDebounced as Restore };
  };

  it('is in place on the norigin service the app loads', async () => {
    await loadSpatialNavigation();
    const { SpatialNavigation } = await import('./spatialRuntime');
    const scheduler = (
      SpatialNavigation as unknown as { setFocusDebounced?: { guarded?: boolean } }
    ).setFocusDebounced;
    expect(scheduler?.guarded).toBe(true);
  });

  it('restores after 300 ms when the focus is lost', () => {
    vi.useFakeTimers();
    const { setFocus, restore } = service();
    restore('content');
    vi.advanceTimersByTime(299);
    expect(setFocus).not.toHaveBeenCalled();
    vi.advanceTimersByTime(1);
    expect(setFocus).toHaveBeenCalledWith('content');
  });

  it('leaves the focus where the reader put it in the meantime', () => {
    vi.useFakeTimers();
    const { setFocus, restore } = service();
    restore('content');
    const save = document.body.appendChild(document.createElement('button'));
    save.focus();
    vi.advanceTimersByTime(300);
    expect(save).toHaveFocus();
    expect(setFocus).not.toHaveBeenCalled();
  });

  it('is cancelled by cancel, which norigin calls on every setFocus', () => {
    vi.useFakeTimers();
    const { setFocus, restore } = service();
    restore('content');
    restore.cancel();
    vi.advanceTimersByTime(300);
    expect(setFocus).not.toHaveBeenCalled();
  });

  it('leaves a service without the scheduler alone', () => {
    expect(guardAutoRestore({ setFocus: vi.fn() })).toBe(false);
  });
});
