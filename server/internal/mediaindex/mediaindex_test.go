package mediaindex

import (
	"math"
	"strings"
	"testing"
	"time"
)

func TestCheck(t *testing.T) {
	for _, tc := range []struct {
		name     string
		kf       []time.Duration
		duration time.Duration
		wantErr  string // substring; empty for a usable index
	}{
		{"H264 MKV Long", ms(longCues...), 90 * time.Second, ""},
		{"last on the duration plus slack", ms(0, 10_000, 21_000), 20 * time.Second, ""},
		{"end gap of 60 s", ms(0, 30_000, 60_000), 120 * time.Second, ""},
		{"first at 5 s", ms(5000, 10_000), 20 * time.Second, ""},
		{"duration unknown", ms(longCues...), 0, "duration unknown"},
		{"one entry", ms(0), 10 * time.Second, "at least 2"},
		{"fewer than one per minute", ms(0, 30_000, 60_000), 3*time.Minute + time.Second, "one per minute"},
		{"negative", []time.Duration{-time.Millisecond, time.Second}, 10 * time.Second, "negative"},
		{"first after 5 s", ms(5001, 10_000), 20 * time.Second, "first keyframe"},
		{"inner gap over 30 s", ms(0, 30_001, 40_000), 50 * time.Second, "gap of"},
		{"unsorted", ms(0, 2000, 1000), 3 * time.Second, "not after"},
		{"duplicate", ms(0, 1000, 1000), 3 * time.Second, "not after"},
		{"last behind the duration", ms(0, 10_000, 21_001), 20 * time.Second, "behind the duration"},
		{"end gap over 60 s", ms(0, 10_000, 20_000, 30_000), 90*time.Second + time.Millisecond, "end gap"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			err := Check(Index{Keyframes: tc.kf}, tc.duration)
			if (tc.wantErr == "" && err != nil) || (tc.wantErr != "" && (err == nil || !strings.Contains(err.Error(), tc.wantErr))) {
				t.Fatalf("Check: %v, want %q", err, tc.wantErr)
			}
		})
	}
}

func TestToDuration(t *testing.T) {
	for _, tc := range []struct {
		x     int64
		scale uint64
		want  time.Duration
		ok    bool
	}{
		{2560, 12800, 200 * time.Millisecond, true},
		{-512, 12800, -40 * time.Millisecond, true},
		{1, 3, 333_333_333, true},
		{9_223_372_036, 1, 9_223_372_036 * time.Second, true}, // the largest whole second
		{92_233_720_369, 10, 0, false},                        // 9223372036.9 s: the remainder overflows
		{math.MaxInt64, 1, 0, false},
		{math.MinInt64, 1 << 32, 0, false},
		{1, 0, 0, false},
	} {
		got, ok := toDuration(tc.x, tc.scale)
		if got != tc.want || ok != tc.ok {
			t.Fatalf("toDuration(%d, %d) = %v, %v, want %v, %v", tc.x, tc.scale, got, ok, tc.want, tc.ok)
		}
	}
}
