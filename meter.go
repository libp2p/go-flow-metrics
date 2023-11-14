package flow

import (
	"fmt"
	"math"
	"sync/atomic"
	"time"
)

// IdleRate the rate at which we declare a meter idle (and stop tracking it
// until it's re-registered).
//
// The default ensures that 1 event every ~30s will keep the meter from going
// idle.
var IdleRate = 1e-13

// Alpha for EWMA of 1s
var alpha = 1 - math.Exp(-1.0)

// Snapshot is a rate/total snapshot.
type Snapshot struct {
	Rate       float64
	Total      uint64
	LastUpdate time.Time
}

type MeterInterface interface {
	String() string
	Mark(count uint64)
	Snapshot() Snapshot
	Reset()
	Update(tdiff time.Duration)
	IsIdle() bool
	SetIdle()
	SetActive()
}

// NewMeter returns a new Meter with the correct idle time.
//
// While zero-value Meters can be used, their "last update" time will start at
// the program start instead of when the meter was created.
func NewMeter() MeterInterface {
	return &Meter{
		fresh: true,
		snapshot: Snapshot{
			LastUpdate: time.Now(),
		},
	}
}

func (s Snapshot) String() string {
	return fmt.Sprintf("%d (%f/s)", s.Total, s.Rate)
}

// Meter is a meter for monitoring a flow.
type Meter struct {
	accumulator uint64

	// managed by the sweeper loop.
	registered bool

	// Take lock.
	snapshot Snapshot

	fresh bool
}

// Mark updates the total.
func (m *Meter) Mark(count uint64) {
	if count > 0 && atomic.AddUint64(&m.accumulator, count) == count {
		// The accumulator is 0 so we probably need to register. We may
		// already _be_ registered however, if we are, the registration
		// loop will notice that `m.registered` is set and ignore us.
		globalSweeper.Register(m)
	}
}

// Snapshot gets a snapshot of the total and rate.
func (m *Meter) Snapshot() Snapshot {
	globalSweeper.snapshotMu.RLock()
	defer globalSweeper.snapshotMu.RUnlock()
	return m.snapshot
}

// Reset sets accumulator, total and rate to zero.
func (m *Meter) Reset() {
	globalSweeper.snapshotMu.Lock()
	atomic.StoreUint64(&m.accumulator, 0)
	m.snapshot.Rate = 0
	m.snapshot.Total = 0
	globalSweeper.snapshotMu.Unlock()
}

func (m *Meter) String() string {
	return m.Snapshot().String()
}

func (m *Meter) Update(tdiff time.Duration) {
	if !m.fresh {
		timeMultiplier := float64(time.Second) / float64(tdiff)
		total := atomic.LoadUint64(&m.accumulator)
		diff := total - m.snapshot.Total
		instant := timeMultiplier * float64(diff)

		if diff > 0 {
			m.snapshot.LastUpdate = cl.Now()
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
			m.SetIdle()
			return
		}
		{
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
					return
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
		}
	} else {
		// Re-add the total to all the newly active accumulators and set the snapshot to the total.
		// 1. We don't do this on register to avoid having to take the snapshot lock.
		// 2. We skip calculating the bandwidth for this round so we get an _accurate_ bandwidth calculation.
		total := atomic.AddUint64(&m.accumulator, m.snapshot.Total)
		if total > m.snapshot.Total {
			m.snapshot.LastUpdate = cl.Now()
		}
		m.snapshot.Total = total
		m.fresh = false
	}
}

func (m *Meter) IsIdle() bool {
	return !m.registered
}

func (m *Meter) SetIdle() {
	m.registered = false
	m.fresh = true
}

func (m *Meter) SetActive() {
	m.registered = true
}
