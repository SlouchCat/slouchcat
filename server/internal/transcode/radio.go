package transcode

import (
	"bufio"
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os/exec"
	"slices"
	"strconv"
	"sync"
	"time"
)

// Live radio packaging (concept 16 section 7.1, ADR 0025): ffmpeg reads the clean audio of a
// station from pipe:0 and writes one fragmented MP4 stream to pipe:1; Go cuts that stream at the
// box level into the init part (ftyp+moov) and the media fragments (moof+mdat). ffmpeg gets no
// path and no URL, and only the pipe protocol is allowed.

// RadioInput describes the audio a relay hands the packager (from sniffing the stream).
type RadioInput struct {
	Format      string // "mp3", "aac" (ADTS), "ogg", "flac", "mpegts", "mp4"
	Codec       string // "mp3", "aac", "he-aac", "vorbis", "opus", "flac"
	BitrateKbps int    // source bitrate, 0 = unknown
	Channels    int    // 0 = unknown
	SampleRate  int    // 0 = unknown
}

// RadioOutput is what the packager writes: Copy repackages the AAC of the source, else ffmpeg
// encodes AAC at BitrateKbps. For Copy, BitrateKbps is the source bitrate (0 = unknown).
type RadioOutput struct {
	Copy        bool
	BitrateKbps int
}

// RadioFragment is one part of the packaged stream: the init part (Init, ftyp+moov) or one media
// fragment (moof+mdat) of Duration.
type RadioFragment struct {
	Init     bool
	Data     []byte
	Duration time.Duration
}

const (
	// radioMaxBox caps one box of ffmpeg's output; a fragment of a few seconds of audio is far
	// smaller.
	radioMaxBox = 8 << 20
	// radioFragmentQueue is how many fragments wait for Next before the reader blocks.
	radioFragmentQueue = 16
	// radioCloseWait is how long Close waits for ffmpeg to flush and exit after its input closed;
	// then it kills ffmpeg.
	radioCloseWait = 10 * time.Second
)

// RadioOutputFor decides between copy and transcode: AAC and HE-AAC are repackaged as they are,
// everything else is encoded as AAC at the source bitrate clamped to 96..192 kbit/s (128 when
// unknown).
func RadioOutputFor(in RadioInput) RadioOutput {
	if in.Codec == "aac" || in.Codec == "he-aac" {
		return RadioOutput{Copy: true, BitrateKbps: in.BitrateKbps}
	}
	if in.BitrateKbps <= 0 {
		return RadioOutput{BitrateKbps: 128}
	}
	return RadioOutput{BitrateKbps: min(max(in.BitrateKbps, 96), 192)}
}

// RadioPackagerArgs returns the ffmpeg arguments of the packager: input pipe:0 in in.Format,
// output fragmented MP4 with fragments of the given duration on pipe:1. Copy mode needs
// +delay_moov: with +empty_moov alone the moov is written before aac_adtstoasc produced the
// AudioSpecificConfig, and MSE refuses an esds without it (plan B-1).
func RadioPackagerArgs(in RadioInput, out RadioOutput, fragment time.Duration) []string {
	args := []string{
		"-hide_banner", "-nostats", "-loglevel", "error",
		"-probesize", "32768", "-analyzeduration", "1000000",
		"-protocol_whitelist", "pipe",
		"-f", in.Format, "-i", "pipe:0",
		"-map", "0:a:0", "-vn", "-sn", "-dn",
	}
	movflags := "+empty_moov+default_base_moof+skip_trailer"
	if out.Copy {
		args = append(args, "-c:a", "copy")
		if in.Format == "aac" || in.Format == "mpegts" {
			args = append(args, "-bsf:a", "aac_adtstoasc")
		}
		movflags = "+empty_moov+delay_moov+default_base_moof+skip_trailer"
	} else {
		channels := 2
		if in.Channels > 0 {
			channels = min(in.Channels, 2)
		}
		args = append(args, "-c:a", "aac", "-b:a", strconv.Itoa(out.BitrateKbps)+"k", "-ac", strconv.Itoa(channels))
		if !slices.Contains([]int{32000, 44100, 48000}, in.SampleRate) {
			args = append(args, "-ar", "48000")
		}
	}
	return append(args, "-f", "mp4", "-movflags", movflags,
		"-frag_duration", strconv.FormatInt(fragment.Microseconds(), 10), "-flush_packets", "1", "pipe:1")
}

// RadioPackager is one running ffmpeg packager. Write feeds audio and Next returns the packaged
// fragments; the two must run on different goroutines (pipes in both directions). The caller
// keeps calling Next until it returns an error, or cancels the context of StartRadioPackager.
type RadioPackager struct {
	ctx    context.Context
	cmd    *exec.Cmd
	stdin  io.WriteCloser
	stdout io.ReadCloser
	stderr *tailWriter
	frags  chan RadioFragment
	done   chan struct{} // closed when the reader goroutine has read stdout to its end
	err    error         // reader error, written before frags closes

	closeWait time.Duration // radioCloseWait; tests shorten it
	closeOnce sync.Once
	closeErr  error
}

