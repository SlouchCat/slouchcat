package api

import (
	"net/http"
	"os"
	"path/filepath"

	"github.com/go-chi/chi/v5"
)

// spikeFiles maps the contract variants to the file names produced by testdata/generate-spike-media.sh.
var spikeFiles = map[string]string{
	"fmp4":      "spike-fmp4.mp4",
	"faststart": "spike-faststart.mp4",
}

// getSpikeFile serves one of the two 1.5 GB spike files with Range support (plan 10.4). The
// directory is fixed configuration (SLOUCHCAT_SPIKE_DIR), the file name comes from a fixed table,
// so no user data reaches the file system (CLAUDE.md rule 4). Temporary until the M1b decision.
func (s *server) getSpikeFile(w http.ResponseWriter, r *http.Request) {
	name, ok := spikeFiles[chi.URLParam(r, "variant")]
	if s.opts.SpikeDir == "" || !ok {
		s.p.write(w, r, http.StatusNotFound, "not_found", "The offline spike is not enabled on this server.")
		return
	}
	f, err := os.Open(filepath.Join(s.opts.SpikeDir, name))
	if err != nil {
		s.p.write(w, r, http.StatusNotFound, "not_found", "Spike file missing; run testdata/generate-spike-media.sh.")
		return
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil {
		s.p.internal(w, r, err)
		return
	}
	w.Header().Set("Content-Type", "video/mp4")
	w.Header().Set("Accept-Ranges", "bytes")
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Access-Control-Allow-Origin", "*") // the spike PWA may run on another origin
	http.ServeContent(w, r, name, info.ModTime(), f)
}
