package mockclocktest

import (
	"math"
	"testing"
	"time"

	flow "github.com/libp2p/go-flow-metrics"

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
	if rate := m.Snapshot().Rate; approxEq(rate, 25000, 1) {
		t.Errorf("expected rate 25000, got %f", rate)
	}

	for i := 0; i < 200; i++ {
		m.Mark(200)
		cl.Add(40 * time.Millisecond)
	}

	// Adjusts
	if rate := m.Snapshot().Rate; approxEq(rate, 5017.776503840969, 0.0001) {
		t.Errorf("expected rate 5017.776503840969, got %f", rate)
	}

	// Let it settle.
	cl.Add(2 * time.Second)
	if total := m.Snapshot().Total; total != 340000 {
		t.Errorf("expected total 3400000, got %d", total)
	}
}

func approxEq(a, b, err float64) bool {
	return math.Abs(a-b) < err
}
