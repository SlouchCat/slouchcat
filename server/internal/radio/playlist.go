package radio

import (
	"fmt"
	"strings"
	"time"
)

// The live playlist (build plan S6), built by Go per request: target duration 4 s, the program
// date-time at the first segment and after every discontinuity, an EXT-X-MAP per generation,
// delta updates (CAN-SKIP-UNTIL, RFC 8216bis 6.2.5.1) and no blocking reload, so _HLS_msn and
// _HLS_part are never asked for. The token goes into the query (rt), never into the path.

const (
	targetDuration = 4
	canSkipUntil   = 36 * time.Second
)

// playlist renders the window for the listen token; skip answers a delta update that replaces
// the segments older than canSkipUntil before the end by EXT-X-SKIP.
func (r *relay) playlist(token string, skip bool) []byte {
	r.mu.Lock()
	defer r.mu.Unlock()
	segs := r.buf.segs
	var b strings.Builder
	b.WriteString("#EXTM3U\n#EXT-X-VERSION:9\n")
	fmt.Fprintf(&b, "#EXT-X-TARGETDURATION:%d\n", targetDuration)
	fmt.Fprintf(&b, "#EXT-X-SERVER-CONTROL:CAN-SKIP-UNTIL=%.1f\n", canSkipUntil.Seconds())
	fmt.Fprintf(&b, "#EXT-X-START:TIME-OFFSET=%d\n", startOffsetSeconds)
	if len(segs) == 0 {
		b.WriteString("#EXT-X-MEDIA-SEQUENCE:0\n#EXT-X-INDEPENDENT-SEGMENTS\n")
		return []byte(b.String())
	}
	fmt.Fprintf(&b, "#EXT-X-MEDIA-SEQUENCE:%d\n", segs[0].seq)
	fmt.Fprintf(&b, "#EXT-X-DISCONTINUITY-SEQUENCE:%d\n", r.buf.discDropped)
	b.WriteString("#EXT-X-INDEPENDENT-SEGMENTS\n")
	skipped := 0
	if skip {
		end := segs[len(segs)-1].pdt.Add(segs[len(segs)-1].dur)
		for skipped < len(segs)-1 && !segs[skipped].pdt.Add(segs[skipped].dur).After(end.Add(-canSkipUntil)) {
			skipped++
		}
		if skipped > 0 {
			fmt.Fprintf(&b, "#EXT-X-SKIP:SKIPPED-SEGMENTS=%d\n", skipped)
		}
	}
	for i, seg := range segs[skipped:] {
		first := i == 0
		if seg.disc && !first {
			b.WriteString("#EXT-X-DISCONTINUITY\n")
		}
		if first || seg.disc {
			fmt.Fprintf(&b, "#EXT-X-MAP:URI=\"init/%d?rt=%s\"\n", seg.gen, token)
			fmt.Fprintf(&b, "#EXT-X-PROGRAM-DATE-TIME:%s\n", seg.pdt.UTC().Format("2006-01-02T15:04:05.000Z07:00"))
		}
		fmt.Fprintf(&b, "#EXTINF:%.6f,\nseg/%d?rt=%s\n", seg.dur.Seconds(), seg.seq, token)
	}
	return []byte(b.String())
}
