package store

import (
	"context"
	"io"
	"log/slog"
	"testing"
)

// The frozen state after concept 25 (migrations 00066 to 00081; testdata/db/wuensche.sql) is
// ordner-metadaten.sql migrated to 00081 with the rows of the wishes: the Seerr connection, Seerr
// users with exactly one shared user and one gone user, the mapping of both accounts, an account
// limit, request rights of accounts, profiles and an invitation, the Seerr log, titles outside the
// library with texts and a scaled image, wishes in every state (shows with seasons, requesters with
// an owner and joined profiles, events), notifications and deliveries of wishes, spotlight entries
// with a TMDB, a TVDB-only and an IMDb id, the series catalog, profile and wish tags, progress, a
// cached discover page, the home row «wishes» and a seerr webhook with an event.
// TestFrozenStatesMigrate replays it like every state with all migrations; this test checks that
// each of those is really in it at its own migration 00081. Later migrations may change these rows
// on purpose: since 00086 (ADR 0045) the request rights follow the ratings, and the frozen account
// and invitation with a right have an age limit.
func TestFrozenWuenscheStateCoversTheConcept(t *testing.T) {
	t.Parallel()
	cfg := t.TempDir()
	replay(t, cfg+"/"+FileName, frozenState(t, "wuensche.sql"))
	db, err := Open(context.Background(), Options{ConfigDir: cfg, Version: "frozen-test", Logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
		migrations: migrationsBefore(t, "00082")})
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	for what, query := range map[string]string{
		"migration 00081 recorded as applied":       `SELECT count(*) FROM goose_db_version WHERE version_id = 81 AND is_applied = 1`,
		"the Seerr connection and wishes on":        `SELECT count(*) FROM settings WHERE key IN ('seerr.enabled', 'wishes.enabled') AND value = '1' HAVING count(*) = 2`,
		"exactly one shared Seerr user":             `SELECT count(*) FROM seerr_users WHERE is_shared = 1 HAVING count(*) = 1`,
		"a gone Seerr user":                         `SELECT count(*) FROM seerr_users WHERE gone_at IS NOT NULL`,
		"a Seerr user with quotas":                  `SELECT count(*) FROM seerr_users WHERE movie_quota_limit > 0 AND tv_quota_limit > 0 AND is_shared = 0`,
		"a linked and a created mapping":            `SELECT count(*) FROM wish_account_map WHERE origin IN ('linked', 'created') AND state = 'ready' HAVING count(DISTINCT origin) = 2`,
		"an account limit":                          `SELECT count(*) FROM wish_account_limits WHERE mode = 'custom'`,
		"request rights of an account":              `SELECT count(*) FROM accounts WHERE can_request = 1 AND is_owner = 0`,
		"request rights of a profile":               `SELECT count(*) FROM profiles WHERE can_request = 1 AND is_default = 0`,
		"an invitation with a wish limit":           `SELECT count(*) FROM invitations WHERE can_request = 1 AND wish_limit_mode = 'custom'`,
		"the Seerr log":                             `SELECT count(*) FROM seerr_log`,
		"a title outside the library with seasons":  `SELECT count(*) FROM external_titles WHERE media = 'show' AND json_array_length(seasons) > 0 AND used_at IS NOT NULL`,
		"texts of an outside title":                 `SELECT count(*) FROM external_title_texts`,
		"a scaled image of an outside title":        `SELECT count(*) FROM external_title_images`,
		"wishes in eight states":                    `SELECT count(DISTINCT state) = 8 FROM wishes`,
		"a wish from Seerr":                         `SELECT count(*) FROM wishes WHERE origin = 'seerr'`,
		"an available wish on a library title":      `SELECT count(*) FROM wishes WHERE state = 'available' AND item_id IN (SELECT id FROM items)`,
		"a failed wish with its reason":             `SELECT count(*) FROM wishes WHERE state = 'failed' AND fail_reason IS NOT NULL`,
		"a partial show with seasons in two states": `SELECT count(*) FROM (SELECT wish_id FROM wish_seasons WHERE wish_id IN (SELECT id FROM wishes WHERE state = 'partial') GROUP BY wish_id HAVING count(DISTINCT state) = 2)`,
		"an owner and a joined requester":           `SELECT count(*) FROM (SELECT wish_id FROM wish_requesters GROUP BY wish_id HAVING count(DISTINCT role) = 2)`,
		"wish events":                               `SELECT count(*) FROM wish_events`,
		"notifications of wishes":                   `SELECT count(DISTINCT kind) = 5 FROM notifications WHERE wish_id IS NOT NULL`,
		"a delivery of a wish":                      `SELECT count(*) FROM deliveries WHERE event IN ('wish_available', 'wish_review')`,
		"a spotlight entry with a TVDB id only":     `SELECT count(*) FROM spotlight_entries WHERE tmdb_id IS NULL AND tvdb_id IS NOT NULL`,
		"a spotlight entry with an IMDb id":         `SELECT count(*) FROM spotlight_entries WHERE imdb_id IS NOT NULL`,
		"the series catalog of a library show":      `SELECT count(*) FROM series_catalog WHERE item_id IN (SELECT id FROM items WHERE kind = 'series')`,
		"a custom profile tag":                      `SELECT count(*) FROM wish_profile_tags WHERE custom = 1`,
		"wish tags in three states":                 `SELECT count(DISTINCT state) = 3 FROM wish_tags`,
		"progress of Radarr and Sonarr":             `SELECT count(DISTINCT service) = 2 FROM wish_progress`,
		"a cached discover page":                    `SELECT count(*) FROM wish_discover_pages WHERE json_array_length(items) > 0`,
		"the home row wishes":                       `SELECT count(*) FROM home_rows WHERE key = 'wishes'`,
		"a seerr webhook with an event":             `SELECT count(*) FROM webhook_events WHERE integration_id IN (SELECT id FROM webhook_integrations WHERE kind = 'seerr')`,
		"foreign keys of the frozen state hold":     `SELECT count(*) = 0 FROM pragma_foreign_key_check`,
	} {
		var n int
		if err := db.Read.QueryRow(query).Scan(&n); err != nil {
			t.Errorf("%s: %v", what, err)
			continue
		}
		if n == 0 {
			t.Errorf("frozen wuensche state has no %s", what)
		}
	}
	for table, want := range map[string]int{
		"seerr_users": 4, "wish_account_map": 2, "external_titles": 4, "wishes": 8, "wish_seasons": 3,
		"wish_requesters": 10, "wish_events": 12, "wish_tags": 3, "wish_progress": 2,
	} {
		var n int
		if err := db.Read.QueryRow(`SELECT count(*) FROM ` + table).Scan(&n); err != nil {
			t.Fatal(err)
		}
		if n != want {
			t.Errorf("%s holds %d rows, want %d", table, n, want)
		}
	}
}
