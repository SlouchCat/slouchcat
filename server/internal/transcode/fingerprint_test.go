package transcode

import (
	"context"
	"math/bits"
	"os/exec"
	"path/filepath"
	"slices"
	"strconv"
	"testing"
	"time"
)

// fingerprintStep is the time between two fingerprint values: 1365 samples at 11025 Hz.
const fingerprintStep = time.Second * 1365 / 11025

// writeMelody writes a WAV of seconds length: a new sine tone every second, so the fingerprint
// changes over time and offsets are measurable.
func writeMelody(t *testing.T, r Runner, path string, seconds int) {
	t.Helper()
	expr := `sin(2*PI*t*(300+170*mod(floor(t)*7\,11)))/4` // the comma is escaped for the filter parser
	cmd := exec.CommandContext(context.Background(), r.FFmpegPath, "-nostdin", "-hide_banner", "-loglevel", "error", "-y",
		"-f", "lavfi", "-i", "aevalsrc="+expr+":s=48000:d="+strconv.Itoa(seconds), "-c:a", "pcm_s16le", path)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("generate audio: %v: %s", err, out)
	}
}

func TestFingerprintRateAndOffset(t *testing.T) {
	r := ffmpegRunner(t)
	path := filepath.Join(t.TempDir(), "melody.wav")
	writeMelody(t, r, path, 60)
	ctx := context.Background()

	full, err := r.Fingerprint(ctx, path, 0, 10*time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	// Chromaprint needs about 21 frames of context, so the count stays a little below 60 s / step.
	frames := int(60 * time.Second / fingerprintStep)
	t.Logf("60 s of audio: %d values, %.2f per second (%d frames of 123.8 ms)", len(full), float64(len(full))/60, frames)
	if len(full) < frames-30 || len(full) > frames {
		t.Fatalf("values = %d, want about %d", len(full), frames-21)
	}

	part, err := r.Fingerprint(ctx, path, 20*time.Second, 20*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	if want := int(20*time.Second/fingerprintStep) - 21; len(part) < want-5 || len(part) > want+5 {
		t.Fatalf("values of 20 s = %d, want about %d", len(part), want)
	}
	// The range starts at value 20 s / step of the full fingerprint (±1 for the frame phase).
	at := int(20 * time.Second / fingerprintStep)
	best, bestDist := -1, 1<<30
	for off := at - 3; off <= at+3; off++ {
		d := 0
		for i, v := range part {
			d += bits.OnesCount32(v ^ full[off+i])
		}
		if d < bestDist {
			best, bestDist = off, d
		}
	}
	if best < at-1 || best > at+1 {
		t.Fatalf("range aligns at value %d, want %d±1", best, at)
	}
	if avg := float64(bestDist) / float64(len(part)); avg > 6 {
		t.Errorf("mean Hamming distance at the aligned offset = %.1f bits, want <= 6", avg)
	}
}

func TestFingerprintErrors(t *testing.T) {
	ctx := context.Background()
	if _, err := (Runner{}).Fingerprint(ctx, "/x.mkv", 0, time.Second); err == nil {
		t.Error("Fingerprint ran without an ffmpeg path")
	}
	r := ffmpegRunner(t)
	if _, err := r.Fingerprint(ctx, "/x.mkv", -time.Second, time.Second); err == nil {
		t.Error("negative start accepted")
	}
	if _, err := r.Fingerprint(ctx, filepath.Join(t.TempDir(), "missing.mkv"), 0, time.Second); err == nil {
		t.Error("missing file fingerprinted")
	}
}

func TestFingerprintArgs(t *testing.T) {
	got := FingerprintArgs("/m/a.mkv", 1500*time.Millisecond, 6*time.Minute)
	want := []string{"-nostdin", "-hide_banner", "-loglevel", "error", "-ss", "1.500", "-t", "360.000", "-i", "/m/a.mkv",
		"-map", "0:a:0", "-ac", "1", "-ar", "11025", "-f", "chromaprint", "-fp_format", "raw", "-"}
	if !slices.Equal(got, want) {
		t.Errorf("args = %q", got)
	}
}
