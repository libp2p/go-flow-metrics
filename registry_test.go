package flow

import (
	"testing"
	"time"
)

func TestRegistry(t *testing.T) {
	r := new(MeterRegistry)
	m1 := r.GetMeter("first")
	m2 := r.GetMeter("second")
	m1.Mark(10)
	m2.Mark(30)

	time.Sleep(2 * time.Second)

	if total := r.GetMeter("first").Snapshot().Total; total != 10 {
		t.Errorf("expected first total to be 10, got %d", total)
	}
	if total := r.GetMeter("second").Snapshot().Total; total != 30 {
		t.Errorf("expected second total to be 30, got %d", total)
	}

	expectedMeters := map[string]*Meter{
		"first":  m1,
		"second": m2,
	}
	r.ForEach(func(n string, m *Meter) {
		if expectedMeters[n] != m {
			t.Errorf("wrong meter '%s'", n)
		}
		delete(expectedMeters, n)
	})
	if len(expectedMeters) != 0 {
		t.Errorf("missing meters: '%v'", expectedMeters)
	}

	r.RemoveMeter("first")

	found := false
	r.ForEach(func(n string, m *Meter) {
		if n != "second" {
			t.Errorf("found unexpected meter: %s", n)
			return
		}
		if found {
			t.Error("found meter twice")
		}
		found = true
	})

	if !found {
		t.Errorf("didn't find second meter")
	}

	m3 := r.GetMeter("first")
	if m3 == m1 {
		t.Error("should have gotten a new meter")
	}
	if total := m3.Snapshot().Total; total != 0 {
		t.Errorf("expected first total to now be 0, got %d", total)
	}

	expectedMeters = map[string]*Meter{
		"first":  m3,
		"second": m2,
	}
	r.ForEach(func(n string, m *Meter) {
		if expectedMeters[n] != m {
			t.Errorf("wrong meter '%s'", n)
		}
		delete(expectedMeters, n)
	})
	if len(expectedMeters) != 0 {
		t.Errorf("missing meters: '%v'", expectedMeters)
	}
}
