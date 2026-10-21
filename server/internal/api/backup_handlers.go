package api

import (
	"context"
	"errors"
	"io"
	"mime"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/slouchcat/slouchcat/server/internal/api/gen"
	"github.com/slouchcat/slouchcat/server/internal/model"
)

// M8 backup and restore (plan M8, A-4, 3.8 point 3; design sections 1 and 3.4). The list, the
// creation, the deletion and the restores are JSON operations of the strict server; the upload of
// restoreBackupUpload arrives as multipart/form-data, whose parts the strict server hands over as a
// stream. The download is a binary endpoint (CLAUDE.md rule 6) behind the same session middleware.

// backupDownloadPath is the binary download route (binaryRoutes, visibility admin).
const backupDownloadPath = "/admin/backups/{name}/download"

// maxPassphraseBytes caps the passphrase part of an upload, like the contract's maxLength.
const maxPassphraseBytes = 1024

func (e serviceProblem) VisitListBackupsResponse(w http.ResponseWriter) error  { return e.visit(w) }
func (e serviceProblem) VisitCreateBackupResponse(w http.ResponseWriter) error { return e.visit(w) }
func (e serviceProblem) VisitDeleteBackupResponse(w http.ResponseWriter) error { return e.visit(w) }
func (e serviceProblem) VisitRestoreBackupUploadResponse(w http.ResponseWriter) error {
	return e.visit(w)
}
func (e serviceProblem) VisitRestoreBackupResponse(w http.ResponseWriter) error { return e.visit(w) }

func (s *server) ListBackups(ctx context.Context, _ gen.ListBackupsRequestObject) (gen.ListBackupsResponseObject, error) {
	list, err := s.opts.Admin.ListBackups(ctx)
	if err != nil {
		return s.problem(ctx, err), nil
	}
	out := make(gen.ListBackups200JSONResponse, 0, len(list))
	for _, b := range list {
		out = append(out, toBackup(b))
	}
	return out, nil
}

func (s *server) CreateBackup(ctx context.Context, req gen.CreateBackupRequestObject) (gen.CreateBackupResponseObject, error) {
	b := deref(req.Body)
	backup, err := s.opts.Admin.CreateBackup(ctx, model.BackupRequest{IncludeSecrets: b.IncludeSecrets, Passphrase: deref(b.Passphrase)})
	if err != nil {
		return s.problem(ctx, err), nil
	}
	return gen.CreateBackup201JSONResponse(toBackup(backup)), nil
}

func (s *server) DeleteBackup(ctx context.Context, req gen.DeleteBackupRequestObject) (gen.DeleteBackupResponseObject, error) {
	if err := s.opts.Admin.DeleteBackup(ctx, req.Name); err != nil {
		return s.problem(ctx, err), nil
	}
	return gen.DeleteBackup204Response{}, nil
}

// RestoreBackupUpload reads the parts of the upload in order: the passphrase part is collected, the
// file part goes to the service as a stream without being buffered, later parts are ignored. A
// client therefore sends the passphrase before the file (the contract says so).
func (s *server) RestoreBackupUpload(ctx context.Context, req gen.RestoreBackupUploadRequestObject) (gen.RestoreBackupUploadResponseObject, error) {
	if req.Body == nil {
		return s.problem(ctx, (&model.ValidationError{}).Add("file", "required")), nil
	}
	var passphrase string
	for {
		part, err := req.Body.NextPart()
		if errors.Is(err, io.EOF) {
			return s.problem(ctx, (&model.ValidationError{}).Add("file", "required")), nil
		}
		if err != nil {
			return s.problem(ctx, (&model.ValidationError{}).Add("file", "invalid_value")), nil
		}
		switch part.FormName() {
		case "passphrase":
			raw, err := io.ReadAll(io.LimitReader(part, maxPassphraseBytes+1))
			_ = part.Close()
			if err != nil {
				return s.problem(ctx, (&model.ValidationError{}).Add("passphrase", "invalid_value")), nil
			}
			if len(raw) > maxPassphraseBytes {
				return s.problem(ctx, (&model.ValidationError{}).Add("passphrase", "too_long")), nil
			}
			passphrase = string(raw)
		case "file":
			res, err := s.opts.Admin.RestoreBackupUpload(ctx, part, passphrase)
			_ = part.Close()
			if err != nil {
				return s.problem(ctx, err), nil
			}
			return gen.RestoreBackupUpload202JSONResponse(toRestoreResult(res)), nil
		default:
			_ = part.Close()
		}
	}
}

func (s *server) RestoreBackup(ctx context.Context, req gen.RestoreBackupRequestObject) (gen.RestoreBackupResponseObject, error) {
	var passphrase string
	if req.Body != nil {
		passphrase = deref(req.Body.Passphrase)
	}
	res, err := s.opts.Admin.RestoreBackup(ctx, req.Name, passphrase)
	if err != nil {
		return s.problem(ctx, err), nil
	}
	return gen.RestoreBackup202JSONResponse(toRestoreResult(res)), nil
}

// downloadBackup serves a backup archive as an attachment (binary endpoint, admin only through the
// session middleware). The service checks the name against the backup list, so no path from the
// request reaches the file system (CLAUDE.md rule 4).
func (s *server) downloadBackup(w http.ResponseWriter, r *http.Request) {
	name := chi.URLParam(r, "name")
	src, err := s.opts.Admin.OpenBackup(r.Context(), name)
	if err != nil {
		s.p.mapError(w, r, err)
		return
	}
	defer src.File.Close()
	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("Content-Disposition", mime.FormatMediaType("attachment", map[string]string{"filename": name}))
	w.Header().Set("Cache-Control", "private, no-store")
	http.ServeContent(w, r, "", src.ModTime, src.File)
}

func toBackup(b model.Backup) gen.Backup {
	return gen.Backup{
		Name: b.Name, CreatedAt: b.CreatedAt.UTC(), SizeBytes: b.SizeBytes, IncludesSecrets: b.IncludesSecrets,
		Encrypted: b.Encrypted, Version: optString(b.Version), Kind: gen.BackupKind(b.Kind),
	}
}

func toRestoreResult(r model.RestoreResult) gen.RestoreResult {
	return gen.RestoreResult{RestartAt: r.RestartAt.UTC(), Version: r.Version, IncludesSecrets: r.IncludesSecrets}
}
