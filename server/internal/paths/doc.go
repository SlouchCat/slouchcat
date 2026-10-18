// Package paths is the only place that turns user-controlled path data into file system paths
// (plan section 3.8, point 6; CLAUDE.md rule 4).
//
// Contract:
//
//   - Every path that comes from users, clients, the database or media metadata is relative to a
//     registered Root (a library root, the cache directory). Code outside this package never joins
//     such a path itself.
//   - Resolve rejects absolute relative paths, empty paths, NUL bytes and every ".." segment,
//     before any cleaning, so "a/../b" is rejected as well.
//   - Symlinks are followed only inside the root. File access therefore goes through os.Root
//     (Go 1.24+), which refuses symlinks and ".." that leave the root; Resolve is the lexical half
//     of that rule for code that only needs the path string, for example for logging or ffmpeg
//     arguments.
//
// The package also detects the file system type of a directory (DetectFS). The server refuses a
// config directory on a network share, and the library scanner (M1a) picks its change detection
// strategy from the same classification (plan section 3.4).
package paths
