package discover

import (
	"context"
	"fmt"

	"github.com/slouchcat/slouchcat/server/internal/model"
	"github.com/slouchcat/slouchcat/server/internal/store/appq"
	"github.com/slouchcat/slouchcat/server/internal/visibility"
)

// «Meine Medien» (concept 21 P-9, P-11): one stored order of libraries, radio, collections and
// picks (table my_media) for the side navigation's group and the tiles of the start page row
// libraries, each place with its own hiding (O-21). Hiding only takes the entry out of that place;
// what a profile sees of a library stays the visibility layer's.

// MyMedia returns the side navigation's group «Meine Medien» for the profile (listMyMedia): the
// stored order without the entries hidden in the side navigation, a library only while the profile
// sees a title of it, radio with the number of the profile's favourite stations (0 without a radio
// service or on its error, like the row radio), collections and picks with their visible counts.
func (s *Service) MyMedia(ctx context.Context, profile model.Profile) ([]model.MyMediaEntry, error) {
	entries, err := s.read.ListMyMedia(ctx)
	if err != nil {
		return nil, fmt.Errorf("list my media: %w", err)
	}
	scope := visibility.Profile(profile)
	libs, err := s.vis.VisibleLibraries(ctx, scope)
	if err != nil {
		return nil, err
	}
	byID := make(map[int64]model.LibraryNav, len(libs))
	for _, l := range libs {
		byID[l.ID] = l
	}
	out := make([]model.MyMediaEntry, 0, len(entries))
	for _, e := range entries {
		if e.HideSidebar == 1 {
			continue
		}
		entry := model.MyMediaEntry{ID: e.ID, Key: e.Key}
		switch e.Key {
		case model.MyMediaLibrary:
			lib, ok := byID[e.LibraryID.Int64]
			if !ok {
				continue
			}
			entry.Library, entry.Count = &lib, lib.ItemCount
		case model.MyMediaRadio:
			entry.Count = len(s.radioRow(ctx, profile))
		default:
			if entry.Count, err = s.myMediaCount(ctx, scope, e.Key); err != nil {
				return nil, err
			}
		}
		out = append(out, entry)
	}
	return out, nil
}

// mediaTiles returns the tiles of the start page row «Meine Medien» (concept 21 P-9): the stored
// order without the entries hidden on the start page, a library tile only while the profile sees
// a title of it, radio with the number of favourite stations and the first model.MyMediaStations of
// them for the logos (none: the tile invites to discover stations, O-15), collections and picks
// with their counts. The library tiles come a second time on their own, in the same order (the
// row's libraries).
func (s *Service) mediaTiles(ctx context.Context, profile model.Profile) ([]model.MyMediaTile, []model.LibraryTile, error) {
	entries, err := s.read.ListMyMedia(ctx)
	if err != nil {
		return nil, nil, fmt.Errorf("list my media: %w", err)
	}
	shown := make([]appq.ListMyMediaRow, 0, len(entries))
	libraries := false
	for _, e := range entries {
		if e.HideHome == 0 {
			shown = append(shown, e)
			libraries = libraries || e.Key == model.MyMediaLibrary
		}
	}
	scope := visibility.Profile(profile)
	byID := map[int64]model.LibraryTile{}
	if libraries {
		tiles, err := s.vis.LibraryTiles(ctx, scope)
		if err != nil {
			return nil, nil, err
		}
		for _, t := range tiles {
			byID[t.ID] = t
		}
	}
	media := make([]model.MyMediaTile, 0, len(shown))
	var libs []model.LibraryTile
	for _, e := range shown {
		tile := model.MyMediaTile{ID: e.ID, Key: e.Key}
		switch e.Key {
		case model.MyMediaLibrary:
			lib, ok := byID[e.LibraryID.Int64]
			if !ok {
				continue
			}
			tile.Library, tile.Count = &lib, lib.ItemCount
			libs = append(libs, lib)
		case model.MyMediaRadio:
			stations := s.radioRow(ctx, profile)
			tile.Count = len(stations)
			for _, st := range stations[:min(len(stations), model.MyMediaStations)] {
				tile.Stations = append(tile.Stations, st.Station)
			}
		default:
			if tile.Count, err = s.myMediaCount(ctx, scope, e.Key); err != nil {
				return nil, nil, err
			}
		}
		media = append(media, tile)
	}
	return media, libs, nil
}

// myMediaCount returns the count of the entry collections or picks; 0 for any other key.
func (s *Service) myMediaCount(ctx context.Context, scope visibility.Scope, key string) (int, error) {
	switch key {
	case model.MyMediaCollections:
		return s.vis.CollectionCount(ctx, scope)
	case model.MyMediaPicks:
		return s.vis.PicksCount(ctx, scope)
	}
	return 0, nil
}
