import { useEffect, useState, type RefObject } from 'react';
import type { SubtitleStyle } from './useSubtitleStyle';
import styles from './Player.module.css';

/**
 * The part of a cue the layer reads: a VTTCue has its text and `getCueAsHTML`; a plain object in
 * tests.
 */
export interface CueLike {
  startTime: number;
  endTime: number;
  text?: string;
  getCueAsHTML?: () => DocumentFragment;
}

/** The cue list of a text track (TextTrackCueList; an array in tests). */
type CueList = ArrayLike<CueLike>;

/** WebVTT markup in a cue's text (<i>, <b>, <c.yellow>, <v Anna>): the layer shows plain text. */
const MARKUP = /<[^>]*>/g;

/** The character references WebVTT knows (the cue text parser of the WebVTT spec). */
const ENTITIES: Record<string, string> = {
  '&amp;': '&',
  '&lt;': '<',
  '&gt;': '>',
  '&nbsp;': '\u00a0',
  '&lrm;': '\u200e',
  '&rlm;': '\u200f',
};
const ENTITY = /&(?:amp|lt|gt|nbsp|lrm|rlm);/g;

/**
 * The plain text of a cue. `VTTCue.text` is the raw cue text: the browser's own parser
 * (`getCueAsHTML`) drops the tags and resolves `&amp;` and the like; an object without it (tests)
 * falls back to removing the tags and resolving the references here.
 */
export function cueText(cue: CueLike): string {
  if (typeof cue.getCueAsHTML === 'function') return cue.getCueAsHTML().textContent ?? '';
  return (cue.text ?? '').replace(MARKUP, '').replace(ENTITY, (entity) => ENTITIES[entity] ?? '');
}

/**
 * The texts of the cues that show at `time` (seconds), in the order of the track. The layer asks
 * with the time minus the offset, so a positive offset shows every cue later (V-6).
 */
export function activeCueTexts(cues: CueList | null | undefined, time: number): string[] {
  if (!cues) return [];
  const texts: string[] = [];
  for (let index = 0; index < cues.length; index += 1) {
    const cue = cues[index]!;
    if (cue.startTime <= time && time < cue.endTime) {
      const text = cueText(cue);
      if (text) texts.push(text);
    }
  }
  return texts;
}

/**
 * Moves every cue of `cues` that `moved` does not hold yet by `seconds` and records it there
 * (picture in picture shows the browser's own cues, 11.10; the offline blob fallback). The cues
 * of a track that is still loading arrive in parts: each one moves once. A TextTrackCueList stays
 * sorted by start time and re-sorts while a cue moves, so walking it by index would move some
 * cues twice and others not at all: a copy keeps the order.
 */
export function shiftCues(
  cues: CueList | null | undefined,
  seconds: number,
  moved: Set<CueLike> = new Set(),
): Set<CueLike> {
  if (!cues || seconds === 0) return moved;
  for (const cue of Array.from(cues)) {
    if (moved.has(cue)) continue;
    cue.startTime += seconds;
    cue.endTime += seconds;
    moved.add(cue);
  }
  return moved;
}

/** Puts the cues `shiftCues` moved back by `seconds`. */
export function unshiftCues(moved: Set<CueLike>, seconds: number): void {
  for (const cue of moved) {
    cue.startTime -= seconds;
    cue.endTime -= seconds;
  }
  moved.clear();
}

// A module function: the React compiler does not let a hook write to an object it received.
function setMode(track: TextTrack, mode: TextTrackMode): void {
  track.mode = mode;
}

const NO_TEXTS: string[] = [];

const same = (a: readonly string[], b: readonly string[]) =>
  a.length === b.length && a.every((text, index) => text === b[index]);

interface SubtitleCuesOptions {
  video: RefObject<HTMLVideoElement | null>;
  track: RefObject<HTMLTrackElement | null>;
  /** The WebVTT URL of the session; none without a text subtitle. */
  src: string | undefined;
  offsetMs: number;
  pictureInPicture: boolean;
}

/**
 * The cues to draw (11.10 «Untertitel»): the text track stays `hidden`, so the browser draws
 * nothing, and the player reads its cues on `cuechange`, on every tick of the video and on every
 * frame while it plays, with the time moved by the offset. In picture in picture only the
 * browser's own cues show: the track switches to `showing` with the cues moved by the offset, and
 * back to `hidden` with the cues where they were when the window closes.
 */
export function useSubtitleCues({
  video,
  track,
  src,
  offsetMs,
  pictureInPicture,
}: SubtitleCuesOptions): string[] {
  const [texts, setTexts] = useState<string[]>([]);

  useEffect(() => {
    const element = track.current;
    const media = video.current;
    const textTrack = element?.track;
    if (!src || !element || !media || !textTrack) return;
    const offset = offsetMs / 1000;
    if (pictureInPicture) {
      // The cues may still be loading: the ones there now move, the rest once the track loaded.
      const moved = new Set<CueLike>();
      const shift = () => {
        shiftCues(textTrack.cues, offset, moved);
      };
      setMode(textTrack, 'showing');
      shift();
      element.addEventListener('load', shift);
      return () => {
        element.removeEventListener('load', shift);
        unshiftCues(moved, offset);
        setMode(textTrack, 'hidden');
      };
    }

    setMode(textTrack, 'hidden');
    const update = () => {
      const next = activeCueTexts(textTrack.cues, media.currentTime - offset);
      setTexts((current) => (same(current, next) ? current : next));
    };
    let frame = 0;
    const tick = () => {
      update();
      frame = media.paused ? 0 : requestAnimationFrame(tick);
    };
    const onPlay = () => {
      if (frame === 0) frame = requestAnimationFrame(tick);
    };
    update();
    if (!media.paused) onPlay();
    textTrack.addEventListener('cuechange', update);
    element.addEventListener('load', update);
    media.addEventListener('timeupdate', update);
    media.addEventListener('seeked', update);
    media.addEventListener('play', onPlay);
    return () => {
      cancelAnimationFrame(frame);
      textTrack.removeEventListener('cuechange', update);
      element.removeEventListener('load', update);
      media.removeEventListener('timeupdate', update);
      media.removeEventListener('seeked', update);
      media.removeEventListener('play', onPlay);
    };
  }, [offsetMs, pictureInPicture, src, track, video]);

  // Without a text subtitle, and while the window shows the browser's cues, the layer is empty.
  return src && !pictureInPicture ? texts : NO_TEXTS;
}

interface SubtitleLayerProps {
  texts: readonly string[];
  style: SubtitleStyle;
  /** The bar is on screen: the lines move up above it. */
  raised: boolean;
}

/**
 * The player's own subtitle layer (V-6): the same size, background, colour and position in every
 * browser, above the bar while it shows. The pointer passes through to the video.
 */
export function SubtitleLayer({ texts, style, raised }: SubtitleLayerProps) {
  if (texts.length === 0) return null;
  return (
    <div
      className={styles.subtitles}
      data-size={style.size}
      data-background={style.background}
      data-color={style.color}
      data-position={style.position}
      data-raised={raised || undefined}
      data-subtitles=""
    >
      {texts.map((text, index) => (
        <p key={`${index}-${text}`} className={styles.cue}>
          <span className={styles.cueText}>{text}</span>
        </p>
      ))}
    </div>
  );
}
