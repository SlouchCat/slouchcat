// Package library manages the media libraries of Slouchcat: their roots, the scanner that turns
// files into titles, the probe jobs, the watchers that notice changes and the sweep of titles whose
// files are gone (plan sections 3.2, 3.4 and 3.6, milestone M1a).
//
// # Scan (job scan.library)
//
// A full scan of a library runs in two phases, so that nothing is written when it aborts:
//
//  1. Read: for every root, open it through package paths (os.Root), walk it and collect the video
//     files (extras, samples, hidden entries and NAS metadata folders are skipped), then match them
//     against the known media files of the root: the same rel_path is the same file (size and
//     mtime are updated, a changed file is probed again); otherwise an equal (dev, inode, size)
//     means renamed or moved on the same device; otherwise an equal partial_hash (SHA-256 over the
//     first and last 64 KiB and the size) means moved across devices; everything else is new.
//     Known files without a match are missing.
//  2. Write: in transactions of at most 500 rows on the writer connection, with the context
//     checked between batches. A new file joins the undeleted item of the library with the same
//     TMDB id, else IMDb id, else (sort_title, year), or creates a movie item and queues
//     metadata.match; every new or changed file queues probe.file. Jobs are inserted in the same
//     transaction as the rows they refer to. Renames keep the media file id and therefore the item
//     and all user data (progress hangs on the item). Missing files get missing_since; a file that
//     reappears clears it and restores a soft-deleted item.
//
// A second scan of an unchanged tree finds every file by its path and writes nothing except the
// scan state, so scans converge and can run again after an interrupted job.
//
// # Loss protection
//
// Before any write the scan aborts with state "aborted" and an error code when
//
//   - root_missing: a root cannot be opened or is not a directory (detached disk or share),
//   - root_empty: the walk of a root finds no video file while the database knows files of it
//     (typical for an unmounted mount point),
//   - loss_over_threshold: more than 20 % of the known, not yet missing files of the library
//     would become missing in this run.
//
// The administrator lifts the last two once with AcceptLoss: the next scan applies the loss and
// clears loss_accepted_at. A missing root is never accepted. Titles whose files all stayed missing
// for 24 h are soft-deleted by the hourly job library.sweep.
//
// # Change detection
//
// Each root has a watch mode chosen from its file system (statfs): inotify for local file systems,
// an incremental stat pass (Poller) for NFS and SMB with the interval max(45 s, 3 × pass duration),
// and both for FUSE and unknown file systems. Every detected change queues a debounced
// scan.library; Start adds a full scan per library every 6 hours.
package library
