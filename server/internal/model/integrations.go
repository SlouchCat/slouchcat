package model

import "time"

// Kinds of webhook integration (I-8, decision 5.6). Slouchcat receives webhooks; it never calls
// Radarr or Sonarr.
const (
	IntegrationRadarr = "radarr"
	IntegrationSonarr = "sonarr"
)

// Results of one received webhook, stored in the event log and shown on the admin page.
const (
	ResultScanQueued     = "scan_queued"     // a scan.path job for the reported directory
	ResultAcknowledged   = "acknowledged"    // the Test event of Radarr/Sonarr
	ResultIgnored        = "ignored"         // an event without an effect (Grab, Health, …)
	ResultOutsideRoots   = "outside_roots"   // the mapped path lies under no library root (422)
	ResultInvalidPayload = "invalid_payload" // the payload of an acting event carries no path (422)
)

// PathMapping translates a prefix of the path Radarr or Sonarr reports into a prefix on this
// server ("/movies" → "/media/Filme"). Both are absolute; the longest matching From wins.
type PathMapping struct {
	From string
	To   string
}

// Integration is one webhook address the administration handed to Radarr or Sonarr. The token
// itself exists only in the answer that created or renewed it (3.8 point 2).
type Integration struct {
	ID        int64
	Name      string
	Kind      string // radarr, sonarr
	TokenTail string // last four characters of the token, for the list
	PathMap   []PathMapping
	LastSeen  time.Time // zero: nothing reported yet
	CreatedAt time.Time

	// Token is the token in clear and the full address; both are set only in the answer of
	// CreateIntegration and RenewToken, never when an integration is read.
	Token string
	URL   string
}

// WebhookPayload is the part of a Radarr or Sonarr body the receiver reads; everything else in
// the body is accepted and ignored, so a new field of a later version breaks nothing.
type WebhookPayload struct {
	Event      string // eventType
	MoviePath  string // movie.folderPath (Radarr)
	SeriesPath string // series.path (Sonarr)
	// Notification is notification_type of a Seerr notification (MEDIA_PENDING, TEST_NOTIFICATION,
	// …); it only names the entry in the log, nothing is read from it (concept 25, F-18).
	Notification string
}

// WebhookReceipt is the outcome of one received webhook.
type WebhookReceipt struct {
	Result string // one of the Result* constants
	Path   string // the directory that is scanned (only with ResultScanQueued)
}

// IntegrationEvent is one received webhook in the log of an integration (the newest 20).
type IntegrationEvent struct {
	ID           int64
	ReceivedAt   time.Time
	Event        string // as Radarr/Sonarr sent it: Download, Rename, Test, …
	ReportedPath string
	ResolvedPath string
	Result       string
}

// WebhookStatus is the "Webhooks" tile of the admin dashboard (A-3, decision 5.6). It keeps the
// integration status filled now that the Arr clients of I-1 are gone.
type WebhookStatus struct {
	Total         int      // configured integrations
	EventsLast24h int      // webhooks received in the last 24 hours
	Silent        []string // names that reported nothing for seven days
}

// The Seerr webhook (concept 25 stage 4, F-18; architecture 10): a notification of Seerr only
// queues a run of the sync wishes.sync; its content is never read for a state.
const (
	IntegrationSeerr = "seerr"       // kind of the integration Seerr calls
	ResultSyncQueued = "sync_queued" // a Seerr notification queued, or joined, a run of the sync
)

// SeerrWebhookSetup is what the administrator enters in Seerr under Settings › Notifications ›
// Webhook (band «Sofortmeldung», administration 4.3): the address of the integration (shown once,
// Integration.URL), the JSON payload and the notification types. Seerr knows one webhook target
// only; this address replaces another one there.
type SeerrWebhookSetup struct {
	JSONPayload string   // the template to paste into «JSON Payload»
	Types       int      // Seerr's bit mask of the notification types to tick
	TypeNames   []string // the same types by Seerr's names, in the order of Seerr's list
}
