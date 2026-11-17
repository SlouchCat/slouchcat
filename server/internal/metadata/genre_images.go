package metadata

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"image"
	"os"

	"github.com/slouchcat/slouchcat/server/internal/model"
	"github.com/slouchcat/slouchcat/server/internal/paths"
)

// The backdrops of the genre tiles of «Entdecken» (concept 25 stage 5, build step 5.4): Seerr names
// a TMDB backdrop per genre; Slouchcat fetches it once in w780 into the cache and serves it from
// there, so the client never talks to TMDB (appendix F 2.12). The caller takes the TMDB path from
// its own cached genre list, never from a request.

const (
	genreImagesDir  = externalTitlesDir + "/genres"
	genreImageWidth = "w780"
)

// OpenGenreBackdrop opens the cached w780 JPEG of the TMDB backdrop src, fetching it on its first
// request. model.ErrNotFound for a path that is not a plain TMDB image path and for an image TMDB
// refuses or that is no JPEG; model.ErrTMDBUnreachable when the download failed for now.
func (s *Service) OpenGenreBackdrop(ctx context.Context, src string) (model.StreamSource, error) {
	if !tmdbImagePath(src) {
		return model.StreamSource{}, model.ErrNotFound
	}
	if s.cacheE != nil {
		return model.StreamSource{}, s.cacheE
	}
	root, err := paths.Open(s.cache)
	if err != nil {
		return model.StreamSource{}, fmt.Errorf("open cache: %w", err)
	}
	defer root.Close()
	sum := sha256.Sum256([]byte(genreImageWidth + src))
	rel := genreImagesDir + "/" + hex.EncodeToString(sum[:12]) + ".jpg"
	if _, err := root.Stat(rel); errors.Is(err, os.ErrNotExist) {
		if err := s.fetchGenreBackdrop(ctx, root, rel, src); err != nil {
			return model.StreamSource{}, err
		}
	}
	f, err := root.Open(rel)
	if err != nil {
		return model.StreamSource{}, fmt.Errorf("open genre backdrop: %w", err)
	}
	info, err := f.Stat()
	if err != nil {
		_ = f.Close()
		return model.StreamSource{}, fmt.Errorf("stat genre backdrop: %w", err)
	}
	return model.StreamSource{File: f, MediaType: "image/jpeg", Size: info.Size(), ModTime: info.ModTime(), Name: rel,
		ETag: `"` + hex.EncodeToString(sum[:12]) + `"`}, nil
}

// fetchGenreBackdrop downloads the image into rel; one caller per image at a time.
func (s *Service) fetchGenreBackdrop(ctx context.Context, root *os.Root, rel, src string) error {
	release, err := s.lockItemImages(ctx, rel)
	if err != nil {
		return err
	}
	defer release()
	if _, err := root.Stat(rel); err == nil {
		return nil // another request stored it meanwhile
	}
	missKey := "genre:" + src
	if s.externalMissed(missKey) {
		return model.ErrNotFound
	}
	data, _, err := s.client.DownloadCDNImage(ctx, genreImageWidth, src)
	if err != nil {
		if ctxErr := ctx.Err(); ctxErr != nil {
			return ctxErr
		}
		if errors.Is(err, model.ErrTMDBUnreachable) {
			return err
		}
		s.externalMiss(missKey)
		return model.ErrNotFound
	}
	if _, format, err := image.DecodeConfig(bytes.NewReader(data)); err != nil || format != "jpeg" {
		s.externalMiss(missKey)
		return model.ErrNotFound
	}
	if err := root.MkdirAll(genreImagesDir, 0o750); err != nil {
		return fmt.Errorf("create genre image dir: %w", err)
	}
	return writeFile(root, genreImagesDir, rel, data)
}
