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
		sw.meters = append(sw.meters, m)
		sw.runActive()
	}
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
			sw.meters = append(sw.meters, m)
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
		total := atomic.LoadUint64(&m.total)
		if total == m.snapshot.Total {
			if m.idleTime == IdleTimeout {
				// remove it.
				sw.meters[i] = sw.meters[len(sw.meters)-1]
				sw.meters[len(sw.meters)-1] = nil
				sw.meters = sw.meters[:len(sw.meters)-1]

				// reset these. The total can stay.
				m.idleTime = 0
				m.snapshot.Rate = 0

				atomic.StoreInt32(&m.registered, 0)
			} else {
				m.idleTime++
			}
			continue
		}

		diff := float64(total - m.snapshot.Total)
		if m.snapshot.Rate == 0 {
			m.snapshot.Rate = diff
		} else {
			m.snapshot.Rate += alpha * (diff - m.snapshot.Rate)
		}
		m.snapshot.Total = total
		m.idleTime = 0

	}
}

func (sw *sweeper) Register(m *Meter) {
	// Short cut. Swap is slow (and rarely needed).
	if atomic.LoadInt32(&m.registered) == 1 {
		return
	}
	if atomic.SwapInt32(&m.registered, 1) == 0 {
		sw.sweepOnce.Do(sw.start)
		sw.registerChannel <- m
	}
}
