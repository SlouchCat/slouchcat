package api

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/slouchcat/slouchcat/server/internal/model"
	"github.com/slouchcat/slouchcat/server/internal/playback"
	"github.com/slouchcat/slouchcat/server/internal/store/appq"
	"github.com/slouchcat/slouchcat/server/internal/visibility/vistest"
)

// Contract tests of M2 (plan 10.2 and the M2 design, section 5) for the operations that are not
// scoped: the registries below cover every `account`, `admin` and `public` operation of
// api/openapi.yaml, like profileRegistry covers the `scoped` ones, and TestContractCoversEveryOperation
// fails for an operation without an entry. The remaining tests change the data under a running
// fixture: rating overrides, guest revocation, library rights and profile languages.

// contractOp is one operation of the contract.
type contractOp struct{ contract, vis string }

// contractOperations returns operationId → operation for every operation of api/openapi.yaml.
func contractOperations(t *testing.T) map[string]contractOp {
	t.Helper()
	data, err := os.ReadFile("../../../api/openapi.yaml")
	if err != nil {
		t.Fatal(err)
	}
	doc := parseYAML(t, data)
	paths, _ := doc["paths"].(map[string]any)
	out := map[string]contractOp{}
	for path, item := range paths {
		ops, _ := item.(map[string]any)
		for _, method := range httpMethods {
			op, ok := ops[method].(map[string]any)
			if !ok {
				continue
			}
			id, _ := op["operationId"].(string)
			vis, _ := op["x-visibility"].(string)
			if id == "" {
				t.Errorf("%s %s: operation without operationId", method, path)
				continue
			}
			out[id] = contractOp{contract: strings.ToUpper(method) + " " + path, vis: vis}
		}
	}
	if len(out) == 0 {
		t.Fatal("no operations in api/openapi.yaml")
	}
	return out
}

// actor is a kind of caller of the account and admin registries.
type actor struct {
	name    string
	account func(f *vistest.Fixture) model.Account
	profile func(f *vistest.Fixture) model.Profile // active profile; zero = none chosen yet
	choose  func(f *vistest.Fixture) model.Profile // profile that selectProfile picks
}

var (
	actorPrimary = actor{name: "primary",
		account: func(f *vistest.Fixture) model.Account { return f.Account },
		profile: func(f *vistest.Fixture) model.Profile { return f.P18 },
		choose:  func(f *vistest.Fixture) model.Profile { return f.P12 }}
	actorNone = actor{name: "none",
		account: func(f *vistest.Fixture) model.Account { return f.Account },
		profile: func(*vistest.Fixture) model.Profile { return model.Profile{} },
		choose:  func(f *vistest.Fixture) model.Profile { return f.P12 }}
	actorGuest = actor{name: "guest",
		account: func(f *vistest.Fixture) model.Account { return f.Guest },
		profile: func(f *vistest.Fixture) model.Profile { return f.PG },
		choose:  func(f *vistest.Fixture) model.Profile { return f.PG18 }}
)

// accountCall is one call of the account registry; want maps an actor to the expected status.
// {profile} in path is the profile that selectProfile picks, or (fresh) a new profile of the
// actor's account that the call may change or delete.
type accountCall struct {
	contract, path, body string
	fresh                bool
	want                 map[string]int
}

func allWant(status int) map[string]int {
	return map[string]int{"primary": status, "none": status, "guest": status}
}

// accountRegistry covers every operation with x-visibility account. A session without a chosen
// profile may call all of them (never 403 profile_required); profile management then answers 403
// primary_profile_required, which is a different rule.
var accountRegistry = map[string]accountCall{
	"listAccountLibraries": {contract: "GET /libraries", path: "/libraries", want: allWant(http.StatusOK)},
	"getSession":           {contract: "GET /auth/session", path: "/auth/session", want: allWant(http.StatusOK)},
	"logout":               {contract: "POST /auth/logout", path: "/auth/logout", want: allWant(http.StatusNoContent)},
	"listMyProfiles":       {contract: "GET /auth/profiles", path: "/auth/profiles", want: allWant(http.StatusOK)},
	"selectProfile":        {contract: "POST /auth/profiles/{id}/select", path: "/auth/profiles/{profile}/select", body: `{}`, want: allWant(http.StatusOK)},
	"listProfiles": {contract: "GET /profiles", path: "/profiles",
		want: map[string]int{"primary": http.StatusOK, "none": http.StatusForbidden, "guest": http.StatusOK}},
	"createProfile": {contract: "POST /profiles", path: "/profiles", body: `{"name":"Registry"}`,
		want: map[string]int{"primary": http.StatusCreated, "none": http.StatusForbidden, "guest": http.StatusCreated}},
	"updateProfile": {contract: "PATCH /profiles/{id}", path: "/profiles/{profile}", body: `{"name":"Renamed","maxRating":6}`, fresh: true,
		want: map[string]int{"primary": http.StatusOK, "none": http.StatusForbidden, "guest": http.StatusOK}},
	"deleteProfile": {contract: "DELETE /profiles/{id}", path: "/profiles/{profile}", fresh: true,
		want: map[string]int{"primary": http.StatusNoContent, "none": http.StatusForbidden, "guest": http.StatusNoContent}},
	"setProfilePin": {contract: "PUT /profiles/{id}/pin", path: "/profiles/{profile}/pin", body: `{"pin":"4711"}`, fresh: true,
		want: map[string]int{"primary": http.StatusNoContent, "none": http.StatusForbidden, "guest": http.StatusNoContent}},
	"removeProfilePin": {contract: "DELETE /profiles/{id}/pin", path: "/profiles/{profile}/pin", fresh: true,
		want: map[string]int{"primary": http.StatusNoContent, "none": http.StatusForbidden, "guest": http.StatusNoContent}},
	"getWatchReport": {contract: "GET /profiles/{id}/watch-time", path: "/profiles/{profile}/watch-time?days=7",
		want: map[string]int{"primary": http.StatusOK, "none": http.StatusForbidden, "guest": http.StatusOK}},
	// M5: the offline PIN (AN-19) like the PIN, and the upload of offline events (O-5), which any
	// session of an account may send; an event of a profile outside the account is rejected (200).
	"setOfflinePin": {contract: "PUT /profiles/{id}/offline-pin", path: "/profiles/{profile}/offline-pin", body: `{"pin":"135790"}`, fresh: true,
		want: map[string]int{"primary": http.StatusNoContent, "none": http.StatusForbidden, "guest": http.StatusNoContent}},
	"removeOfflinePin": {contract: "DELETE /profiles/{id}/offline-pin", path: "/profiles/{profile}/offline-pin", fresh: true,
		want: map[string]int{"primary": http.StatusNoContent, "none": http.StatusForbidden, "guest": http.StatusNoContent}},
	"uploadOfflineEvents": {contract: "POST /offline/events", path: "/offline/events", want: allWant(http.StatusOK),
		body: `{"sentAt":"2026-10-20T18:00:00Z","events":[{"profileId":999999,"itemId":"` + fixtureFilm6 + `","playbackId":"01JC0000000000000000000009",` +
			`"positionMs":10000,"durationMs":7200000,"playedMs":10000,"seq":1,"state":"playing","clientTime":"2026-10-20T17:59:50Z"}]}`},
	// M8: the own account and the notification channels (TestContractChannels checks their rules).
	// A wrong current password is refused for every session of an account, with or without a
	// profile. The fixture's server has an e-mail server and a VAPID pair but no Telegram bot.
	"updateAccount": {contract: "PUT /auth/account", path: "/auth/account", body: `{"currentPassword":"wrong-password","email":"renamed@example.ch"}`,
		want: allWant(http.StatusUnauthorized)},
	"getMyChannels": {contract: "GET /me/channels", path: "/me/channels", want: allWant(http.StatusOK)},
	"updateMyChannel": {contract: "PUT /me/channels/{channel}", path: "/me/channels/email", body: `{"enabled":false,"events":["announcements"]}`,
		want: allWant(http.StatusOK)},
	"testMyChannel": {contract: "POST /me/channels/{channel}/test", path: "/me/channels/telegram/test",
		want: allWant(http.StatusUnprocessableEntity)},
	"createTelegramLink": {contract: "POST /me/channels/telegram/link", path: "/me/channels/telegram/link",
		want: allWant(http.StatusUnprocessableEntity)},
	"deleteTelegramLink": {contract: "DELETE /me/channels/telegram/link", path: "/me/channels/telegram/link", want: allWant(http.StatusNoContent)},
	"getPushPublicKey":   {contract: "GET /push/vapid-key", path: "/push/vapid-key", want: allWant(http.StatusOK)},
	"createPushSubscription": {contract: "POST /me/push-subscriptions", path: "/me/push-subscriptions",
		body: pushSubscriptionBody("https://push.example.test/send/abc"), want: allWant(http.StatusCreated)},
	"deletePushSubscription": {contract: "DELETE /me/push-subscriptions/{id}", path: "/me/push-subscriptions/9999", want: allWant(http.StatusNotFound)},
}

