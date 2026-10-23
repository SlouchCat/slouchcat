package api

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"fmt"
	"mime"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/slouchcat/slouchcat/server/internal/api/gen"
	"github.com/slouchcat/slouchcat/server/internal/visibility/vistest"
)

// M8 backups (A-4) over the API: create, list, download, restore from the upload and delete. The
// encrypted variants and the restore itself are covered in package admin; here the scrypt work of
// age would only slow the race detector down.
func TestContractBackupFlow(t *testing.T) {
	t.Parallel()
	pf := newProfileFixture(t)
	h := pf.h

	for body, code := range map[string]string{
		`{"includeSecrets":true}`:                            "passphrase_required",
		`{"includeSecrets":true,"passphrase":"elf-zeichen"}`: "passphrase_too_short",
	} {
		rec := send(t, h, http.MethodPost, BasePath+"/admin/backups", "p18", body, nil)
		if rec.Code != http.StatusUnprocessableEntity || problemCode(t, rec) != code {
			t.Errorf("%s: status %d (%s), want 422 %s", body, rec.Code, rec.Body.String(), code)
		}
	}

	rec := send(t, h, http.MethodGet, BasePath+"/admin/backups", "p18", "", nil)
	if rec.Code != http.StatusOK || strings.TrimSpace(rec.Body.String()) != "[]" {
		t.Fatalf("empty list: %d %s, want 200 []", rec.Code, rec.Body.String())
	}
	rec = send(t, h, http.MethodPost, BasePath+"/admin/backups", "p18", `{"includeSecrets":false}`, nil)
	if rec.Code != http.StatusCreated {
		t.Fatalf("create: %d %s", rec.Code, rec.Body.String())
	}
	var created gen.Backup
	decode(t, rec, &created)
	if !strings.HasPrefix(created.Name, "slouchcat-backup-test-") || created.Kind != gen.BackupKindManual || created.IncludesSecrets ||
		created.Encrypted || created.SizeBytes <= 0 || created.Version == nil || *created.Version != "test" {
		t.Fatalf("created %+v", created)
	}
	rec = send(t, h, http.MethodGet, BasePath+"/admin/backups", "p18", "", nil)
	var list []gen.Backup
	decode(t, rec, &list)
	if len(list) != 1 || list[0].Name != created.Name {
		t.Fatalf("list %+v", list)
	}

	rec = send(t, h, http.MethodGet, BasePath+"/admin/backups/"+created.Name+"/download", "p18", "", nil)
	if rec.Code != http.StatusOK || rec.Header().Get("Content-Type") != "application/octet-stream" ||
		int64(rec.Body.Len()) != created.SizeBytes || !bytes.HasPrefix(rec.Body.Bytes(), []byte{0x1f, 0x8b}) {
		t.Fatalf("download: %d, %v, %d bytes", rec.Code, rec.Header(), rec.Body.Len())
	}
	if _, params, err := mime.ParseMediaType(rec.Header().Get("Content-Disposition")); err != nil || params["filename"] != created.Name {
		t.Errorf("Content-Disposition %q (%v), want an attachment named %s", rec.Header().Get("Content-Disposition"), err, created.Name)
	}
	archive := rec.Body.Bytes()
	for _, path := range []string{"/admin/backups/missing.tar.gz/download", "/admin/backups/..%2Fslouchcat.db/download", "/admin/backups/slouchcat.db/download"} {
		if rec := send(t, h, http.MethodGet, BasePath+path, "p18", "", nil); rec.Code != http.StatusNotFound {
			t.Errorf("%s: status %d, want 404", path, rec.Code)
		}
	}

	// The upload: the passphrase part (ignored for an unencrypted archive) before the file part.
	upload := func(file []byte) *httptest.ResponseRecorder {
		var buf bytes.Buffer
		mw := multipart.NewWriter(&buf)
		_ = mw.WriteField("passphrase", "FAKE-not-needed-passphrase")
		fw, _ := mw.CreateFormFile("file", "backup.tar.gz")
		_, _ = fw.Write(file)
		_ = mw.Close()
		return send(t, h, http.MethodPost, BasePath+"/admin/backups/restore", "p18", buf.String(),
			map[string]string{"Content-Type": mw.FormDataContentType()})
	}
	rec = upload(archive[:len(archive)/2])
	if rec.Code != http.StatusUnprocessableEntity || problemCode(t, rec) != "backup_invalid" {
		t.Errorf("truncated upload: %d %s, want 422 backup_invalid", rec.Code, rec.Body.String())
	}
	rec = upload(archive)
	if rec.Code != http.StatusAccepted {
		t.Fatalf("restore upload: %d %s", rec.Code, rec.Body.String())
	}
	var res gen.RestoreResult
	decode(t, rec, &res)
	if res.Version != "test" || res.IncludesSecrets || time.Until(res.RestartAt) <= 0 {
		t.Errorf("restore result %+v", res)
	}
	rec = send(t, h, http.MethodPost, BasePath+"/admin/backups/"+created.Name+"/restore", "p18", `{}`, nil)
	if rec.Code != http.StatusAccepted {
		t.Errorf("restore from the list: %d %s", rec.Code, rec.Body.String())
	}

	if rec := send(t, h, http.MethodDelete, BasePath+"/admin/backups/"+created.Name, "p18", "", nil); rec.Code != http.StatusNoContent {
		t.Fatalf("delete: %d %s", rec.Code, rec.Body.String())
	}
	if rec := send(t, h, http.MethodDelete, BasePath+"/admin/backups/"+created.Name, "p18", "", nil); rec.Code != http.StatusNotFound {
		t.Errorf("delete again: %d, want 404", rec.Code)
	}
}

