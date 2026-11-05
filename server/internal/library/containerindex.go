package library

import (
	"context"
	"errors"
	"hash/fnv"
	"io"
	"os"
	"strings"
	"time"

	"github.com/slouchcat/slouchcat/server/internal/mediaindex"
	"github.com/slouchcat/slouchcat/server/internal/paths"
	"github.com/slouchcat/slouchcat/server/internal/transcode"
)

// indexReadTimeout bounds mediaindex.Read (concept 14, 6.1): a read on a hanging network share
// cannot be cancelled, so the parser runs in its own goroutine (measured maximum 0.76 s).
const indexReadTimeout = 10 * time.Second

// errIndexReadTimeout: the container index was not read within indexReadTimeout.
var errIndexReadTimeout = errors.New("reading the container index timed out")

// indexWorthy reports whether a media file gets a keyframe index (concept 14, 2.1 and 4.1): a
// present file whose video the server can remux (transcode.CopyableVideo), outside audiobooks
// and podcast episodes, which the automatic paths leave out (O-18). Only these files are read by
// probe.file, the full read and the backfill; the SQL of the backfill and the coverage counts
// must state the same condition.
func indexWorthy(present bool, videoCodec, kind string) bool {
	return present && transcode.CopyableVideo(videoCodec) && kind != "audiobook" && kind != "podcast_episode"
}

// indexTarget is a probed media file for indexFromContainer.
type indexTarget struct {
	id        string     // media file id: seeds the sample, names the file in logs
	root      paths.Root // its library root
	dir       *os.Root   // the root opened through paths.Open
	rel       string     // path relative to the root
	container string     // media_files.container (canonicalContainer)
	duration  time.Duration
	codec     string // codec of the main video stream, "" without video
	kind      string // items.kind of its title
	muxer     string // writing application from the container tags, for the log; may be ""
}

// containerIndex is the outcome of indexFromContainer.
type containerIndex struct {
	// worthy is false for a file that gets no index and no full read (indexWorthy).
	worthy bool
	// keyframes is the checked index from the container with its source (KeyframesCues,
	// KeyframesMP4); empty for a worthy file means the full read (probe.keyframes) is needed.
	keyframes []time.Duration
	source    string
	// timedOut is set when the container could not be read within indexReadTimeout (the
	// backfill stops after three in a row, concept 14, 6.1).
	timedOut bool
}

// indexFromContainer decides whether a probed file is indexworthy and, if so, reads the keyframe
// index its container carries (concept 14, 2.2): mediaindex.Read in its own goroutine with
// indexReadTimeout, then mediaindex.Check and the sample transcode.Runner.VerifyKeyframes. Any
// failure means «full read needed»; an index that fails the checks is logged as a warning. It
// runs outside any transaction: it reads the file and starts ffprobe. The error is only that of
// ctx.
func (s *Service) indexFromContainer(ctx context.Context, t indexTarget) (containerIndex, error) {
	if !indexWorthy(true, t.codec, t.kind) {
		return containerIndex{}, nil
	}
	res := containerIndex{worthy: true}
	switch t.container {
	case "mkv", "webm", "mp4", "m4v", "mov":
	default:
		return res, nil // no index in the container: the full read
	}
	ix, err := s.readContainerIndex(ctx, t)
	switch {
	case ctx.Err() != nil:
		return containerIndex{}, ctx.Err()
	case errors.Is(err, errIndexReadTimeout):
		s.log.WarnContext(ctx, "container index not read in time, full read queued", "media_file_id", t.id, "path", t.rel,
			"timeout", indexReadTimeout)
		res.timedOut = true
		return res, nil
	case err != nil:
		s.log.InfoContext(ctx, "no keyframe index in the container, full read queued", "media_file_id", t.id, "path", t.rel,
			"err", s.relErr(t, err))
		return res, nil
	}
	if err := mediaindex.Check(ix, t.duration); err != nil {
		s.log.WarnContext(ctx, "implausible keyframe index in the container, full read queued", "media_file_id", t.id,
			"path", t.rel, "source", string(ix.Source), "muxer", t.muxer, "err", err)
		return res, nil
	}
	abs, err := paths.Resolve(t.root, t.rel)
	if err != nil {
		return res, nil
	}
	verify := s.probe.VerifyKeyframes
	if s.keyframeVerifier != nil {
		verify = s.keyframeVerifier
	}
	if err := verify(ctx, abs, ix.Keyframes, sampleSeed(t.id)); err != nil {
		if ctx.Err() != nil {
			return containerIndex{}, ctx.Err()
		}
		msg := "keyframe index of the container failed the sample, full read queued"
		if !errors.Is(err, transcode.ErrKeyframeMismatch) {
			msg = "keyframe index of the container not verified, full read queued"
		}
		s.log.WarnContext(ctx, msg, "media_file_id", t.id, "path", t.rel, "source", string(ix.Source), "muxer", t.muxer,
			"err", strings.ReplaceAll(err.Error(), abs, t.rel))
		return res, nil
	}
	res.keyframes, res.source = ix.Keyframes, string(ix.Source)
	return res, nil
}

// readContainerIndex opens the file below t.dir and runs mediaindex.Read on it in its own
// goroutine; after indexReadTimeout (or the end of ctx) it returns without waiting, and the
// goroutine closes the file when its read returns.
func (s *Service) readContainerIndex(ctx context.Context, t indexTarget) (mediaindex.Index, error) {
	read := mediaindex.Read
	if s.indexReader != nil {
		read = s.indexReader
	}
	type result struct {
		ix  mediaindex.Index
		err error
	}
	done := make(chan result, 1)
	go func() {
		f, err := t.dir.Open(t.rel)
		if err != nil {
			done <- result{err: err}
			return
		}
		defer f.Close()
		st, err := f.Stat()
		if err != nil {
			done <- result{err: err}
			return
		}
		ix, err := read(f, st.Size(), t.container)
		done <- result{ix, err}
	}()
	timer := time.NewTimer(indexReadTimeout)
	defer timer.Stop()
	select {
	case r := <-done:
		return r.ix, r.err
	case <-timer.C:
		return mediaindex.Index{}, errIndexReadTimeout
	case <-ctx.Done():
		return mediaindex.Index{}, ctx.Err()
	}
}

// relErr returns the message of err with the root directory taken out, so a log line names the
// file relative to its root only.
func (s *Service) relErr(t indexTarget, err error) string {
	return strings.ReplaceAll(err.Error(), t.root.Dir()+"/", "")
}

// sampleSeed derives the seed of the sample of VerifyKeyframes from the media file id, so a file
// is always checked at the same entries.
func sampleSeed(mediaFileID string) uint64 {
	h := fnv.New64a()
	_, _ = io.WriteString(h, mediaFileID)
	return h.Sum64()
}
