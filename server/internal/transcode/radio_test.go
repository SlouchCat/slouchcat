package transcode

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"
)

func TestRadioOutputFor(t *testing.T) {
	for _, tc := range []struct {
		in   RadioInput
		want RadioOutput
	}{
		{RadioInput{Format: "aac", Codec: "aac", BitrateKbps: 64}, RadioOutput{Copy: true, BitrateKbps: 64}},
		{RadioInput{Format: "aac", Codec: "he-aac", BitrateKbps: 48}, RadioOutput{Copy: true, BitrateKbps: 48}},
		{RadioInput{Format: "mp3", Codec: "mp3", BitrateKbps: 64}, RadioOutput{BitrateKbps: 96}},
		{RadioInput{Format: "mp3", Codec: "mp3", BitrateKbps: 320}, RadioOutput{BitrateKbps: 192}},
		{RadioInput{Format: "ogg", Codec: "opus", BitrateKbps: 128}, RadioOutput{BitrateKbps: 128}},
		{RadioInput{Format: "ogg", Codec: "vorbis"}, RadioOutput{BitrateKbps: 128}},
	} {
		if got := RadioOutputFor(tc.in); got != tc.want {
			t.Errorf("RadioOutputFor(%+v) = %+v, want %+v", tc.in, got, tc.want)
		}
	}
}

func TestRadioPackagerArgsHaveNoURL(t *testing.T) {
	for _, in := range []RadioInput{
		{Format: "mp3", Codec: "mp3", BitrateKbps: 128, Channels: 2, SampleRate: 44100},
		{Format: "aac", Codec: "aac", BitrateKbps: 64},
		{Format: "mpegts", Codec: "aac"},
		{Format: "ogg", Codec: "opus", Channels: 6, SampleRate: 48000},
		{Format: "flac", Codec: "flac", SampleRate: 96000},
	} {
		out := RadioOutputFor(in)
		args := RadioPackagerArgs(in, out, 2*time.Second)
		for _, a := range args {
			if strings.Contains(a, "://") {
				t.Errorf("%s: argument %q holds a URL", in.Format, a)
			}
		}
		wl, input := slices.Index(args, "-protocol_whitelist"), slices.Index(args, "-i")
		if wl < 0 || args[wl+1] != "pipe" || input < wl {
			t.Errorf("%s: -protocol_whitelist pipe must precede -i: %v", in.Format, args)
		}
		if input < 0 || args[input+1] != "pipe:0" || args[len(args)-1] != "pipe:1" {
			t.Errorf("%s: input/output not pipe:0/pipe:1: %v", in.Format, args)
		}
		joined := strings.Join(args, " ")
		if out.Copy != strings.Contains(joined, "delay_moov") {
			t.Errorf("%s: delay_moov must be set in copy mode only: %v", in.Format, args)
		}
		if wantBSF := out.Copy && in.Format != "mp4"; wantBSF != strings.Contains(joined, "aac_adtstoasc") {
			t.Errorf("%s: aac_adtstoasc only for ADTS/TS copy: %v", in.Format, args)
		}
		if !strings.Contains(joined, "-frag_duration 2000000") {
			t.Errorf("%s: fragment duration missing: %v", in.Format, args)
		}
		if in.Codec == "opus" && !strings.Contains(joined, "-ac 2") {
			t.Errorf("opus 5.1 must be downmixed to stereo: %v", args)
		}
		if in.Codec == "flac" && !strings.Contains(joined, "-ar 48000") {
			t.Errorf("96 kHz must be resampled: %v", args)
		}
	}
}

// radioTone generates seconds of a sine tone with codec in the ffmpeg muxer format.
func radioTone(t *testing.T, r Runner, format, codec, bitrate string, seconds int) []byte {
	t.Helper()
	path := filepath.Join(t.TempDir(), "tone."+format)
	runFFmpeg(t, r, "-f", "lavfi", "-i", "sine=frequency=440:sample_rate=44100:duration="+strconv.Itoa(seconds),
		"-ac", "2", "-c:a", codec, "-b:a", bitrate, "-f", format, path)
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return data
}

