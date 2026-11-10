package audioseries

import (
	"strings"
	"testing"
)

// The initials of Marco's spellings (concept 19 D3) become «J. R. R.»; other words stay.
func TestFormatInitials(t *testing.T) {
	t.Parallel()
	for in, want := range map[string]string{
		"J R R Tolkien":      "J. R. R. Tolkien",
		"J.R.R. Tolkien":     "J. R. R. Tolkien",
		"J. R. R. Tolkien":   "J. R. R. Tolkien",
		"J.R.R Tolkien":      "J. R. R. Tolkien",
		"J K Rowling":        "J. K. Rowling",
		"J.K. Rowling":       "J. K. Rowling",
		"James S.A. Corey":   "James S. A. Corey",
		"Marc-Uwe Kling":     "Marc-Uwe Kling",
		"Marc Uwe  Kling":    "Marc Uwe Kling",
		"Malcolm X":          "Malcolm X",
		"Ortega y Gasset":    "Ortega y Gasset",
		"Dr. Seuss":          "Dr. Seuss",
		"Antoine St.Exupéry": "Antoine St.Exupéry",
		"":                   "",
	} {
		if got := FormatInitials(in); got != want {
			t.Errorf("FormatInitials(%q) = %q, want %q", in, got, want)
		}
	}
}

// SplitAuthors keeps the whole field (joined back it is the field) and its first name has the
// first-author key of Author.
func TestSplitAuthors(t *testing.T) {
	t.Parallel()
	for _, field := range []string{
		"J.R.R. Tolkien, Wolfgang Krege", "Marc–Uwe Kling & Jochen Malmsheimer", "A UND B", "A / B; C",
		" Chris Bradford ", "A,,B", "", "AC/DC",
	} {
		parts := SplitAuthors(field)
		var b strings.Builder
		for _, p := range parts {
			b.WriteString(p.Lead + p.Name + p.Trail + p.Sep)
		}
		if b.String() != field {
			t.Errorf("SplitAuthors(%q) joins to %q", field, b.String())
		}
		if got, want := AuthorKey(parts[0].Name), Author(field); got != want {
			t.Errorf("SplitAuthors(%q): first key %q, Author %q", field, got, want)
		}
	}
	if parts := SplitAuthors("A UND B"); len(parts) != 2 || parts[1].Name != "B" || parts[0].Sep != " UND " {
		t.Errorf("SplitAuthors(A UND B) = %+v", parts)
	}
}

func TestSwitchOn(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		value string
		found bool
		want  bool
	}{{"", false, true}, {"1", true, true}, {"0", true, false}, {"false", true, false}, {"true", true, true}} {
		if got := SwitchOn(c.value, c.found); got != c.want {
			t.Errorf("SwitchOn(%q, %v) = %v", c.value, c.found, got)
		}
	}
	if got := Default().Shown(false).Display("Bodyguard - Band"); got != "Bodyguard - Band" {
		t.Errorf("Shown(false).Display = %q, want the suffix kept", got)
	}
	if got := Default().Shown(true).Display("Bodyguard - Band"); got != "Bodyguard" {
		t.Errorf("Shown(true).Display = %q", got)
	}
}
