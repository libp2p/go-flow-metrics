package flow

import (
	"math"
	"sync"
	"sync/atomic"
	"time"
)

// IdleTimeout is the amount of time a meter is allowed to be idle before it is suspended.
var IdleTimeout uint32 = 60 // a minute

// Alpha for EWMA of 1s
var alpha = 1 - math.Exp(-1.0)

// The global sweeper.
var globalSweeper sweeper

type sweeper struct {
	sweepOnce       sync.Once
	meters          []*Meter
	mutex           sync.RWMutex
	registerChannel chan *Meter
}

func (sw *sweeper) start() {
	sw.registerChannel = make(chan *Meter, 16)
	go sw.run()
}

func (sw *sweeper) run() {
	for m := range sw.registerChannel {
		sw.register(m)
		sw.runActive()
	}
}

func (sw *sweeper) register(m *Meter) {
	// Add back the snapshot total. If we unregistered this
	// one, we set it to zero.
	atomic.AddUint64(&m.accumulator, m.snapshot.Total)
	sw.meters = append(sw.meters, m)
}

func (sw *sweeper) runActive() {
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	for len(sw.meters) > 0 {
		// Scale back allocation.
		if len(sw.meters)*2 < cap(sw.meters) {
			newMeters := make([]*Meter, len(sw.meters))
			copy(newMeters, sw.meters)
			sw.meters = newMeters
		}

		select {
		case t := <-ticker.C:
			sw.update(t)
		case m := <-sw.registerChannel:
			sw.register(m)
		}
	}
	sw.meters = nil
	// Till next time.
}

func (sw *sweeper) update(t time.Time) {
	sw.mutex.Lock()
	defer sw.mutex.Unlock()
	for i := 0; i < len(sw.meters); i++ {
		m := sw.meters[i]
		total := atomic.LoadUint64(&m.accumulator)
		diff := total - m.snapshot.Total

		if m.snapshot.Rate == 0 {
			m.snapshot.Rate = float64(diff)
		} else {
			m.snapshot.Rate += alpha * (float64(diff) - m.snapshot.Rate)
		}
		m.snapshot.Total = total

		// This is equivalent to one zeros, then one, then 30 zeros.
		// We'll consider that to be "idle".
		if m.snapshot.Rate < 1e-13 {
			// Reset the rate, keep the total.
			m.snapshot.Rate = 0

			// Mark this as idle by zeroing the total.
			swappedTotal := atomic.SwapUint64(&m.accumulator, 0)

			// Not so idle...
			// OK, now we need to make sure this gets re-registered.
			if swappedTotal > total {
				// First, add back what we removed. If we can do
				// this fast enough, we can put it back before
				// anything notices.
				currentTotal := atomic.AddUint64(&m.accumulator, swappedTotal)

				// Did we make it?
				if currentTotal == swappedTotal {
					// Yes! Nobody noticed! Move along.
					continue
				}

				// Otherwise, someone noticed (and has or will
				// put this meter into the registration
				// channel).
				//
				// Remove the snapshot total before
				// unregistering, we'll add it back on
				// registration.
				//
				// (that's two's compliment, see the docs)
				atomic.AddUint64(&m.accumulator, ^uint64(m.snapshot.Total-1))
			}

			// remove it.
			sw.meters[i] = sw.meters[len(sw.meters)-1]
			sw.meters[len(sw.meters)-1] = nil
			sw.meters = sw.meters[:len(sw.meters)-1]
			i--
		}
	}
}

func (sw *sweeper) Register(m *Meter) {
	sw.sweepOnce.Do(sw.start)
	sw.registerChannel <- m
}
