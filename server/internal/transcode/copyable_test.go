package transcode

import "testing"

func TestCopyableVideo(t *testing.T) {
	t.Parallel()
	for codec, want := range map[string]bool{
		"h264": true, "hevc": true, "av1": true, "vp9": true,
		"vc1": false, "mpeg2video": false, "mpeg4": false, "msmpeg4v3": false, "mjpeg": false,
		"H264": false, "h265": false, "": false,
	} {
		if got := CopyableVideo(codec); got != want {
			t.Errorf("CopyableVideo(%q) = %v, want %v", codec, got, want)
		}
	}
}
