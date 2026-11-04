package trackpick

import "strings"

// languageCodes maps the language codes and names that files, sidecars and TMDB use to the ISO
// 639-1 code profiles store (concept 13 section 11.3, "Sprachcodes"): ISO 639-2/B and /T ("ger",
// "deu"), English and native names of external subtitles ("german", "deutsch") and the TMDB code
// "cn". Two-letter codes and codes without an entry stand for themselves. The web client keeps the
// same table (LANGUAGE_CODES in web/src/features/player/labels.ts); TestLanguageParity holds the
// two in step.
var languageCodes = map[string]string{
	"afr": "af", "afrikaans": "af",
	"alb": "sq", "sqi": "sq", "albanian": "sq",
	"ara": "ar", "arabic": "ar",
	"arm": "hy", "hye": "hy", "armenian": "hy",
	"baq": "eu", "eus": "eu", "basque": "eu",
	"ben": "bn", "bengali": "bn",
	"bos": "bs", "bosnian": "bs",
	"bul": "bg", "bulgarian": "bg",
	"cat": "ca", "catalan": "ca",
	"chi": "zh", "zho": "zh", "chinese": "zh", "cn": "zh",
	"cze": "cs", "ces": "cs", "czech": "cs",
	"dan": "da", "danish": "da", "dansk": "da",
	"dut": "nl", "nld": "nl", "dutch": "nl", "nederlands": "nl",
	"eng": "en", "english": "en",
	"est": "et", "estonian": "et",
	"fil": "tl", "tgl": "tl", "filipino": "tl", "tagalog": "tl",
	"fin": "fi", "finnish": "fi", "suomi": "fi",
	"fre": "fr", "fra": "fr", "french": "fr", "francais": "fr",
	"geo": "ka", "kat": "ka", "georgian": "ka",
	"ger": "de", "deu": "de", "german": "de", "deutsch": "de",
	"gle": "ga", "irish": "ga",
	"glg": "gl", "galician": "gl",
	"gre": "el", "ell": "el", "greek": "el",
	"heb": "he", "iw": "he", "hebrew": "he",
	"hin": "hi", "hindi": "hi",
	"hrv": "hr", "croatian": "hr",
	"hun": "hu", "hungarian": "hu", "magyar": "hu",
	"ice": "is", "isl": "is", "icelandic": "is",
	"ind": "id", "indonesian": "id",
	"ita": "it", "italian": "it", "italiano": "it",
	"jpn": "ja", "japanese": "ja",
	"kor": "ko", "korean": "ko",
	"lat": "la", "latin": "la",
	"lav": "lv", "latvian": "lv",
	"lit": "lt", "lithuanian": "lt",
	"mac": "mk", "mkd": "mk", "macedonian": "mk",
	"may": "ms", "msa": "ms", "malay": "ms",
	"nor": "no", "nob": "no", "nno": "no", "nb": "no", "nn": "no", "norwegian": "no", "norsk": "no",
	"per": "fa", "fas": "fa", "persian": "fa",
	"pol": "pl", "polish": "pl", "polski": "pl",
	"por": "pt", "portuguese": "pt",
	"rum": "ro", "ron": "ro", "romanian": "ro",
	"rus": "ru", "russian": "ru",
	"slo": "sk", "slk": "sk", "slovak": "sk",
	"slv": "sl", "slovenian": "sl",
	"spa": "es", "spanish": "es", "espanol": "es",
	"srp": "sr", "serbian": "sr",
	"swa": "sw", "swahili": "sw",
	"swe": "sv", "swedish": "sv", "svenska": "sv",
	"tam": "ta", "tamil": "ta",
	"tel": "te", "telugu": "te",
	"tha": "th", "thai": "th",
	"tur": "tr", "turkish": "tr", "turkce": "tr",
	"ukr": "uk", "ukrainian": "uk",
	"urd": "ur", "urdu": "ur",
	"vie": "vi", "vietnamese": "vi",
	"wel": "cy", "cym": "cy", "welsh": "cy",
}

// noLanguage are the codes that name no language: undetermined, no linguistic content and the
// TMDB placeholder (NO_LANGUAGE of the web client).
var noLanguage = map[string]bool{"und": true, "zxx": true, "xx": true}

// Normalize returns the ISO 639-1 code of a language code or name as files, sidecars and TMDB
// write it ("ger" → "de", "pt-BR" → "pt", "German" → "de"); Swiss German ("gsw") counts as
// German. It returns "" for no language, "und", "zxx", "xx" and several languages ("mul"). The web
// client's baseLanguage follows the same rule.
func Normalize(code string) string {
	lower := strings.ToLower(strings.TrimSpace(code))
	if lower == "" || noLanguage[lower] || lower == "mul" {
		return ""
	}
	primary, _, _ := strings.Cut(strings.ReplaceAll(lower, "_", "-"), "-")
	if primary == "gsw" {
		return "de"
	}
	if c, ok := languageCodes[primary]; ok {
		return c
	}
	if noLanguage[primary] {
		return ""
	}
	return primary
}
