package flow

import (
	"testing"
	"time"

	"github.com/benbjohnson/clock"
)

var mockClock = clock.NewMock()

func init() {
	SetClock(mockClock)
}

// regression test for libp2p/go-libp2p-core#65
func TestIdleInconsistency(t *testing.T) {
	r := new(MeterRegistry)
	m1 := r.Get("first").(*Meter)
	m2 := r.Get("second").(*Meter)
	m3 := r.Get("third").(*Meter)

	m1.Mark(10)
	m2.Mark(20)
	m3.Mark(30)

	// make m1 and m3 go idle
	for i := 0; i < 30; i++ {
		mockClock.Add(time.Second)
		m2.Mark(1)
	}

	mockClock.Add(time.Second)

	// re-activate m3
	m3.Mark(20)
	mockClock.Add(time.Second)

	// check the totals
	if total := r.Get("first").Snapshot().Total; total != 10 {
		t.Errorf("expected first total to be 10, got %d", total)
	}

	if total := r.Get("second").Snapshot().Total; total != 50 {
		t.Errorf("expected second total to be 50, got %d", total)
	}

	if total := r.Get("third").Snapshot().Total; total != 50 {
		t.Errorf("expected third total to be 50, got %d", total)
	}
}
