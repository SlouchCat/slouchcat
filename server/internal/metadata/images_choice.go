package metadata

import (
	"context"
	"fmt"
	"slices"
	"strconv"

	"github.com/slouchcat/slouchcat/server/internal/model"
	"github.com/slouchcat/slouchcat/server/internal/store/systemq"
)

// The image choice of concept 10 section 6.2 (ADR 0018): for the image kinds poster, backdrop and
// logo the fill (and the match, for the identifier's record) collects the candidates of every
// source in the fixed order ["local", …folder list, "fanart"] and decides per kind in six steps:
//
//  1. A chosen row (item_images.chosen = 1, the administrator's pick) wins; the field is not touched.
//  2. A local candidate with width >= the minimum width wins (a smaller one stays a candidate of
//     the image choice but never wins by itself, O-1).
//  3. Chain outside, source inside: for every entry of the language chain of the kind (a language
//     or "" = without text) the candidates of every provider in that language with width >= the
//     minimum width are collected; with image_by_vote the one with the most votes among those with
//     at least minVoteCount votes wins (ties by vote count, then by source rank), without it the
//     first in source order.
//  4. When no candidate meets chain and width, the same pass without the minimum width.
//  5. Otherwise the candidate the first source names as its default (poster_path).
//  6. Otherwise none; the kind stays empty.
//
// Local files have lang "" (unknown) and no votes; TVDB artworks carry a score instead of votes,
// so the vote rule applies to TMDB and Fanart.tv (both deliver vote counts) and leaves TVDB rows to
// the source order within a language.

// Source names of the image chain that never stand in a folder's provider list.
const (
	ImageSourceLocal  = model.ProviderLocal // files next to the media (item_local_images)
	ImageSourceFanart = "fanart"            // Fanart.tv (step 6)
	ImageSourceCover  = "cover"             // the cover of an audio title (coverSource)
	ImageSourceManual = "manual"            // an uploaded image
)

// minVoteCount is the number of votes an image needs before its vote average counts (6.2).
const minVoteCount = 3

// localImageSource is the item_images.source of a local file: file:<rel>?<size>-<mtime>, the
// pattern of coverSource, so a replaced poster.jpg counts as a changed source.
func localImageSource(rel string, size, mtime int64) string {
	return coverFilePrefix + rel + "?" + strconv.FormatInt(size, 10) + "-" + strconv.FormatInt(mtime, 10)
}

// localCandidates turns the scanner's rows of one title into candidates of the source "local".
func localCandidates(rows []systemq.ItemLocalImage) []ImageCandidate {
	out := make([]ImageCandidate, 0, len(rows))
	for _, r := range rows {
		out = append(out, ImageCandidate{Kind: r.Kind, Width: int(r.Width), Height: int(r.Height),
			Source: localImageSource(r.RelPath, r.Size, r.Mtime)})
	}
	return out
}

// imageSources is the source order of the image fields: local first (when switched on), then
// the folder's list, then Fanart.tv (when switched on). local and fanart never stand in the list.
func imageSources(st Settings, order []string) []string {
	out := make([]string, 0, len(order)+2)
	if st.LocalImages {
		out = append(out, ImageSourceLocal)
	}
	for _, p := range order {
		if p != ImageSourceLocal && p != ImageSourceFanart {
			out = append(out, p)
		}
	}
	if st.FanartEnabled {
		out = append(out, ImageSourceFanart)
	}
	return out
}

// sourceCandidates are the candidates of one source in source order.
type sourceCandidates struct {
	source string
	images []ImageCandidate
}

// chosenImage is the outcome of chooseImage.
type chosenImage struct {
	source string // the winning source (provider name, local, fanart)
	cand   ImageCandidate
}

// chooseImage runs steps 2 to 6 of the choice for one kind; ok is false when no source has an
// image of the kind (step 6). Step 1 (a chosen row) is the caller's: it never calls this then.
func chooseImage(kind string, st Settings, sources []sourceCandidates) (chosenImage, bool) {
	minWidth := st.ImageMinWidth[kind]
	// Step 2: a local file wide enough.
	for _, s := range sources {
		if s.source != ImageSourceLocal {
			continue
		}
		for _, c := range s.images {
			if c.Kind == kind && c.Width >= minWidth {
				return chosenImage{source: s.source, cand: c}, true
			}
		}
	}
	chain := st.ImageLangs[kind]
	if chain == nil {
		chain = []string{""}
	}
	// Steps 3 and 4: the chain with, then without the minimum width.
	if c, ok := chooseByChain(kind, chain, minWidth, st.ImageByVote, sources); ok {
		return c, true
	}
	if minWidth > 0 {
		if c, ok := chooseByChain(kind, chain, 0, st.ImageByVote, sources); ok {
			return c, true
		}
	}
	// Step 5: the default of the first source that names one.
	for _, s := range sources {
		if s.source == ImageSourceLocal {
			continue
		}
		for _, c := range s.images {
			if c.Kind == kind && c.Default {
				return chosenImage{source: s.source, cand: c}, true
			}
		}
	}
	return chosenImage{}, false
}

