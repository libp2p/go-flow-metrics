package flow

import (
	"testing"
	"time"
)

func TestResetMeter(t *testing.T) {
	meter := new(Meter)

	meter.Mark(30)

	mockClock.Add(time.Millisecond)
	mockClock.Add(1 * time.Second)

	if total := meter.Snapshot().Total; total != 30 {
		t.Errorf("total = %d; want 30", total)
	}

	meter.Reset()

	if total := meter.Snapshot().Total; total != 0 {
		t.Errorf("total = %d; want 0", total)
	}
}
