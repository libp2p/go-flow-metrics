package flow

import (
	"sync"
	"sync/atomic"
)

// Snapshot is a rate/total snapshot.
type Snapshot struct {
	Rate  float64
	Total uint64
}

// Meter is a meter for monitoring a flow.
type Meter struct {
	runningTotal uint64
	registered   int32

	// Take lock.
	snapshot Snapshot
}

// Mark updates the total.
func (m *Meter) Mark(count uint64) {
	if count > 0 && atomic.AddUint64(&m.runningTotal, count) == count {
		// I'm the first one to bump this above 0.
		// Register it.
		globalSweeper.Register(m)
	}
}

// Snapshot gets a consistent snapshot of the total and rate.
func (m *Meter) Snapshot() Snapshot {
	globalSweeper.mutex.RLock()
	defer globalSweeper.mutex.RUnlock()
	return m.snapshot
}

// MeterRegistry is a registry for named meters.
type MeterRegistry struct {
	meters sync.Map
}

// GetMeter gets (or creates) a meter by name.
func (r *MeterRegistry) GetMeter(name string) *Meter {
	if m, ok := r.meters.Load(name); ok {
		return m.(*Meter)
	}
	m, _ := r.meters.LoadOrStore(name, new(Meter))
	return m.(*Meter)
}
