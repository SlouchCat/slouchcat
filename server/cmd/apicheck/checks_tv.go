package main

import (
	"context"
	"errors"
	"fmt"
	"strings"
)

// tvCapabilities is the report of the Android TV app with a 5.1 sound system that passes AC-3
// and E-AC-3 through (concept 26 5.2, stage 0): it selects tracks itself and draws PGS and ASS.
func tvCapabilities(containers ...string) *playbackCapabilities {
	return &playbackCapabilities{
		Containers:       containers,
		Video:            []string{"h264", "hevc"},
		Audio:            []string{"aac", "ac3", "eac3"},
		HLS:              "mse",
		MaxWidth:         1920,
		MaxHeight:        1080,
		Name:             "apicheck TV",
		Type:             "tv",
		MaxAudioChannels: 6,
		SelectsTracks:    true,
		Subtitles:        []string{"pgs", "ass"},
	}
}

// checkTVPlayback starts the MKV with its English AC-3 5.1 track as a TV (concept 26 stage 1):
// direct play of that track where the TV plays MKV (it selects the track itself); a remux and a
// transcode (480p asked) that copy the AC-3 track, with ac-3 in the CODECS of the master playlist
// and segments that come.
func checkTVPlayback(ctx context.Context, s *state) (string, error) {
	var tracks trackList
	if err := s.c.get(ctx, "/items/"+s.multi.ID+"/tracks", nil, &tracks); err != nil {
		return "", err
	}
	var ac3 *track
	for i, t := range tracks.Audio {
		if t.Codec == "ac3" && t.Channels == 6 && !t.Default {
			ac3 = &tracks.Audio[i]
		}
	}
	if ac3 == nil {
		return "", fmt.Errorf("%q carries no AC-3 5.1 track besides the default one", s.multi.Title)
	}
	index := ac3.Index

	direct, err := s.play(ctx, s.multi.ID, playbackRequest{Capabilities: tvCapabilities("mp4", "mkv"), AudioTrack: &index})
	if err != nil {
		return "", err
	}
	defer s.stop(ctx, direct.PlaybackSessionID)
	if direct.Mode != "direct" || direct.Selection.AudioTrack == nil || *direct.Selection.AudioTrack != index {
		return "", fmt.Errorf("a TV that selects tracks got %q with audio %v instead of direct play of track %d",
			direct.Mode, direct.Selection.AudioTrack, index)
	}

	var evidence []string
	for _, run := range []struct{ name, quality, mode string }{{"remux", "", "remux"}, {"transcode", "480p", "transcode"}} {
		info, err := s.play(ctx, s.multi.ID, playbackRequest{Capabilities: tvCapabilities("mp4"), AudioTrack: &index, Quality: run.quality})
		if err != nil {
			return "", err
		}
		line, err := s.tvHLS(ctx, info, run.mode)
		s.stop(ctx, info.PlaybackSessionID)
		if err != nil {
			return "", fmt.Errorf("%s: %w", run.name, err)
		}
		evidence = append(evidence, line)
	}
	return fmt.Sprintf("Fernseher (5.1, wählt Spuren, PGS/ASS): MKV mit %q → direct; nur MP4: %s",
		ac3.Title, strings.Join(evidence, "; ")), nil
}

// tvHLS checks an HLS playback of the TV: the mode, ac-3 in the CODECS of the master playlist, the
// init segment and the first two segments (the second one is the first that the delayed moov
// moves into a later file).
func (s *state) tvHLS(ctx context.Context, info playbackInfo, mode string) (string, error) {
	if info.Mode != "hls" || info.Transcoding == nil || info.Transcoding.Mode != mode {
		return "", fmt.Errorf("got %q (%+v) instead of HLS as %s", info.Mode, info.Transcoding, mode)
	}
	master, err := s.text(ctx, info.StreamURL)
	if err != nil {
		return "", err
	}
	codecs := attribute(master, "#EXT-X-STREAM-INF:", "CODECS")
	if !strings.HasSuffix(codecs, ",ac-3") {
		return "", fmt.Errorf("the master playlist names the codecs %q, want the copied AC-3 (ac-3)", codecs)
	}
	mediaURL := resolve(info.StreamURL, firstURI(master))
	media, err := s.text(ctx, mediaURL)
	if err != nil {
		return "", err
	}
	initSegment, err := s.bytes(ctx, resolve(mediaURL, attribute(media, "#EXT-X-MAP:", "URI")))
	if err != nil {
		return "", err
	}
	var refs []string
	for _, line := range strings.Split(media, "\n") {
		if line = strings.TrimSpace(line); line != "" && !strings.HasPrefix(line, "#") {
			refs = append(refs, line)
		}
	}
	if len(refs) < 2 {
		return "", fmt.Errorf("the media playlist names %d segments, want at least two", len(refs))
	}
	sizes := make([]int, 2)
	for i, ref := range refs[:2] {
		seg, err := s.bytes(ctx, resolve(mediaURL, ref))
		if err != nil {
			return "", err
		}
		if len(seg) < 8 || string(seg[4:8]) != "moof" {
			return "", fmt.Errorf("segment %d does not start with a moof box", i)
		}
		sizes[i] = len(seg)
	}
	if len(initSegment) < 8 || string(initSegment[4:8]) != "ftyp" {
		return "", errors.New("the init segment does not start with ftyp")
	}
	return fmt.Sprintf("%s (Grund %s), CODECS %s, init.mp4 %d Bytes, Segmente 0 und 1 mit %d und %d Bytes",
		mode, info.Transcoding.Reason, codecs, len(initSegment), sizes[0], sizes[1]), nil
}
