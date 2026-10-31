package metadata

import (
	"regexp"
	"slices"
	"strings"
)

// The static list of metadata languages the server accepts (concept 10 section 4.1, E-10): the
// ISO 639-1 codes TMDB lists under /configuration/languages (plus cn, Cantonese, and xx, no
// language, which TMDB uses too) and the regional variants of TMDB's primary translations. It
// is embedded, never fetched, so saving a language list does not depend on a TMDB key. A code
// outside this list is 422 lang_unknown. Extending the list is a code change.

// langPattern is the shape of a code: a two- or three-letter primary subtag, optionally a region.
var langPattern = regexp.MustCompile(`^[a-z]{2,3}(-[A-Z]{2})?$`)

// iso6391 are the primary subtags.
var iso6391 = toSet(strings.Fields(`
aa ab ae af ak am an ar as av ay az ba be bg bh bi bm bn bo br bs ca ce ch cn co cr cs cu cv cy da de dv dz
ee el en eo es et eu fa ff fi fj fo fr fy ga gd gl gn gu gv ha he hi ho hr ht hu hy hz ia id ie ig ii ik io
is it iu ja jv ka kg ki kj kk kl km kn ko kr ks ku kv kw ky la lb lg li ln lo lt lu lv mg mh mi mk ml mn mo
mr ms mt my na nb nd ne ng nl nn no nr nv ny oc oj om or os pa pi pl ps pt qu rm rn ro ru rw sa sc sd se sg
sh si sk sl sm sn so sq sr ss st su sv sw ta te tg th ti tk tl tn to tr ts tt tw ty ug uk ur uz ve vi vo wa
wo xh xx yi yo za zh zu`))

// regionVariants are the codes with a region TMDB knows as primary translations.
var regionVariants = toSet(strings.Fields(`
af-ZA ar-AE ar-SA be-BY bg-BG bn-BD ca-ES ch-GU cs-CZ cy-GB da-DK de-AT de-CH de-DE el-GR en-AU en-CA en-GB
en-IE en-NZ en-US eo-EO es-ES es-MX et-EE eu-ES fa-IR fi-FI fr-CA fr-FR ga-IE gd-GB gl-ES he-IL hi-IN hr-HR
hu-HU id-ID it-IT ja-JP ka-GE kk-KZ kn-IN ko-KR ky-KG lt-LT lv-LV ml-IN mr-IN ms-MY ms-SG nb-NO nl-BE nl-NL
no-NO pa-IN pl-PL pt-BR pt-PT ro-RO ru-RU si-LK sk-SK sl-SI sq-AL sr-RS sv-SE ta-IN te-IN th-TH tl-PH tr-TR
uk-UA ur-PK uz-UZ vi-VN zh-CN zh-HK zh-SG zh-TW zu-ZA`))

func toSet(words []string) map[string]bool {
	out := make(map[string]bool, len(words))
	for _, w := range words {
		out[w] = true
	}
	return out
}

// KnownLanguage reports whether code is a metadata language the server accepts: a bare ISO
// 639-1 code of the list, or one of the regional variants.
func KnownLanguage(code string) bool {
	if !langPattern.MatchString(code) {
		return false
	}
	if strings.Contains(code, "-") {
		return regionVariants[code]
	}
	return iso6391[code]
}

// KnownLanguages returns every accepted code, bare codes first, for the interface's picker.
func KnownLanguages() []string {
	out := make([]string, 0, len(iso6391)+len(regionVariants))
	for c := range iso6391 {
		out = append(out, c)
	}
	for c := range regionVariants {
		out = append(out, c)
	}
	slices.Sort(out)
	return out
}
