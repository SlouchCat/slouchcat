package transcode

import (
	"bufio"
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"os"
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

// publishSegment moves a finished segment from the run directory to dst. When the file starts
// with ftyp+moov (first file of a run) the header is cut off, and written to initPath if needInit.
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