// chooseByChain is step 3: chain outside, source inside, local files excluded (they have no
// language and were step 2).
func chooseByChain(kind string, chain []string, minWidth int, byVote bool, sources []sourceCandidates) (chosenImage, bool) {
	for _, lang := range chain {
		var best chosenImage
		found := false
		for rank, s := range sources {
			if s.source == ImageSourceLocal {
				continue
			}
			for _, c := range s.images {
				if c.Kind != kind || PrimaryLanguage(c.Lang) != PrimaryLanguage(lang) || c.Width < minWidth {
					continue
				}
				cand := chosenImage{source: s.source, cand: c}
				if !found {
					best, found = cand, true
					if !byVote {
						break
					}
					continue
				}
				if byVote && better(cand, best, rank, slices.IndexFunc(sources, func(x sourceCandidates) bool { return x.source == best.source })) {
					best = cand
				}
			}
			if found && !byVote {
				break // the first source with a candidate in this language wins
			}
		}
		if found {
			return best, true
		}
	}
	return chosenImage{}, false
}

// better reports whether a beats b under the vote rule: more votes among those with at least
// minVoteCount votes; ties by vote count; then the earlier source.
func better(a, b chosenImage, rankA, rankB int) bool {
	av, bv := a.cand.Votes, b.cand.Votes
	if a.cand.VoteCount < minVoteCount {
		av = 0
	}
	if b.cand.VoteCount < minVoteCount {
		bv = 0
	}
	switch {
	case av != bv:
		return av > bv
	case a.cand.VoteCount != b.cand.VoteCount:
		return a.cand.VoteCount > b.cand.VoteCount
	default:
		return rankA < rankB
	}
}

// imageKindsOf lists the image kinds a title of this kind carries, after the switches of the
// settings: poster and backdrop always for films and series, the logo when switched on; the
// season poster and the episode still when switched on.
func imageKindsOf(itemKind string, st Settings) []string {
	switch itemKind {
	case "movie", "series":
		kinds := []string{"poster", "backdrop"}
		if st.HasImageKind(ImageKindLogo) {
			kinds = append(kinds, "logo")
		}
		return kinds
	case "season":
		if st.HasImageKind(ImageKindSeasonPoster) {
			return []string{"poster"}
		}
	case "episode":
		if st.HasImageKind(ImageKindEpisodeStill) {
			return []string{"backdrop"}
		}
	}
	return nil
}

// imageDecision is what decideImages writes for one kind.
type imageDecision struct {
	kind    string
	source  string // provider of item_images / item_field_sources
	cand    ImageCandidate
	changed bool // the source address differs from the stored row
}

// decideImages runs the choice for every image kind of the item: cands are the fields of the
// providers in list order, order the folder's list, local the scanner's rows of the title,
// stored the item_images rows. Locked kinds and chosen rows are skipped (steps 0 and 1).
// It returns the decisions (one per kind with a winner) and the kinds that end empty.
func decideImages(itemKind string, st Settings, order []string, cands []providerFields, local []systemq.ItemLocalImage,
	stored []systemq.ItemImage, locked map[string]bool,
) (decisions []imageDecision, empty []string) {
	byProvider := map[string][]ImageCandidate{}
	for _, c := range cands {
		byProvider[c.provider] = append(byProvider[c.provider], c.images...)
	}
	current := map[string]systemq.ItemImage{}
	for _, r := range stored {
		current[r.Kind] = r
	}
	var sources []sourceCandidates
	for _, src := range imageSources(st, order) {
		switch src {
		case ImageSourceLocal:
			sources = append(sources, sourceCandidates{source: src, images: localCandidates(local)})
		default:
			sources = append(sources, sourceCandidates{source: src, images: byProvider[src]})
		}
	}
	for _, kind := range imageKindsOf(itemKind, st) {
		if locked[kind] {
			continue
		}
		if row, ok := current[kind]; ok && row.Chosen == 1 {
			continue
		}
		c, ok := chooseImage(kind, st, sources)
		if !ok {
			empty = append(empty, kind)
			continue
		}
		row, had := current[kind]
		decisions = append(decisions, imageDecision{kind: kind, source: c.source, cand: c.cand,
			changed: !had || row.Source != c.cand.Source || row.Provider != c.source})
	}
	return decisions, empty
}

// writeImageDecisions upserts the item_images rows of the decisions and the provenance of the
// image fields (item_field_sources with lang ""), and reports whether any address changed.
func writeImageDecisions(ctx context.Context, q *systemq.Queries, itemID string, decisions []imageDecision, now int64) (bool, error) {
	changed := false
	for _, d := range decisions {
		if d.changed {
			if err := q.UpsertItemImage(ctx, systemq.UpsertItemImageParams{
				ItemID: itemID, Kind: d.kind, Provider: d.source, Lang: d.cand.Lang, Source: d.cand.Source, Chosen: 0, Now: now,
			}); err != nil {
				return false, fmt.Errorf("write %s image of %s: %w", d.kind, itemID, err)
			}
			changed = true
		}
		if err := q.UpsertItemFieldSource(ctx, systemq.UpsertItemFieldSourceParams{
			ItemID: itemID, Field: d.kind, Lang: "", Provider: d.source,
		}); err != nil {
			return false, fmt.Errorf("write source of %s image of %s: %w", d.kind, itemID, err)
		}
	}
	return changed, nil
}
