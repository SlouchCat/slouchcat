import { describe, expect, it } from 'vitest';
import { nextInOrder } from './spatial';

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
