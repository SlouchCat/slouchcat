package limit

import (
	"testing"
	"time"
)

type clock struct{ t time.Time }

func (c *clock) now() time.Time { return c.t }

func TestBurstAndRefill(t *testing.T) {
	c := &clock{t: time.Unix(1000, 0)}
	l := New(1, 3, 10)
	l.SetClock(c.now)
	for i := range 3 {
		if !l.Allow("a") {
			t.Fatalf("request %d of the burst refused", i+1)
		}
	}
	if l.Allow("a") {
		t.Fatal("fourth request allowed past the burst")
	}
	if !l.Allow("b") {
		t.Fatal("another key shares the bucket")
	}
	c.t = c.t.Add(1500 * time.Millisecond)
	if !l.Allow("a") {
		t.Fatal("no token after 1.5 s at 1/s")
	}
	if l.Allow("a") {
		t.Fatal("two tokens after 1.5 s at 1/s")
	}
}

func TestMaxKeys(t *testing.T) {
	c := &clock{t: time.Unix(1000, 0)}
	l := New(1, 2, 2)
	l.SetClock(c.now)
	l.Allow("a")
	l.Allow("b")
	if l.Allow("c") {
		t.Fatal("a third key was admitted to a full table of busy buckets")
	}
	c.t = c.t.Add(time.Minute) // a and b are full again and can go
	if !l.Allow("c") {
		t.Fatal("a new key was refused although the table held only idle buckets")
	}
	l.Sweep()
	if n := l.Len(); n != 1 {
		t.Fatalf("Len after sweep = %d, want 1", n)
	}
}
