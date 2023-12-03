package flow

import (
	"sync"
	"time"

	"github.com/benbjohnson/clock"
)

// The global sweeper.
var globalSweeper sweeper

var cl = clock.New()

// SetClock sets a clock to use in the sweeper.
// This will probably only ever be useful for testing purposes.
func SetClock(c clock.Clock) {
	cl = c
}

type SweeperInterface interface {
	Register(m MeterInterface)
}

type sweeper struct {
	sweepOnce sync.Once

	snapshotMu sync.RWMutex
	meters     []MeterInterface

	lastUpdateTime  time.Time
	registerChannel chan MeterInterface
}

func (sw *sweeper) start() {
	sw.registerChannel = make(chan MeterInterface, 16)
	go sw.run()
}

func (sw *sweeper) run() {
	for m := range sw.registerChannel {
		sw.register(m)
		sw.runActive()
	}
}

func (sw *sweeper) register(m MeterInterface) {
	if !m.IsIdle() {
		// registered twice, move on.
		return
	}
	m.SetActive()
	sw.meters = append(sw.meters, m)
}

func (sw *sweeper) runActive() {
	ticker := cl.Ticker(time.Second)
	defer ticker.Stop()

	sw.lastUpdateTime = cl.Now()
	for len(sw.meters) > 0 {
		// Scale back allocation.
		if len(sw.meters)*2 < cap(sw.meters) {
			newMeters := make([]MeterInterface, len(sw.meters))
			copy(newMeters, sw.meters)
			sw.meters = newMeters
		}

		select {
		case <-ticker.C:
			sw.update()
		case m := <-sw.registerChannel:
			sw.register(m)
		}
	}
	sw.meters = nil
	// Till next time.
}

func (sw *sweeper) update() {
	sw.snapshotMu.Lock()
	defer sw.snapshotMu.Unlock()

	now := cl.Now()
	tdiff := now.Sub(sw.lastUpdateTime)
	if tdiff <= 0 {
		return
	}
	sw.lastUpdateTime = now

	// Calculate the bandwidth for all active meters.
	for i, m := range sw.meters {
		m.Update(tdiff, now)
		// Reset the rate, keep the total.
		if m.IsIdle() {
			sw.meters[i] = nil
		}
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
}

func (sw *sweeper) Register(m MeterInterface) {
	sw.sweepOnce.Do(sw.start)
	sw.registerChannel <- m
}
