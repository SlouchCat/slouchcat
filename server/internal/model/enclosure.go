package model

import "strings"

// EnclosureContainer maps the MIME type of a podcast enclosure to a container name.
func EnclosureContainer(mime string) string {
	switch strings.ToLower(strings.TrimSpace(strings.Split(mime, ";")[0])) {
	case "audio/mpeg", "audio/mp3":
		return "mp3"
	case "audio/mp4", "audio/x-m4a", "audio/aac", "video/mp4":
		return "m4a"
	case "audio/ogg", "audio/opus":
		return "ogg"
	case "audio/flac", "audio/x-flac":
		return "flac"
	}
	return "unknown"
}

// EnclosureCodec is the audio codec an enclosure of the MIME type usually carries ("" when
// unknown): the playback of a feed episode and the stream overview name it (no probe).
func EnclosureCodec(mime string) string {
	switch EnclosureContainer(mime) {
	case "mp3":
		return "mp3"
	case "m4a":
		return "aac"
	case "flac":
		return "flac"
	}
	return ""
}
