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
// After a successful scan the optional image check (WithImageCheck, metadata's
// QueueIncompleteImages) queues image.fetch for matched titles whose stored images miss a size
// (a size added in a later release) or whose image files were deleted from the cache.
//
// # Path scan (job scan.path)
//
// A watcher reports the directories that changed (Poller: new, gone or with a different listing;
// inotify: the parent of every event path, a created directory with everything below it, a
// removed watched directory). triggerPathScan merges them into one job per library with the key
// scan:<id>:path, priority 6 and the debounce window of 2 s; the payload names the directories per
// root, relative and sorted: {"libraryId": 1, "roots": [{"rootId": 1, "dirs": [".", "Up (2009)"]}]}.
// Directories reported while the job is queued are merged into its payload
// (Queue.SetQueuedPayload); while it runs they wait in memory and are queued once it has finished.
// More than MaxChangedDirs directories, or an inotify overflow, queue a full scan instead.
//
// The path scan reads only the named directories, each without its subdirectories (a removed
// directory is a directory of its own in the report), and runs the phases of the full scan on
// them:
//
//  1. Read: check the root (root_missing, and root_empty when the root has no entry at all while
//     files are known), list each directory, stat its video files. A directory that no longer
//     exists is gone: every known file below it is in scope.
//  2. Match: the same cascade as the full scan (path, then dev/inode/size, then partial_hash)
//     against the known files of the whole root, so a file moved in from a folder that was not
//     scanned keeps its id. A known file outside the scanned directories matches only when it is
//     missing already or its old path is gone; otherwise the new file is a copy or a hard link.
//     Known files in scope without a match become missing.
//  3. Write: as in the full scan, with the same NFO handling, item lookup, restore of
//     soft-deleted items and probe.file and metadata.match jobs in the same transactions.
//
// A path scan sets the scan state running and done like the full scan but leaves
// scan_files_seen and scan_files_total at the values of the last full scan (the item and file
// counts of the library are counted live). It skips the image check after the scan: new titles
// get their images through metadata.match, and the full scan every 6 hours runs the check.
// Full and path scans of one library never run at the same time (lockScan); each reloads the
// library once it holds the lock.
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
// A path scan applies the same codes. Its loss rule counts the whole library over the grace period
// of 24 h: it aborts when, with its own losses, more than 20 % of the files known in that period
// are missing, and either the share was at most 20 % before it or its own losses alone exceed
// 20 % of the present files. So neither one large change nor a series of small path scans removes
// more than a fifth of the library without the administrator's consent.
//
// The administrator lifts the last two once with AcceptLoss: the next scan applies the loss and
// clears loss_accepted_at (a path scan clears it only when it needed it). A missing root is never
// accepted. Titles whose files all stayed missing
// for 24 h are soft-deleted by the hourly job library.sweep.
//
// # Change detection
//
// Each root has a watch mode chosen from its file system (statfs): inotify for local file systems,
// an incremental stat pass (Poller) for NFS and SMB with the interval max(45 s, 3 × pass duration),
// and both for FUSE and unknown file systems. Every detected change queues a debounced scan.path
// of the changed directories; Start adds a full scan per library every 6 hours.
package library
