package main

import (
	"reflect"
	"testing"
)

func TestParseOrigins(t *testing.T) {
	got, err := parseOrigins("https://Push.Example.Test, https://push-probe.example.test:8443/")
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"https://push.example.test", "https://push-probe.example.test:8443"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("parseOrigins = %v, want %v", got, want)
	}
	for _, bad := range []string{"", "push.example.test", "ftp://push.example.test", "https://push.example.test/v1"} {
		if _, err := parseOrigins(bad); err == nil {
			t.Errorf("parseOrigins(%q) accepted", bad)
		}
	}
}