// adminCall is one call of the admin registry; ownerStatus is the answer for the owner with the
// primary profile (harmless bodies against unknown ids or invalid input). header replaces request
// headers, such as the content type of a multipart body.
type adminCall struct {
	contract, path, body string
	header               map[string]string
	ownerStatus          int
}

// restoreUploadBody is a multipart body of restoreBackupUpload whose file is no backup.
const restoreUploadBody = "--registry\r\nContent-Disposition: form-data; name=\"file\"; filename=\"backup.tar.gz\"\r\n" +
	"Content-Type: application/octet-stream\r\n\r\nnot a backup\r\n--registry--\r\n"

const unknownItemID = "01J0000000000000000000XXXX"

var adminRegistry = map[string]adminCall{
	"listLibraries": {contract: "GET /admin/libraries", path: "/admin/libraries", ownerStatus: http.StatusOK},
	// M9 webhook integrations (I-8): the fixture has none, so every id is unknown.
	"listIntegrations": {contract: "GET /admin/integrations", path: "/admin/integrations", ownerStatus: http.StatusOK},
	"createIntegration": {contract: "POST /admin/integrations", path: "/admin/integrations",
		body: `{"name":"","kind":"radarr"}`, ownerStatus: http.StatusUnprocessableEntity},
	"deleteIntegration":     {contract: "DELETE /admin/integrations/{id}", path: "/admin/integrations/9999", ownerStatus: http.StatusNotFound},
	"renewIntegrationToken": {contract: "POST /admin/integrations/{id}/token", path: "/admin/integrations/9999/token", ownerStatus: http.StatusNotFound},
	"setIntegrationPathMap": {contract: "PUT /admin/integrations/{id}/path-map", path: "/admin/integrations/9999/path-map",
		body: `{"pathMap":[]}`, ownerStatus: http.StatusNotFound},
	"listIntegrationEvents": {contract: "GET /admin/integrations/{id}/events", path: "/admin/integrations/9999/events", ownerStatus: http.StatusNotFound},
	"createLibrary":         {contract: "POST /admin/libraries", path: "/admin/libraries", body: `{"name":"","kind":"movies","path":""}`, ownerStatus: http.StatusUnprocessableEntity},
	// Idea 02, the folder browser: the fixture has no browsable root, so the list is empty and
	// every id is unknown -- the same 404 a path outside the roots gets.
	"listBrowseRoots": {contract: "GET /admin/fs/roots", path: "/admin/fs/roots", ownerStatus: http.StatusOK},
	"browseServerPaths": {contract: "POST /admin/fs/browse", path: "/admin/fs/browse",
		body: `{"rootId":"does-not-exist"}`, ownerStatus: http.StatusNotFound},
	"getLibrary":        {contract: "GET /admin/libraries/{id}", path: "/admin/libraries/777", ownerStatus: http.StatusNotFound},
	"scanLibrary":       {contract: "POST /admin/libraries/{id}/scan", path: "/admin/libraries/777/scan", ownerStatus: http.StatusNotFound},
	"acceptLibraryLoss": {contract: "POST /admin/libraries/{id}/accept-loss", path: "/admin/libraries/777/accept-loss", ownerStatus: http.StatusNotFound},
	// Idea 02, E8 and E12: the fixture has no library 777, so both answer 404 for the owner.
	"addLibraryRoot": {contract: "POST /admin/libraries/{id}/roots", path: "/admin/libraries/777/roots",
		body: `{"path":"/media/does-not-exist"}`, ownerStatus: http.StatusNotFound},
	"removeLibraryRoot": {contract: "POST /admin/libraries/{id}/roots/{rootId}/removal", path: "/admin/libraries/777/roots/888/removal",
		body: `{"confirmPath":"/media/does-not-exist"}`, ownerStatus: http.StatusNotFound},
	"listInvitations":  {contract: "GET /admin/invitations", path: "/admin/invitations", ownerStatus: http.StatusOK},
	"createInvitation": {contract: "POST /admin/invitations", path: "/admin/invitations", body: `{"maxRating":6,"allLibraries":true}`, ownerStatus: http.StatusCreated},
	"revokeInvitation": {contract: "DELETE /admin/invitations/{id}", path: "/admin/invitations/9999", ownerStatus: http.StatusNotFound},
	"listGuests":       {contract: "GET /admin/guests", path: "/admin/guests", ownerStatus: http.StatusOK},
	"updateGuest":      {contract: "PATCH /admin/guests/{id}", path: "/admin/guests/9999", body: `{"maxRating":6}`, ownerStatus: http.StatusNotFound},
	"revokeGuest":      {contract: "DELETE /admin/guests/{id}", path: "/admin/guests/9999", ownerStatus: http.StatusNotFound},
	"setItemRating":    {contract: "PUT /admin/items/{id}/rating", path: "/admin/items/" + unknownItemID + "/rating", body: `{"rating":12}`, ownerStatus: http.StatusNotFound},
	// M3: requests against fixture titles that leave the fixture unchanged (the fake metadata and
	// library services of the contract fixture record nothing).
	"cancelLibraryScan": {contract: "DELETE /admin/libraries/{id}/scan", path: "/admin/libraries/777/scan", ownerStatus: http.StatusNotFound},
	"getAdminItem":      {contract: "GET /admin/items/{id}", path: "/admin/items/" + fixtureFilm6, ownerStatus: http.StatusOK},
	"setItemMatch": {contract: "PUT /admin/items/{id}/match", path: "/admin/items/" + fixtureFilm6 + "/match",
		body: `{"provider":"tmdb","providerId":"603"}`, ownerStatus: http.StatusAccepted},
	"refreshItem": {contract: "POST /admin/items/{id}/refresh", path: "/admin/items/" + fixtureFilm6 + "/refresh", ownerStatus: http.StatusAccepted},
	"createItemTrailer": {contract: "POST /admin/items/{id}/trailers", path: "/admin/items/" + fixtureFilm6 + "/trailers",
		body: `{"lang":"de","url":"https://youtu.be/dQw4w9WgXcQ"}`, ownerStatus: http.StatusCreated},
	"deleteItemTrailer": {contract: "DELETE /admin/items/{id}/trailers/{trailerId}", path: "/admin/items/" + fixtureFilm6 + "/trailers/999", ownerStatus: http.StatusNotFound},
	"setItemsRating": {contract: "POST /admin/items/rating", path: "/admin/items/rating",
		body: `{"itemIds":["` + unknownItemID + `"],"rating":12}`, ownerStatus: http.StatusOK},
	"searchMetadata": {contract: "GET /admin/metadata/search", path: "/admin/metadata/search?provider=tvdb&kind=series&query=Show",
		ownerStatus: http.StatusUnprocessableEntity},
	"listMaintenanceIssues": {contract: "GET /admin/maintenance/issues", path: "/admin/maintenance/issues?kind=unrated", ownerStatus: http.StatusOK},
	"listDuplicates":        {contract: "GET /admin/maintenance/duplicates", path: "/admin/maintenance/duplicates", ownerStatus: http.StatusOK},
	"mergeItems": {contract: "POST /admin/maintenance/merge", path: "/admin/maintenance/merge",
		body: `{"targetId":"` + fixtureFilm6 + `","sourceIds":["` + fixtureSeries + `"]}`, ownerStatus: http.StatusUnprocessableEntity},
	"cleanupItems": {contract: "POST /admin/maintenance/cleanup", path: "/admin/maintenance/cleanup",
		body: `{"itemIds":["` + fixtureFilm6 + `"]}`, ownerStatus: http.StatusOK},
	// M4: device and stream overviews (W-6, W-7).
	"listDevices":  {contract: "GET /admin/devices", path: "/admin/devices", ownerStatus: http.StatusOK},
	"deleteDevice": {contract: "DELETE /admin/devices/{id}", path: "/admin/devices/zzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzz", ownerStatus: http.StatusNotFound},
	"listStreams":  {contract: "GET /admin/streams", path: "/admin/streams", ownerStatus: http.StatusOK},
	"stopStream":   {contract: "DELETE /admin/streams/{id}", path: "/admin/streams/01JZZZZZZZZZZZZZZZZZZZZZZZ", ownerStatus: http.StatusNotFound},
	// M6: library settings and podcast subscriptions; requests that leave the fixture unchanged
	// (TestContractAudioAdmin changes it).
	"updateLibrary":  {contract: "PATCH /admin/libraries/{id}", path: "/admin/libraries/777", body: `{"name":"Neu"}`, ownerStatus: http.StatusNotFound},
	"listPodcasts":   {contract: "GET /admin/podcasts", path: "/admin/podcasts", ownerStatus: http.StatusOK},
	"createPodcast":  {contract: "POST /admin/podcasts", path: "/admin/podcasts", body: `{"libraryId":777,"feedUrl":"http://127.0.0.1:9/feed.xml"}`, ownerStatus: http.StatusUnprocessableEntity},
	"deletePodcast":  {contract: "DELETE /admin/podcasts/{id}", path: "/admin/podcasts/" + unknownItemID, ownerStatus: http.StatusNotFound},
	"refreshPodcast": {contract: "POST /admin/podcasts/{id}/refresh", path: "/admin/podcasts/" + unknownItemID + "/refresh", ownerStatus: http.StatusNotFound},
	// M7 curation (stream B1; the Trakt entries belong to stream B2). Unknown ids answer 404,
	// refused input 422.
	"listTopPicks":       {contract: "GET /admin/picks", path: "/admin/picks", ownerStatus: http.StatusOK},
	"createTopPicks":     {contract: "POST /admin/picks", path: "/admin/picks", body: `{"itemId":"` + unknownItemID + `"}`, ownerStatus: http.StatusUnprocessableEntity},
	"reorderTopPicks":    {contract: "PUT /admin/picks/order", path: "/admin/picks/order", body: `{"ids":[]}`, ownerStatus: http.StatusUnprocessableEntity},
	"updateTopPick":      {contract: "PATCH /admin/picks/{id}", path: "/admin/picks/9999", body: `{"note":"x"}`, ownerStatus: http.StatusNotFound},
	"deleteTopPick":      {contract: "DELETE /admin/picks/{id}", path: "/admin/picks/9999", ownerStatus: http.StatusNotFound},
	"searchAdminItems":   {contract: "GET /admin/items/search", path: "/admin/items/search?q=Fi", ownerStatus: http.StatusOK},
	"listCollections":    {contract: "GET /admin/collections", path: "/admin/collections", ownerStatus: http.StatusOK},
	"createCollection":   {contract: "POST /admin/collections", path: "/admin/collections", body: `{"name":"Registry"}`, ownerStatus: http.StatusCreated},
	"getAdminCollection": {contract: "GET /admin/collections/{id}", path: "/admin/collections/" + unknownItemID, ownerStatus: http.StatusNotFound},
	"updateCollection":   {contract: "PATCH /admin/collections/{id}", path: "/admin/collections/" + unknownItemID, body: `{"name":"Neu"}`, ownerStatus: http.StatusNotFound},
	"deleteCollection":   {contract: "DELETE /admin/collections/{id}", path: "/admin/collections/" + unknownItemID, ownerStatus: http.StatusNotFound},
	"setCollectionItems": {contract: "PUT /admin/collections/{id}/items", path: "/admin/collections/" + unknownItemID + "/items", body: `{"itemIds":[]}`, ownerStatus: http.StatusNotFound},
	"getHomeRows":        {contract: "GET /admin/home-rows", path: "/admin/home-rows", ownerStatus: http.StatusOK},
	"setHomeRows": {contract: "PUT /admin/home-rows", path: "/admin/home-rows", ownerStatus: http.StatusOK,
		body: `[{"key":"spotlight","position":0,"hidden":false},{"key":"picks","position":1,"hidden":false},{"key":"continue","position":2,"hidden":false},` +
			`{"key":"recent","position":3,"hidden":false},{"key":"released","position":4,"hidden":false},{"key":"watchlist","position":5,"hidden":false},` +
			`{"key":"collections","position":6,"hidden":false},{"key":"genres","position":7,"hidden":false}]`},
	"getTraktStatus": {contract: "GET /admin/trakt", path: "/admin/trakt", ownerStatus: http.StatusOK},
	// The fixture has no Trakt client id: 422 provider_not_configured.
	"refreshTrakt": {contract: "POST /admin/trakt/refresh", path: "/admin/trakt/refresh", ownerStatus: http.StatusUnprocessableEntity},
	// M8 administration: jobs 9999 do not exist; the fixture has no e-mail server (422
	// email_not_configured), checks a mock manifest and writes backups into a temporary directory.
	"getSystemInfo":       {contract: "GET /admin/system", path: "/admin/system", ownerStatus: http.StatusOK},
	"getDashboard":        {contract: "GET /admin/dashboard", path: "/admin/dashboard", ownerStatus: http.StatusOK},
	"retryJob":            {contract: "POST /admin/jobs/{id}/retry", path: "/admin/jobs/9999/retry", ownerStatus: http.StatusNotFound},
	"cancelJob":           {contract: "DELETE /admin/jobs/{id}", path: "/admin/jobs/9999", ownerStatus: http.StatusNotFound},
	"getAdminSettings":    {contract: "GET /admin/settings", path: "/admin/settings", ownerStatus: http.StatusOK},
	"updateAdminSettings": {contract: "PUT /admin/settings", path: "/admin/settings", body: `{"updateCheckEnabled":true}`, ownerStatus: http.StatusOK},
	"sendTestEmail":       {contract: "POST /admin/settings/email/test", path: "/admin/settings/email/test", body: `{}`, ownerStatus: http.StatusUnprocessableEntity},
	"rotatePushKeys":      {contract: "POST /admin/settings/push/rotate", path: "/admin/settings/push/rotate", ownerStatus: http.StatusOK},
	"listBackups":         {contract: "GET /admin/backups", path: "/admin/backups", ownerStatus: http.StatusOK},
	"createBackup":        {contract: "POST /admin/backups", path: "/admin/backups", body: `{"includeSecrets":false}`, ownerStatus: http.StatusCreated},
	"downloadBackup": {contract: "GET /admin/backups/{name}/download", path: "/admin/backups/missing.tar.gz/download",
		ownerStatus: http.StatusNotFound},
	"deleteBackup": {contract: "DELETE /admin/backups/{name}", path: "/admin/backups/missing.tar.gz", ownerStatus: http.StatusNotFound},
	"restoreBackupUpload": {contract: "POST /admin/backups/restore", path: "/admin/backups/restore", body: restoreUploadBody,
		header: map[string]string{"Content-Type": "multipart/form-data; boundary=registry"}, ownerStatus: http.StatusUnprocessableEntity},
	"restoreBackup": {contract: "POST /admin/backups/{name}/restore", path: "/admin/backups/missing.tar.gz/restore", body: `{}`,
		ownerStatus: http.StatusNotFound},
	"getUpdateState":      {contract: "GET /admin/update", path: "/admin/update", ownerStatus: http.StatusOK},
	"checkForUpdate":      {contract: "POST /admin/update/check", path: "/admin/update/check", ownerStatus: http.StatusOK},
	"listAccounts":        {contract: "GET /admin/accounts", path: "/admin/accounts", ownerStatus: http.StatusOK},
	"getStats":            {contract: "GET /admin/stats", path: "/admin/stats?days=7", ownerStatus: http.StatusOK},
	"listAnnouncements":   {contract: "GET /admin/announcements", path: "/admin/announcements", ownerStatus: http.StatusOK},
	"createAnnouncement":  {contract: "POST /admin/announcements", path: "/admin/announcements", body: `{"title":"Hallo","body":"Text"}`, ownerStatus: http.StatusCreated},
	"updateAnnouncement":  {contract: "PUT /admin/announcements/{id}", path: "/admin/announcements/9999", body: `{"title":"Hallo","body":"Text"}`, ownerStatus: http.StatusNotFound},
	"deleteAnnouncement":  {contract: "DELETE /admin/announcements/{id}", path: "/admin/announcements/9999", ownerStatus: http.StatusNotFound},
	"publishAnnouncement": {contract: "POST /admin/announcements/{id}/publish", path: "/admin/announcements/9999/publish", ownerStatus: http.StatusNotFound},
}

