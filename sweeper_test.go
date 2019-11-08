package flow

import (
	"testing"
	"time"
)

// regression test for libp2p/go-libp2p-core#65
func TestIdleInconsistency(t *testing.T) {
	r := new(MeterRegistry)
	m1 := r.Get("first")
	m2 := r.Get("second")
	m3 := r.Get("third")

	m1.Mark(10)
	m2.Mark(20)
	m3.Mark(30)

	// make m1 and m3 go idle
	for i := 0; i < 30; i++ {
		time.Sleep(time.Second)
		m2.Mark(1)
	}

	time.Sleep(time.Second)

	// re-activate m3
	m3.Mark(20)
	time.Sleep(time.Second + time.Millisecond)

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
