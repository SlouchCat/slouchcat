package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"

	"github.com/slouchcat/slouchcat/server/internal/api/gen"
	"github.com/slouchcat/slouchcat/server/internal/model"
)

// uploadImageContentType is the multipart type of uploadImageBody.
const uploadImageContentType = "multipart/form-data; boundary=upload"

// uploadImageBody is a multipart body with the part file (name filename, declared as JPEG).
func uploadImageBody(filename, content string) string {
	return "--upload\r\nContent-Disposition: form-data; name=\"file\"; filename=\"" + filename + "\"\r\n" +
		"Content-Type: image/jpeg\r\n\r\n" + content + "\r\n--upload--\r\n"
}

// fieldErrors returns "<field> <code>" of a validation problem.
func fieldErrors(t *testing.T, rec *httptest.ResponseRecorder) []string {
	t.Helper()
	var p gen.Problem
	if err := json.Unmarshal(rec.Body.Bytes(), &p); err != nil || p.Errors == nil {
		t.Fatalf("status %d, body %q: no field errors (%v)", rec.Code, rec.Body.String(), err)
	}
	var out []string
	for _, f := range *p.Errors {
		out = append(out, f.Field+" "+f.Code)
	}
	return out
}

// Contract 1.15.0 (concept 15 7.5): uploadItemImage is a binary handler for the administrator.
// It reads the part file of a multipart body (up to 20 MB, the route's body limit is 20 MB plus
// framing), hands it to the service, which checks the content, and answers the administration
// view with imageKinds. Wrong bodies are 422 on field file, an unknown title 404; the access rules
// (403 for everybody but an administrator with the primary profile, 401 without a session) are
// in TestContractAdminOperations.
func TestContractUploadItemImage(t *testing.T) {
	t.Parallel()
	pf := newProfileFixture(t)
	jpeg := "\xff\xd8\xff\xe0 a JPEG for the fake"
	path := BasePath + "/admin/items/" + fixtureFilm6 + "/images/poster/upload"
	multipart := map[string]string{"Content-Type": uploadImageContentType}

	rec := send(t, pf.h, http.MethodPut, path, "p18", uploadImageBody("poster.png", jpeg), multipart)
	if rec.Code != http.StatusOK || !strings.HasPrefix(rec.Header().Get("Content-Type"), "application/json") {
		t.Fatalf("upload: %d %s %s", rec.Code, rec.Header().Get("Content-Type"), rec.Body.String())
	}
	var it gen.AdminItem
	decode(t, rec, &it)
	if it.Id != fixtureFilm6 || it.ImageKinds == nil || !slices.Equal(*it.ImageKinds, []gen.ImageKind{gen.ImageKindPoster, gen.ImageKindBackdrop, gen.ImageKindLogo}) {
		t.Fatalf("answer id %s, imageKinds %v", it.Id, it.ImageKinds)
	}
	if rec := send(t, pf.h, http.MethodGet, BasePath+"/admin/items/"+fixtureFilm6, "p18", "", nil); !strings.Contains(rec.Body.String(), `"imageKinds":["poster","backdrop","logo"]`) {
		t.Errorf("getAdminItem without imageKinds: %s", rec.Body.String())
	}

	big := strings.Repeat("x", model.MaxImageUploadBytes+1)
	beyond := strings.Repeat("x", MaxImageUploadBodyBytes)
	for name, tc := range map[string]struct {
		path, body string
		header     map[string]string
		status     int
		want       string
	}{
		"text named .jpg": {path, uploadImageBody("poster.jpg", "no image at all"), multipart, http.StatusUnprocessableEntity, "file unsupported_format"},
		"over 20 MB":      {path, uploadImageBody("poster.jpg", "\xff\xd8\xff"+big), multipart, http.StatusUnprocessableEntity, "file too_large"},
		"over the limit":  {path, uploadImageBody("poster.jpg", "\xff\xd8\xff"+beyond), multipart, http.StatusUnprocessableEntity, "file too_large"},
		"no file part": {path, "--upload\r\nContent-Disposition: form-data; name=\"other\"\r\n\r\nx\r\n--upload--\r\n", multipart,
			http.StatusUnprocessableEntity, "file required"},
		"json body":     {path, `{"file":"x"}`, nil, http.StatusUnprocessableEntity, "file invalid_value"},
		"kind":          {BasePath + "/admin/items/" + fixtureFilm6 + "/images/season/upload", uploadImageBody("p.jpg", jpeg), multipart, http.StatusUnprocessableEntity, "kind invalid_value"},
		"unknown title": {BasePath + "/admin/items/" + unknownItemID + "/images/poster/upload", uploadImageBody("p.jpg", jpeg), multipart, http.StatusNotFound, ""},
		// The title and the kind are checked before the body is read: a body that is no
		// multipart or over the limit does not count then.
		"kind before body":  {BasePath + "/admin/items/" + fixtureFilm6 + "/images/season/upload", `{"file":"x"}`, nil, http.StatusUnprocessableEntity, "kind invalid_value"},
		"title before body": {BasePath + "/admin/items/" + unknownItemID + "/images/poster/upload", uploadImageBody("poster.jpg", "\xff\xd8\xff"+beyond), multipart, http.StatusNotFound, ""},
	} {
		rec := send(t, pf.h, http.MethodPut, tc.path, "p18", tc.body, tc.header)
		if rec.Code != tc.status {
			t.Errorf("%s: %d %s, want %d", name, rec.Code, rec.Body.String(), tc.status)
			continue
		}
		if tc.want != "" {
			if got := fieldErrors(t, rec); !slices.Equal(got, []string{tc.want}) {
				t.Errorf("%s: field errors %v, want %s", name, got, tc.want)
			}
		}
	}

	// The CSRF header rule holds for the binary route as well.
	req := httptest.NewRequest(http.MethodPut, path, strings.NewReader(uploadImageBody("p.jpg", jpeg)))
	req.Header.Set("Content-Type", uploadImageContentType)
	req.AddCookie(&http.Cookie{Name: SessionCookie, Value: "p18"})
	rec = httptest.NewRecorder()
	pf.h.ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden || problemCode(t, rec) != "csrf_header_missing" {
		t.Errorf("without the CSRF header: %d %s", rec.Code, rec.Body.String())
	}
}
