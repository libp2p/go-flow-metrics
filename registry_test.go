package flow

import (
	"testing"
	"time"
)

func TestRegistry(t *testing.T) {
	r := new(MeterRegistry)
	m1 := r.Get("first")
	m2 := r.Get("second")

	m1Update := m1.Snapshot().LastUpdate
	mockClock.Add(5 * time.Second)

	m1.Mark(10)
	m2.Mark(30)

	mockClock.Add(1 * time.Second)
	mockClock.Add(1 * time.Second)
	mockClock.Add(1 * time.Millisecond)

	if total := r.Get("first").Snapshot().Total; total != 10 {
		t.Errorf("expected first total to be 10, got %d", total)
	}
	if total := r.Get("second").Snapshot().Total; total != 30 {
		t.Errorf("expected second total to be 30, got %d", total)
	}

	if lu := m1.Snapshot().LastUpdate; !lu.After(m1Update) {
		t.Errorf("expected the last update (%s) to have after (%s)", lu, m1Update)
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

	r.Remove("first")

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

	m3 := r.Get("first")
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

	before := mockClock.Now()
	mockClock.Add(time.Millisecond)
	m3.Mark(1)
	mockClock.Add(2 * time.Second)
	after := mockClock.Now()
	if len(r.FindIdle(before)) != 1 {
		t.Error("expected 1 idle timer")
	}
	if len(r.FindIdle(after)) != 2 {
		t.Error("expected 2 idle timers")
	}

	count := r.TrimIdle(after)
	if count != 2 {
		t.Error("expected to trim 2 idle timers")
	}
}

func TestClearRegistry(t *testing.T) {
	r := new(MeterRegistry)
	m1 := r.Get("first")
	m2 := r.Get("second")

	m1.Mark(10)
	m2.Mark(30)

	mockClock.Add(2 * time.Second)

	r.Clear()

	r.ForEach(func(n string, _m *Meter) {
		t.Errorf("expected no meters at all, found a meter %s", n)
	})

	if total := r.Get("first").Snapshot().Total; total != 0 {
		t.Errorf("expected first total to be 0, got %d", total)
	}
	if total := r.Get("second").Snapshot().Total; total != 0 {
		t.Errorf("expected second total to be 0, got %d", total)
	}
}
