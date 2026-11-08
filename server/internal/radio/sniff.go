package radio

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"time"

	"github.com/slouchcat/slouchcat/server/internal/model"
	"github.com/slouchcat/slouchcat/server/internal/transcode"
)

// Content sniffing of a stream's first bytes (concept 16 section 7.8): the content type of a
// catalog stream is often missing or wrong, so the reader decides by content.

// sniffKind is what the first bytes of a response are.
type sniffKind int

const (
	kindAudio    sniffKind = iota + 1 // audio the packager reads (RadioInput says which)
	kindHLS                           // an HLS playlist (#EXTM3U with #EXT-X- tags)
	kindPlaylist                      // a .pls or .m3u playlist naming the stream
)

// sniffMinBytes is how much the reader collects before it sniffs (or the whole response, when
// shorter).
const sniffMinBytes = 16 << 10

var utf8BOM = []byte{0xEF, 0xBB, 0xBF}

// frameHeader is one parsed MP3 or ADTS frame header.
type frameHeader struct {
	codec       string // "mp3" or "aac"
	version     int    // MP3: 1, 2 or 25 (2.5); ADTS: the audio object type (profile + 1)
	sampleRate  int
	channels    int
	bitrateKbps int // MP3 only
	length      int // frame length in bytes, header included
	samples     int // audio samples per channel the frame decodes to
}

// duration is how long the audio of the frame plays (concept 16 section 7.6: the relay stamps the
// titles with the audio position it counts from these).
func (h frameHeader) duration() time.Duration {
	if h.sampleRate <= 0 {
		return 0
	}
	return time.Duration(int64(h.samples) * int64(time.Second) / int64(h.sampleRate))
}

// sameStream reports whether h and o come from the same encoder settings: codec, version or
// object type, sample rate and channels (the bitrate of VBR streams varies).
func (h frameHeader) sameStream(o frameHeader) bool {
	return h.codec == o.codec && h.version == o.version && h.sampleRate == o.sampleRate && h.channels == o.channels
}

var (
	mp3Bitrates1 = [16]int{0, 32, 40, 48, 56, 64, 80, 96, 112, 128, 160, 192, 224, 256, 320, 0}
	mp3Bitrates2 = [16]int{0, 8, 16, 24, 32, 40, 48, 56, 64, 80, 96, 112, 128, 144, 160, 0}
	mp3Rates     = map[int][3]int{1: {44100, 48000, 32000}, 2: {22050, 24000, 16000}, 25: {11025, 12000, 8000}}
	adtsRates    = [13]int{96000, 88200, 64000, 48000, 44100, 32000, 24000, 22050, 16000, 12000, 11025, 8000, 7350}
)

// parseMP3Header parses an MPEG audio layer III frame header at b[0:4].
func parseMP3Header(b []byte) (frameHeader, bool) {
	if len(b) < 4 || b[0] != 0xFF || b[1]&0xE0 != 0xE0 {
		return frameHeader{}, false
	}
	var version int
	switch (b[1] >> 3) & 3 {
	case 0:
		version = 25
	case 2:
		version = 2
	case 3:
		version = 1
	default:
		return frameHeader{}, false
	}
	if (b[1]>>1)&3 != 1 { // layer III
		return frameHeader{}, false
	}
	bi, si := b[2]>>4, (b[2]>>2)&3
	if si == 3 {
		return frameHeader{}, false
	}
	br := mp3Bitrates1[bi]
	if version != 1 {
		br = mp3Bitrates2[bi]
	}
	if br == 0 {
		return frameHeader{}, false
	}
	rate := mp3Rates[version][si]
	pad := int(b[2]>>1) & 1
	coef := 144
	if version != 1 {
		coef = 72
	}
	channels := 2
	if b[3]>>6 == 3 {
		channels = 1
	}
	samples := 1152 // MPEG-1 Layer III; MPEG-2 and 2.5 Layer III frames hold half as many
	if version != 1 {
		samples = 576
	}
	return frameHeader{codec: "mp3", version: version, sampleRate: rate, channels: channels, bitrateKbps: br,
		length: coef*br*1000/rate + pad, samples: samples}, true
}

