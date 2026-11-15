package model

import "slices"

// UILanguages are the interface languages a profile may choose: exactly the language packs the
// web app ships and the enum Language of the contract (concept 22 section 4.3, ADR 0033). It is
// the one place the server checks a profile language against; TestUILanguagesMatchContract keeps
// it equal to the contract. A new code is added together with its language pack, one release
// after the web app learnt to fall back to English for an unknown code. The order is the one of
// the language choice (concept 22 table 4.1): German in two variants, English, then the twenty
// languages of stage 4.
var UILanguages = []string{
	"de", "de-DE", "en", "fr", "es", "it", "nl", "pt-BR", "pt-PT", "pl", "sv", "da",
	"nb", "fi", "cs", "hu", "tr", "ru", "uk", "ja", "ko", "zh-CN", "zh-TW",
}

// DefaultUILanguage is the language of a new profile when the request names none.
const DefaultUILanguage = "de"

// IsUILanguage reports whether code is one of UILanguages.
func IsUILanguage(code string) bool { return slices.Contains(UILanguages, code) }

// FallbackLanguage is the second link of the text chain of a viewer (ADR 0018, ADR 0033): German
// for both German variants (de, de-DE), English for every other interface language. It is also
// the language of genre and collection names, which exist in German and English only.
func FallbackLanguage(ui string) string {
	if ui == "de" || ui == "de-DE" {
		return "de"
	}
	return "en"
}

// MetadataBase is the metadata language a profile reads while its metadata_language is unset
// (concept 22 section 4.3): de-DE reads German, Norwegian Bokmål reads "no" (the code TMDB keeps
// its Norwegian texts under, checked in step 0 of stage 1), every other interface language reads
// its own code (pt-BR, pt-PT, zh-CN and zh-TW keep their region).
func MetadataBase(ui string) string {
	switch ui {
	case "de-DE":
		return "de"
	case "nb":
		return "no"
	}
	return ui
}
