package transcode

import (
	"bytes"
	"context"
	"image"
	"image/color"
	"image/jpeg"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"
)

// ffmpegRunner returns a Runner with the ffmpeg from PATH, or skips the test without it.
func ffmpegRunner(t *testing.T) Runner {
	t.Helper()
	bin, err := exec.LookPath("ffmpeg")
	if err != nil {
		t.Skipf("ffmpeg not installed: %v", err)
	}
	return Runner{FFmpegPath: bin}
}

// writeJPEG writes a w×h JPEG whose left half is fill and right half grey.
func writeJPEG(t *testing.T, path string, w, h int, fill color.NRGBA) {
	t.Helper()
	img := image.NewNRGBA(image.Rect(0, 0, w, h))
	for x := range w {
		for y := range h {
			c := fill
			if x >= w/2 {
				c = color.NRGBA{R: 128, G: 128, B: 128, A: 255}
			}
			img.SetNRGBA(x, y, c)
		}
	}
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, img, &jpeg.Options{Quality: 95}); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, buf.Bytes(), 0o600); err != nil {
		t.Fatal(err)
	}
}

func TestScaleImageArgs(t *testing.T) {
	got, err := ScaleImageArgs("/c/in.jpg", []ImageOutput{
		{Path: "/c/a", Width: 200, Format: ImageWebP},
		{Path: "/c/b", Width: 200, Format: ImageJPEG},
		{Path: "/c/c", Width: 400, Format: ImageWebP},
	})
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"-nostdin", "-hide_banner", "-loglevel", "error", "-y", "-i", "/c/in.jpg", "-filter_complex",
		"[0:v]split=2[s0][s1];[s0]scale=200:-2:flags=lanczos,split=2[o0][o1];[s1]scale=400:-2:flags=lanczos,split=1[o2]",
		"-map", "[o0]", "-frames:v", "1", "-c:v", "libwebp", "-quality", "82", "-compression_level", "4", "-f", "webp", "/c/a",
		"-map", "[o1]", "-frames:v", "1", "-c:v", "mjpeg", "-q:v", "3", "-pix_fmt", "yuvj420p", "-f", "mjpeg", "/c/b",
		"-map", "[o2]", "-frames:v", "1", "-c:v", "libwebp", "-quality", "82", "-compression_level", "4", "-f", "webp", "/c/c",
	}
	if !slices.Equal(got, want) {
		t.Fatalf("args\n got %q\nwant %q", got, want)
	}
	for _, bad := range [][]ImageOutput{nil, {{Path: "/x", Width: 0, Format: ImageJPEG}}, {{Path: "/x", Width: 10, Format: "gif"}}, {{Width: 10, Format: ImageJPEG}}} {
		if _, err := ScaleImageArgs("/in", bad); err == nil {
			t.Errorf("ScaleImageArgs(%+v) accepted", bad)
		}
	}
}

func TestScaleImageAndPalette(t *testing.T) {
	r := ffmpegRunner(t)
	dir := t.TempDir()
	in := filepath.Join(dir, "original.jpg")
	writeJPEG(t, in, 300, 450, color.NRGBA{R: 200, G: 40, B: 40, A: 255})
	outs := []ImageOutput{
		{Path: filepath.Join(dir, ".tmp-1"), Width: 200, Format: ImageWebP},
		{Path: filepath.Join(dir, ".tmp-2"), Width: 200, Format: ImageJPEG},
		{Path: filepath.Join(dir, ".tmp-3"), Width: 120, Format: ImageJPEG},
	}
	if err := r.ScaleImage(context.Background(), in, outs); err != nil {
		t.Fatal(err)
	}
	for _, o := range outs {
		data, err := os.ReadFile(o.Path)
		if err != nil {
			t.Fatal(err)
		}
		switch o.Format {
		case ImageWebP:
			if len(data) < 12 || string(data[:4]) != "RIFF" || string(data[8:12]) != "WEBP" {
				t.Fatalf("%s is not WebP", o.Path)
			}
		case ImageJPEG:
			cfg, err := jpeg.DecodeConfig(bytes.NewReader(data))
			if err != nil {
				t.Fatalf("%s: %v", o.Path, err)
			}
			if wantH := o.Width * 3 / 2; cfg.Width != o.Width || cfg.Height != wantH {
				t.Fatalf("%s is %dx%d, want %dx%d", o.Path, cfg.Width, cfg.Height, o.Width, wantH)
			}
		}
	}

	colors, err := r.Palette(context.Background(), outs[1].Path, 8)
	if err != nil {
		t.Fatal(err)
	}
	if len(colors) == 0 || len(colors) > 8 {
		t.Fatalf("palette %v, want 1..8 colours", colors)
	}
	accent, ok := AccentColor(colors)
	if !ok {
		t.Fatalf("no accent from %v", colors)
	}
	// The red half wins over the grey half; JPEG and the median cut shift the exact value.
	c := parseHex(t, accent)
	if c.R < 150 || c.G > 90 || c.B > 90 {
		t.Fatalf("accent %s from %v, want the red", accent, colors)
	}
}

