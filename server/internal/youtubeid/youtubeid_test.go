package youtubeid

import "testing"

// The forms of design 4.2 and those metadata accepted before (M3 review, finding 12).
func TestParse(t *testing.T) {
	for in, want := range map[string]string{
		"dQw4w9WgXcQ": "dQw4w9WgXcQ",
		"https://www.youtube.com/watch?v=dQw4w9WgXcQ&t=10s": "dQw4w9WgXcQ",
		"youtube.com/watch?v=dQw4w9WgXcQ&list=x":            "dQw4w9WgXcQ",
		"http://youtube.com/watch?v=dQw4w9WgXcQ":            "dQw4w9WgXcQ",
		"https://m.youtube.com/watch?v=dQw4w9WgXcQ":         "dQw4w9WgXcQ",
		"https://music.youtube.com/watch?v=dQw4w9WgXcQ":     "dQw4w9WgXcQ",
		"https://youtu.be/dQw4w9WgXcQ?si=abc":               "dQw4w9WgXcQ",
		"https://www.youtube-nocookie.com/embed/x_y-z123":   "x_y-z123",
		"https://www.youtube.com/embed/dQw4w9WgXcQ":         "dQw4w9WgXcQ",
		"https://www.youtube.com/shorts/dQw4w9WgXcQ":        "dQw4w9WgXcQ",
		"https://www.youtube.com/live/dQw4w9WgXcQ":          "dQw4w9WgXcQ",
		"https://www.youtube.com/v/dQw4w9WgXcQ":             "dQw4w9WgXcQ",
		"  https://www.youtube.com/watch?v=dQw4w9WgXcQ  ":   "dQw4w9WgXcQ",
		"":                                       "",
		"short":                                  "",
		"https://vimeo.com/123456789":            "",
		"https://youtube.com/watch?v=short":      "",
		"https://www.youtube.com/watch?v=bad!id": "",
		"https://youtube.com/channel/dQw4w9WgXcQ":            "",
		"https://youtube.com/channel/UC123456789":            "",
		"https://evil.example/watch?v=dQw4w9WgXcQ":           "",
		"javascript:alert(1)":                                "",
		"ftp://youtube.com/watch?v=dQw4w9WgXcQ":              "",
		"https://www.youtube-nocookie.com/watch?v=dQw4w9WgX": "",
	} {
		got, ok := Parse(in)
		if got != want || ok != (want != "") {
			t.Errorf("Parse(%q) = %q, %v; want %q", in, got, ok, want)
		}
	}
}