// Fixed ids of the vistest fixture (vistest.addMovie: "01J9000000000000000000R0" + label).
const (
	fixtureFilm6  = "01J9000000000000000000R006"
	fixtureSeries = "01J9000000000000000000R0SS"
)

// publicCall is one call of the public registry (no session).
type publicCall struct {
	contract, path, body string
	status               int
}

const unknownInvitationToken = "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA" // 43 characters

const unknownWebhookToken = "BBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBB" // 43 characters

var publicRegistry = map[string]publicCall{
	"getHealth":      {contract: "GET /healthz", path: "/healthz", status: http.StatusOK},
	"getSetupStatus": {contract: "GET /setup", path: "/setup", status: http.StatusOK},
	"runSetup": {contract: "POST /setup", path: "/setup", status: http.StatusConflict,
		body: `{"email":"setup@example.ch","password":"long-enough-password","library":{"name":"A","kind":"movies","path":"/x"}}`},
	"login":         {contract: "POST /auth/login", path: "/auth/login", body: `{"email":"nobody@example.ch","password":"wrong-password"}`, status: http.StatusUnauthorized},
	"getInvitation": {contract: "GET /invitations/{token}", path: "/invitations/" + unknownInvitationToken, status: http.StatusNotFound},
	"acceptInvitation": {contract: "POST /invitations/{token}/accept", path: "/invitations/" + unknownInvitationToken + "/accept", status: http.StatusNotFound,
		body: `{"email":"guest2@example.ch","password":"long-enough-password","profileName":"G"}`},
	// M5: without the device cookie the revocation list is empty (K-7).
	"checkOfflineRevocations": {contract: "POST /offline/revocations", path: "/offline/revocations", status: http.StatusOK,
		body: `{"ids":["01J9AAAAAAAAAAAAAAAAAAAAAA"]}`},
	// M9: an unknown webhook token is 401, whatever the body says (I-8).
	"receiveWebhook": {contract: "POST /webhooks/{token}", path: "/webhooks/" + unknownWebhookToken,
		body: `{"eventType":"Test"}`, status: http.StatusUnauthorized},
	// M8 wizard checks: 409 setup_done, the fixture has an account.
	"checkSetupPath": {contract: "POST /setup/path-check", path: "/setup/path-check", body: `{"path":"/tmp"}`, status: http.StatusConflict},
	// Idea 02: the wizard's root list ends with the wizard, like the two checks above.
	"listSetupRoots":    {contract: "GET /setup/roots", path: "/setup/roots", status: http.StatusConflict},
	"checkSetupTmdbKey": {contract: "POST /setup/tmdb-check", path: "/setup/tmdb-check", body: `{"tmdbApiKey":"not-a-key"}`, status: http.StatusConflict},
}

