import type { Chapter, PlaybackInfo, QualityOption, Track } from '@slouchcat/api-client';
import type { TFunction } from 'i18next';
import { formatPlayerTime } from '@/lib/format';

/**
 * The one table of language codes (review step 4, m-7): ISO 639-2 codes and the names that files
 * and sidecars use ("ger", "eng", "german", "cn") in the two-letter form profiles store and Intl
 * knows best. The server chooses tracks by the same table (server/internal/trackpick/languages.go,
 * kept in step by its parity test). Swiss German keeps its own name on screen (languageName) and
 * counts as German for the order of the panels (baseLanguage, 11.3).
 */
export const LANGUAGE_CODES: Readonly<Record<string, string>> = {
  afr: 'af',
  afrikaans: 'af',
  alb: 'sq',
  sqi: 'sq',
  albanian: 'sq',
  ara: 'ar',
  arabic: 'ar',
  arm: 'hy',
  hye: 'hy',
  armenian: 'hy',
  baq: 'eu',
  eus: 'eu',
  basque: 'eu',
  ben: 'bn',
  bengali: 'bn',
  bos: 'bs',
  bosnian: 'bs',
  bul: 'bg',
  bulgarian: 'bg',
  cat: 'ca',
  catalan: 'ca',
  chi: 'zh',
  zho: 'zh',
  chinese: 'zh',
  cn: 'zh',
  cze: 'cs',
  ces: 'cs',
  czech: 'cs',
  dan: 'da',
  danish: 'da',
  dansk: 'da',
  dut: 'nl',
  nld: 'nl',
  dutch: 'nl',
  nederlands: 'nl',
  eng: 'en',
  english: 'en',
  est: 'et',
  estonian: 'et',
  fil: 'tl',
  tgl: 'tl',
  filipino: 'tl',
  tagalog: 'tl',
  fin: 'fi',
  finnish: 'fi',
  suomi: 'fi',
  fre: 'fr',
  fra: 'fr',
  french: 'fr',
  francais: 'fr',
  geo: 'ka',
  kat: 'ka',
  georgian: 'ka',
  ger: 'de',
  deu: 'de',
  german: 'de',
  deutsch: 'de',
  gle: 'ga',
  irish: 'ga',
  glg: 'gl',
  galician: 'gl',
  gre: 'el',
  ell: 'el',
  greek: 'el',
  heb: 'he',
  iw: 'he',
  hebrew: 'he',
  hin: 'hi',
  hindi: 'hi',
  hrv: 'hr',
  croatian: 'hr',
  hun: 'hu',
  hungarian: 'hu',
  magyar: 'hu',
  ice: 'is',
  isl: 'is',
  icelandic: 'is',
  ind: 'id',
  indonesian: 'id',
  ita: 'it',
  italian: 'it',
  italiano: 'it',
  jpn: 'ja',
  japanese: 'ja',
  kor: 'ko',
  korean: 'ko',
  lat: 'la',
  latin: 'la',
  lav: 'lv',
  latvian: 'lv',
  lit: 'lt',
  lithuanian: 'lt',
  mac: 'mk',
  mkd: 'mk',
  macedonian: 'mk',
  may: 'ms',
  msa: 'ms',
  malay: 'ms',
  nor: 'no',
  nob: 'no',
  nno: 'no',
  nb: 'no',
  nn: 'no',
  norwegian: 'no',
  norsk: 'no',
  per: 'fa',
  fas: 'fa',
  persian: 'fa',
  pol: 'pl',
  polish: 'pl',
  polski: 'pl',
  por: 'pt',
  portuguese: 'pt',
  rum: 'ro',
  ron: 'ro',
  romanian: 'ro',
  rus: 'ru',
  russian: 'ru',
  slo: 'sk',
  slk: 'sk',
  slovak: 'sk',
  slv: 'sl',
  slovenian: 'sl',
  spa: 'es',
  spanish: 'es',
  espanol: 'es',
  srp: 'sr',
  serbian: 'sr',
  swa: 'sw',
  swahili: 'sw',
  swe: 'sv',
  swedish: 'sv',
  svenska: 'sv',
  tam: 'ta',
  tamil: 'ta',
  tel: 'te',
  telugu: 'te',
  tha: 'th',
  thai: 'th',
  tur: 'tr',
  turkish: 'tr',
  turkce: 'tr',
  ukr: 'uk',
  ukrainian: 'uk',
  urd: 'ur',
  urdu: 'ur',
  vie: 'vi',
  vietnamese: 'vi',
  wel: 'cy',
  cym: 'cy',
  welsh: 'cy',
};

/** Codes that name no language: undetermined, no linguistic content, a placeholder. */
export const NO_LANGUAGE: ReadonlySet<string> = new Set(['und', 'zxx', 'xx']);

/** Name of a language code in the UI language ("ger" → "Deutsch"); undefined when unknown. */
export function languageName(code: string | undefined, uiLanguage: string): string | undefined {
  if (!code || NO_LANGUAGE.has(code.toLowerCase())) return undefined;
  const normalized = LANGUAGE_CODES[code.toLowerCase()] ?? code;
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
 * the channel layout of an audio track, and whether it is forced or a sidecar file. The player's
 * panels show «erzwungen» as a mark instead (`forced: false`).
 */
export function trackLabel(
  t: TFunction,
  track: Track,
  position: number,
  uiLanguage: string,
  { forced = true }: { forced?: boolean } = {},
) {
  const name =
    track.title ??
    languageName(track.language, uiLanguage) ??
    t('player.tracks.unknown', { n: position + 1 });
  const codec = codecName(track.codec);
  const parts = [name];
  if (!name.toLowerCase().includes(codec.toLowerCase())) parts.push(codec);
  if (track.kind === 'audio' && track.channels) parts.push(channelLayout(track.channels));
  if (forced && track.forced) parts.push(t('player.tracks.forced'));
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

/**
 * Label of a quality option; `auto` names the rung the server picked when it says so, `original`
 * (contract 1.11.0) is a word, every other rung its name (`2160p`, `1080p`, …).
 */
export function qualityLabel(t: TFunction, option: QualityOption, info: PlaybackInfo): string {
  if (option.quality === 'original') return t('player.quality.original');
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