// parseADTSHeader parses an ADTS frame header at b[0:7].
func parseADTSHeader(b []byte) (frameHeader, bool) {
	if len(b) < 7 || b[0] != 0xFF || b[1]&0xF6 != 0xF0 {
		return frameHeader{}, false
	}
	si := (b[2] >> 2) & 0xF
	if int(si) >= len(adtsRates) {
		return frameHeader{}, false
	}
	hdr := 7
	if b[1]&1 == 0 {
		hdr = 9 // CRC
	}
	length := int(b[3]&3)<<11 | int(b[4])<<3 | int(b[5]>>5)
	if length <= hdr {
		return frameHeader{}, false
	}
	channels := int(b[2]&1)<<2 | int(b[3]>>6)
	blocks := int(b[6]&3) + 1 // raw data blocks of 1024 samples each
	return frameHeader{codec: "aac", version: int(b[2]>>6) + 1, sampleRate: adtsRates[si], channels: channels,
		length: length, samples: 1024 * blocks}, true
}

// parseFrameHeader parses a header of codec ("mp3" or "aac") at b.
func parseFrameHeader(codec string, b []byte) (frameHeader, bool) {
	if codec == "aac" {
		return parseADTSHeader(b)
	}
	return parseMP3Header(b)
}

// syncAt reports whether b holds a valid frame header of codec at i and another one right after
// that frame: the two-header rule against a false sync inside data. need is true when b is too
// short to decide.
func syncAt(codec string, b []byte, i int) (h frameHeader, ok, need bool) {
	h, ok = parseFrameHeader(codec, b[i:])
	if !ok {
		return frameHeader{}, false, len(b)-i < 9
	}
	next := i + h.length
	if len(b) < next+9 {
		return h, false, true
	}
	h2, ok := parseFrameHeader(codec, b[next:])
	return h, ok && h2.sameStream(h), false
}

// id3Length returns the length of the ID3v2 tag at the start of b (header, syncsafe size and an
// optional footer), 0 when b starts with none, -1 when b is too short to tell.
func id3Length(b []byte) int {
	if len(b) < 10 {
		if bytes.HasPrefix([]byte("ID3"), b[:min(len(b), 3)]) {
			return -1
		}
		return 0
	}
	if string(b[:3]) != "ID3" || b[3] == 0xFF || b[4] == 0xFF || (b[6]|b[7]|b[8]|b[9])&0x80 != 0 {
		return 0
	}
	n := 10 + (int(b[6])<<21 | int(b[7])<<14 | int(b[8])<<7 | int(b[9]))
	if b[5]&0x10 != 0 {
		n += 10
	}
	return n
}

// sniff decides what b, the first bytes of a response (ICY metadata already removed), is. For
// audio it returns the packager input with what the frame or stream headers tell; anything
// unknown is model.ErrRadioNotAudio.
func sniff(b []byte) (transcode.RadioInput, sniffKind, error) {
	text := bytes.TrimLeft(bytes.TrimPrefix(b, utf8BOM), " \t\r\n")
	switch {
	case bytes.HasPrefix(text, []byte("#EXTM3U")):
		if bytes.Contains(text, []byte("#EXT-X-")) {
			return transcode.RadioInput{}, kindHLS, nil
		}
		return transcode.RadioInput{}, kindPlaylist, nil
	case len(text) >= 10 && bytes.EqualFold(text[:10], []byte("[playlist]")):
		return transcode.RadioInput{}, kindPlaylist, nil
	}
	if n := id3Length(b); n > 0 && n <= len(b) {
		b = b[n:]
	}
	switch {
	case bytes.HasPrefix(b, []byte("OggS")):
		in, err := sniffOgg(b)
		return in, kindAudio, err
	case bytes.HasPrefix(b, []byte("fLaC")):
		in := transcode.RadioInput{Format: "flac", Codec: "flac"}
		if len(b) >= 8+18 { // STREAMINFO: 20 bits sample rate, 3 bits channels - 1
			v := binary.BigEndian.Uint32(b[8+10:])
			in.SampleRate, in.Channels = int(v>>12), int((v>>9)&7)+1
		}
		return in, kindAudio, nil
	case len(b) >= 8 && isBoxType(string(b[4:8])):
		return transcode.RadioInput{Format: "mp4", Codec: "aac"}, kindAudio, nil
	case len(b) > 376 && b[0] == 0x47 && b[188] == 0x47 && b[376] == 0x47:
		in, err := sniffTS(b)
		return in, kindAudio, err
	}
	for i := 0; i+4 <= len(b); i++ {
		if b[i] != 0xFF {
			continue
		}
		for _, codec := range []string{"aac", "mp3"} {
			if h, ok, _ := syncAt(codec, b, i); ok {
				in := transcode.RadioInput{Format: "mp3", Codec: "mp3", BitrateKbps: h.bitrateKbps,
					Channels: h.channels, SampleRate: h.sampleRate}
				if codec == "aac" {
					in = transcode.RadioInput{Format: "aac", Codec: "aac", Channels: h.channels, SampleRate: h.sampleRate}
				}
				return in, kindAudio, nil
			}
		}
	}
	return transcode.RadioInput{}, 0, model.ErrRadioNotAudio
}