// A missing registry entry fails the test: every operation of the contract is in exactly the
// registry of its x-visibility, with the same method and path.
func TestContractCoversEveryOperation(t *testing.T) {
	t.Parallel()
	ops := contractOperations(t)
	ids := make([]string, 0, len(ops))
	for id := range ops {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	check := func(id, registry, contract string, ok bool, op contractOp) {
		switch {
		case !ok:
			t.Errorf("operation %s (%s, x-visibility %s) has no entry in %s", id, op.contract, op.vis, registry)
		case contract != op.contract:
			t.Errorf("%s entry %s targets %q, the contract says %q", registry, id, contract, op.contract)
		}
	}
	for _, id := range ids {
		op := ops[id]
		switch op.vis {
		case "scoped":
			c, ok := profileRegistry[id]
			check(id, "profileRegistry", c.contract, ok, op)
		case "account":
			c, ok := accountRegistry[id]
			check(id, "accountRegistry", c.contract, ok, op)
		case "admin":
			c, ok := adminRegistry[id]
			check(id, "adminRegistry", c.contract, ok, op)
		case "public":
			c, ok := publicRegistry[id]
			check(id, "publicRegistry", c.contract, ok, op)
		default:
			t.Errorf("operation %s has x-visibility %q, which no registry covers", id, op.vis)
		}
	}
	for name, reg := range map[string][]string{
		"profileRegistry": keysOf(profileRegistry), "accountRegistry": keysOf(accountRegistry),
		"adminRegistry": keysOf(adminRegistry), "publicRegistry": keysOf(publicRegistry),
	} {
		for _, id := range reg {
			if _, ok := ops[id]; !ok {
				t.Errorf("%s entry %s is not an operation of the contract", name, id)
			}
		}
	}
}

func keysOf[V any](m map[string]V) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}

// secretsIn lists markers that no response may carry: PIN hashes, password hashes, tokens.
var secretsIn = []string{"argon2id", "pin_hash", "pinHash", "password_hash", "tokenHash"}

func assertNoSecrets(t *testing.T, label string, rec *httptest.ResponseRecorder) {
	t.Helper()
	for _, s := range secretsIn {
		if strings.Contains(rec.Body.String(), s) {
			t.Errorf("%s: response carries %q: %s", label, s, rec.Body.String())
		}
	}
}

// Account operations work without a chosen profile, never answer 403 profile_required, never
// contain a title id and never expose hashes.
func TestContractAccountOperations(t *testing.T) {
	t.Parallel()
	ops := contractOperations(t)
	pf := newProfileFixture(t)
	f, h := pf.f, pf.h
	titles := f.All()
	q := appq.New(f.DB.Write)

	ids := keysOf(accountRegistry)
	sort.Strings(ids)
	for _, id := range ids {
		call := accountRegistry[id]
		if ops[id].vis != "account" {
			continue // reported by TestContractCoversEveryOperation
		}
		method := strings.Fields(call.contract)[0]
		for _, a := range []actor{actorPrimary, actorNone, actorGuest} {
			t.Run(id+"/"+a.name, func(t *testing.T) {
				account := a.account(f)
				name := id + "-" + a.name
				pf.newSession(t, name, a.profile(f))
				target := a.choose(f)
				if call.fresh {
					row, err := q.InsertProfile(context.Background(), appq.InsertProfileParams{
						AccountID: account.ID, Name: "Target", MaxRating: 18, Language: "de", AllLibraries: 1, Now: 1,
					})
					if err != nil {
						t.Fatal(err)
					}
					target = model.Profile{ID: row.ID}
				}
				path := strings.ReplaceAll(call.path, "{profile}", fmt.Sprint(target.ID))
				rec := send(t, h, method, BasePath+path, name, call.body, nil)
				if want := call.want[a.name]; rec.Code != want {
					t.Fatalf("status %d (%s), want %d", rec.Code, rec.Body.String(), want)
				}
				if rec.Code >= 400 && problemCode(t, rec) == "profile_required" {
					t.Errorf("an account operation answered profile_required: %s", rec.Body.String())
				}
				if a.name == "none" && rec.Code == http.StatusForbidden && problemCode(t, rec) != "primary_profile_required" {
					t.Errorf("session without profile on profile management: code %s, want primary_profile_required", problemCode(t, rec))
				}
				if found := itemIDsIn(t, rec, titles); len(found) != 0 {
					t.Errorf("response contains title ids %v", found)
				}
				assertNoSecrets(t, id, rec)
			})
		}
	}
}

// Admin operations: the guest, the owner's non-primary profile and an owner without a chosen
// profile get 403 forbidden; the owner with the primary profile gets through.
func TestContractAdminOperations(t *testing.T) {
	t.Parallel()
	ops := contractOperations(t)
	pf := newProfileFixture(t)
	h := pf.h

	ids := keysOf(adminRegistry)
	sort.Strings(ids)
	for _, id := range ids {
		call := adminRegistry[id]
		if ops[id].vis != "admin" {
			continue
		}
		method := strings.Fields(call.contract)[0]
		t.Run(id, func(t *testing.T) {
			for _, token := range []string{"pg", "pg18", "p6", "p0", "none"} {
				rec := send(t, h, method, BasePath+call.path, token, call.body, call.header)
				if rec.Code != http.StatusForbidden || problemCode(t, rec) != "forbidden" {
					t.Errorf("%s: status %d (%s), want 403 forbidden", token, rec.Code, rec.Body.String())
				}
			}
			if rec := send(t, h, method, BasePath+call.path, "", call.body, call.header); rec.Code != http.StatusUnauthorized {
				t.Errorf("without a session: status %d, want 401", rec.Code)
			}
			rec := send(t, h, method, BasePath+call.path, "p18", call.body, call.header)
			if rec.Code != call.ownerStatus {
				t.Errorf("owner with the primary profile: status %d (%s), want %d", rec.Code, rec.Body.String(), call.ownerStatus)
			}
			// Administration views name titles by id (unfiltered, owner only) but never as ItemRef,
			// which carries scoped image URLs.
			if refs := itemRefsIn(t, rec); len(refs) != 0 {
				t.Errorf("response contains ItemRef objects %v", refs)
			}
			if strings.Contains(rec.Body.String(), "/images/") {
				t.Errorf("response contains image URLs: %s", rec.Body.String())
			}
			assertNoSecrets(t, id, rec)
		})
	}
}

// Public operations work without a session and carry no title ids.
func TestContractPublicOperations(t *testing.T) {
	t.Parallel()
	ops := contractOperations(t)
	pf := newProfileFixture(t)
	titles := pf.f.All()
	ids := keysOf(publicRegistry)
	sort.Strings(ids)
	for _, id := range ids {
		call := publicRegistry[id]
		if ops[id].vis != "public" {
			continue
		}
		t.Run(id, func(t *testing.T) {
			rec := send(t, pf.h, strings.Fields(call.contract)[0], BasePath+call.path, "", call.body, nil)
			if rec.Code != call.status {
				t.Fatalf("status %d (%s), want %d", rec.Code, rec.Body.String(), call.status)
			}
			if found := itemIDsIn(t, rec, titles); len(found) != 0 {
				t.Errorf("response contains title ids %v", found)
			}
			assertNoSecrets(t, id, rec)
		})
	}
}

// getTitle returns the status of getItem for a token.
func getTitle(t *testing.T, h http.Handler, token, id string) int {
	t.Helper()
	return send(t, h, http.MethodGet, BasePath+"/items/"+id, token, "", nil).Code
}

