import { describe, expect, it } from 'vitest';
import { cubicBezier } from './reveal';

describe('cubicBezier', () => {
  it('matches the CSS endpoints and the linear curve', () => {
    const linear = cubicBezier(0, 0, 1, 1);
    for (const x of [0, 0.1, 0.25, 0.5, 0.9, 1]) expect(linear(x)).toBeCloseTo(x, 4);
    const standard = cubicBezier(0.2, 0, 0, 1);
    expect(standard(0)).toBe(0);
    expect(standard(1)).toBe(1);
  });

  it('follows motion.easing.standard: quick start, long settle, monotonic', () => {
    const standard = cubicBezier(0.2, 0, 0, 1);
    // Reference values of cubic-bezier(0.2, 0, 0, 1), sampled densely from the parametric curve.
    expect(standard(0.25)).toBeCloseTo(0.6072, 3);
    expect(standard(0.5)).toBeCloseTo(0.8778, 3);
    let previous = 0;
    for (let x = 0; x <= 1; x += 0.05) {
      const y = standard(x);
      expect(y).toBeGreaterThanOrEqual(previous);
      previous = y;
    }
  });
});