func TestScaleImageErrors(t *testing.T) {
	r := ffmpegRunner(t)
	dir := t.TempDir()
	bad := filepath.Join(dir, "broken.jpg")
	if err := os.WriteFile(bad, []byte("not an image"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := r.ScaleImage(context.Background(), bad, []ImageOutput{{Path: filepath.Join(dir, "o"), Width: 10, Format: ImageJPEG}}); err == nil {
		t.Fatal("ScaleImage accepted a broken input")
	}
	if _, err := r.Palette(context.Background(), bad, 8); err == nil {
		t.Fatal("Palette accepted a broken input")
	}
	if _, err := r.Palette(context.Background(), bad, 1); err == nil {
		t.Fatal("Palette accepted one colour")
	}
	if err := (Runner{}).ScaleImage(context.Background(), bad, []ImageOutput{{Path: "o", Width: 10, Format: ImageJPEG}}); err == nil {
		t.Fatal("ScaleImage ran without an ffmpeg path")
	}
}

func parseHex(t *testing.T, s string) color.NRGBA {
	t.Helper()
	var c color.NRGBA
	if len(s) != 7 || s[0] != '#' {
		t.Fatalf("accent %q is not #rrggbb", s)
	}
	for i, p := range []*uint8{&c.R, &c.G, &c.B} {
		v := 0
		for _, ch := range s[1+2*i : 3+2*i] {
			v *= 16
			switch {
			case ch >= '0' && ch <= '9':
				v += int(ch - '0')
			case ch >= 'a' && ch <= 'f':
				v += int(ch-'a') + 10
			default:
				t.Fatalf("accent %q is not lower-case hex", s)
			}
		}
		*p = uint8(v)
	}
	c.A = 255
	return c
}

func TestAccentColor(t *testing.T) {
	rgb := func(r, g, b uint8) color.NRGBA { return color.NRGBA{R: r, G: g, B: b, A: 255} }
	tests := []struct {
		name   string
		colors []color.NRGBA
		want   string
		ok     bool
	}{
		{"empty", nil, "", false},
		{"only transparent", []color.NRGBA{{R: 255}}, "", false},
		// A saturated dark red (l 0.2) loses against a medium blue: too dark for the tint.
		{"medium lightness wins", []color.NRGBA{rgb(102, 0, 0), rgb(40, 80, 160), rgb(128, 128, 128)}, "#2850a0", true},
		{"highest saturation in range", []color.NRGBA{rgb(120, 100, 90), rgb(200, 30, 30), rgb(60, 140, 60)}, "#c81e1e", true},
		// Pure white and black are out of range; pale pink (l 0.9) too: fallback = most saturated.
		{"fallback most saturated", []color.NRGBA{rgb(0, 0, 0), rgb(255, 230, 230), rgb(255, 255, 255)}, "#ffe6e6", true},
		{"grey only", []color.NRGBA{rgb(100, 100, 100)}, "#646464", true},
		{"tie keeps first", []color.NRGBA{rgb(200, 0, 0), rgb(0, 0, 200)}, "#c80000", true},
		{"transparent ignored", []color.NRGBA{{R: 255, A: 0}, rgb(90, 90, 120)}, "#5a5a78", true},
		// #7f0000 has lightness 0.249 (just too dark), #800000 0.251 (in range).
		{"just below the range", []color.NRGBA{rgb(0x7f, 0, 0), rgb(100, 60, 60)}, "#643c3c", true},
		{"just inside the range", []color.NRGBA{rgb(0x80, 0, 0), rgb(100, 60, 60)}, "#800000", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := AccentColor(tt.colors)
			if got != tt.want || ok != tt.ok {
				t.Fatalf("AccentColor(%v) = %q, %v; want %q, %v", tt.colors, got, ok, tt.want, tt.ok)
			}
		})
	}
}

func TestPaletteColors(t *testing.T) {
	img := image.NewNRGBA(image.Rect(0, 0, 4, 4))
	seq := []color.NRGBA{{R: 1, A: 255}, {G: 2, A: 255}, {B: 3, A: 255}}
	for i := range 16 {
		c := seq[min(i, len(seq)-1)] // palettegen repeats the last colour
		img.SetNRGBA(i%4, i/4, c)
	}
	if got := paletteColors(img, 8); !slices.Equal(got, seq) {
		t.Fatalf("paletteColors = %v, want %v", got, seq)
	}
	if got := paletteColors(img, 2); !slices.Equal(got, seq[:2]) {
		t.Fatalf("paletteColors(2) = %v", got)
	}
}

// BenchmarkImagePipelineTitle measures the image work of one title as image.fetch does it: the
// poster original (2000x3000, TMDB's usual original size) into w200/w400/w800 and the backdrop
// original (3840x2160) into w780/w1280/w1920, each as WebP and JPEG in one ffmpeg run per kind,
// plus the palette of the w200 poster. The originals are synthetic test pictures with grain, so
// the encoders see photo-like entropy; SLOUCHCAT_BENCH_POSTER and SLOUCHCAT_BENCH_BACKDROP name real
// originals instead (docs/perf/M1b.md, section 5).
func BenchmarkImagePipelineTitle(b *testing.B) {
	bin, err := exec.LookPath("ffmpeg")
	if err != nil {
		b.Skipf("ffmpeg not installed: %v", err)
	}
	r := Runner{FFmpegPath: bin}
	dir := b.TempDir()
	ctx := context.Background()
	originals := map[string]string{"poster": "2000x3000", "backdrop": "3840x2160"}
	for kind, size := range originals {
		if src := os.Getenv("SLOUCHCAT_BENCH_" + strings.ToUpper(kind)); src != "" {
			data, err := os.ReadFile(src)
			if err != nil {
				b.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(dir, kind+".jpg"), data, 0o600); err != nil {
				b.Fatal(err)
			}
			continue
		}
		cmd := exec.CommandContext(ctx, bin, "-nostdin", "-loglevel", "error", "-f", "lavfi", "-i", "testsrc2=s="+size+":d=1",
			"-vf", "noise=alls=24:allf=t", "-frames:v", "1", "-q:v", "2", filepath.Join(dir, kind+".jpg"))
		if out, err := cmd.CombinedOutput(); err != nil {
			b.Fatalf("generate %s original: %v: %s", kind, err, out)
		}
	}
	widths := map[string][]int{"poster": {200, 400, 800}, "backdrop": {780, 1280, 1920}}
	b.ResetTimer()
	for b.Loop() {
		for _, kind := range []string{"poster", "backdrop"} {
			var outs []ImageOutput
			for _, w := range widths[kind] {
				for _, f := range []ImageFormat{ImageJPEG, ImageWebP} {
					outs = append(outs, ImageOutput{Path: filepath.Join(dir, kind+"-"+strconv.Itoa(w)+"."+string(f)), Width: w, Format: f})
				}
			}
			if err := r.ScaleImage(ctx, filepath.Join(dir, kind+".jpg"), outs); err != nil {
				b.Fatal(err)
			}
		}
		colors, err := r.Palette(ctx, filepath.Join(dir, "poster-200.jpeg"), 8)
		if err != nil {
			b.Fatal(err)
		}
		if _, ok := AccentColor(colors); !ok {
			b.Fatal("no accent")
		}
	}
}
