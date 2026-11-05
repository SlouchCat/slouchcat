import { useEffect, useRef, type RefObject } from 'react';

/**
 * Night mode (V-16): a compressor that makes quiet passages louder and loud ones quieter. The
 * values follow the usual «dynamic range compression» presets of players. There is no make-up
 * gain of our own: the Web Audio compressor applies one itself (spec «DynamicsCompressorNode»,
 * makeup gain = (1 / curve(1))^0.6; at −30 dB and 6:1 a full-scale signal comes out at about
 * −25 dB, so the node raises it by about 5.6×). A second factor on top would clip loud scenes.
 */
export const NIGHT_MODE = {
  threshold: -30,
  knee: 20,
  ratio: 6,
  attack: 0.005,
  release: 0.3,
} as const;

interface Graph {
  context: AudioContext;
  source: MediaElementAudioSourceNode;
  compressor: DynamicsCompressorNode;
  gain: GainNode;
  night: boolean | null;
}

/**
 * The graph of each media element. An element feeds one source node for its whole life
 * (`createMediaElementSource` a second time throws InvalidStateError), so a hook that mounts again
 * on the same element (StrictMode, Fast Refresh) takes the graph it built before.
 */
const graphs = new WeakMap<HTMLMediaElement, Graph>();

// Module functions: the React compiler does not let a hook write to objects it received.

/** media element → (compressor) → gain → speakers; the element plays through the graph from now. */
function buildGraph(element: HTMLMediaElement): Graph {
  const known = graphs.get(element);
  if (known) return known;
  const context = new AudioContext();
  let source: MediaElementAudioSourceNode;
  try {
    source = context.createMediaElementSource(element);
  } catch (error) {
    void context.close().catch(() => undefined);
    throw error;
  }
  const compressor = context.createDynamicsCompressor();
  compressor.threshold.value = NIGHT_MODE.threshold;
  compressor.knee.value = NIGHT_MODE.knee;
  compressor.ratio.value = NIGHT_MODE.ratio;
  compressor.attack.value = NIGHT_MODE.attack;
  compressor.release.value = NIGHT_MODE.release;
  const gain = context.createGain();
  gain.connect(context.destination);
  compressor.connect(gain);
  const graph = { context, source, compressor, gain, night: null };
  graphs.set(element, graph);
  return graph;
}

/** Routes the source through the compressor or past it and sets the gain. */
function applyGraph(graph: Graph, night: boolean, gain: number): void {
  if (graph.night !== night) {
    graph.source.disconnect();
    graph.source.connect(night ? graph.compressor : graph.gain);
    graph.night = night;
  }
  graph.gain.gain.value = gain;
  // A context made without a fresh gesture starts suspended; the page has had one by now.
  if (graph.context.state === 'suspended') void graph.context.resume().catch(() => undefined);
}

/**
 * Closes the graph of an element that left the document. A cleanup of StrictMode or Fast Refresh
 * runs while the element stays: it is checked after the current task's microtasks, when React has
 * mounted again or removed the element.
 */
function releaseGraph(element: HTMLMediaElement): void {
  queueMicrotask(() => {
    const graph = graphs.get(element);
    if (!graph || element.isConnected) return;
    graphs.delete(element);
    void graph.context.close().catch(() => undefined);
  });
}

interface AudioGraphOptions {
  media: RefObject<HTMLMediaElement | null>;
  /** Web Audio may be used (`supportsWebAudioBoost`: not in Safari, 11.15). */
  enabled: boolean;
  nightMode: boolean;
  /** The part of the volume above 100 % (1 … 1.5); the element plays up to 100 % itself. */
  gain: number;
  /** The browser refused the graph: the page hides night mode and the volume above 100 %. */
  onUnavailable?: () => void;
}

/**
 * Night mode and the volume above 100 % through Web Audio (11.10 «Lautstärke und Ton-Optionen»):
 * `createMediaElementSource` → `DynamicsCompressorNode` → `GainNode`. The graph is built only when
 * one of them is first switched on, which only a viewer's action does, and then stays for the
 * life of the element (a media element can feed one source node only, and taking it out again
 * would cut the sound). Without night mode the source goes past the compressor.
 */
export function useAudioGraph({
  media,
  enabled,
  nightMode,
  gain,
  onUnavailable,
}: AudioGraphOptions): void {
  const element = useRef<HTMLMediaElement | null>(null);
  const failed = useRef(false);

  useEffect(() => {
    const current = media.current;
    if (!enabled || !current || failed.current) return;
    element.current = current;
    let graph = graphs.get(current);
    if (!graph) {
      if (!nightMode && gain <= 1) return;
      try {
        graph = buildGraph(current);
      } catch {
        failed.current = true;
        onUnavailable?.();
        return;
      }
    }
    applyGraph(graph, nightMode, gain);
  }, [enabled, gain, media, nightMode, onUnavailable]);

  useEffect(
    () => () => {
      if (element.current) releaseGraph(element.current);
    },
    [],
  );
}