// StartRadioPackager starts ffmpeg with RadioPackagerArgs. Canceling ctx stops ffmpeg.
func (r Runner) StartRadioPackager(ctx context.Context, in RadioInput, out RadioOutput, fragment time.Duration,
	log *slog.Logger) (*RadioPackager, error) {
	if r.FFmpegPath == "" {
		return nil, errors.New("ffmpeg path is not configured")
	}
	stderr := &tailWriter{}
	cmd, stdin, stdout, err := startFFmpegPiped(ctx, r.FFmpegPath, RadioPackagerArgs(in, out, fragment), stderr, log)
	if err != nil {
		return nil, err
	}
	p := &RadioPackager{ctx: ctx, cmd: cmd, stdin: stdin, stdout: stdout, stderr: stderr,
		frags: make(chan RadioFragment, radioFragmentQueue), done: make(chan struct{}), closeWait: radioCloseWait}
	go p.read()
	return p, nil
}

// Write feeds audio bytes to ffmpeg.
func (p *RadioPackager) Write(b []byte) (int, error) {
	n, err := p.stdin.Write(b)
	if err != nil {
		return n, fmt.Errorf("write to ffmpeg: %w", err)
	}
	return n, nil
}

// Next blocks until the next fragment; the first one is the init part. It returns io.EOF once
// ffmpeg ended after Close, another error for a malformed output.
func (p *RadioPackager) Next() (RadioFragment, error) {
	if f, ok := <-p.frags; ok {
		return f, nil
	}
	if p.err != nil {
		return RadioFragment{}, p.err
	}
	return RadioFragment{}, io.EOF
}

// Close closes ffmpeg's input, so it flushes its last fragment and exits, waits until its output
// is read and returns its exit error with the stderr tail. An ffmpeg that has not ended its output
// closeWait after its input closed is killed.
func (p *RadioPackager) Close() error {
	p.closeOnce.Do(func() {
		_ = p.stdin.Close()
		t := time.NewTimer(p.closeWait)
		defer t.Stop()
		killed := false
		select {
		case <-p.done:
		case <-t.C:
			killed = true
			_ = p.cmd.Process.Kill()
			<-p.done
		}
		err := p.cmd.Wait()
		switch {
		case killed:
			p.closeErr = fmt.Errorf("ffmpeg packager killed %v after its input closed: %s", p.closeWait, p.stderr.String())
		case err != nil && p.ctx.Err() == nil:
			p.closeErr = fmt.Errorf("ffmpeg packager: %w: %s", err, p.stderr.String())
		}
	})
	return p.closeErr
}

// read cuts ffmpeg's output into fragments until EOF. On a malformed box it records the error
// and discards the rest of the output, so ffmpeg never blocks on a full pipe.
func (p *RadioPackager) read() {
	defer close(p.done)
	br := bufio.NewReaderSize(p.stdout, 64<<10)
	err := p.cut(br)
	if errors.Is(err, io.EOF) {
		err = nil
	}
	if err != nil {
		p.err = fmt.Errorf("ffmpeg packager output: %w", err)
		_, _ = io.Copy(io.Discard, br)
	}
	close(p.frags)
}

func (p *RadioPackager) cut(br *bufio.Reader) error {
	var head, moof []byte // ftyp (and anything) before moov; the pending moof
	var timescale, trexDuration uint32
	for {
		typ, data, err := readRadioBox(br)
		if err != nil {
			return err
		}
		var f RadioFragment
		switch {
		case typ == "moov":
			timescale, trexDuration = radioTrackDefaults(data)
			f = RadioFragment{Init: true, Data: append(head, data...)}
			head = nil
		case timescale == 0: // before the moov
			head = append(head, data...)
			continue
		case typ == "moof":
			moof = data
			continue
		case typ == "mdat" && moof != nil:
			units := radioFragmentUnits(moof, trexDuration)
			f = RadioFragment{Data: append(moof, data...),
				Duration: time.Duration(units * uint64(time.Second) / uint64(timescale))} //nolint:gosec // G115: sample units of seconds of audio
			moof = nil
		default: // free, sidx, prft and the like carry nothing a player needs
			continue
		}
		select {
		case p.frags <- f:
		case <-p.ctx.Done():
			return p.ctx.Err()
		}
	}
}

