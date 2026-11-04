import { renderHook } from '@testing-library/react';
import { StrictMode } from 'react';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { NIGHT_MODE, useAudioGraph } from './useAudioGraph';

/** A node that records what it is connected to. */
class FakeNode {
  readonly targets: FakeNode[] = [];
  constructor(readonly name: string) {}
  connect(target: FakeNode) {
    this.targets.push(target);
    return target;
  }
  disconnect() {
    this.targets.length = 0;
  }
}

const param = () => ({ value: 0 });

class FakeContext {
  static instances: FakeContext[] = [];
  /** Like Chromium: an element feeds one source node for its whole life. */
  static connected = new WeakSet<HTMLMediaElement>();
  state: AudioContextState = 'suspended';
  readonly destination = new FakeNode('destination');
  readonly sources: HTMLMediaElement[] = [];
  source = new FakeNode('source');
  compressor = Object.assign(new FakeNode('compressor'), {
    threshold: param(),
    knee: param(),
    ratio: param(),
    attack: param(),
    release: param(),
  });
  gain = Object.assign(new FakeNode('gain'), { gain: param() });
  resume = vi.fn(() => {
    this.state = 'running';
    return Promise.resolve();
  });
  close = vi.fn(() => Promise.resolve());
  constructor() {
    FakeContext.instances.push(this);
  }
  createMediaElementSource(element: HTMLMediaElement) {
    if (FakeContext.connected.has(element)) {
      throw new DOMException(
        'HTMLMediaElement already connected previously to a different MediaElementSourceNode.',
        'InvalidStateError',
      );
    }
    FakeContext.connected.add(element);
    this.sources.push(element);
    return this.source;
  }
  createDynamicsCompressor() {
    return this.compressor;
  }
  createGain() {
    return this.gain;
  }
}

function setup({
  strict = false,
  initialProps = { nightMode: false, gain: 1, enabled: true },
  element = document.createElement('video'),
} = {}) {
  FakeContext.instances.length = 0;
  vi.stubGlobal('AudioContext', FakeContext);
  const media = { current: element };
  const onUnavailable = vi.fn();
  const hook = renderHook(
    ({ nightMode, gain, enabled }) =>
      useAudioGraph({ media, enabled, nightMode, gain, onUnavailable }),
    { initialProps, wrapper: strict ? StrictMode : undefined },
  );
  return { media, hook, onUnavailable };
}

/** The graph closes after the microtasks of the unmount (releaseGraph). */
const microtasks = () => new Promise<void>((resolve) => queueMicrotask(resolve));

describe('useAudioGraph (11.10: night mode and 150 %)', () => {
  afterEach(() => vi.unstubAllGlobals());

  it('builds nothing until night mode or a boost is switched on', () => {
    const { hook } = setup();
    hook.rerender({ nightMode: false, gain: 1, enabled: true });
    expect(FakeContext.instances).toHaveLength(0);
  });

  it('a boost: element → gain → speakers, the gain is the part above 100 %', () => {
    const { media, hook } = setup();
    hook.rerender({ nightMode: false, gain: 1.3, enabled: true });
    const [context] = FakeContext.instances;
    expect(context!.sources).toEqual([media.current]);
    expect(context!.source.targets).toEqual([context!.gain]);
    expect(context!.gain.targets).toEqual([context!.destination]);
    expect(context!.gain.gain.value).toBeCloseTo(1.3);
    // The context may start suspended; the viewer has acted by now.
    expect(context!.resume).toHaveBeenCalled();
  });

  it('night mode goes through the compressor and back past it; the graph stays', () => {
    const { hook } = setup();
    hook.rerender({ nightMode: true, gain: 1, enabled: true });
    const [context] = FakeContext.instances;
    expect(context!.source.targets).toEqual([context!.compressor]);
    expect(context!.compressor.targets).toEqual([context!.gain]);
    expect(context!.compressor.ratio.value).toBe(NIGHT_MODE.ratio);
    expect(context!.compressor.threshold.value).toBe(NIGHT_MODE.threshold);
    // The compressor makes its own make-up gain: no second factor on top.
    expect(context!.gain.gain.value).toBe(1);

    hook.rerender({ nightMode: false, gain: 1, enabled: true });
    expect(context!.source.targets).toEqual([context!.gain]);
    expect(context!.gain.gain.value).toBe(1);
    // Off again, the graph is kept: one source per element, for the life of the player.
    hook.rerender({ nightMode: true, gain: 1.2, enabled: true });
    expect(FakeContext.instances).toHaveLength(1);
    expect(context!.gain.gain.value).toBeCloseTo(1.2);
  });

  it('closes the context when the player goes', async () => {
    const { hook } = setup();
    hook.rerender({ nightMode: true, gain: 1, enabled: true });
    hook.unmount();
    await microtasks();
    expect(FakeContext.instances[0]!.close).toHaveBeenCalled();
  });

  it('keeps the graph of an element through a remount of StrictMode (one source node)', async () => {
    const element = document.body.appendChild(document.createElement('video'));
    try {
      const { hook, onUnavailable } = setup({
        strict: true,
        element,
        initialProps: { nightMode: true, gain: 1.3, enabled: true },
      });
      await microtasks();
      expect(FakeContext.instances).toHaveLength(1);
      const [context] = FakeContext.instances;
      expect(context!.sources).toEqual([element]);
      expect(context!.close).not.toHaveBeenCalled();
      expect(onUnavailable).not.toHaveBeenCalled();
      expect(context!.source.targets).toEqual([context!.compressor]);
      hook.rerender({ nightMode: false, gain: 1.5, enabled: true });
      expect(context!.gain.gain.value).toBeCloseTo(1.5);
    } finally {
      element.remove();
    }
  });

  it('reports a graph the browser refuses, once, and builds nothing more', () => {
    const element = document.createElement('video');
    // Another graph took the element before (as after a remount that lost the map).
    FakeContext.connected.add(element);
    const { hook, onUnavailable } = setup({ element });
    hook.rerender({ nightMode: true, gain: 1, enabled: true });
    expect(onUnavailable).toHaveBeenCalledTimes(1);
    expect(FakeContext.instances[0]!.close).toHaveBeenCalled();
    hook.rerender({ nightMode: true, gain: 1.4, enabled: true });
    expect(onUnavailable).toHaveBeenCalledTimes(1);
    expect(FakeContext.instances).toHaveLength(1);
  });

  it('never builds a graph where Web Audio is not offered (Safari)', () => {
    const { hook } = setup();
    hook.rerender({ nightMode: true, gain: 1.5, enabled: false });
    expect(FakeContext.instances).toHaveLength(0);
  });
});
