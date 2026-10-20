package transcode

import "testing"

// Dolby Vision by base-layer compatibility id: 2 (profiles 8.2, 9.2) is SDR, 1 and 4 are HDR10
// and HLG, profiles 5 and 7 are HDR whatever they signal.
func TestStreamIsHDRDolbyVision(t *testing.T) {
	for _, c := range []struct {
		s      Stream
		hdr    bool
		format string
	}{
		{Stream{DolbyVisionProfile: 8, DolbyVisionBLCompat: 1, ColorTransfer: "smpte2084"}, true, HDRDolbyVision},
		{Stream{DolbyVisionProfile: 8, DolbyVisionBLCompat: 2, ColorTransfer: "bt709"}, false, HDRNone},
		{Stream{DolbyVisionProfile: 8, DolbyVisionBLCompat: 2}, false, HDRNone},
		{Stream{DolbyVisionProfile: 9, DolbyVisionBLCompat: 2}, false, HDRNone},
		{Stream{DolbyVisionProfile: 8, DolbyVisionBLCompat: 4, ColorTransfer: "arib-std-b67"}, true, HDRDolbyVision},
		{Stream{DolbyVisionProfile: 5}, true, HDRDolbyVision},
		{Stream{DolbyVisionProfile: 7, DolbyVisionBLCompat: 2}, true, HDRDolbyVision},
		{Stream{ColorTransfer: "bt709", ColorPrimaries: "bt2020"}, false, HDRNone},
	} {
		if c.s.IsHDR() != c.hdr || c.s.HDRFormat() != c.format || isHDR(c.s) != c.hdr {
			t.Errorf("%+v: hdr %v format %q, want %v %q", c.s, c.s.IsHDR(), c.s.HDRFormat(), c.hdr, c.format)
		}
	}
}
