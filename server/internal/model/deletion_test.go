package model

import "testing"

// Two deletes may hold the same title; it is free again only when both released it. A nil set
// holds nothing.
func TestDeletingSetCountsHolds(t *testing.T) {
	t.Parallel()
	d := NewDeletingSet()
	a := d.Hold([]string{"x", "y"})
	b := d.Hold([]string{"x"})
	a()
	if !d.Has("x") || d.Has("y") {
		t.Errorf("after the first release: x %v, y %v", d.Has("x"), d.Has("y"))
	}
	b()
	if d.Has("x") {
		t.Error("x is still held")
	}
	var none *DeletingSet
	none.Hold([]string{"x"})()
	if none.Has("x") {
		t.Error("a nil set holds something")
	}
}