// readRadioBox reads one top-level box (header included). A box without size (size 0, "to the
// end") cannot be cut from a stream and is an error, as is one above radioMaxBox.
func readRadioBox(br *bufio.Reader) (string, []byte, error) {
	var buf [16]byte
	hdr := buf[:8]
	if _, err := io.ReadFull(br, hdr); err != nil {
		if errors.Is(err, io.ErrUnexpectedEOF) {
			return "", nil, fmt.Errorf("truncated box header: %w", err)
		}
		return "", nil, err
	}
	size := uint64(binary.BigEndian.Uint32(hdr))
	typ := string(hdr[4:8])
	switch size {
	case 0:
		return "", nil, fmt.Errorf("box %q without size", typ)
	case 1:
		hdr = buf[:]
		if _, err := io.ReadFull(br, hdr[8:]); err != nil {
			return "", nil, fmt.Errorf("truncated large box header: %w", err)
		}
		size = binary.BigEndian.Uint64(hdr[8:])
	}
	if size < uint64(len(hdr)) || size > radioMaxBox {
		return "", nil, fmt.Errorf("box %q has invalid size %d", typ, size)
	}
	data := make([]byte, size)
	copy(data, hdr)
	if _, err := io.ReadFull(br, data[len(hdr):]); err != nil {
		return "", nil, fmt.Errorf("truncated box %q: %w", typ, err)
	}
	return typ, data, nil
}

// boxPayload returns the payload of the first child box of b (a whole box, header included) on
// the path of types; nil when one is missing. b's own header is skipped first.
func boxPayload(b []byte, path ...string) []byte {
	cur := boxBody(b)
	for _, typ := range path {
		boxes, _ := walkBoxes(cur)
		i := slices.IndexFunc(boxes, func(bx box) bool { return bx.Type == typ })
		if i < 0 {
			return nil
		}
		cur = boxBody(cur[boxes[i].Offset : boxes[i].Offset+boxes[i].Size])
	}
	return cur
}

// boxBody strips the header of the whole box b.
func boxBody(b []byte) []byte {
	if len(b) >= 16 && binary.BigEndian.Uint32(b) == 1 {
		return b[16:]
	}
	if len(b) < 8 {
		return nil
	}
	return b[8:]
}

// radioTrackDefaults reads the mdhd timescale of the first track and the trex default sample
// duration from a moov box.
func radioTrackDefaults(moov []byte) (timescale, defaultDuration uint32) {
	if mdhd := boxPayload(moov, "trak", "mdia", "mdhd"); len(mdhd) >= 4 {
		off := 12 // version 0: flags, creation, modification
		if mdhd[0] == 1 {
			off = 20
		}
		if len(mdhd) >= off+4 {
			timescale = binary.BigEndian.Uint32(mdhd[off:])
		}
	}
	if trex := boxPayload(moov, "mvex", "trex"); len(trex) >= 16 {
		defaultDuration = binary.BigEndian.Uint32(trex[12:])
	}
	if timescale == 0 {
		timescale = 1 // a broken moov: durations come out absurd, never a division by zero
	}
	return timescale, defaultDuration
}

// radioFragmentUnits sums the sample durations of a moof in timescale units: per-sample durations
// of the trun, else sample count times the tfhd or trex default.
func radioFragmentUnits(moof []byte, trexDuration uint32) uint64 {
	traf := boxPayload(moof, "traf")
	boxes, _ := walkBoxes(traf)
	def := trexDuration
	var units uint64
	for _, bx := range boxes {
		b := boxBody(traf[bx.Offset : bx.Offset+bx.Size])
		if len(b) < 8 {
			continue
		}
		flags := binary.BigEndian.Uint32(b) & 0xFFFFFF
		switch bx.Type {
		case "tfhd":
			off := 8 // version/flags, track_ID
			if flags&0x01 != 0 {
				off += 8
			}
			if flags&0x02 != 0 {
				off += 4
			}
			if flags&0x08 != 0 && len(b) >= off+4 {
				def = binary.BigEndian.Uint32(b[off:])
			}
		case "trun":
			count := binary.BigEndian.Uint32(b[4:])
			if flags&0x100 == 0 {
				units += uint64(count) * uint64(def)
				continue
			}
			off := 8
			if flags&0x01 != 0 {
				off += 4
			}
			if flags&0x04 != 0 {
				off += 4
			}
			stride := 4
			for _, f := range []uint32{0x200, 0x400, 0x800} {
				if flags&f != 0 {
					stride += 4
				}
			}
			for i := uint32(0); i < count && len(b) >= off+4; i++ {
				units += uint64(binary.BigEndian.Uint32(b[off:]))
				off += stride
			}
		}
	}
	return units
}

// CheckRadioAudio decodes the file at path completely and fails on the first decoding error
// (ffmpeg -xerror): the proof that a packaged stream is clean (concept 16, plan M-1).
func (r Runner) CheckRadioAudio(ctx context.Context, path string) error {
	cmd := exec.CommandContext(ctx, r.FFmpegPath, "-nostdin", "-hide_banner", "-loglevel", "error", "-xerror", //nolint:gosec // G204: configured binary, fixed argument list
		"-i", path, "-f", "null", "-")
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("decode %s: %w: %s", path, err, tail(out, stderrTail))
	}
	return nil
}
