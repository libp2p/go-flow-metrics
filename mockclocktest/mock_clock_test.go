package mockclocktest

import (
	"testing"
	"time"

	"github.com/libp2p/go-flow-metrics"

	"github.com/benbjohnson/clock"
)

var cl = clock.NewMock()

func init() {
	flow.SetClock(cl)
}

func TestBasic(t *testing.T) {
	m := new(flow.Meter)
	for i := 0; i < 300; i++ {
		m.Mark(1000)
		cl.Add(40 * time.Millisecond)
	}
	if rate := m.Snapshot().Rate; rate != 25000 {
		t.Errorf("expected rate 25000, got %f", rate)
	}

	for i := 0; i < 200; i++ {
		m.Mark(200)
		cl.Add(40 * time.Millisecond)
	}

	// Adjusts
	if rate := m.Snapshot().Rate; rate != 5017.776503840969 {
		t.Errorf("expected rate 5017.776503840969, got %f", rate)
	}

	// Let it settle.
	cl.Add(2 * time.Second)
	if total := m.Snapshot().Total; total != 340000 {
		t.Errorf("expected total 3400000, got %d", total)
	}
}
