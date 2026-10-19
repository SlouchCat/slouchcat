package transcode

import (
	"bytes"
	"fmt"
	"math"
	"strconv"
)

// mediaPlaylist renders the complete VOD playlist of a plan (fMP4, RFC 8216 version 7).
func mediaPlaylist(p Plan, initURL string, segmentURL func(i int) string) []byte {
	target := 1
	for _, s := range p.Segments {
		target = max(target, int(math.Ceil(s.Duration.Seconds())))
	}
	var b bytes.Buffer
	b.WriteString("#EXTM3U\n#EXT-X-VERSION:7\n")
	fmt.Fprintf(&b, "#EXT-X-TARGETDURATION:%d\n", target)
	b.WriteString("#EXT-X-MEDIA-SEQUENCE:0\n#EXT-X-PLAYLIST-TYPE:VOD\n#EXT-X-INDEPENDENT-SEGMENTS\n")
	fmt.Fprintf(&b, "#EXT-X-MAP:URI=%q\n", initURL)
	for i, s := range p.Segments {
		fmt.Fprintf(&b, "#EXTINF:%s,\n%s\n", strconv.FormatFloat(s.Duration.Seconds(), 'f', 6, 64), segmentURL(i))
	}
	b.WriteString("#EXT-X-ENDLIST\n")
	return b.Bytes()
}

// MasterPlaylist renders a master playlist with one variant. codecs is the RFC 6381 list
// (CodecsString); width and height are left out when unknown.
func MasterPlaylist(mediaURL string, codecs string, width, height int, bandwidth int) []byte {
	var b bytes.Buffer
	b.WriteString("#EXTM3U\n#EXT-X-VERSION:7\n#EXT-X-INDEPENDENT-SEGMENTS\n")
	fmt.Fprintf(&b, "#EXT-X-STREAM-INF:BANDWIDTH=%d", bandwidth)
	if codecs != "" {
		fmt.Fprintf(&b, ",CODECS=%q", codecs)
	}
	if width > 0 && height > 0 {
		fmt.Fprintf(&b, ",RESOLUTION=%dx%d", width, height)
	}
	fmt.Fprintf(&b, "\n%s\n", mediaURL)
	return b.Bytes()
}
