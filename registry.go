package flow

import (
	"sync"
)

type registryMeter struct {
	meter     Meter
	lastValue uint64
}

// MeterRegistry is a registry for named meters.
type MeterRegistry struct {
	meters sync.Map
}

// Get gets (or creates) a meter by name.
func (r *MeterRegistry) Get(name string) *Meter {
	if m, ok := r.meters.Load(name); ok {
		return &m.(*registryMeter).meter
	}
	m, _ := r.meters.LoadOrStore(name, new(registryMeter))
	return &m.(*registryMeter).meter
}

// Marks idle meters and forgets about meters that haven't been updated since
// the last call to this function. Returns the number of meters removed and the
// number remaining.
//
// This function should be called periodically.
func (r *MeterRegistry) MarkAndTrimIdle() (trimmed, remaining int) {
	idle, total := r.findIdle()
	for _, key := range idle {
		r.meters.Delete(key)
	}
	return len(idle), total
}

func (r *MeterRegistry) findIdle() ([]interface{}, int) {
	// Yes, this is a global lock. However, all taking this does is pause
	// snapshotting.
	globalSweeper.mutex.RLock()
	defer globalSweeper.mutex.RUnlock()

	// don't cast from interface -> string, we'd need to allocate when
	// casting back.
	var idle []interface{}
	var total int
	r.meters.Range(func(k, v interface{}) bool {
		total++
		rm := v.(*registryMeter)
		oldTotal := rm.lastValue
		rm.lastValue = rm.meter.snapshot.Total
		if rm.lastValue == oldTotal {
			idle = append(idle, k)
		}
		return true
	})
	return idle, total
}

// Remove removes the named meter from the registry.
//
// Note: The only reason to do this is to save a bit of memory. Unused meters
// don't consume any CPU (after they go idle).
func (r *MeterRegistry) Remove(name string) {
	r.meters.Delete(name)
}

// ForEach calls the passed function for each registered meter.
func (r *MeterRegistry) ForEach(iterFunc func(string, *Meter)) {
	r.meters.Range(func(k, v interface{}) bool {
		iterFunc(k.(string), &v.(*registryMeter).meter)
		return true
	})
}
