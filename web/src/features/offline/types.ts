import type { components, ItemRef } from '@slouchcat/api-client';

// Offline playback (M5): the contract's types of the `offline` tag and what the device keeps of
// them in IndexedDB (db.ts). The server decides; the device stores its answers and looks them up
// (plan principle 1).

type Schemas = components['schemas'];

export type DownloadQuality = Schemas['DownloadQuality'];
export type DownloadStateValue = Schemas['DownloadStateValue'];
export type DownloadState = Schemas['DownloadState'];
export type DownloadList = Schemas['DownloadList'];
export type DownloadOptions = Schemas['DownloadOptions'];
export type DownloadQualityOption = Schemas['DownloadQualityOption'];
export type DownloadRequest = Schemas['DownloadRequest'];
export type DownloadManifest = Schemas['DownloadManifest'];
export type DownloadSegment = Schemas['DownloadSegment'];
export type DownloadSubtitle = Schemas['DownloadSubtitle'];
export type DownloadRules = Schemas['DownloadRules'];
export type OfflineSync = Schemas['OfflineSync'];
export type OfflineSyncRequest = Schemas['OfflineSyncRequest'];
export type OfflineProfile = Schemas['OfflineProfile'];
export type OfflinePinVerifier = Schemas['OfflinePinVerifier'];
export type OfflineWatchTime = Schemas['OfflineWatchTime'];
export type OfflineEvent = Schemas['OfflineEvent'];
export type OfflineEventBatch = Schemas['OfflineEventBatch'];
export type OfflineEventResult = Schemas['OfflineEventResult'];
export type RevocationList = Schemas['RevocationList'];

/** Iterations of the PBKDF2 verifiers when the server names none (`model.OfflinePinIterations`). */
export const OFFLINE_PIN_ITERATIONS = 600_000;

/**
 * A download as the device keeps it: the server's last state of it, the title data (kept when a
 * later sync omits `item` because another profile is active), the manifest and how far the file
 * is in OPFS (`/downloads/<id>/`).
 */
export interface StoredDownload {
  id: string;
  /** The account the download belongs to; the device may hold downloads of several accounts. */
  accountId: number;
  state: DownloadStateValue;
  quality: DownloadQuality;
  auto: boolean;
  /** Profiles that may see the title, computed by the server (plan 3.3). The only filter offline. */
  allowedProfileIds: number[];
  estimatedBytes: number;
  sizeBytes?: number;
  segmentsDone: number;
  segmentsTotal?: number;
  error?: string;
  createdAt: string;
  readyAt?: string;
  /** Title data from the last answer that carried it. */
  item?: ItemRef;
  /** Fragment index, codecs, subtitles and file URL (with the download token). */
  manifest?: DownloadManifest;
  /** Bytes of media.mp4 that are durably written, from offset 0. */
  bytesWritten: number;
  /** Subtitle track ids stored as `<track>.vtt`. */
  subtitlesStored: number[];
  posterStored: boolean;
  /** File, subtitles and poster are on the device (`complete` of the next sync). */
  complete: boolean;
  /** Error code of the last local fetch attempt (`network`, `evicted`, `gone`, `storage`). */
  fetchError?: string;
  /** Deleted on this device while offline; the engine sends the delete when it is online again. */
  pendingDelete?: boolean;
  /** Last position per profile (offline playback), so the player resumes without the server. */
  positions?: Record<string, { positionMs: number; at: number }>;
  updatedAt: number;
}

/**
 * Watch time of a profile for the offline player (AN-18): the server's remaining minutes at the
 * sync, what the device played since (by its clock) and the period that play counts toward.
 */
export interface StoredWatchTime {
  limitMinutes: number;
  remainingMinutes: number;
  /** Server time (ms) of the next reset; the full limit applies from then on, every 24 h. */
  resetsAt: number;
  /** Device time of the sync that delivered the snapshot. */
  syncedAt: number;
  /** -1: before `resetsAt`, else the number of the 24-h period after it. */
  period: number;
  /** Played milliseconds in `period` that the snapshot does not include yet. */
  playedMs: number;
}

/** PBKDF2-SHA256 verifier the device derived from the normal PIN (salt from the server, hex). */
export interface LocalPinVerifier {
  salt: string;
  iterations: number;
  /** base64url without padding. */
  hash: string;
}

/** Local brake of the offline PIN (30 s after the third failure, doubling up to 15 min). */
export interface PinLock {
  failures: number;
  /** Device time until which no attempt is accepted; 0 without a lock. */
  until: number;
}

/** A profile as the offline library needs it, from the last sync that listed it. */
export interface StoredProfile {
  id: number;
  accountId: number;
  snapshot: OfflineProfile;
  syncedAt: number;
  pinVerifier?: LocalPinVerifier;
  watch?: StoredWatchTime;
  lock: PinLock;
  /** The profile's auto-download rule on this device, when it was active at a sync. */
  rules?: DownloadRules;
}

/** A progress event of offline playback waiting for the upload (O-5). */
export interface QueuedEvent {
  key?: number;
  accountId: number;
  event: OfflineEvent;
}
