package transcode

import (
	"fmt"
	"testing"
	"time"
)

func TestMediaPlaylist(t *testing.T) {
	p := Plan{Segments: []Segment{
		{Start: 0, Duration: sec(5)}, {Start: sec(5), Duration: sec(4.1)}, {Start: sec(9.1), Duration: 4*time.Second + 500*time.Millisecond},
	}}
	got := string(mediaPlaylist(p, "init.mp4?st=x", func(i int) string { return fmt.Sprintf("seg/%d?st=x", i) }))
	want := `#EXTM3U
#EXT-X-VERSION:7
#EXT-X-TARGETDURATION:5
#EXT-X-MEDIA-SEQUENCE:0
#EXT-X-PLAYLIST-TYPE:VOD
#EXT-X-INDEPENDENT-SEGMENTS
#EXT-X-MAP:URI="init.mp4?st=x"
#EXTINF:5.000000,
seg/0?st=x
#EXTINF:4.100000,
seg/1?st=x
#EXTINF:4.500000,
seg/2?st=x
#EXT-X-ENDLIST
`
	if got != want {
		t.Errorf("playlist:\n%s\nwant:\n%s", got, want)
	}
}

func TestMasterPlaylist(t *testing.T) {
	got := string(MasterPlaylist("media.m3u8?st=x", "avc1.640029,mp4a.40.2", 1920, 1080, 8160000))
	want := `#EXTM3U
#EXT-X-VERSION:7
#EXT-X-INDEPENDENT-SEGMENTS
#EXT-X-STREAM-INF:BANDWIDTH=8160000,CODECS="avc1.640029,mp4a.40.2",RESOLUTION=1920x1080
media.m3u8?st=x
`
	if got != want {
		t.Errorf("master:\n%s\nwant:\n%s", got, want)
	}
	got = string(MasterPlaylist("m.m3u8", "", 0, 0, 128000))
	want = "#EXTM3U\n#EXT-X-VERSION:7\n#EXT-X-INDEPENDENT-SEGMENTS\n#EXT-X-STREAM-INF:BANDWIDTH=128000\nm.m3u8\n"
	if got != want {
		t.Errorf("master without codecs:\n%s\nwant:\n%s", got, want)
	}
}
