package main

import "testing"

func TestParsePairCode(t *testing.T) {
	id := "AAECAwQFBgcICQoLDA0ODw"
	key := "QCYdnDrDMuTaRSfMmHi3L0lWboCDrMRS8Qe0vcuBxnk"
	got, err := parsePairCode("http://10.0.2.2:8180/tv#v=1&i=" + id + "&k=" + key)
	if err != nil {
		t.Fatal(err)
	}
	if got.relay != "http://10.0.2.2:8180" || got.id != id || got.tvKey != key {
		t.Fatalf("parsed %+v", got)
	}
	for _, bad := range []string{
		"https://pair.slouchcat.com/tv",
		"https://pair.slouchcat.com/tv#v=2&i=" + id + "&k=" + key,
		"https://pair.slouchcat.com/help#v=1&i=" + id + "&k=" + key,
		"ftp://pair.slouchcat.com/tv#v=1&i=" + id + "&k=" + key,
		"https://pair.slouchcat.com/tv#v=1&i=short&k=" + key,
		"https://pair.slouchcat.com/tv#v=1&i=" + id + "&k=short",
	} {
		if _, err := parsePairCode(bad); err == nil {
			t.Errorf("%q read as a code", bad)
		}
	}
}