// M8 statistics (A-8): the owner gets every list as an array (never null) and plain StatTitles; a
// period other than 7, 30, 90 or 365 days is refused.
func TestContractStatsShape(t *testing.T) {
	t.Parallel()
	pf := newProfileFixture(t)
	rec := send(t, pf.h, http.MethodGet, BasePath+"/admin/stats", "p18", "", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d %s", rec.Code, rec.Body.String())
	}
	var raw map[string]json.RawMessage
	decode(t, rec, &raw)
	for _, key := range []string{"topTitles", "watchTime", "devices"} {
		if !bytes.HasPrefix(raw[key], []byte("[")) {
			t.Errorf("%s = %s, want an array", key, raw[key])
		}
	}
	var st gen.Stats
	decode(t, rec, &st)
	if st.Days != 30 {
		t.Errorf("days = %d, want the default 30", st.Days)
	}
	for _, path := range []string{"/admin/stats?days=8", "/admin/stats?days=0"} {
		rec := send(t, pf.h, http.MethodGet, BasePath+path, "p18", "", nil)
		if rec.Code != http.StatusUnprocessableEntity || problemCode(t, rec) != "validation_failed" {
			t.Errorf("%s: %d %s, want 422 validation_failed", path, rec.Code, rec.Body.String())
		}
	}
	if rec := send(t, pf.h, http.MethodGet, BasePath+"/admin/stats?days=week", "p18", "", nil); rec.Code != http.StatusBadRequest {
		t.Errorf("days=week: %d, want 400", rec.Code)
	}
}

// A-8: a progress event of a browser belongs to the device of its session (the device cookie), so
// the statistics count that device.
func TestContractProgressCountsTheDevice(t *testing.T) {
	t.Parallel()
	pf := newProfileFixture(t)
	m := pf.f.Rated(6)
	body := fmt.Sprintf(`{"mediaFileId":%q,"positionMs":10000,"durationMs":%d,"playedMs":10000,"seq":1,"state":"playing","clientTime":%q}`,
		m.FileID, vistest.DurationMs, time.Now().UTC().Format(time.RFC3339))
	if rec := send(t, pf.h, http.MethodPost, BasePath+"/items/"+m.ID+"/progress", "p18", body, nil); rec.Code != http.StatusOK {
		t.Fatalf("progress: %d %s", rec.Code, rec.Body.String())
	}
	var device sql.NullString
	if err := pf.f.DB.Read.QueryRow(`SELECT device_id FROM progress_events WHERE item_id = ?`, m.ID).Scan(&device); err != nil {
		t.Fatal(err)
	}
	if want := pf.sessions["p18"].DeviceID; want == "" || device.String != want {
		t.Errorf("device of the event = %+v, want the session's %q", device, want)
	}
	rec := send(t, pf.h, http.MethodGet, BasePath+"/admin/stats?days=7", "p18", "", nil)
	var st gen.Stats
	decode(t, rec, &st)
	devices := 0
	for _, d := range st.Devices {
		devices += d.Count
	}
	if rec.Code != http.StatusOK || devices != 1 {
		t.Errorf("stats: %d, devices %+v, want one device", rec.Code, st.Devices)
	}
}