// sniffOgg reads the codec of the first logical stream from its first packet.
func sniffOgg(b []byte) (transcode.RadioInput, error) {
	if len(b) < 27 {
		return transcode.RadioInput{}, fmt.Errorf("%w: short ogg page", model.ErrRadioNotAudio)
	}
	start := 27 + int(b[26])
	if len(b) < start {
		return transcode.RadioInput{}, fmt.Errorf("%w: short ogg page", model.ErrRadioNotAudio)
	}
	p := b[start:]
	switch {
	case len(p) >= 24 && string(p[:7]) == "\x01vorbis": // version, channels, rate, max, nominal bitrate
		return transcode.RadioInput{Format: "ogg", Codec: "vorbis", Channels: int(p[11]),
			SampleRate:  int(binary.LittleEndian.Uint32(p[12:])),
			BitrateKbps: max(int(int32(binary.LittleEndian.Uint32(p[20:])))/1000, 0)}, nil //nolint:gosec // G115: the nominal bitrate is a signed 32-bit field
	case len(p) >= 19 && string(p[:8]) == "OpusHead":
		return transcode.RadioInput{Format: "ogg", Codec: "opus", Channels: int(p[9]), SampleRate: 48000}, nil
	case len(p) >= 5 && string(p[:5]) == "\x7fFLAC":
		return transcode.RadioInput{Format: "ogg", Codec: "flac"}, nil
	}
	return transcode.RadioInput{}, fmt.Errorf("%w: unknown ogg codec", model.ErrRadioNotAudio)
}

// TS stream types of the audio the packager reads (ISO 13818-1 table 2-34).
const (
	tsStreamMPEG1Audio = 0x03
	tsStreamMPEG2Audio = 0x04
	tsStreamADTS       = 0x0F
)

// sniffTS finds the first audio stream in the program map of an MPEG-TS start.
func sniffTS(b []byte) (transcode.RadioInput, error) {
	pmtPID := -1
	for off := 0; off+188 <= len(b); off += 188 {
		pkt := b[off : off+188]
		if pkt[0] != 0x47 {
			break
		}
		pid := int(pkt[1]&0x1F)<<8 | int(pkt[2])
		payload := tsPayload(pkt)
		if payload == nil || pkt[1]&0x40 == 0 { // only packets that start a section
			continue
		}
		if len(payload) < 1 || int(payload[0])+1 > len(payload) {
			continue
		}
		sec := payload[1+int(payload[0]):] // pointer field
		if len(sec) < 12 {
			continue
		}
		secLen := int(sec[1]&0x0F)<<8 | int(sec[2])
		end := min(len(sec), 3+secLen-4) // without CRC
		switch {
		case pid == 0 && sec[0] == 0x00:
			for i := 8; i+4 <= end; i += 4 {
				if prog := int(sec[i])<<8 | int(sec[i+1]); prog != 0 {
					pmtPID = int(sec[i+2]&0x1F)<<8 | int(sec[i+3])
					break
				}
			}
		case pid == pmtPID && sec[0] == 0x02:
			i := 12 + (int(sec[10]&0x0F)<<8 | int(sec[11]))
			for i+5 <= end {
				switch sec[i] {
				case tsStreamADTS:
					return transcode.RadioInput{Format: "mpegts", Codec: "aac"}, nil
				case tsStreamMPEG1Audio, tsStreamMPEG2Audio:
					return transcode.RadioInput{Format: "mpegts", Codec: "mp3"}, nil
				}
				i += 5 + (int(sec[i+3]&0x0F)<<8 | int(sec[i+4]))
			}
			return transcode.RadioInput{}, fmt.Errorf("%w: no audio stream in the transport stream", model.ErrRadioNotAudio)
		}
	}
	return transcode.RadioInput{}, fmt.Errorf("%w: no program map in the transport stream", model.ErrRadioNotAudio)
}

// tsPayload returns the payload of a TS packet, nil when it has none.
func tsPayload(pkt []byte) []byte {
	afc := (pkt[3] >> 4) & 3
	switch afc {
	case 1:
		return pkt[4:]
	case 3:
		if n := 5 + int(pkt[4]); n < len(pkt) {
			return pkt[n:]
		}
	}
	return nil
}
