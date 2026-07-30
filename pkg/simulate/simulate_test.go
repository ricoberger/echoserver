package simulate

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// simulation is a helper type describing one of the exported simulation
// functions, so the shared "completes" and "context cancellation" behavior can
// be tested for all of them in a table driven way.
type simulation struct {
	name string
	run  func(ctx context.Context, duration time.Duration)
}

func simulations() []simulation {
	return []simulation{
		{name: "cpu", run: func(ctx context.Context, d time.Duration) { CPU(ctx, d) }},
		{name: "memory", run: func(ctx context.Context, d time.Duration) { Memory(ctx, d, 1024*1024) }},
		{name: "goroutines", run: func(ctx context.Context, d time.Duration) { Goroutines(ctx, d, 10) }},
		{name: "mutex", run: func(ctx context.Context, d time.Duration) { Mutex(ctx, d, 4) }},
		{name: "block", run: func(ctx context.Context, d time.Duration) { Block(ctx, d, 4) }},
	}
}

func TestSimulateCompletes(t *testing.T) {
	for _, s := range simulations() {
		t.Run("should complete for a short duration: "+s.name, func(t *testing.T) {
			done := make(chan struct{})
			go func() {
				s.run(context.Background(), 50*time.Millisecond)
				close(done)
			}()

			select {
			case <-done:
			case <-time.After(5 * time.Second):
				require.Fail(t, "simulation did not complete within the timeout")
			}
		})
	}
}

func TestSimulateRespectsContextCancellation(t *testing.T) {
	for _, s := range simulations() {
		t.Run("should return early when the context is cancelled: "+s.name, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			cancel()

			start := time.Now()
			done := make(chan struct{})
			go func() {
				// Pass a long duration to prove the early return is caused by
				// the cancelled context and not by the duration elapsing.
				s.run(ctx, time.Hour)
				close(done)
			}()

			select {
			case <-done:
				require.Less(t, time.Since(start), 5*time.Second)
			case <-time.After(5 * time.Second):
				require.Fail(t, "simulation did not return after context cancellation")
			}
		})
	}
}
