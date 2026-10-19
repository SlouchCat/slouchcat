package transcode

import (
	"encoding/binary"
	"os"
	"path/filepath"
	"slices"
	"testing"
)

// mkBox builds a box with the given payload.
func mkBox(typ string, payload ...byte) []byte {
	b := binary.BigEndian.AppendUint32(nil, uint32(8+len(payload)))
	b = append(b, typ...)
	return append(b, payload...)
}

func TestWalkBoxes(t *testing.T) {
	file := slices.Concat(mkBox("ftyp", 1, 2, 3, 4), mkBox("moov", 5), mkBox("moof", 6, 7), mkBox("mdat", 8, 9, 10))
	boxes, err := walkBoxes(file)
	if err != nil {
		t.Fatal(err)
	}
	var types []string
	for _, b := range boxes {
		types = append(types, b.Type)
	}
	if !slices.Equal(types, []string{"ftyp", "moov", "moof", "mdat"}) {
		t.Fatalf("types %v", types)
	}
	if off, err := firstMoof(file); err != nil || off != 21 {
		t.Fatalf("firstMoof = %d, %v; want 21", off, err)
	}

	// 64-bit size and size 0 (to the end of the file).
	large := binary.BigEndian.AppendUint32(nil, 1)
	large = append(large, "mdat"...)
	large = binary.BigEndian.AppendUint64(large, 20)
	large = append(large, 1, 2, 3, 4)
	toEnd := append(binary.BigEndian.AppendUint32(nil, 0), "mdat"...)
	toEnd = append(toEnd, 1, 2, 3)
	boxes, err = walkBoxes(slices.Concat(mkBox("moof"), large, toEnd))
	if err != nil || len(boxes) != 3 || boxes[1].Size != 20 || boxes[2].Size != 11 {
		t.Fatalf("boxes %+v, %v", boxes, err)
	}

	for name, bad := range map[string][]byte{
		"truncated header": {0, 0, 0},
		"size too large":   append(binary.BigEndian.AppendUint32(nil, 100), "mdat"...),
		"size too small":   append(binary.BigEndian.AppendUint32(nil, 4), "mdat"...),
	} {
		if _, err := walkBoxes(bad); err == nil {
			t.Errorf("%s: want error", name)
		}
	}
	if _, err := firstMoof(mkBox("ftyp")); err == nil {
		t.Error("want error without moof")
	}
}

func TestParseSegmentList(t *testing.T) {
	got := parseSegmentList([]byte("seg00003.m4s\nseg00004.m4s\njunk\nseg00005.m4"))
	if !slices.Equal(got, []int{3, 4}) {
		t.Errorf("parseSegmentList = %v, want [3 4] (the unfinished last line is ignored)", got)
	}
	if got := parseSegmentList([]byte("seg00001.m4s")); got != nil {
		t.Errorf("no complete line: %v", got)
	}
}

func TestPublishSegment(t *testing.T) {
	dir := t.TempDir()
	header := slices.Concat(mkBox("ftyp", 1), mkBox("moov", 2))
	media := slices.Concat(mkBox("moof", 3), mkBox("mdat", 4, 5))
	first := filepath.Join(dir, "run", "seg00000.m4s")
	if err := os.MkdirAll(filepath.Dir(first), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(first, slices.Concat(header, media), 0o600); err != nil {
		t.Fatal(err)
	}
	initPath := filepath.Join(dir, "init.mp4")
	wrote, err := publishSegment(first, filepath.Join(dir, "seg00000.m4s"), initPath, true)
	if err != nil || !wrote {
		t.Fatalf("publish first: %v, %v", wrote, err)
	}
	assertFile(t, initPath, header)
	assertFile(t, filepath.Join(dir, "seg00000.m4s"), media)
	if _, err := os.Stat(first); !os.IsNotExist(err) {
		t.Errorf("run file still there: %v", err)
	}

	// A later file is moved as is; the header of a later run's first file is dropped.
	later := filepath.Join(dir, "run", "seg00001.m4s")
	if err := os.WriteFile(later, media, 0o600); err != nil {
		t.Fatal(err)
	}
	if wrote, err := publishSegment(later, filepath.Join(dir, "seg00001.m4s"), initPath, false); err != nil || wrote {
		t.Fatalf("publish later: %v, %v", wrote, err)
	}
	assertFile(t, filepath.Join(dir, "seg00001.m4s"), media)
	restart := filepath.Join(dir, "run", "seg00007.m4s")
	if err := os.WriteFile(restart, slices.Concat(header, media), 0o600); err != nil {
		t.Fatal(err)
	}
	if wrote, err := publishSegment(restart, filepath.Join(dir, "seg00007.m4s"), initPath, false); err != nil || wrote {
		t.Fatalf("publish restart: %v, %v", wrote, err)
	}
	assertFile(t, filepath.Join(dir, "seg00007.m4s"), media)
	assertFile(t, initPath, header)
}

func assertFile(t *testing.T, path string, want []byte) {
	t.Helper()
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(got, want) {
		t.Errorf("%s = %v, want %v", filepath.Base(path), got, want)
	}
}
