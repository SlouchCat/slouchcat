package visibility

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/slouchcat/slouchcat/server/internal/model"
	"github.com/slouchcat/slouchcat/server/internal/store/scopedq"
	"github.com/slouchcat/slouchcat/server/internal/transcode"
)

// Reads of M4 (design sections 2 and 5): the audio and subtitle tracks of a media file (embedded
// streams and sidecar files, I-5), the detected intro and outro of a title (W-9), the next episode
// for auto-next and the file details of the detail page (bit rate, HDR format, tracks).

// textCodecs are the subtitle formats that can be delivered as WebVTT; everything else (PGS, DVD
// and VobSub bitmaps) is burned in.
var textCodecs = map[string]bool{"subrip": true, "srt": true, "ass": true, "ssa": true, "webvtt": true, "mov_text": true, "text": true}

// TextSubtitle reports whether a subtitle codec is a text format (deliverable as WebVTT).
func TextSubtitle(codec string) bool { return textCodecs[codec] }

// Tracks returns the audio and subtitle tracks of a media file of a visible title. A file of
// another title, an unknown file and an invisible title return model.ErrNotFound. RelPath of a
// sidecar track is relative to the file's library root (MediaFileLocation.RootPath); it never
// leaves the server.
func (r *Reader) Tracks(ctx context.Context, s Scope, itemID, fileID string) (model.TrackList, error) {
	pid, err := r.profile(ctx, s)
	if err != nil {
		return model.TrackList{}, err
	}
	pf, err := r.q.GetVisibleMediaFileProbe(ctx, scopedq.GetVisibleMediaFileProbeParams{ProfileID: pid, ItemID: itemID, MediaFileID: fileID})
	if err != nil {
		return model.TrackList{}, notFound(err)
	}
	rows, err := r.q.ListVisibleMediaFileTracks(ctx, scopedq.ListVisibleMediaFileTracksParams{ProfileID: pid, ItemID: itemID, MediaFileID: fileID})
	if err != nil {
		return model.TrackList{}, fmt.Errorf("list tracks: %w", err)
	}
	return trackList(fileID, rows, channelsOf(parseProbe(pf.Probe.String))), nil
}

// Segments returns the detected intro and outro of a visible title; zero Segments when none is
// known and for a title the profile may not see (callers check visibility first).
func (r *Reader) Segments(ctx context.Context, s Scope, itemID string) (model.Segments, error) {
	pid, err := r.profile(ctx, s)
	if err != nil {
		return model.Segments{}, err
	}
	rows, err := r.q.ListVisibleSegments(ctx, scopedq.ListVisibleSegmentsParams{ProfileID: pid, ItemID: itemID})
	if err != nil {
		return model.Segments{}, fmt.Errorf("list segments: %w", err)
	}
	var out model.Segments
	for _, row := range rows {
		seg := &model.Segment{StartMs: row.StartMs, EndMs: row.EndMs, Source: row.Source}
		switch row.Kind {
		case "intro":
			out.Intro = seg
		case "outro":
			out.Outro = seg
		}
	}
	return out, nil
}

// NextEpisode returns the next visible episode after a visible episode (the rule of the detail
// page: a higher number in the same season, else the first episode of a later season; specials
// only follow a special), or nil for the last episode and for other kinds.
func (r *Reader) NextEpisode(ctx context.Context, s Scope, itemID string) (*model.ItemSummary, error) {
	pid, err := r.profile(ctx, s)
	if err != nil {
		return nil, err
	}
	it, err := r.q.GetVisibleItem(ctx, scopedq.GetVisibleItemParams{ProfileID: pid, ItemID: itemID})
	if err != nil {
		return nil, notFound(err)
	}
	if it.Kind != "episode" {
		return nil, nil
	}
	var d model.ItemDetail
	if err := r.episodeChain(ctx, s, pid, it, &d); err != nil {
		return nil, err
	}
	return d.NextEpisode, nil
}

// fileDetails completes the media files of a detail with their tracks, bit rate and HDR format.
func (r *Reader) fileDetails(ctx context.Context, pid int64, itemID string, files []model.MediaFile) error {
	if len(files) == 0 {
		return nil
	}
	probes, err := r.q.ListVisibleMediaFileProbes(ctx, scopedq.ListVisibleMediaFileProbesParams{ProfileID: pid, ItemID: itemID})
	if err != nil {
		return fmt.Errorf("list media file probes: %w", err)
	}
	byFile := map[string]*transcode.Probe{}
	for _, p := range probes {
		byFile[p.ID] = parseProbe(p.Probe.String)
	}
	rows, err := r.q.ListVisibleItemTracks(ctx, scopedq.ListVisibleItemTracksParams{ProfileID: pid, ItemID: itemID})
	if err != nil {
		return fmt.Errorf("list tracks: %w", err)
	}
	tracks := map[string][]scopedq.Track{}
	for _, t := range rows {
		tracks[t.MediaFileID] = append(tracks[t.MediaFileID], t)
	}
	for i := range files {
		f := &files[i]
		p := byFile[f.ID]
		f.Bitrate, f.HDR = probeDetails(p)
		list := trackList(f.ID, tracks[f.ID], channelsOf(p))
		f.Tracks = &list
	}
	return nil
}

// parseProbe decodes a stored probe; nil when the file is not probed or the JSON is unreadable.
func parseProbe(raw string) *transcode.Probe {
	if raw == "" {
		return nil
	}
	var p transcode.Probe
	if err := json.Unmarshal([]byte(raw), &p); err != nil {
		return nil
	}
	return &p
}

// probeDetails returns the container bit rate and the HDR format of the main video stream.
func probeDetails(p *transcode.Probe) (bitrate int64, hdr string) {
	if p == nil {
		return 0, ""
	}
	for _, s := range p.Streams {
		if s.Type == "video" && !s.AttachedPic {
			hdr = s.HDRFormat()
			break
		}
	}
	return p.Format.BitRate, hdr
}

// channelsOf maps the stream index of every audio stream to its channel count.
func channelsOf(p *transcode.Probe) map[int]int {
	out := map[int]int{}
	if p == nil {
		return out
	}
	for _, s := range p.Streams {
		if s.Type == "audio" && s.Channels > 0 {
			out[s.Index] = s.Channels
		}
	}
	return out
}

// trackList maps the track rows of one file; sidecar tracks get the index -1.
func trackList(fileID string, rows []scopedq.Track, channels map[int]int) model.TrackList {
	out := model.TrackList{MediaFileID: fileID, Audio: []model.Track{}, Subtitles: []model.Track{}}
	for _, t := range rows {
		tr := model.Track{
			ID: t.ID, Index: int(t.StreamIndex), Kind: t.Kind, Codec: t.Codec.String, Language: t.Language.String,
			Title: t.Title.String, Default: t.IsDefault == 1, Forced: t.IsForced == 1, Source: t.Source, RelPath: t.RelPath.String,
		}
		if t.Source == "sidecar" {
			tr.Index = -1
		}
		switch t.Kind {
		case "audio":
			tr.Channels = channels[tr.Index]
			out.Audio = append(out.Audio, tr)
		case "subtitle":
			tr.Text = TextSubtitle(tr.Codec)
			out.Subtitles = append(out.Subtitles, tr)
		}
	}
	return out
}
