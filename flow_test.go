package flow

import (
	"math"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/benbjohnson/clock"
)

func TestBasic(t *testing.T) {
	if testing.Short() {
		t.Skip("short testing requested")
	}
	var wg sync.WaitGroup
	wg.Add(100)
	for i := 0; i < 100; i++ {
		go func() {
			defer wg.Done()
			ticker := time.NewTicker(40 * time.Millisecond)
			defer ticker.Stop()

			m := new(Meter)
			for i := 0; i < 300; i++ {
				m.Mark(1000)
				<-ticker.C
			}
			actual := m.Snapshot()
			if !approxEq(actual.Rate, 25000, 1000) {
				t.Errorf("expected rate 25000 (±1000), got %f", actual.Rate)
			}

			for i := 0; i < 200; i++ {
				m.Mark(200)
				<-ticker.C
			}

			// Adjusts
			actual = m.Snapshot()
			if !approxEq(actual.Rate, 5000, 200) {
				t.Errorf("expected rate 5000 (±200), got %f", actual.Rate)
			}

			// Let it settle.
			time.Sleep(2 * time.Second)

			// get the right total
			actual = m.Snapshot()
			if actual.Total != 340000 {
				t.Errorf("expected total %d, got %d", 340000, actual.Total)
			}
		}()
	}
	wg.Wait()
}

func TestShared(t *testing.T) {
	if testing.Short() {
		t.Skip("short testing requested")
	}
	var wg sync.WaitGroup
	wg.Add(20 * 21)
	for i := 0; i < 20; i++ {
		m := new(Meter)
		for j := 0; j < 20; j++ {
			go func() {
				defer wg.Done()
				ticker := time.NewTicker(40 * time.Millisecond)
				defer ticker.Stop()
				for i := 0; i < 300; i++ {
					m.Mark(50)
					<-ticker.C
				}

				for i := 0; i < 200; i++ {
					m.Mark(10)
					<-ticker.C
				}
			}()
		}
		go func() {
			defer wg.Done()
			time.Sleep(40 * 300 * time.Millisecond)
			actual := m.Snapshot()
			if !approxEq(actual.Rate, 25000, 250) {
				t.Errorf("expected rate 25000 (±250), got %f", actual.Rate)
			}

			time.Sleep(40 * 200 * time.Millisecond)

			// Adjusts
			actual = m.Snapshot()
			if !approxEq(actual.Rate, 5000, 50) {
				t.Errorf("expected rate 5000 (±50), got %f", actual.Rate)
			}

			// Let it settle.
			time.Sleep(2 * time.Second)

			// get the right total
			actual = m.Snapshot()
			if actual.Total != 340000 {
				t.Errorf("expected total %d, got %d", 340000, actual.Total)
			}
		}()
	}
	wg.Wait()
}

func TestUnregister(t *testing.T) {
	if testing.Short() {
		t.Skip("short testing requested")
	}
	var wg sync.WaitGroup
	wg.Add(100 * 2)
	for i := 0; i < 100; i++ {
		m := new(Meter)
		go func() {
			defer wg.Done()
			ticker := time.NewTicker(100 * time.Millisecond)
			defer ticker.Stop()
			for i := 0; i < 40; i++ {
				m.Mark(1)
				<-ticker.C
			}

			time.Sleep(62 * time.Second)

			for i := 0; i < 40; i++ {
				m.Mark(2)
				<-ticker.C
			}
		}()
		go func() {
			defer wg.Done()
			time.Sleep(40 * 100 * time.Millisecond)

			actual := m.Snapshot()
			if !approxEq(actual.Rate, 10, 1) {
				t.Errorf("expected rate 10 (±1), got %f", actual.Rate)
			}

			time.Sleep(60 * time.Second)
			if atomic.LoadUint64(&m.accumulator) != 0 {
				t.Error("expected meter to be paused")
			}

			actual = m.Snapshot()
			if actual.Total != 40 {
				t.Errorf("expected total 4000, got %d", actual.Total)
			}
			time.Sleep(2*time.Second + 40*100*time.Millisecond)

			actual = m.Snapshot()
			if !approxEq(actual.Rate, 20, 4) {
				t.Errorf("expected rate 20 (±4), got %f", actual.Rate)
			}
			time.Sleep(2 * time.Second)
			actual = m.Snapshot()
			if actual.Total != 120 {
				t.Errorf("expected total 120, got %d", actual.Total)
			}
			if atomic.LoadUint64(&m.accumulator) == 0 {
				t.Error("expected meter to be active")
			}
		}()

	}
	wg.Wait()
}

func TestMockClock(t *testing.T) {
	mockClock := useMockClock()
	defer RestoreClock()

	m := new(Meter)
	for i := 0; i < 300; i++ {
		m.Mark(1000)
		mockClock.Add(40 * time.Millisecond)
	}
	actual := m.Snapshot()
	checkApproxEq(t, actual.Rate, 25000, 100)

	for i := 0; i < 200; i++ {
		m.Mark(200)
		mockClock.Add(40 * time.Millisecond)
	}

	// Adjusts
	actual = m.Snapshot()
	checkApproxEq(t, actual.Rate, 5000, 20)

	// Let it settle.
	mockClock.Add(2 * time.Second)

	// get the right total
	actual = m.Snapshot()
	if actual.Total != 340000 {
		t.Errorf("expected total %d, got %d", 340000, actual.Total)
	}
}

func checkApproxEq(t *testing.T, rate float64, expected, err int) {
	if !approxEq(rate, float64(expected), float64(err)) {
		t.Errorf("expected rate %d (±%d), got %d", expected, err, int(rate))
	}
}

func approxEq(a, b, err float64) bool {
	return math.Abs(a-b) < err
}

func useMockClock() *clock.Mock {
	mockClock := clock.NewMock()
	SetClock(mockClock)
	return mockClock
}
