package playback

import (
	"testing"

	"github.com/slouchcat/slouchcat/server/internal/transcode"
)

// MaxTranscodeHeight follows the backend (ADR 0021): 1080 on software (AN-17), 2160 on hardware.
func TestMaxTranscodeHeight(t *testing.T) {
	for b, want := range map[transcode.Backend]int{
		transcode.BackendSoftware: 1080, "": 1080,
		transcode.BackendNVENC: 2160, transcode.BackendQSV: 2160, transcode.BackendVAAPI: 2160,
	} {
		if got := MaxTranscodeHeight(b); got != want {
			t.Errorf("MaxTranscodeHeight(%q) = %d, want %d", b, got, want)
		}
	}
}
