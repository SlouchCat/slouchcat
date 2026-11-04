import { useEffect, useState, type RefObject } from 'react';
import type { SubtitleStyle } from './useSubtitleStyle';
import styles from './Player.module.css';

/** The part of a cue the layer reads (a VTTCue has a text; a plain object in tests). */
export interface CueLike {
  startTime: number;
  endTime: number;
  text?: string;
}

/** The cue list of a text track (TextTrackCueList; an array in tests). */
type CueList = ArrayLike<CueLike>;

/** WebVTT markup in a cue's text (<i>, <b>, <c.yellow>, <v Anna>): the layer shows plain text. */
const MARKUP = /<[^>]*>/g;

/**
 * The texts of the cues that show at `time` (seconds), in the order of the track. The layer asks
 * with the time minus the offset, so a positive offset shows every cue later (V-6).
 */
export function activeCueTexts(cues: CueList | null | undefined, time: number): string[] {
  if (!cues) return [];
  const texts: string[] = [];
  for (let index = 0; index < cues.length; index += 1) {
    const cue = cues[index]!;
    if (cue.startTime <= time && time < cue.endTime && cue.text) {
      texts.push(cue.text.replace(MARKUP, ''));
    }
  }
  return texts;
}

/** Moves every cue by `seconds` (picture in picture shows the browser's own cues, 11.10). */
export function shiftCues(cues: CueList | null | undefined, seconds: number): void {
  if (!cues || seconds === 0) return;
  for (let index = 0; index < cues.length; index += 1) {
    const cue = cues[index]!;
    cue.startTime += seconds;
    cue.endTime += seconds;
  }
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
      // The cues may load only now; they are moved once they are there.
      let shifted: CueList | null = null;
      const shift = () => {
        if (shifted || !textTrack.cues || textTrack.cues.length === 0) return;
        shifted = textTrack.cues;
        shiftCues(shifted, offset);
      };
      setMode(textTrack, 'showing');
      shift();
      element.addEventListener('load', shift);
      return () => {
        element.removeEventListener('load', shift);
        shiftCues(shifted, -offset);
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
