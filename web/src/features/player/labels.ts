import type { Chapter, PlaybackInfo, QualityOption, Track } from '@slouchcat/api-client';
import type { TFunction } from 'i18next';
import { formatPlayerTime } from '@/lib/format';

/** ISO 639-2 codes of files ("ger", "eng") in the two-letter form Intl knows best. */
const twoLetter: Record<string, string> = {
  ger: 'de',
  deu: 'de',
  eng: 'en',
  fre: 'fr',
  fra: 'fr',
  ita: 'it',
  spa: 'es',
  por: 'pt',
  dut: 'nl',
  nld: 'nl',
  jpn: 'ja',
  kor: 'ko',
  chi: 'zh',
  zho: 'zh',
  rus: 'ru',
  pol: 'pl',
  swe: 'sv',
  dan: 'da',
  nor: 'no',
  fin: 'fi',
  tur: 'tr',
  gsw: 'gsw',
};

/** Name of a language code in the UI language ("ger" → "Deutsch"); undefined when unknown. */
export function languageName(code: string | undefined, uiLanguage: string): string | undefined {
  if (!code || code === 'und' || code === 'zxx') return undefined;
  const normalized = twoLetter[code.toLowerCase()] ?? code;
  try {
    const name = new Intl.DisplayNames([uiLanguage], { type: 'language' }).of(normalized);
    return name && name !== normalized ? name : code;
  } catch {
    return code;
  }
}

/** Short names of the ffprobe codecs viewers know; anything else in capitals. */
const codecNames: Record<string, string> = {
  subrip: 'SRT',
  ass: 'ASS',
  ssa: 'SSA',
  webvtt: 'WebVTT',
  mov_text: 'MOV-Text',
  hdmv_pgs_subtitle: 'PGS',
  dvd_subtitle: 'VobSub',
  eac3: 'E-AC3',
  truehd: 'TrueHD',
};

export const codecName = (codec: string) => codecNames[codec] ?? codec.toUpperCase();

/** "5.1" for six channels, "2.0" for two. */
function channelLayout(channels: number): string {
  if (channels === 6) return '5.1';
  if (channels === 8) return '7.1';
  return `${channels}.0`;
}

/**
 * Menu label of a track: its title or language, the codec (unless the title names it already),
 * the channel layout of an audio track, and whether it is forced or a sidecar file.
 */
export function trackLabel(t: TFunction, track: Track, position: number, uiLanguage: string) {
  const name =
    track.title ??
    languageName(track.language, uiLanguage) ??
    t('player.tracks.unknown', { n: position + 1 });
  const codec = codecName(track.codec);
  const parts = [name];
  if (!name.toLowerCase().includes(codec.toLowerCase())) parts.push(codec);
  if (track.kind === 'audio' && track.channels) parts.push(channelLayout(track.channels));
  if (track.forced) parts.push(t('player.tracks.forced'));
  if (track.source === 'sidecar') parts.push(t('player.tracks.external'));
  return parts.join(t('format.separator'));
}

/** Menu key of an audio track (its stream index) and of a subtitle track (its id), or "off". */
export const audioKey = (index: number) => `audio:${index}`;
export const subtitleKey = (id: number | undefined) => `subtitle:${id ?? 'off'}`;

/** The audio track the playback delivers: the server's selection, else the default track. */
export function selectedAudio(info: PlaybackInfo): Track | undefined {
  const { audio } = info.tracks;
  const index = info.selection.audioTrack;
  return (
    audio.find((track) => track.index === index) ?? audio.find((track) => track.default) ?? audio[0]
  );
}

export function selectedSubtitle(info: PlaybackInfo): Track | undefined {
  const id = info.selection.subtitleTrack;
  return id === undefined ? undefined : info.tracks.subtitles.find((track) => track.id === id);
}

/** Label of a quality option; `auto` names the rung the server picked when it says so. */
export function qualityLabel(t: TFunction, option: QualityOption, info: PlaybackInfo): string {
  if (option.quality !== 'auto') return option.quality;
  const height = info.transcoding?.height;
  return height
    ? t('player.quality.autoWith', { quality: t('format.height', { height }) })
    : t('player.quality.auto');
}

/** Index of the chapter that contains `ms`, or -1 before the first one. */
export function chapterAt(chapters: readonly Chapter[], ms: number): number {
  let current = -1;
  chapters.forEach((chapter, index) => {
    if (chapter.startMs <= ms) current = index;
  });
  return current;
}

export function chapterLabel(
  t: TFunction,
  chapter: Chapter,
  index: number,
  durationMs: number,
): string {
  return t('player.chapters.item', {
    title: chapter.title ?? t('player.chapters.untitled', { n: index + 1 }),
    time: formatPlayerTime(chapter.startMs, durationMs),
  });
}
