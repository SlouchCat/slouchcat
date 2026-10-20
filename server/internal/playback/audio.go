package playback

import (
	"context"

	"github.com/slouchcat/slouchcat/server/internal/model"
	"github.com/slouchcat/slouchcat/server/internal/transcode"
	"github.com/slouchcat/slouchcat/server/internal/visibility"
)

// Audiobooks (M6 design 3.3): a book plays as one timeline. A book of one file is played like any
// other file; a book of several parts always plays as audio-only HLS over all parts in order
// (transcode.Source.Parts), so positions, progress and chapters are book-wide.

// input is what a playback reads: the chosen (or primary) media file and, for an audiobook, its
// parts in order with the probe of the whole book.
type input struct {
	loc   visibility.MediaFileLocation
	parts []visibility.MediaFileLocation // audiobook parts in order; nil for other kinds
	probe *transcode.Probe               // nil when a file is not probed yet
	kf    []byte                         // keyframe index of loc (single files only)
}

// multiPart reports whether the input is an audiobook of several files.
func (in input) multiPart() bool { return len(in.parts) > 1 }

// file is the media file the decision and the client see: the chosen file, for a book of several
// parts its first part with the duration and size of the whole book.
func (in input) file() model.MediaFile {
	f := in.loc.File
	if !in.multiPart() {
		return f
	}
	f.DurationMs, f.SizeBytes = 0, 0
	for _, p := range in.parts {
		f.DurationMs += p.File.DurationMs
		f.SizeBytes += p.File.SizeBytes
	}
	return f
}

// playbackParts lists the parts of a book of several files on its timeline (PlaybackInfo.parts,
// information only); nil for anything else.
func (in input) playbackParts() []model.PlaybackPart {
	if !in.multiPart() {
		return nil
	}
	out := make([]model.PlaybackPart, 0, len(in.parts))
	var start int64
	for _, p := range in.parts {
		out = append(out, model.PlaybackPart{MediaFileID: p.File.ID, StartMs: start, DurationMs: p.File.DurationMs})
		start += p.File.DurationMs
	}
	return out
}

// loadInput reads the media of a visible title: the media file fileID (empty: the primary one)
// with its stored probe, or for an audiobook its present parts in order. The probe of a book of
// several parts is the merged one (transcode.ConcatProbe), nil while a part is not probed. A
// missing file is model.ErrNotFound.
func (s *Service) loadInput(ctx context.Context, scope visibility.Scope, item model.ItemSummary, fileID string) (input, error) {
	if item.Kind != "audiobook" {
		loc, err := s.vis.MediaFile(ctx, scope, item.ID, fileID)
		if err != nil {
			return input{}, err
		}
		if !loc.File.Available {
			return input{}, model.ErrNotFound
		}
		probe, kf, err := s.storedProbe(ctx, scope, loc)
		if err != nil {
			return input{}, err
		}
		return input{loc: loc, probe: probe, kf: kf}, nil
	}
	parts, err := s.vis.Parts(ctx, scope, item.ID)
	if err != nil {
		return input{}, err
	}
	in := input{loc: parts[0], parts: parts}
	if !in.multiPart() {
		in.probe, in.kf, err = s.storedProbe(ctx, scope, in.loc)
		return in, err
	}
	probes := make([]transcode.Probe, 0, len(parts))
	for _, p := range parts {
		pr, _, err := s.storedProbe(ctx, scope, p)
		if err != nil {
			return input{}, err
		}
		if pr == nil {
			return in, nil
		}
		probes = append(probes, *pr)
	}
	book := transcode.ConcatProbe(probes)
	in.probe = &book
	return in, nil
}

// bookChapters returns the book-wide chapters of an audiobook from the chapters table; nil for
// other kinds and for a book without stored chapters (the probe's chapter marks apply then).
func (s *Service) bookChapters(ctx context.Context, scope visibility.Scope, item model.ItemSummary) ([]model.Chapter, error) {
	if item.Kind != "audiobook" {
		return nil, nil
	}
	c, err := s.vis.Chapters(ctx, scope, item.ID)
	if err != nil || len(c) == 0 {
		return nil, err
	}
	return c, nil
}
