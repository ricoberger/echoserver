// Package simulate provides functions which generate different kinds of heavy
// work. They are intended to exercise the profile types collected by the
// application (CPU, memory, goroutines, mutex and block profiles), so the
// captured profiles can be tested.
//
// All functions are synchronous: they block until the simulated work is
// finished (bounded by the provided duration) and return early when the given
// context is cancelled. The package is intentionally free of any OpenTelemetry
// dependencies; tracing is owned by the callers (the HTTP and gRPC servers).
package simulate

import (
	"context"
	"runtime"
	"sync"
	"time"
)

// CPU keeps GOMAXPROCS goroutines busy with a compute loop for the given
// duration, generating load for the CPU profile.
func CPU(ctx context.Context, duration time.Duration) {
	ctx, cancel := context.WithTimeout(ctx, duration)
	defer cancel()

	var wg sync.WaitGroup
	for range runtime.GOMAXPROCS(0) {
		wg.Go(func() {
			// A tight loop which performs some arithmetic to burn CPU cycles.
			// The context is only checked every few thousand iterations to keep
			// the goroutine busy instead of parking on the channel.
			var x uint64
			for i := 0; ; i++ {
				x = x*1664525 + 1013904223
				if i%4096 == 0 {
					select {
					case <-ctx.Done():
						runtime.KeepAlive(x)
						return
					default:
					}
				}
			}
		})
	}

	wg.Wait()
}

// Memory allocates sizeBytes of memory, touches every page so it is actually
// committed, and retains it for the given duration, generating load for the
// memory (alloc and inuse) profiles.
func Memory(ctx context.Context, duration time.Duration, sizeBytes int) {
	if sizeBytes < 0 {
		sizeBytes = 0
	}

	buf := make([]byte, sizeBytes)
	// Touch every page (typically 4KiB) so the memory is committed and shows up
	// in the inuse profile instead of being lazily backed by zero pages.
	for i := 0; i < len(buf); i += 4096 {
		buf[i] = 1
	}

	select {
	case <-ctx.Done():
	case <-time.After(duration):
	}

	// Keep the allocation alive until the duration has elapsed.
	runtime.KeepAlive(buf)
}

// Goroutines spawns count goroutines which stay parked for the given duration,
// generating load for the goroutine profile.
func Goroutines(ctx context.Context, duration time.Duration, count int) {
	ctx, cancel := context.WithTimeout(ctx, duration)
	defer cancel()

	var wg sync.WaitGroup
	for range count {
		wg.Go(func() {
			<-ctx.Done()
		})
	}

	wg.Wait()
}

// Mutex runs workers goroutines which contend on a single mutex for the given
// duration, generating load for the mutex profile.
func Mutex(ctx context.Context, duration time.Duration, workers int) {
	ctx, cancel := context.WithTimeout(ctx, duration)
	defer cancel()

	var mu sync.Mutex
	var wg sync.WaitGroup
	for range workers {
		wg.Go(func() {
			for {
				select {
				case <-ctx.Done():
					return
				default:
				}

				mu.Lock()
				// Hold the lock briefly to create contention between the
				// competing goroutines.
				time.Sleep(time.Millisecond)
				mu.Unlock()
			}
		})
	}

	wg.Wait()
}

// Block runs workers goroutines which block on a channel receive for the given
// duration, generating load for the block profile. The goroutines are released
// once the duration has elapsed or the context is cancelled.
func Block(ctx context.Context, duration time.Duration, workers int) {
	release := make(chan struct{})

	var wg sync.WaitGroup
	for range workers {
		wg.Go(func() {
			<-release
		})
	}

	select {
	case <-ctx.Done():
	case <-time.After(duration):
	}

	close(release)
	wg.Wait()
}
