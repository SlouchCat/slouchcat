package model

import "testing"

func TestSortKey(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		lang, title, want string
	}{
		{"de", "Der Herr der Ringe", "herr der ringe"},
		{"de", "Die Ärzte", "arzte"},
		{"de", "Das Boot", "boot"},
		{"de", "Ein Fisch namens Wanda", "fisch namens wanda"},
		{"de", "Eine Hochzeit", "hochzeit"},
		{"de", "Einer flog über das Kuckucksnest", "einer flog uber das kuckucksnest"},
		{"de", "Die", "die"},
		{"de", "Chihiros Reise ins Zauberland", "chihiros reise ins zauberland"},
		{"de", "Straße der Besten", "strasse der besten"},
		{"en", "The Matrix", "matrix"},
		{"en", "A Bug's Life", "bug's life"},
		{"en", "An American Tail", "american tail"},
		{"en", "Amélie", "amelie"},
		{"en", "Das Boot", "das boot"},
		{"en", "  Spaced   Out  ", "spaced out"},
		{"fr", "Le Fabuleux Destin", "le fabuleux destin"},
		{"de", "Ødegaard", "odegaard"},
		{"de", "", ""},
	} {
		if got := SortKey(tc.lang, tc.title); got != tc.want {
			t.Errorf("SortKey(%q, %q) = %q, want %q", tc.lang, tc.title, got, tc.want)
		}
	}
}

func TestSortLetter(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct{ key, want string }{
		{"matrix", "M"}, {"Zebra", "Z"}, {"2001", "#"}, {"", "#"}, {"ärzte", "#"}, {"東京", "#"}, {"#hashtag", "#"},
	} {
		if got := SortLetter(tc.key); got != tc.want {
			t.Errorf("SortLetter(%q) = %q, want %q", tc.key, got, tc.want)
		}
	}
}