// homeIDs returns every title id of the start page of token.
func homeIDs(t *testing.T, h http.Handler, token string) []string {
	t.Helper()
	_, rows := homeRows(t, send(t, h, http.MethodGet, BasePath+"/home", token, "", nil))
	var out []string
	for _, ids := range rows {
		out = append(out, ids...)
	}
	return out
}

func setRating(t *testing.T, h http.Handler, token, itemID, body string) *httptest.ResponseRecorder {
	t.Helper()
	return send(t, h, http.MethodPut, BasePath+"/admin/items/"+itemID+"/rating", token, body, nil)
}

// Plan 10.2 "Freigabe-Änderung": the owner raises the series from 12 to 16; for P12 the series, the
// season and both episodes vanish at once (404 by id, absent from the start page) and a running
// stream of an episode answers 404 on its next segment request. Back to 12 they return, while the
// episode with its own rating 16 stays invisible for P12.
func TestContractRatingChange(t *testing.T) {
	t.Parallel()
	pf := newProfileFixture(t)
	f, h := pf.f, pf.h
	show := f.Show
	segment := func(ps, st string) int {
		return send(t, h, http.MethodGet, BasePath+"/playback/sessions/"+ps+"/seg/0?st="+url.QueryEscape(st), "", "", nil).Code
	}

	for _, m := range []vistest.Movie{show.Series, show.Season, show.E1} {
		if got := getTitle(t, h, "p12", m.ID); got != http.StatusOK {
			t.Fatalf("P12 getItem %s %s before the change: %d", m.Kind, m.Label, got)
		}
	}
	if got := getTitle(t, h, "p12", show.E2.ID); got != http.StatusNotFound {
		t.Fatalf("P12 getItem E2 (own rating 16) before the change: %d, want 404", got)
	}
	if !slices.Contains(homeIDs(t, h, "p12"), show.Series.ID) {
		t.Fatal("the series is absent from P12's start page before the change")
	}
	ps, st := pf.mint(t, "p12", show.E1, "remux")
	if got := segment(ps, st); got != http.StatusOK {
		t.Fatalf("segment of the running stream before the change: %d", got)
	}

	// Only the owner with the primary profile may override: the guest's attempt changes nothing.
	if rec := setRating(t, h, "pg", show.Series.ID, `{"rating":16}`); rec.Code != http.StatusForbidden {
		t.Fatalf("guest override: %d", rec.Code)
	}
	if rec := setRating(t, h, "p18", show.Series.ID, `{"rating":7}`); rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("invalid rating: %d (%s)", rec.Code, rec.Body.String())
	}

	rec := setRating(t, h, "p18", show.Series.ID, `{"rating":16}`)
	var view struct {
		ID              string `json:"id"`
		Kind            string `json:"kind"`
		OwnRating       *int   `json:"ownRating"`
		EffectiveRating *int   `json:"effectiveRating"`
		RatingSource    string `json:"ratingSource"`
	}
	decode(t, rec, &view)
	if rec.Code != http.StatusOK || view.ID != show.Series.ID || view.Kind != "series" || view.OwnRating == nil || *view.OwnRating != 16 ||
		view.EffectiveRating == nil || *view.EffectiveRating != 16 || view.RatingSource != "manual" {
		t.Fatalf("setItemRating 16 = %d %s", rec.Code, rec.Body.String())
	}
	if locked, source := f.ItemFlags(t, show.Series.ID); source != "manual" || !strings.Contains(locked, "own_rating") {
		t.Errorf("after the override locked_fields = %s, rating_source = %s, want own_rating locked and manual", locked, source)
	}
	home := homeIDs(t, h, "p12")
	for _, m := range show.All() {
		if got := getTitle(t, h, "p12", m.ID); got != http.StatusNotFound {
			t.Errorf("P12 getItem %s %s after the raise: %d, want 404", m.Kind, m.Label, got)
		}
		if slices.Contains(home, m.ID) {
			t.Errorf("P12 start page still lists %s %s after the raise", m.Kind, m.Label)
		}
	}
	if got := segment(ps, st); got != http.StatusNotFound {
		t.Errorf("segment of the running stream after the raise: %d, want 404", got)
	}
	if rec := send(t, h, http.MethodGet, BasePath+"/items/"+show.Series.ID+"/trailers", "p12", "", nil); rec.Code != http.StatusNotFound {
		t.Errorf("P12 trailers of the raised series: %d, want 404", rec.Code)
	}
	if got := getTitle(t, h, "p16", show.Series.ID); got != http.StatusOK {
		t.Errorf("P16 must see the series rated 16: %d", got)
	}
	if got := getTitle(t, h, "p18", show.E2.ID); got != http.StatusOK {
		t.Errorf("P18 must still see everything: %d", got)
	}

	// Back to 12: the tree returns for P12 except E2, whose own rating 16 keeps it hidden.
	if rec := setRating(t, h, "p18", show.Series.ID, `{"rating":12}`); rec.Code != http.StatusOK {
		t.Fatalf("setItemRating 12 = %d %s", rec.Code, rec.Body.String())
	}
	home = homeIDs(t, h, "p12")
	for _, m := range []vistest.Movie{show.Series, show.Season, show.E1, show.E3, show.Specials, show.Special, show.Extra} {
		if got := getTitle(t, h, "p12", m.ID); got != http.StatusOK {
			t.Errorf("P12 getItem %s %s after the way back: %d, want 200", m.Kind, m.Label, got)
		}
	}
	if d := getDetail(t, h, "p12", show.Series.ID); !slices.Equal(refIDs(d.Seasons), []string{show.Season.ID, show.Specials.ID}) ||
		!slices.Equal(refIDs(d.Extras), []string{show.Extra.ID}) || d.TrailerCount != 2 {
		t.Errorf("P12 series after the way back: seasons %v, extras %v, trailers %d", refIDs(d.Seasons), refIDs(d.Extras), d.TrailerCount)
	}
	if got := getTitle(t, h, "p12", show.E2.ID); got != http.StatusNotFound {
		t.Errorf("P12 getItem E2 after the way back: %d, want 404 (own rating 16)", got)
	}
	if !slices.Contains(home, show.Series.ID) || slices.Contains(home, show.E2.ID) {
		t.Errorf("P12 start page after the way back: %v", home)
	}
	if got := segment(ps, st); got != http.StatusOK {
		t.Errorf("segment of the same stream after the way back: %d, want 200", got)
	}

	// Clearing the override drops the lock and the source; the title counts as unrated until the
	// metadata job fills it in again (unknown = 18), then an explicit level restores it.
	rec = setRating(t, h, "p18", show.Series.ID, `{"rating":null}`)
	view.OwnRating, view.RatingSource = nil, ""
	decode(t, rec, &view)
	if rec.Code != http.StatusOK || view.OwnRating != nil || view.RatingSource != "" {
		t.Fatalf("setItemRating null = %d %s", rec.Code, rec.Body.String())
	}
	if locked, source := f.ItemFlags(t, show.Series.ID); source != "" || strings.Contains(locked, "own_rating") {
		t.Errorf("after clearing locked_fields = %s, rating_source = %s", locked, source)
	}
	if got := getTitle(t, h, "p12", show.Series.ID); got != http.StatusNotFound {
		t.Errorf("P12 on the unrated series: %d, want 404", got)
	}
	if rec := setRating(t, h, "p18", show.Series.ID, `{"rating":12}`); rec.Code != http.StatusOK {
		t.Fatalf("restore 12: %d", rec.Code)
	}

	// The extras of a film follow the film: raised to 16, its trailer and featurette vanish for P12.
	m6 := f.Rated(6)
	for _, x := range f.Extras {
		if got := getTitle(t, h, "p12", x.ID); got != http.StatusOK {
			t.Fatalf("P12 getItem extra %s before the raise: %d", x.Label, got)
		}
	}
	if rec := setRating(t, h, "p18", m6.ID, `{"rating":16}`); rec.Code != http.StatusOK {
		t.Fatalf("raise the 6-rated film: %d", rec.Code)
	}
	for _, x := range f.Extras {
		if got := getTitle(t, h, "p12", x.ID); got != http.StatusNotFound {
			t.Errorf("P12 getItem extra %s of the raised film: %d, want 404", x.Label, got)
		}
		if got := getTitle(t, h, "p16", x.ID); got != http.StatusOK {
			t.Errorf("P16 getItem extra %s of the film rated 16: %d, want 200", x.Label, got)
		}
	}
	if rec := setRating(t, h, "p18", m6.ID, `{"rating":6}`); rec.Code != http.StatusOK {
		t.Fatalf("restore the film to 6: %d", rec.Code)
	}

	// The guest's profile PG18 allows 18 but its account stops at 12: nothing above 12, nothing
	// of library X, on every path.
	for _, token := range []string{"pg", "pg18"} {
		profile := pf.tokens[token]
		titles := f.All()
		for _, m := range titles {
			if want := f.VisibleTo(profile, m); want != (getTitle(t, h, token, m.ID) == http.StatusOK) {
				t.Errorf("%s getItem %s: want visible %v", profile.Name, m.Label, want)
			}
		}
		home := send(t, h, http.MethodGet, BasePath+"/home", token, "", nil)
		assertOnlyVisible(t, f, "getHome", profile, titles, itemIDsIn(t, home, titles))
		other := edgeCase(f, func(m vistest.Movie) bool { return m.OtherLibrary })
		for _, m := range []vistest.Movie{f.Rated(16), f.Rated(18), f.Unrated(), show.E2, other} {
			if got := getTitle(t, h, token, m.ID); got != http.StatusNotFound {
				t.Errorf("%s getItem %s: %d, want 404", profile.Name, m.Label, got)
			}
			if slices.Contains(homeIDs(t, h, token), m.ID) {
				t.Errorf("%s start page lists %s", profile.Name, m.Label)
			}
		}
		if got := getTitle(t, h, token, f.Rated(12).ID); got != http.StatusOK {
			t.Errorf("%s getItem on the 12-rated movie: %d, want 200 (off-by-one)", profile.Name, got)
		}
	}
}

