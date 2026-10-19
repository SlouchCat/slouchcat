package transcode

import (
	"bufio"
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"slices"
	"strings"
)

// The fMP4 layout written by the segment muxer with -individual_header_trailer 0 (ADR 0003): the
// first file of every ffmpeg run starts with ftyp+moov followed by moof+mdat, all later files
// are pure moof+mdat. init.mp4 is the part before the first moof of the first file ever
// published; a media segment is a file from its first moof.

// box is one top-level ISO BMFF box.
type box struct {
	Type   string
	Offset int64
	Size   int64
}

// walkBoxes lists the top-level boxes of b. It fails on a truncated or malformed box header.
func walkBoxes(b []byte) ([]box, error) {
	var boxes []box
	var off int64
	n := int64(len(b))
	for off < n {
		if n-off < 8 {
			return boxes, fmt.Errorf("truncated box header at %d", off)
		}
		size := int64(binary.BigEndian.Uint32(b[off:]))
		typ := string(b[off+4 : off+8])
		switch size {
		case 0: // extends to the end of the file
			size = n - off
		case 1: // 64-bit size follows the type
			if n-off < 16 {
				return boxes, fmt.Errorf("truncated large box header at %d", off)
			}
			size = int64(binary.BigEndian.Uint64(b[off+8:])) //nolint:gosec // G115: checked against the file size below
		}
		if size < 8 || size > n-off {
			return boxes, fmt.Errorf("box %q at %d has invalid size %d", typ, off, size)
		}
		boxes = append(boxes, box{Type: typ, Offset: off, Size: size})
		off += size
	}
	return boxes, nil
}

// firstMoof returns the offset of the first top-level moof box: the length of the init part.
func firstMoof(b []byte) (int64, error) {
	boxes, err := walkBoxes(b)
	if err != nil {
		return 0, err
	}
	for _, bx := range boxes {
		if bx.Type == "moof" {
			return bx.Offset, nil
		}
	}
	return 0, errors.New("no moof box")
}

// parseSegmentList returns the segment indexes of a flat segment list (one file name per line,
// appended by ffmpeg when a segment is finished). A last line without newline is still being
// written and is ignored.
func parseSegmentList(data []byte) []int {
	if i := bytes.LastIndexByte(data, '\n'); i >= 0 {
		data = data[:i+1]
	} else {
		return nil
	}
	var idx []int
	sc := bufio.NewScanner(bytes.NewReader(data))
	for sc.Scan() {
		var n int
		if _, err := fmt.Sscanf(strings.TrimSpace(sc.Text()), "seg%05d.m4s", &n); err == nil {
			idx = append(idx, n)
		}
	}
	return idx
}

// errIncomplete marks a listed segment whose data is not (completely) on disk yet: ffmpeg 6.1.1's
// segment muxer appends the list entry before it writes the segment data (ADR 0003).
var errIncomplete = errors.New("segment not completely written")

// checkComplete reports whether the segment file at path is structurally whole: its top-level
// boxes end exactly at the file size, the last one is an mdat and it holds a moof; a file that
// starts with ftyp (first file of a run) must continue with moov. Only box headers are read. A
// missing, empty or truncated file returns an error wrapping errIncomplete; a file that cannot
// become valid by growing returns another error.
func checkComplete(path string) error {
	f, err := os.Open(path) //nolint:gosec // G304: path built by the session from its own work dir
	if errors.Is(err, fs.ErrNotExist) {
		return fmt.Errorf("%w: %s missing", errIncomplete, path)
	}
	if err != nil {
		return err
	}
	defer f.Close()
	fi, err := f.Stat()
	if err != nil {
		return err
	}
	size := fi.Size()
	var types []string
	hdr := make([]byte, 16)
	for off := int64(0); off < size; {
		if size-off < 8 {
			return fmt.Errorf("%w: %s: partial box header at %d", errIncomplete, path, off)
		}
		n := min(size-off, 16)
		if _, err := f.ReadAt(hdr[:n], off); err != nil {
			return fmt.Errorf("%w: %s: %w", errIncomplete, path, err)
		}
		bsize := int64(binary.BigEndian.Uint32(hdr))
		typ := string(hdr[4:8])
		switch bsize {
		case 0: // "to the end of the file": the muxer has not patched the size yet
			return fmt.Errorf("%w: %s: box %q without size", errIncomplete, path, typ)
		case 1:
			if n < 16 {
				return fmt.Errorf("%w: %s: partial large box header at %d", errIncomplete, path, off)
			}
			bsize = int64(binary.BigEndian.Uint64(hdr[8:])) //nolint:gosec // G115: compared with the file size below
		}
		if bsize < 8 {
			return fmt.Errorf("segment %s: box %q at %d has invalid size %d", path, typ, off, bsize)
		}
		if bsize > size-off {
			return fmt.Errorf("%w: %s: box %q at %d needs %d bytes, %d on disk", errIncomplete, path, typ, off, bsize, size-off)
		}
		types = append(types, typ)
		off += bsize
	}
	switch {
	case len(types) == 0:
		return fmt.Errorf("%w: %s is empty", errIncomplete, path)
	case types[0] == "ftyp" && (len(types) < 2 || types[1] != "moov"):
		if len(types) < 2 {
			return fmt.Errorf("%w: %s: no moov yet", errIncomplete, path)
		}
		return fmt.Errorf("segment %s: ftyp followed by %q, want moov", path, types[1])
	case types[0] != "ftyp" && types[0] != "moof":
		return fmt.Errorf("segment %s starts with %q, want moof or ftyp", path, types[0])
	case !slices.Contains(types, "moof") || types[len(types)-1] != "mdat":
		return fmt.Errorf("%w: %s: boxes %v do not end with moof+mdat yet", errIncomplete, path, types)
	}
	return nil
}

// publishSegment moves a finished segment from the run directory to dst; the caller checked it
// with checkComplete. When the file starts with ftyp+moov (first file of a run) the header is cut
// off, and written to initPath if needInit.
func publishSegment(src, dst, initPath string, needInit bool) (wroteInit bool, err error) {
	f, err := os.Open(src) //nolint:gosec // G304: path built by the session from its own work dir
	if err != nil {
		return false, err
	}
	head := make([]byte, 8)
	_, err = io.ReadFull(f, head)
	_ = f.Close()
	if err != nil {
		return false, fmt.Errorf("read %s: %w", src, err)
	}
	if string(head[4:8]) == "moof" {
		return false, os.Rename(src, dst)
	}
	data, err := os.ReadFile(src) //nolint:gosec // G304: see above
	if err != nil {
		return false, err
	}
	off, err := firstMoof(data)
	if err != nil {
		return false, fmt.Errorf("segment %s: %w", src, err)
	}
	if needInit {
		if err := writeFileAtomic(initPath, data[:off]); err != nil {
			return false, err
		}
	}
	if err := writeFileAtomic(dst, data[off:]); err != nil {
		return needInit, err
	}
	return needInit, os.Remove(src)
}

// writeFileAtomic writes data to a temporary file and renames it over path, so a reader never
// sees a partial file.
func writeFileAtomic(path string, data []byte) error {
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o600); err != nil { //nolint:gosec // G703: session work dir and a fixed file name
		return err
	}
	return os.Rename(tmp, path)
}
