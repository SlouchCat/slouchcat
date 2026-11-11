package transcode

import (
	"context"
	"errors"
	"slices"
)

// Tags of stored podcast episodes (concept 19 part A, 6.5, O-7; ADR 0029, decision 4): before a
// downloaded episode is entered, ffmpeg writes its title, its podcast, the podcast's author and its
// publication date into a copy of the file, without re-encoding. Tags of the provider that
// Slouchcat does not set stay, and so does an embedded cover.

// ErrNotTaggable is the answer of TagAudio for a format it does not tag (only mp3, m4a and m4b).
var ErrNotTaggable = errors.New("format is not tagged")

// AudioTags are the tags TagAudio sets; an empty field leaves the tag of the file as it is.
type AudioTags struct {
	Title  string // the episode
	Album  string // the podcast
	Artist string // the author of the feed
	Date   string // the publication, RFC 3339 in UTC
}

// tagFormats are the muxers TagAudio writes per extension, with their options: ID3v2.4 for mp3,
// MP4 tags (the ipod muxer of .m4a and .m4b) for m4a and m4b.
var tagFormats = map[string][]string{
	"mp3": {"-id3v2_version", "4", "-f", "mp3"},
	"m4a": {"-f", "ipod"},
	"m4b": {"-f", "ipod"},
}

// TagAudio copies the audio file in (absolute path, resolved by the caller through package paths)
// to out with tags, all streams copied (-map 0 -c copy), in the format of ext; out is overwritten.
// Another extension than mp3, m4a or m4b is ErrNotTaggable and writes nothing. The caller probes
// out before it uses it.
func (r Runner) TagAudio(ctx context.Context, in, out, ext string, tags AudioTags) error {
	format, ok := tagFormats[ext]
	if !ok {
		return ErrNotTaggable
	}
	args := []string{"-nostdin", "-hide_banner", "-loglevel", "error", "-y", "-i", in, "-map", "0", "-c", "copy"}
	for _, kv := range [][2]string{{"title", tags.Title}, {"album", tags.Album}, {"artist", tags.Artist}, {"date", tags.Date}} {
		if kv[1] != "" {
			args = append(args, "-metadata", kv[0]+"="+kv[1])
		}
	}
	_, err := r.runFFmpeg(ctx, nil, slices.Concat(args, format, []string{out})...)
	return err
}
