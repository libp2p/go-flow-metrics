package flow

import (
	"math"
	"sync"
	"testing"
	"time"
)

func TestBasic(t *testing.T) {
	m := new(Meter)
	for i := 0; i < 300; i++ {
		m.Mark(1000)
		mockClock.Add(40 * time.Millisecond)
	}
	if rate := m.Snapshot().Rate; rate != 25000 {
		t.Errorf("expected rate 25000, got %f", rate)
	}

	for i := 0; i < 400; i++ {
		m.Mark(200)
		mockClock.Add(40 * time.Millisecond)
	}

	if rate := m.Snapshot().Rate; !approxEq(rate, 5000, 1) {
		t.Errorf("expected rate 5000, got %f", rate)
	}

	mockClock.Add(time.Second)

	if total := m.Snapshot().Total; total != 380000 {
		t.Errorf("expected total %d, got %d", 380000, total)
	}
}

func TestShared(t *testing.T) {
	var wg sync.WaitGroup
	wg.Add(20)
	m := new(Meter)
	for j := 0; j < 20; j++ {
		go func() {
			defer wg.Done()
			for i := 0; i < 300; i++ {
				m.Mark(50)
				mockClock.Sleep(40 * time.Millisecond)
			}

			for i := 0; i < 300; i++ {
				m.Mark(10)
				mockClock.Sleep(40 * time.Millisecond)
			}
		}()
	}

	time.Sleep(time.Millisecond)
	mockClock.Add(20 * 300 * time.Millisecond)
	time.Sleep(time.Millisecond)
	mockClock.Add(20 * 300 * time.Millisecond)
	time.Sleep(time.Millisecond)

	actual := m.Snapshot()
	if !approxEq(actual.Rate, 25000, 1) {
		t.Errorf("expected rate 25000, got %f", actual.Rate)
	}

	time.Sleep(time.Millisecond)
	mockClock.Add(20 * 300 * time.Millisecond)
	time.Sleep(time.Millisecond)
	mockClock.Add(20 * 300 * time.Millisecond)
	time.Sleep(time.Millisecond)

	// Adjusts
	actual = m.Snapshot()
	if !approxEq(actual.Rate, 5000, 1) {
		t.Errorf("expected rate 5000, got %f", actual.Rate)
	}

	// Let it settle.
	time.Sleep(time.Millisecond)
	mockClock.Add(time.Second)
	time.Sleep(time.Millisecond)
	mockClock.Add(time.Second)
	time.Sleep(time.Millisecond)

	// get the right total
	actual = m.Snapshot()
	if actual.Total != 360000 {
		t.Errorf("expected total %d, got %d", 360000, actual.Total)
	}
	wg.Wait()
}

func TestUnregister(t *testing.T) {
	m := new(Meter)

	for i := 0; i < 40; i++ {
		m.Mark(1)
		mockClock.Add(100 * time.Millisecond)
	}

	actual := m.Snapshot()
	if actual.Rate != 10 {
		t.Errorf("expected rate 10, got %f", actual.Rate)
	}

	mockClock.Add(62 * time.Second)

	if m.accumulator.Load() != 0 {
		t.Error("expected meter to be paused")
	}

	actual = m.Snapshot()
	if actual.Total != 40 {
		t.Errorf("expected total 4000, got %d", actual.Total)
	}

	for i := 0; i < 40; i++ {
		m.Mark(2)
		mockClock.Add(100 * time.Millisecond)
	}

	actual = m.Snapshot()
	if actual.Rate != 20 {
		t.Errorf("expected rate 20, got %f", actual.Rate)
	}

	mockClock.Add(2 * time.Second)

	actual = m.Snapshot()
	if actual.Total != 120 {
		t.Errorf("expected total 120, got %d", actual.Total)
	}
	if m.accumulator.Load() == 0 {
		t.Error("expected meter to be active")
	}

}

func approxEq(a, b, err float64) bool {
	return math.Abs(a-b) < err
}