// packageRadio streams data in 64-KiB writes through a packager and returns the init part and
// the media fragments.
func packageRadio(t *testing.T, r Runner, in RadioInput, data []byte) (RadioFragment, []RadioFragment) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	p, err := r.StartRadioPackager(ctx, in, RadioOutputFor(in), 2*time.Second, slog.New(slog.DiscardHandler))
	if err != nil {
		t.Fatal(err)
	}
	writeErr := make(chan error, 1)
	go func() {
		for b := data; len(b) > 0; {
			n := min(len(b), 64<<10)
			if _, err := p.Write(b[:n]); err != nil {
				writeErr <- err
				return
			}
			b = b[n:]
		}
		writeErr <- p.Close()
	}()
	var init RadioFragment
	var frags []RadioFragment
	for {
		f, err := p.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			t.Fatal(err)
		}
		switch {
		case f.Init && init.Data == nil && frags == nil:
			init = f
		case f.Init:
			t.Fatal("second init part")
		default:
			if init.Data == nil {
				t.Fatal("fragment before the init part")
			}
			frags = append(frags, f)
		}
	}
	if err := <-writeErr; err != nil {
		t.Fatal(err)
	}
	return init, frags
}

func checkRadioFragments(t *testing.T, r Runner, init RadioFragment, frags []RadioFragment) {
	t.Helper()
	if n := len(frags); n < 5 || n > 7 {
		t.Fatalf("%d fragments, want 6 ±1", n)
	}
	var total time.Duration
	for i, f := range frags {
		total += f.Duration
		if f.Duration > 2100*time.Millisecond || (i < len(frags)-1 && f.Duration < 1900*time.Millisecond) {
			t.Errorf("fragment %d lasts %v, want 2 s ±0.1 s", i, f.Duration)
		}
		if typ := string(f.Data[4:8]); typ != "moof" {
			t.Errorf("fragment %d starts with %q", i, typ)
		}
	}
	if total < 11800*time.Millisecond || total > 12200*time.Millisecond {
		t.Errorf("fragments last %v together, want 12 s", total)
	}
	path := filepath.Join(t.TempDir(), "joined.mp4")
	joined := slices.Clone(init.Data)
	for _, f := range frags {
		joined = append(joined, f.Data...)
	}
	if err := os.WriteFile(path, joined, 0o600); err != nil {
		t.Fatal(err)
	}
	probe, err := r.Probe(context.Background(), path)
	if err != nil {
		t.Fatal(err)
	}
	if len(probe.Streams) != 1 || probe.Streams[0].Codec != "aac" {
		t.Fatalf("joined fragments: streams %+v, want one aac stream", probe.Streams)
	}
	if err := r.CheckRadioAudio(context.Background(), path); err != nil {
		t.Fatal(err)
	}
}

func TestRadioPackagerFragmentsMP3(t *testing.T) {
	r := hlsRunner(t)
	data := radioTone(t, r, "mp3", "libmp3lame", "128k", 12)
	init, frags := packageRadio(t, r, RadioInput{Format: "mp3", Codec: "mp3", BitrateKbps: 128, Channels: 2, SampleRate: 44100}, data)
	checkRadioFragments(t, r, init, frags)
}

func TestRadioPackagerFragmentsADTSCopy(t *testing.T) {
	r := hlsRunner(t)
	data := radioTone(t, r, "adts", "aac", "64k", 12)
	init, frags := packageRadio(t, r, RadioInput{Format: "aac", Codec: "aac", BitrateKbps: 64, Channels: 2, SampleRate: 44100}, data)
	checkRadioFragments(t, r, init, frags)
}

func TestRadioPackagerInitHasAudioSpecificConfig(t *testing.T) {
	r := hlsRunner(t)
	for _, tc := range []struct {
		name string
		in   RadioInput
		data []byte
	}{
		{"copy", RadioInput{Format: "aac", Codec: "aac", BitrateKbps: 64}, radioTone(t, r, "adts", "aac", "64k", 6)},
		{"transcode", RadioInput{Format: "mp3", Codec: "mp3", BitrateKbps: 128}, radioTone(t, r, "mp3", "libmp3lame", "128k", 6)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			init, frags := packageRadio(t, r, tc.in, tc.data)
			asc, err := audioSpecificConfig(init.Data)
			if err != nil {
				t.Fatal(err)
			}
			if len(asc) < 2 {
				t.Fatalf("DecoderSpecificInfo has %d bytes, want ≥ 2", len(asc))
			}
			if aot := asc[0] >> 3; aot != 2 && aot != 5 {
				t.Fatalf("audio object type %d, want 2 (LC) or 5 (SBR)", aot)
			}
			path := filepath.Join(t.TempDir(), "first.mp4")
			if err := os.WriteFile(path, append(slices.Clone(init.Data), frags[0].Data...), 0o600); err != nil {
				t.Fatal(err)
			}
			probe, err := r.Probe(context.Background(), path)
			if err != nil {
				t.Fatal(err)
			}
			if len(probe.Streams) != 1 || probe.Streams[0].Profile == "" || probe.Streams[0].Profile == "-1" {
				t.Fatalf("ffprobe streams %+v, want an AAC profile", probe.Streams)
			}
		})
	}
}

