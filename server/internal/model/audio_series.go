package model

// Audiobook series and the row «Reihen und Sammlungen» (contract 1.25.0, concept 19 stage D2).

// Kinds of an AudioGroup.
const (
	AudioGroupSeries     = "series"
	AudioGroupCollection = "collection"
)

// AudioGroup is one tile of the row «Reihen und Sammlungen» (listAudioGroups): a series of the
// library or a manual collection of audiobooks, computed from the visible volumes only.
type AudioGroup struct {
	ID        string // AudioSeriesID or CollectionID
	Kind      string // AudioGroupSeries, AudioGroupCollection
	Name      string
	Author    string // series only
	Count     int    // visible volumes or members
	Matching  int    // of Count, those that pass the filters
	Completed int    // of Count, those the profile completed
	Cover     *ItemSummary
}

// AudioGroupList is the answer of listAudioGroups: series and collections together, by name.
type AudioGroupList struct {
	Groups          []AudioGroup
	SeriesCount     int
	CollectionCount int
}

// AudioSeriesVolume is one visible volume of a series page.
type AudioSeriesVolume struct {
	Item        ItemSummary
	SeriesIndex *float64
	Narrator    string
}

// AudioSeriesSpelling is a series name as the visible volumes write it, with their number.
type AudioSeriesSpelling struct {
	Name  string
	Count int
}

// AudioSeries is the page of a series (getAudioSeries) over its visible volumes.
type AudioSeries struct {
	ID         string
	LibraryID  int64
	Name       string
	NameLocked bool
	Author     string
	Narrator   string // only when every visible volume has the same
	DurationMs int64
	Cover      *ItemSummary
	Next       *ItemSummary
	Volumes    []AudioSeriesVolume
	Spellings  []AudioSeriesSpelling
}

// AdminAudioSeries is the answer of updateAudioSeries; VolumeCount counts every book of the
// series that is not deleted, visible or not.
type AdminAudioSeries struct {
	ID          string
	LibraryID   int64
	Name        string
	NameLocked  bool
	VolumeCount int
}

// MaxAudioSeriesName is the longest name «Reihe bearbeiten» accepts, in characters.
const MaxAudioSeriesName = 200