// K-7: revoking a guest answers 401 for its cookie and its stream tokens from the next request on
// and aborts a Range response that is already streaming.
// Not parallel: K-7 asks for the open Range response to end within one second of the
// revocation, and that second is measured on the clock.
func TestContractGuestRevocation(t *testing.T) {
	pf := newProfileFixture(t)
	f, h := pf.f, pf.h
	m := f.Rated(6)
	// A long film: a sparse file far larger than the socket buffers.
	if err := os.Truncate(filepath.Join(f.MediaDir, filepath.FromSlash(m.RelPath)), 1<<30); err != nil {
		t.Fatal(err)
	}
	_, directST := pf.mint(t, "pg", m, "direct")
	hlsID, hlsST := pf.mint(t, "pg", f.Show.E1, "remux")
	direct := BasePath + "/items/" + m.ID + "/stream?st=" + url.QueryEscape(directST)
	hls := BasePath + "/playback/sessions/" + hlsID + "/media.m3u8?st=" + url.QueryEscape(hlsST)
	if got := send(t, h, http.MethodGet, BasePath+"/auth/session", "pg", "", nil).Code; got != http.StatusOK {
		t.Fatalf("guest session before: %d", got)
	}
	if got := send(t, h, http.MethodGet, hls, "", "", nil).Code; got != http.StatusOK {
		t.Fatalf("guest HLS stream before: %d", got)
	}

	closed := make(chan struct{})
	var once sync.Once
	srv := httptest.NewUnstartedServer(h)
	srv.Config.ConnState = func(_ net.Conn, state http.ConnState) {
		if state == http.StateClosed {
			once.Do(func() { close(closed) })
		}
	}
	srv.Start()
	defer srv.Close()
	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, srv.URL+direct, nil)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Range", "bytes=0-")
	resp, err := srv.Client().Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusPartialContent {
		t.Fatalf("range: %d", resp.StatusCode)
	}
	if _, err := io.CopyN(io.Discard, resp.Body, 1<<20); err != nil {
		t.Fatal(err)
	}
	time.Sleep(200 * time.Millisecond) // the server fills the socket buffers and blocks in a write

	// The owner finds the guest in the list and revokes it.
	var guests []struct {
		ID           int64  `json:"id"`
		Email        string `json:"email"`
		MaxRating    int    `json:"maxRating"`
		ProfileCount int    `json:"profileCount"`
	}
	decode(t, send(t, h, http.MethodGet, BasePath+"/admin/guests", "p18", "", nil), &guests)
	if len(guests) != 1 || guests[0].ID != f.Guest.ID || guests[0].MaxRating != 12 || guests[0].ProfileCount != 2 {
		t.Fatalf("guest list = %+v", guests)
	}
	start := time.Now()
	if rec := send(t, h, http.MethodDelete, BasePath+fmt.Sprintf("/admin/guests/%d", f.Guest.ID), "p18", "", nil); rec.Code != http.StatusNoContent {
		t.Fatalf("revokeGuest: %d %s", rec.Code, rec.Body.String())
	}
	checks := map[string]*httptest.ResponseRecorder{
		"cookie":   send(t, h, http.MethodGet, BasePath+"/auth/session", "pg", "", nil),
		"cookie 2": send(t, h, http.MethodGet, BasePath+"/home", "pg18", "", nil),
		"direct":   send(t, h, http.MethodGet, direct, "", "", map[string]string{"Range": "bytes=0-9"}),
		"hls":      send(t, h, http.MethodGet, hls, "", "", nil),
	}
	elapsed := time.Since(start)
	for name, rec := range checks {
		if rec.Code != http.StatusUnauthorized {
			t.Errorf("%s after the revocation: %d, want 401", name, rec.Code)
		}
	}
	t.Logf("revoke guest -> cookie and stream token answer 401 after %v", elapsed)
	if elapsed > time.Second {
		t.Errorf("revocation took %v, want < 1 s", elapsed)
	}
	if stops := pf.tc.stops(); !strings.Contains(strings.Join(stops, ","), hlsID) {
		t.Errorf("the guest's HLS output was not stopped: %v", stops)
	}

	// The open Range response is aborted although the client does not read on.
	select {
	case <-closed:
		t.Logf("revoke guest -> open Range response closed after %v", time.Since(start))
	case <-time.After(time.Second):
		t.Fatal("the guest's connection is still open a second after the revocation")
	}
	if n, err := io.Copy(io.Discard, resp.Body); err == nil {
		t.Errorf("response completed after the revocation (%d more bytes)", n)
	}

	// Account, profiles and sessions are gone; the owner's data is untouched.
	var left int
	if err := f.DB.Read.QueryRow(`SELECT (SELECT count(*) FROM accounts WHERE id = ?) + (SELECT count(*) FROM profiles WHERE account_id = ?)
	                              + (SELECT count(*) FROM sessions WHERE account_id = ?)`, f.Guest.ID, f.Guest.ID, f.Guest.ID).Scan(&left); err != nil {
		t.Fatal(err)
	}
	if left != 0 {
		t.Errorf("%d rows of the guest account remain", left)
	}
	if got := send(t, h, http.MethodGet, BasePath+"/auth/session", "p18", "", nil).Code; got != http.StatusOK {
		t.Errorf("owner session after the guest's revocation: %d", got)
	}
	if rec := send(t, h, http.MethodDelete, BasePath+fmt.Sprintf("/admin/guests/%d", f.Guest.ID), "p18", "", nil); rec.Code != http.StatusNotFound {
		t.Errorf("second revocation: %d, want 404", rec.Code)
	}
}

