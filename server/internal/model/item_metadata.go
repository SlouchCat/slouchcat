package model

// The administrator's metadata editor (concept 15 7.4, 7.5, P-6; contract 1.15.0): the editable
// fields of a title in the server's metadata language, their locks and their provenance.

// Fields of the metadata editor (AdminMetadataField).
const (
	MetaFieldTitle         = "title"
	MetaFieldOriginalTitle = "originalTitle"
	MetaFieldSortTitle     = "sortTitle"
	MetaFieldYear          = "year"
	MetaFieldReleaseDate   = "releaseDate"
	MetaFieldTagline       = "tagline"
	MetaFieldOverview      = "overview"
	MetaFieldGenres        = "genres"
)

// MetaFields are the fields of the editor in the order of the dialog.
var MetaFields = []string{MetaFieldTitle, MetaFieldOriginalTitle, MetaFieldSortTitle, MetaFieldYear,
	MetaFieldReleaseDate, MetaFieldTagline, MetaFieldOverview, MetaFieldGenres}

// TextLockName is the lock of a text field (title, overview, tagline, sort_title) in one language
// in items.locked_fields: title:de (concept 15 7.5). The older lock without language (title)
// holds every language.
func TextLockName(field, lang string) string { return field + ":" + lang }

// AdminItemMetadata is the editable metadata of a title in the server's metadata language Lang:
// the current values (nil when there is none), the locked fields and who wrote which field.
type AdminItemMetadata struct {
	Lang          string
	Title         string
	OriginalTitle *string
	SortTitle     *string // item_texts.sort_title of Lang while locked, else Title
	Year          *int
	ReleaseDate   *string
	Tagline       *string
	Overview      *string
	Genres        []string
	Locked        []string // MetaField* names
	Sources       []MetadataSource
}

// MetadataSource names the provider that wrote a field of the editor.
type MetadataSource struct {
	Field    string // a MetaField* name
	Provider string
}

// ItemMetadataEdit is one request of the editor (updateItemMetadata): every non-nil field is
// written and locked; Unlock lifts the locks of the named fields.
type ItemMetadataEdit struct {
	Title, OriginalTitle, SortTitle, ReleaseDate, Tagline, Overview *string
	Year                                                            *int
	Genres                                                          *[]string
	Unlock                                                          []string
}
