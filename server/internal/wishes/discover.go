package wishes

import "context"

// The wishes area «Entdecken», «Filme», «Serien» and «Suche» (concept 25 stage 5, architecture 21;
// N-50 to N-58).

// SettingDiscoverPages is the switch «Seiten Entdecken, Filme, Serien und Suche» (N-57, data model
// 2.12): 0/1, default 1. The administration's wish settings write it; it is read here with its
// default like the other switches of stage 4.
const SettingDiscoverPages = "wishes.discover_pages"

// DiscoverPagesOn reports whether the pages of the wishes area are switched on.
func (s *Service) DiscoverPagesOn(ctx context.Context) (bool, error) {
	return s.boolState(ctx, SettingDiscoverPages, true)
}