// Removing a library right takes effect with the next request: after the owner's primary profile
// takes library 1 away from PX, the titles of library 1 answer 404 for PX and leave its start page.
func TestContractLibraryRightRevoked(t *testing.T) {
	t.Parallel()
	pf := newProfileFixture(t)
	f, h := pf.f, pf.h
	movie := f.Rated(12)
	if got := getTitle(t, h, "px", movie.ID); got != http.StatusOK {
		t.Fatalf("PX before: %d", got)
	}
	if !slices.Contains(homeIDs(t, h, "px"), movie.ID) {
		t.Fatal("PX start page lacks the movie before the change")
	}
	_, st := pf.mint(t, "px", movie, "direct")
	stream := BasePath + "/items/" + movie.ID + "/stream?st=" + url.QueryEscape(st)
	if got := send(t, h, http.MethodGet, stream, "", "", map[string]string{"Range": "bytes=0-9"}).Code; got != http.StatusPartialContent {
		t.Fatalf("PX stream before: %d", got)
	}

	// Another profile cannot do it: P6 is not the primary profile.
	body := fmt.Sprintf(`{"allLibraries":false,"libraryIds":[%d]}`, f.OtherLibraryID())
	if rec := send(t, h, http.MethodPatch, BasePath+fmt.Sprintf("/profiles/%d", f.PX.ID), "p6", body, nil); rec.Code != http.StatusForbidden || problemCode(t, rec) != "primary_profile_required" {
		t.Fatalf("update by a non-primary profile: %d %s", rec.Code, rec.Body.String())
	}
	rec := send(t, h, http.MethodPatch, BasePath+fmt.Sprintf("/profiles/%d", f.PX.ID), "p18", body, nil)
	var p struct {
		AllLibraries bool    `json:"allLibraries"`
		LibraryIDs   []int64 `json:"libraryIds"`
	}
	decode(t, rec, &p)
	if rec.Code != http.StatusOK || p.AllLibraries || !slices.Equal(p.LibraryIDs, []int64{f.OtherLibraryID()}) {
		t.Fatalf("updateProfile = %d %s", rec.Code, rec.Body.String())
	}
	other := edgeCase(f, func(m vistest.Movie) bool { return m.OtherLibrary })
	for _, m := range f.Movies {
		want := http.StatusNotFound
		if m.ID == other.ID {
			want = http.StatusOK
		}
		if got := getTitle(t, h, "px", m.ID); got != want {
			t.Errorf("PX getItem %s after the change: %d, want %d", m.Label, got, want)
		}
	}
	for _, m := range f.Show.All() {
		if got := getTitle(t, h, "px", m.ID); got != http.StatusNotFound {
			t.Errorf("PX getItem %s %s after the change: %d, want 404", m.Kind, m.Label, got)
		}
	}
	if home := homeIDs(t, h, "px"); slices.Contains(home, movie.ID) || !slices.Contains(home, other.ID) {
		t.Errorf("PX start page after the change: %v", home)
	}
	if got := send(t, h, http.MethodGet, stream, "", "", map[string]string{"Range": "bytes=0-9"}).Code; got != http.StatusNotFound {
		t.Errorf("PX stream after the change: %d, want 404", got)
	}

	// Library rights beyond the account are refused; an unknown library id too.
	for _, bad := range []string{`{"allLibraries":false,"libraryIds":[9999]}`} {
		rec := send(t, h, http.MethodPatch, BasePath+fmt.Sprintf("/profiles/%d", f.PX.ID), "p18", bad, nil)
		if rec.Code != http.StatusUnprocessableEntity || !strings.Contains(rec.Body.String(), "unknown_library") {
			t.Errorf("unknown library: %d %s", rec.Code, rec.Body.String())
		}
	}
	guestBad := fmt.Sprintf(`{"allLibraries":false,"libraryIds":[%d]}`, f.OtherLibraryID())
	if rec := send(t, h, http.MethodPatch, BasePath+fmt.Sprintf("/profiles/%d", f.PG18.ID), "pg", guestBad, nil); rec.Code != http.StatusUnprocessableEntity ||
		!strings.Contains(rec.Body.String(), "unknown_library") {
		t.Errorf("guest profile with a library outside the account: %d %s", rec.Code, rec.Body.String())
	}
	if rec := send(t, h, http.MethodPatch, BasePath+fmt.Sprintf("/profiles/%d", f.PG18.ID), "pg", `{"maxRating":18}`, nil); rec.Code != http.StatusUnprocessableEntity ||
		!strings.Contains(rec.Body.String(), "above_account_limit") {
		t.Errorf("guest profile above the account ceiling: %d %s", rec.Code, rec.Body.String())
	}
}

// B-3: the texts follow the profile language, falling back to English.
func TestContractProfileLanguage(t *testing.T) {
	t.Parallel()
	pf := newProfileFixture(t)
	f, h := pf.f, pf.h
	title := func(token, id string) string {
		t.Helper()
		var d struct {
			Title string `json:"title"`
		}
		decode(t, send(t, h, http.MethodGet, BasePath+"/items/"+id, token, "", nil), &d)
		return d.Title
	}
	if got := title("p6", f.Rated(6).ID); got != "Movie 06" {
		t.Errorf("P6 (en) title = %q, want Movie 06", got)
	}
	if got := title("p12", f.Rated(12).ID); got != "Film 12" {
		t.Errorf("P12 (de) title = %q, want Film 12", got)
	}
	// The home rows use the same language.
	var page struct {
		Rows []struct {
			Items []struct {
				ID, Title string
			} `json:"items"`
		} `json:"rows"`
	}
	decode(t, send(t, h, http.MethodGet, BasePath+"/home", "p6", "", nil), &page)
	found := false
	for _, row := range page.Rows {
		for _, it := range row.Items {
			if it.ID == f.Rated(6).ID {
				found = true
				if it.Title != "Movie 06" {
					t.Errorf("P6 home title = %q, want Movie 06", it.Title)
				}
			}
		}
	}
	if !found {
		t.Error("the 6-rated movie is missing from P6's start page")
	}
	// Switching the language of a profile applies from the next request.
	if rec := send(t, h, http.MethodPatch, BasePath+fmt.Sprintf("/profiles/%d", f.P6.ID), "p18", `{"language":"de"}`, nil); rec.Code != http.StatusOK {
		t.Fatalf("language change: %d %s", rec.Code, rec.Body.String())
	}
	if got := title("p6", f.Rated(6).ID); got != "Film 06" {
		t.Errorf("P6 after the switch to de: title = %q, want Film 06", got)
	}
}

// listAccountLibraries: the main account sees every library, a guest the ones of its account, both
// by name, without a profile and without titles.
func TestListAccountLibraries(t *testing.T) {
	t.Parallel()
	pf := newProfileFixture(t)
	f := pf.f
	type lib struct {
		ID            int64  `json:"id"`
		Name          string `json:"name"`
		Kind          string `json:"kind"`
		DefaultRating *int   `json:"defaultRating"`
	}
	list := func(token string) []lib {
		t.Helper()
		rec := send(t, pf.h, http.MethodGet, BasePath+"/libraries", token, "", nil)
		if rec.Code != http.StatusOK {
			t.Fatalf("%s: %d %s", token, rec.Code, rec.Body.String())
		}
		var out []lib
		decode(t, rec, &out)
		return out
	}
	zero := 0
	want := []lib{{ID: f.LibraryID(), Name: "Filme", Kind: "movies"}, {ID: f.OtherLibraryID(), Name: "Filme X", Kind: "movies"},
		{ID: f.Audio.AudiobookLibraryID, Name: "Hörbücher", Kind: "audiobooks", DefaultRating: &zero},
		{ID: f.Audio.PodcastLibraryID, Name: "Podcasts", Kind: "podcasts", DefaultRating: &zero}}
	same := func(a, b lib) bool {
		return a.ID == b.ID && a.Name == b.Name && a.Kind == b.Kind && (a.DefaultRating == nil) == (b.DefaultRating == nil) &&
			(a.DefaultRating == nil || *a.DefaultRating == *b.DefaultRating)
	}
	for _, token := range []string{"p18", "p6", "px", "none"} {
		if got := list(token); !slices.EqualFunc(got, want, same) {
			t.Errorf("%s libraries = %+v, want %+v", token, got, want)
		}
	}
	for _, token := range []string{"pg", "pg18"} {
		if got := list(token); !slices.EqualFunc(got, want[:1], same) {
			t.Errorf("%s libraries = %+v, want only %+v", token, got, want[:1])
		}
	}
	if rec := send(t, pf.h, http.MethodGet, BasePath+"/libraries", "", "", nil); rec.Code != http.StatusUnauthorized {
		t.Errorf("without a session: %d", rec.Code)
	}
}

// A guest (with its own primary profile) cannot touch the profiles of another account: its
// management calls with the owner's profile ids answer 404, and nothing changes.
// Not parallel: waits a second for the stream of a revoked guest to close.
func TestContractGuestCannotReachOwnerProfiles(t *testing.T) {
	pf := newProfileFixture(t)
	f, h := pf.f, pf.h
	target := fmt.Sprint(f.P12.ID)
	for _, tc := range []struct{ method, path, body string }{
		{http.MethodPost, "/auth/profiles/" + target + "/select", `{}`},
		{http.MethodPatch, "/profiles/" + target, `{"name":"Gehackt"}`},
		{http.MethodPut, "/profiles/" + target + "/pin", `{"pin":"1234"}`},
		{http.MethodDelete, "/profiles/" + target + "/pin", ""},
		{http.MethodDelete, "/profiles/" + target, ""},
	} {
		rec := send(t, h, tc.method, BasePath+tc.path, "pg", tc.body, nil)
		if rec.Code != http.StatusNotFound || problemCode(t, rec) != "not_found" {
			t.Errorf("%s %s as the guest: %d %s, want 404", tc.method, tc.path, rec.Code, rec.Body.String())
		}
	}
	var list []struct {
		ID     int64  `json:"id"`
		Name   string `json:"name"`
		HasPin bool   `json:"hasPin"`
	}
	decode(t, send(t, h, http.MethodGet, BasePath+"/profiles", "p18", "", nil), &list)
	i := slices.IndexFunc(list, func(p struct {
		ID     int64  `json:"id"`
		Name   string `json:"name"`
		HasPin bool   `json:"hasPin"`
	}) bool {
		return p.ID == f.P12.ID
	})
	if i < 0 || list[i].Name != "P12" || list[i].HasPin {
		t.Errorf("the owner's profile changed: %+v", list)
	}
	// And the guest's session still has its own profile.
	if info := sessionOf(t, send(t, h, http.MethodGet, BasePath+"/auth/session", "pg", "", nil)); info.Profile == nil || info.Profile.Id != f.PG.ID {
		t.Errorf("guest session after the attempts: %+v", info)
	}
}

