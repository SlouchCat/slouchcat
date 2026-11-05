package jobs

import (
	"context"
	"fmt"
	"testing"
	"time"
)

// BenchmarkClaimWithQueuedBacklog measures one claim while tens of thousands of probe jobs wait,
// the situation of the first scan of a large collection (plan M3 step 5). With the index
// jobs_claim of migration 00009 a claim reads the first matching row; before it, every candidate
// row was sorted (about 15 ms per claim with 50'000 queued jobs).
func BenchmarkClaimWithQueuedBacklog(b *testing.B) {
	q, c := newBenchQueue(b)
	ctx := context.Background()
	for i := range 50_000 {
		if _, err := q.Enqueue(ctx, NewJob{Type: "probe.file", Priority: 3, IdempotencyKey: fmt.Sprintf("probe:%d", i)}); err != nil {
			b.Fatal(err)
		}
	}
	c.Advance(time.Second)
	types := []string{"scan.library", "scan.path", "probe.file", "probe.keyframes", "library.sweep", "metadata.match", "image.fetch", "trailer.find"}
	b.ResetTimer()
	for range b.N {
		j, err := q.Claim(ctx, "w", types)
		if err != nil || j == nil {
			b.Fatal(err)
		}
		if err := q.Complete(ctx, "w", j.ID); err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkEnqueueOverridingPauseWithQueuedBacklog measures the insert of a playback request
// while tens of thousands of jobs wait: the cap of MaxPauseOverrides counts the queued and running
// rows with override_pause over the index jobs_claim (concept 14, 6.4).
func BenchmarkEnqueueOverridingPauseWithQueuedBacklog(b *testing.B) {
	q, _ := newBenchQueue(b)
	ctx := context.Background()
	for i := range 50_000 {
		if _, err := q.Enqueue(ctx, NewJob{Type: "probe.keyframes", Priority: 1, IdempotencyKey: fmt.Sprintf("keyframes:%d", i)}); err != nil {
			b.Fatal(err)
		}
	}
	b.ResetTimer()
	for i := range b.N {
		if _, err := q.EnqueueOverridingPause(ctx, NewJob{Type: "probe.keyframes", Priority: 9, IdempotencyKey: fmt.Sprintf("playback:%d", i)}); err != nil {
			b.Fatal(err)
		}
	}
}
