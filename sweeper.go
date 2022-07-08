package flow

import (
	"context"
	"math"
	"sync"
	"sync/atomic"
	"time"

	"github.com/benbjohnson/clock"
)

// IdleRate the rate at which we declare a meter idle (and stop tracking it
// until it's re-registered).
//
// The default ensures that 1 event every ~30s will keep the meter from going
// idle.
var IdleRate = 1e-13

// Alpha for EWMA of 1s
var alpha = 1 - math.Exp(-1.0)

// The global sweeper.
var globalSweeper sweeper

// SetClock puts a global clock in place for testing purposes only.
// It should be called only once and before using any of the Meter or
// MeterRegistry APIs of this package and should be restored with
// RestoreClock after the test.
func SetClock(clock clock.Clock) {
	globalSweeper.stop()
	globalSweeper = sweeper{}
	globalSweeper.clock = clock
}

func RestoreClock() {
	globalSweeper.stop()
	globalSweeper = sweeper{}
}

type sweeper struct {
	sweepOnce sync.Once
	ctx       context.Context
	cancel    context.CancelFunc
	stopped   chan struct{}

	snapshotMu   sync.RWMutex
	meters       []*Meter
	activeMeters int

	lastUpdateTime time.Time
	// Consumed each time a Meter has been updated (`Mark`ed).
	registerChannel chan *Meter

	clock clock.Clock
}

func (sw *sweeper) start() {
	sw.registerChannel = make(chan *Meter, 16)
	if sw.clock == nil {
		sw.clock = clock.New()
	}
	var ctx context.Context
	ctx, sw.cancel = context.WithCancel(context.Background())
	sw.stopped = make(chan struct{})
	go sw.run(ctx)
}

func (sw *sweeper) stop() {
	if sw.cancel != nil {
		sw.cancel()
		<-sw.stopped
	}
}

func (sw *sweeper) run(ctx context.Context) {
	defer close(sw.stopped)
	for ctx.Err() == nil {
		select {
		case m := <-sw.registerChannel:
			sw.register(m)
			sw.runActive(ctx)
		case <-ctx.Done():
			return
		}
	}
}

func (sw *sweeper) register(m *Meter) {
	if m.registered {
		// registered twice, move on.
		return
	}
	m.registered = true
	sw.meters = append(sw.meters, m)
}

func (sw *sweeper) runActive(ctx context.Context) {
	ticker := sw.clock.Ticker(time.Second)
	defer ticker.Stop()

	sw.lastUpdateTime = sw.clock.Now()
	for len(sw.meters) > 0 {
		// Scale back allocation.
		if len(sw.meters)*2 < cap(sw.meters) {
			newMeters := make([]*Meter, len(sw.meters))
			copy(newMeters, sw.meters)
			sw.meters = newMeters
		}

		select {
		case <-ticker.C:
			sw.update()
		case m := <-sw.registerChannel:
			sw.register(m)
		case <-ctx.Done():
			return
		}
	}
	sw.meters = nil
	// Till next time.
}

func (sw *sweeper) update() {
	sw.snapshotMu.Lock()
	defer sw.snapshotMu.Unlock()

	now := sw.clock.Now()
	tdiff := now.Sub(sw.lastUpdateTime)
	if tdiff <= 0 {
		return
	}
	sw.lastUpdateTime = now
	timeMultiplier := float64(time.Second) / float64(tdiff)

	// Calculate the bandwidth for all active meters.
	for i, m := range sw.meters[:sw.activeMeters] {
		total := atomic.LoadUint64(&m.accumulator)
		diff := total - m.snapshot.Total
		instant := timeMultiplier * float64(diff)

		if diff > 0 {
			m.snapshot.LastUpdate = now
		}

		if m.snapshot.Rate == 0 {
			m.snapshot.Rate = instant
		} else {
			m.snapshot.Rate += alpha * (instant - m.snapshot.Rate)
		}
		m.snapshot.Total = total

		// This is equivalent to one zeros, then one, then 30 zeros.
		// We'll consider that to be "idle".
		if m.snapshot.Rate > IdleRate {
			continue
		}

		// Ok, so we are idle...

		// Mark this as idle by zeroing the accumulator.
		swappedTotal := atomic.SwapUint64(&m.accumulator, 0)

		// So..., are we really idle?
		if swappedTotal > total {
			// Not so idle...
			// Now we need to make sure this gets re-registered.

			// First, add back what we removed. If we can do this
			// fast enough, we can put it back before anyone
			// notices.
			currentTotal := atomic.AddUint64(&m.accumulator, swappedTotal)

			// Did we make it?
			if currentTotal == swappedTotal {
				// Yes! Nobody noticed, move along.
				continue
			}
			// No. Someone noticed and will (or has) put back into
			// the registration channel.
			//
			// Remove the snapshot total, it'll get added back on
			// registration.
			//
			// `^uint64(total - 1)` is the two's complement of
			// `total`. It's the "correct" way to subtract
			// atomically in go.
			atomic.AddUint64(&m.accumulator, ^uint64(m.snapshot.Total-1))
		}

		// Reset the rate, keep the total.
		m.registered = false
		m.snapshot.Rate = 0
		sw.meters[i] = nil
	}

	// Re-add the total to all the newly active accumulators and set the snapshot to the total.
	// 1. We don't do this on register to avoid having to take the snapshot lock.
	// 2. We skip calculating the bandwidth for this round so we get an _accurate_ bandwidth calculation.
	for _, m := range sw.meters[sw.activeMeters:] {
		total := atomic.AddUint64(&m.accumulator, m.snapshot.Total)
		if total > m.snapshot.Total {
			m.snapshot.LastUpdate = now
		}
		m.snapshot.Total = total
	}

	// compress and trim the meter list
	var newLen int
	for _, m := range sw.meters {
		if m != nil {
			sw.meters[newLen] = m
			newLen++
		}
	}

	sw.meters = sw.meters[:newLen]

	// Finally, mark all meters still in the list as "active".
	sw.activeMeters = len(sw.meters)
}

func (sw *sweeper) Register(m *Meter) {
	sw.sweepOnce.Do(sw.start)
	sw.registerChannel <- m
}
