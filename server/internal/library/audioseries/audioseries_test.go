package audioseries

import "testing"

// The spellings of Marco's folder (concept 19 D3): series names with and without suffix and
// authors with initials and hyphens give one key; another author and the article rule as D4.1
// says.
func TestKey(t *testing.T) {
	t.Parallel()
	n := Default()
	same := [][2][2]string{
		{{"Bodyguard", "Chris Bradford"}, {"Bodyguard - Band", "Chris Bradford"}},
		{{"Bodyguard", "Chris Bradford"}, {"bodyguard  -Reihe", "chris bradford"}},
		{{"Harry Potter", "J.K. Rowling"}, {"Harry Potter - Gesprochen von Rufus Beck", "J K Rowling"}},
		{{"Herr der Ringe", "J.R.R. Tolkien"}, {"Der Herr Der Ringe - Band", "J R R Tolkien"}},
		{{"Herr der Ringe", "J. R. R. Tolkien"}, {"Herr der Ringe", "J.R.R. Tolkien, Wolfgang Krege"}},
		{{"Känguru", "Marc-Uwe Kling"}, {"Känguru [German Edition]", "Marc Uwe Kling"}},
		{{"Känguru", "Marc-Uwe Kling"}, {"Känguru", "Marc–Uwe Kling & Jochen Malmsheimer"}},
		{{"The Expanse", "James S. A. Corey"}, {"Expanse", "James S.A. Corey"}},
		// A decomposed umlaut (u + U+0308, as macOS writes file names) is the composed one.
		{{"Känguru gekürzt", "Jörg"}, {"Ka\u0308nguru Geku\u0308rzt", "Jo\u0308rg"}},
	}
	for _, c := range same {
		a, b := n.Key(c[0][0], c[0][1]), n.Key(c[1][0], c[1][1])
		if a == "" || a != b {
			t.Errorf("Key(%q, %q) = %q, Key(%q, %q) = %q, want one key", c[0][0], c[0][1], a, c[1][0], c[1][1], b)
		}
	}
	apart := [][2][2]string{
		{{"Bodyguard", "Chris Bradford"}, {"Bodyguard", "Andere Autorin"}},
		{{"Die Drei", "X"}, {"Drei Fragezeichen", "X"}},
		{{"Theater", "X"}, {"Ater", "X"}}, // «the» only as a whole word
	}
	for _, c := range apart {
		if a, b := n.Key(c[0][0], c[0][1]), n.Key(c[1][0], c[1][1]); a == b {
			t.Errorf("Key(%q, %q) = Key(%q, %q) = %q, want two keys", c[0][0], c[0][1], c[1][0], c[1][1], a)
		}
	}
	for in, want := range map[string]string{
		"Die":            "die", // nothing but an article keeps it
		"Band":           "band",
		"Reihe - Band":   "reihe",
		"  Der  Hobbit ": "hobbit",
		"Les Misérables": "misérables",
		"Ein Teil":       "ein",
		"":               "",
	} {
		if got := n.Name(in); got != want {
			t.Errorf("Name(%q) = %q, want %q", in, got, want)
		}
	}
	if n.Key("", "Chris Bradford") != "" || n.Key(" - ", "x") != "" {
		t.Error("a book without series name has a key")
	}
}

func TestDisplay(t *testing.T) {
	t.Parallel()
	n := Default()
	for in, want := range map[string]string{
		"Bodyguard - Band": "Bodyguard",
		"Bodyguard":        "Bodyguard",
		"Harry Potter - Gesprochen von Rufus Beck": "Harry Potter",
		"Der Herr Der Ringe - Band":                "Der Herr Der Ringe",
		"Känguru [German Edition]":                 "Känguru",
		"Warrior Cats-Reihe":                       "Warrior Cats",
		"  Band  ":                                 "Band",
		"Die  Drei ???":                            "Die Drei ???",
	} {
		if got := n.Display(in); got != want {
			t.Errorf("Display(%q) = %q, want %q", in, got, want)
		}
	}
	if got := n.Display("Ka\u0308nguru - Geku\u0308rzt"); got != "Känguru" {
		t.Errorf("Display of a decomposed name = %q, want the composed Känguru", got)
	}
	if a, b := Parse("band\n"), Parse("band\nteil\n"); a.Version() == b.Version() || a.Version() != Parse("band\n").Version() || n.Version() == "" {
		t.Errorf("versions %q %q %q", a.Version(), b.Version(), n.Version())
	}
	if got := (Names{}).Display("Bodyguard - Band"); got != "Bodyguard - Band" {
		t.Errorf("without suffixes Display = %q", got)
	}
	if got := Parse("# c\n\nband\n").Name("Bodyguard Reihe Band"); got != "bodyguard reihe" {
		t.Errorf("parsed list: %q", got)
	}
}

func TestMostCommon(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		in   []string
		want string
	}{
		{nil, ""},
		{[]string{"", ""}, ""},
		{[]string{"b", "a"}, "a"},
		{[]string{"b", "a", "b"}, "b"},
		{[]string{"", "", "x"}, "x"},
	} {
		if got := MostCommon(c.in); got != c.want {
			t.Errorf("MostCommon(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}
