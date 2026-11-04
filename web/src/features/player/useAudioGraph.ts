import { useEffect, useRef, type RefObject } from 'react';

/**
 * Night mode (V-16): a compressor that makes quiet passages louder and loud ones quieter. The
 * values follow the usual «dynamic range compression» presets of players; the make-up gain brings
 * the compressed signal back to about the loudness of the original.
 */
export const NIGHT_MODE = {
  threshold: -30,
  knee: 20,
  ratio: 6,
  attack: 0.005,
  release: 0.3,
  makeUp: 1.6,
} as const;

interface Graph {
  context: AudioContext;
  source: MediaElementAudioSourceNode;
  compressor: DynamicsCompressorNode;
  gain: GainNode;
  night: boolean | null;
}

// Module functions: the React compiler does not let a hook write to objects it received.

/** media element → (compressor) → gain → speakers; the element plays through the graph from now. */
function buildGraph(element: HTMLMediaElement): Graph {
  const context = new AudioContext();
  const source = context.createMediaElementSource(element);
  const compressor = context.createDynamicsCompressor();
  compressor.threshold.value = NIGHT_MODE.threshold;
  compressor.knee.value = NIGHT_MODE.knee;
  compressor.ratio.value = NIGHT_MODE.ratio;
  compressor.attack.value = NIGHT_MODE.attack;
  compressor.release.value = NIGHT_MODE.release;
  const gain = context.createGain();
  gain.connect(context.destination);
  compressor.connect(gain);
  return { context, source, compressor, gain, night: null };
}

/** Routes the source through the compressor or past it and sets the gain. */
function applyGraph(graph: Graph, night: boolean, gain: number): void {
  if (graph.night !== night) {
    graph.source.disconnect();
    graph.source.connect(night ? graph.compressor : graph.gain);
    graph.night = night;
  }
  graph.gain.gain.value = gain * (night ? NIGHT_MODE.makeUp : 1);
  // A context made without a fresh gesture starts suspended; the page has had one by now.
  if (graph.context.state === 'suspended') void graph.context.resume();
}

function closeGraph(graph: Graph): void {
  void graph.context.close();
}

interface AudioGraphOptions {
  media: RefObject<HTMLMediaElement | null>;
  /** Web Audio may be used (`supportsWebAudioBoost`: not in Safari, 11.15). */
  enabled: boolean;
  nightMode: boolean;
  /** The part of the volume above 100 % (1 … 1.5); the element plays up to 100 % itself. */
  gain: number;
}

/**
 * Night mode and the volume above 100 % through Web Audio (11.10 «Lautstärke und Ton-Optionen»):
 * `createMediaElementSource` → `DynamicsCompressorNode` → `GainNode`. The graph is built only when
 * one of them is first switched on, which only a viewer's action does, and then stays for the
 * life of the player (a media element can feed one source node only, and taking it out again
 * would cut the sound). Without night mode the source goes past the compressor.
 */
export function useAudioGraph({ media, enabled, nightMode, gain }: AudioGraphOptions): void {
  const graph = useRef<Graph | null>(null);

  useEffect(() => {
    const element = media.current;
    if (!enabled || !element) return;
    if (!graph.current) {
      if (!nightMode && gain <= 1) return;
      graph.current = buildGraph(element);
    }
    applyGraph(graph.current, nightMode, gain);
  }, [enabled, gain, media, nightMode]);

  useEffect(
    () => () => {
      if (graph.current) closeGraph(graph.current);
      graph.current = null;
    },
    [],
  );
}