// openRange opens a Range response "bytes=0-" of stream URL on a real server and reads a megabyte,
// so the server sits in a blocked write like it does for a browser with a full buffer. closed is
// closed when the server closes the connection.
func openRange(t *testing.T, h http.Handler, streamURL string) (closed <-chan struct{}, body io.ReadCloser) {
	t.Helper()
	done := make(chan struct{})
	var once sync.Once
	srv := httptest.NewUnstartedServer(h)
	srv.Config.ConnState = func(_ net.Conn, state http.ConnState) {
		if state == http.StateClosed {
			once.Do(func() { close(done) })
		}
	}
	srv.Start()
	t.Cleanup(srv.Close)
	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, srv.URL+streamURL, nil)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Range", "bytes=0-")
	resp, err := srv.Client().Do(req)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = resp.Body.Close() })
	if resp.StatusCode != http.StatusPartialContent {
		t.Fatalf("range: %d", resp.StatusCode)
	}
	if _, err := io.CopyN(io.Discard, resp.Body, 1<<20); err != nil {
		t.Fatal(err)
	}
	time.Sleep(200 * time.Millisecond)
	return done, resp.Body
}

func expectClosed(t *testing.T, what string, closed <-chan struct{}) {
	t.Helper()
	select {
	case <-closed:
	case <-time.After(time.Second):
		t.Fatalf("%s: the connection is still open a second after the change", what)
	}
}

// Direct play of an episode that stops being visible: the open Range response ends when the
// administrator raises the series (plan 10.2 for direct play; HLS ends with its next segment request).
func TestContractRatingChangeEndsOpenDirectStream(t *testing.T) {
	t.Parallel()
	pf := newProfileFixture(t)
	f, h := pf.f, pf.h
	e1 := f.Show.E1
	if err := os.Truncate(filepath.Join(f.MediaDir, filepath.FromSlash(e1.RelPath)), 1<<30); err != nil {
		t.Fatal(err)
	}
	_, st := pf.mint(t, "p12", e1, "direct")
	closed, body := openRange(t, h, BasePath+"/items/"+e1.ID+"/stream?st="+url.QueryEscape(st))

	if rec := setRating(t, h, "p18", f.Show.Series.ID, `{"rating":16}`); rec.Code != http.StatusOK {
		t.Fatalf("raise: %d %s", rec.Code, rec.Body.String())
	}
	expectClosed(t, "open Range response of P12 after the series went to 16", closed)
	if n, err := io.Copy(io.Discard, body); err == nil {
		t.Errorf("response completed after the change (%d more bytes)", n)
	}
	// The token itself still resolves, but the title is gone: 404 like every other request.
	if got := send(t, h, http.MethodGet, BasePath+"/items/"+e1.ID+"/stream?st="+url.QueryEscape(st), "", "", map[string]string{"Range": "bytes=0-9"}).Code; got != http.StatusNotFound {
		t.Errorf("new Range request after the change: %d, want 404", got)
	}
}

// M3 review, findings 9 and 10: merging a title into another deletes the source, so an open
// direct stream of the source ends like after a rating change.
func TestContractMergeEndsOpenDirectStream(t *testing.T) {
	t.Parallel()
	pf := newProfileFixture(t)
	f, h := pf.f, pf.h
	source, target := f.Rated(6), f.Rated(0)
	if err := os.Truncate(filepath.Join(f.MediaDir, filepath.FromSlash(source.RelPath)), 1<<30); err != nil {
		t.Fatal(err)
	}
	_, st := pf.mint(t, "p18", source, "direct")
	closed, _ := openRange(t, h, BasePath+"/items/"+source.ID+"/stream?st="+url.QueryEscape(st))
	body := `{"targetId":"` + target.ID + `","sourceIds":["` + source.ID + `"]}`
	if rec := send(t, h, http.MethodPost, BasePath+"/admin/maintenance/merge", "p18", body, nil); rec.Code != http.StatusOK {
		t.Fatalf("merge: %d %s", rec.Code, rec.Body.String())
	}
	expectClosed(t, "open Range response of the merged source", closed)
}

// Lowering a guest's ceiling, taking a library right from a profile and changing a profile's
// rating all end the open direct streams they hide.
func TestContractLoweredAccessEndsOpenDirectStreams(t *testing.T) {
	t.Parallel()
	pf := newProfileFixture(t)
	f, h := pf.f, pf.h
	m := f.Rated(12)
	if err := os.Truncate(filepath.Join(f.MediaDir, filepath.FromSlash(m.RelPath)), 1<<30); err != nil {
		t.Fatal(err)
	}
	stream := func(token string) (<-chan struct{}, io.ReadCloser) {
		_, st := pf.mint(t, token, m, "direct")
		return openRange(t, h, BasePath+"/items/"+m.ID+"/stream?st="+url.QueryEscape(st))
	}

	guestClosed, _ := stream("pg")
	pxClosed, _ := stream("px")
	p12Closed, _ := stream("p12")
	// Lower the guest to 6: the 12-rated movie is above it.
	if rec := send(t, h, http.MethodPatch, BasePath+fmt.Sprintf("/admin/guests/%d", f.Guest.ID), "p18", `{"maxRating":6}`, nil); rec.Code != http.StatusOK {
		t.Fatalf("updateGuest: %d %s", rec.Code, rec.Body.String())
	}
	expectClosed(t, "guest stream after the ceiling dropped to 6", guestClosed)
	select {
	case <-pxClosed:
		t.Fatal("the stream of PX ended for a change that does not concern it")
	case <-p12Closed:
		t.Fatal("the stream of P12 ended for a change that does not concern it")
	default:
	}
	// Take the library from PX.
	body := fmt.Sprintf(`{"allLibraries":false,"libraryIds":[%d]}`, f.OtherLibraryID())
	if rec := send(t, h, http.MethodPatch, BasePath+fmt.Sprintf("/profiles/%d", f.PX.ID), "p18", body, nil); rec.Code != http.StatusOK {
		t.Fatalf("updateProfile: %d %s", rec.Code, rec.Body.String())
	}
	expectClosed(t, "PX stream after its library right was removed", pxClosed)
	// Lower P12's rating below the title.
	if rec := send(t, h, http.MethodPatch, BasePath+fmt.Sprintf("/profiles/%d", f.P12.ID), "p18", `{"maxRating":6}`, nil); rec.Code != http.StatusOK {
		t.Fatalf("updateProfile: %d %s", rec.Code, rec.Body.String())
	}
	expectClosed(t, "P12 stream after its rating dropped to 6", p12Closed)
}

// slipPlayback interleaves a step into the first EndAccount call, after the real one.
type slipPlayback struct {
	*playback.Service
	calls int
	slip  func()
}

func (p *slipPlayback) EndAccount(ctx context.Context, id int64) error {
	p.calls++
	err := p.Service.EndAccount(ctx, id)
	if p.calls == 1 && p.slip != nil {
		p.slip()
	}
	return err
}

// The order of a guest revocation: the sessions are revoked before the playback ends (so nothing
// new can be authorised), and a stream that a request already past its checks opens between the
// steps is still ended by the second call after the account is gone.
func TestContractGuestRevocationOrder(t *testing.T) {
	t.Parallel()
	pf := newProfileFixture(t)
	f := pf.f
	m := f.Rated(6)
	ctx := context.Background()
	_, st := pf.mint(t, "pg", m, "direct")
	grant, err := pf.pb.Authorize(ctx, st) // the request is past its token check
	if err != nil {
		t.Fatal(err)
	}

	var slipped model.StreamSource
	wrapper := &slipPlayback{Service: pf.pb}
	wrapper.slip = func() {
		if _, _, err := pf.pa.Resolve(ctx, "pg"); err == nil {
			t.Error("the guest session still resolves while its playback ends: sessions must be revoked first")
		}
		if _, err := pf.pb.Authorize(ctx, st); err == nil {
			t.Error("a stream token still authorises after the sessions were revoked")
		}
		var accounts int
		if err := f.DB.Read.QueryRow(`SELECT count(*) FROM accounts WHERE id = ?`, f.Guest.ID).Scan(&accounts); err != nil || accounts != 1 {
			t.Errorf("the account must still exist during the first end: %d (%v)", accounts, err)
		}
		src, err := pf.pb.OpenGrant(ctx, grant) // the late stream
		if err != nil {
			t.Error(err)
			return
		}
		slipped = src
	}
	h := pf.handlerWithPlayback(wrapper)
	if rec := send(t, h, http.MethodDelete, BasePath+fmt.Sprintf("/admin/guests/%d", f.Guest.ID), "p18", "", nil); rec.Code != http.StatusNoContent {
		t.Fatalf("revokeGuest: %d %s", rec.Code, rec.Body.String())
	}
	if wrapper.calls != 2 {
		t.Errorf("EndAccount ran %d times, want 2 (before and after the delete)", wrapper.calls)
	}
	if slipped.File == nil {
		t.Fatal("the late stream was not opened")
	}
	defer slipped.File.Close()
	select {
	case <-slipped.File.(interface{ Done() <-chan struct{} }).Done():
	default:
		t.Error("the stream that slipped in between survived the revocation")
	}
}