// audioSpecificConfig walks moov/trak/mdia/minf/stbl/stsd/mp4a/esds of an init part to the
// DecoderSpecificInfo (tag 0x05) inside the DecoderConfigDescriptor (tag 0x04).
func audioSpecificConfig(init []byte) ([]byte, error) {
	boxes, err := walkBoxes(init)
	if err != nil {
		return nil, err
	}
	i := slices.IndexFunc(boxes, func(b box) bool { return b.Type == "moov" })
	if i < 0 {
		return nil, errors.New("no moov")
	}
	stsd := boxPayload(init[boxes[i].Offset:boxes[i].Offset+boxes[i].Size], "trak", "mdia", "minf", "stbl", "stsd")
	if len(stsd) < 8+36 {
		return nil, errors.New("no stsd entry")
	}
	entry := stsd[8:] // version/flags, entry_count
	if string(entry[4:8]) != "mp4a" {
		return nil, errors.New("first sample entry is " + string(entry[4:8]))
	}
	size := binary.BigEndian.Uint32(entry)
	esds := boxPayload(append([]byte{0, 0, 0, 8, 'w', 'r', 'a', 'p'}, entry[8+28:size]...), "esds")
	if len(esds) < 4 {
		return nil, errors.New("no esds")
	}
	r := bytes.NewReader(esds[4:])
	tag, body, err := readDescriptor(r)
	if err != nil || tag != 0x03 {
		return nil, errors.New("no ES_Descriptor")
	}
	if len(body) < 3 {
		return nil, errors.New("short ES_Descriptor")
	}
	flags, off := body[2], 3
	if flags&0x80 != 0 {
		off += 2
	}
	if flags&0x40 != 0 && len(body) > off {
		off += 1 + int(body[off])
	}
	if flags&0x20 != 0 {
		off += 2
	}
	if off > len(body) {
		return nil, errors.New("short ES_Descriptor")
	}
	tag, body, err = readDescriptor(bytes.NewReader(body[off:]))
	if err != nil || tag != 0x04 || len(body) < 13 {
		return nil, errors.New("no DecoderConfigDescriptor")
	}
	tag, body, err = readDescriptor(bytes.NewReader(body[13:]))
	if err != nil || tag != 0x05 {
		return nil, errors.New("no DecoderSpecificInfo (tag 0x05)")
	}
	return body, nil
}

func readDescriptor(r *bytes.Reader) (byte, []byte, error) {
	tag, err := r.ReadByte()
	if err != nil {
		return 0, nil, err
	}
	var n int
	for range 4 {
		b, err := r.ReadByte()
		if err != nil {
			return 0, nil, err
		}
		n = n<<7 | int(b&0x7F)
		if b&0x80 == 0 {
			break
		}
	}
	body := make([]byte, n)
	if _, err := io.ReadFull(r, body); err != nil {
		return 0, nil, err
	}
	return tag, body, nil
}

// An ffmpeg that keeps its output open after its input closed is killed by Close instead of
// holding the relay forever.
func TestRadioPackagerCloseKillsHungFFmpeg(t *testing.T) {
	fake := filepath.Join(t.TempDir(), "ffmpeg")
	if err := os.WriteFile(fake, []byte("#!/bin/sh\nexec sleep 30\n"), 0o700); err != nil {
		t.Fatal(err)
	}
	p, err := Runner{FFmpegPath: fake}.StartRadioPackager(context.Background(), RadioInput{Format: "mp3", Codec: "mp3"},
		RadioOutput{BitrateKbps: 128}, 2*time.Second, slog.New(slog.DiscardHandler))
	if err != nil {
		t.Fatal(err)
	}
	p.closeWait = 200 * time.Millisecond
	start := time.Now()
	if err := p.Close(); err == nil || !strings.Contains(err.Error(), "killed") {
		t.Fatalf("Close = %v, want the kill reported", err)
	}
	if d := time.Since(start); d > 5*time.Second {
		t.Fatalf("Close took %v", d)
	}
	if _, err := p.Next(); err == nil {
		t.Fatal("Next after a killed packager returned a fragment")
	}
}
