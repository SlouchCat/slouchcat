package transcode

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
)

// Tags of stored podcast episodes (concept 19 part A, 6.5, O-7; ADR 0029, decision 4): before a
// downloaded episode is entered, ffmpeg writes its title, its podcast, the podcast's author and its
// publication date into a copy of the file, without re-encoding. Tags of the provider that
// Slouchcat does not set stay, and so does an embedded cover. ffmpeg never opens the copy by a
// path in the podcast folder (CLAUDE.md rule 4): it writes into a private temporary directory of
// the server, and the copy goes from there into a writer the caller created through the folder's
// *os.Root. A pipe does not do: both muxers seek back to finish their header (the size of the
// ID3v2 tag, the index of MP4), and without the seek the tags are lost.

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
// with tags, all streams copied (-map 0 -c copy), in the format of ext, into out: ffmpeg writes
// into a private temporary directory below tmpDir (the server's cache; "" is the system's), which
// goes when the copy is in out. Another extension than mp3, m4a or m4b is ErrNotTaggable and
// writes nothing. The caller probes the copy before it uses it.
func (r Runner) TagAudio(ctx context.Context, in string, out io.Writer, ext string, tags AudioTags, tmpDir string) error {
	format, ok := tagFormats[ext]
	if !ok {
		return ErrNotTaggable
	}
	args := []string{"-nostdin", "-hide_banner", "-loglevel", "error", "-i", in, "-map", "0", "-c", "copy"}
	for _, kv := range [][2]string{{"title", tags.Title}, {"album", tags.Album}, {"artist", tags.Artist}, {"date", tags.Date}} {
		if kv[1] != "" {
			args = append(args, "-metadata", kv[0]+"="+kv[1])
		}
	}
	dir, err := os.MkdirTemp(tmpDir, "podcast-tags-")
	if err != nil {
		return fmt.Errorf("temporary directory of the tags: %w", err)
	}
	defer func() { _ = os.RemoveAll(dir) }()
	tmp := filepath.Join(dir, "tagged."+ext)
	if err := r.ffmpegTo(ctx, io.Discard, slices.Concat(args, format, []string{tmp})...); err != nil {
		return err
	}
	f, err := os.Open(tmp) //nolint:gosec // G304: the file ffmpeg wrote into the private temporary directory above
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = io.Copy(out, f)
	return err
}

// ffmpegTo runs ffmpeg with args and its standard output in out, without buffering it.
func (r Runner) ffmpegTo(ctx context.Context, out io.Writer, args ...string) error {
	if r.FFmpegPath == "" {
		return errNoFFmpeg
	}
	var stderr bytes.Buffer
	cmd := exec.CommandContext(ctx, r.FFmpegPath, args...) //nolint:gosec // G204: configured binary, fixed argument list, paths resolved by package paths
	cmd.Stdout = out
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("ffmpeg: %w: %s", err, imageErrTail(strings.TrimSpace(stderr.String()), 2048))
	}
	return nil
}
