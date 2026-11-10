package model

import (
	"slices"
	"testing"
)

// The provider rules of a folder (ADR 0015) with the book sources of concept 19 D6.2: the files
// stay first in an audiobook folder, the five book sources may follow in any order, and they stand
// in the list of no other content.
func TestRootProvidersWithBookSources(t *testing.T) {
	t.Parallel()
	books := []string{ProviderAudible, ProviderDNB, ProviderAppleBooks, ProviderOpenLibrary, ProviderGoogleBooks}
	for _, p := range books {
		if !slices.Contains(Providers, p) {
			t.Errorf("Providers lacks %s", p)
		}
	}
	if got, want := ProvidersForContent(RootAudiobook), append([]string{ProviderLocal}, books...); !slices.Equal(got, want) {
		t.Errorf("ProvidersForContent(audiobook) = %v, want %v", got, want)
	}
	for _, c := range []struct {
		content   string
		providers []string
		want      bool
	}{
		{RootAudiobook, []string{ProviderLocal}, true},
		{RootAudiobook, []string{ProviderLocal, ProviderAudible, ProviderDNB, ProviderAppleBooks}, true},
		{RootAudiobook, []string{ProviderLocal, ProviderGoogleBooks, ProviderOpenLibrary, ProviderDNB, ProviderAppleBooks, ProviderAudible}, true},
		{RootAudiobook, []string{ProviderAudible}, false},                                 // a book source never identifies
		{RootAudiobook, []string{ProviderLocal, ProviderAudible, ProviderAudible}, false}, // no duplicates
		{RootAudiobook, []string{ProviderLocal, ProviderTMDB}, false},
		{RootPodcast, []string{ProviderLocal, ProviderAudible}, false},
		{RootMovie, []string{ProviderTMDB, ProviderDNB}, false},
		{RootSeries, []string{ProviderTVDB, ProviderOpenLibrary}, false},
	} {
		if got := ValidRootProviders(c.content, c.providers); got != c.want {
			t.Errorf("ValidRootProviders(%s, %v) = %v, want %v", c.content, c.providers, got, c.want)
		}
	}
	// Marco's decision «alle samt Audible an»: a new audiobook folder starts with Audible, DNB and
	// Apple Books; a podcast folder stays with the files.
	if got, want := DefaultProviders(RootAudiobook), []string{ProviderLocal, ProviderAudible, ProviderDNB, ProviderAppleBooks}; !slices.Equal(got, want) {
		t.Errorf("DefaultProviders(audiobook) = %v, want %v", got, want)
	}
	if got := DefaultProviders(RootPodcast); !slices.Equal(got, []string{ProviderLocal}) {
		t.Errorf("DefaultProviders(podcast) = %v, want [local]", got)
	}
	for _, content := range []string{RootMovie, RootSeries, RootAudiobook, RootPodcast} {
		if !ValidRootProviders(content, DefaultProviders(content)) {
			t.Errorf("DefaultProviders(%s) = %v breaks the rules", content, DefaultProviders(content))
		}
	}
}
