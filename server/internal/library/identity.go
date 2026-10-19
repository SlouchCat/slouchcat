package library

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"io"
	"io/fs"
	"os"
)

// partialChunk is the size of the head and the tail that partial_hash covers.
const partialChunk = 64 << 10

// PartialHash returns the hex SHA-256 over the first 64 KiB, the last 64 KiB and the size
// (little-endian, 8 bytes) of the file rel inside r (plan 3.2). Files of at most 128 KiB are
// hashed whole, followed by the size. It identifies a file that moved to another device, where
// (dev, inode) changes.
func PartialHash(r *os.Root, rel string) (string, error) {
	f, err := r.Open(rel)
	if err != nil {
		return "", err
	}
	defer f.Close()
	fi, err := f.Stat()
	if err != nil {
		return "", err
	}
	size := fi.Size()
	h := sha256.New()
	if size <= 2*partialChunk {
		if _, err := io.Copy(h, f); err != nil {
			return "", fmt.Errorf("hash %s: %w", rel, err)
		}
	} else {
		if _, err := io.CopyN(h, f, partialChunk); err != nil {
			return "", fmt.Errorf("hash head of %s: %w", rel, err)
		}
		if _, err := io.Copy(h, io.NewSectionReader(f, size-partialChunk, partialChunk)); err != nil {
			return "", fmt.Errorf("hash tail of %s: %w", rel, err)
		}
	}
	var sz [8]byte
	binary.LittleEndian.PutUint64(sz[:], uint64(size)) //nolint:gosec // G115: a file size is never negative
	h.Write(sz[:])
	return hex.EncodeToString(h.Sum(nil)), nil
}

// fileID is the device and inode of a file; zero when the platform does not provide them.
type fileID struct {
	dev, inode uint64
}

// fileIdentity reads device and inode from the stat result (see identity_linux.go).
func fileIdentity(fi fs.FileInfo) fileID {
	return sysFileID(fi.Sys())
}

// toInt64 stores an unsigned identity value in an SQLite INTEGER column bit for bit. The value is
// only compared for equality, so the sign does not matter.
func toInt64(v uint64) int64 {
	return int64(v) //nolint:gosec // G115: bit-for-bit storage of dev/inode, compared for equality only
}
