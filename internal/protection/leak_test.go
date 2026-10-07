package protection

import (
	"testing"

	"heatpulse/internal/hydraulic"
)

func pBalance(stationID string, watermark uint64, leak bool) hydraulic.Balance {
	ratio := 0.0
	if leak {
		ratio = 0.5
	}
	return hydraulic.Balance{StationID: stationID, Watermark: watermark, ImbalanceRatio: ratio, LeakLikely: leak}
}

func TestApplyFiresNewlyConfirmedOncePerEpisode(t *testing.T) {
	p := NewLeakProtection(hydraulic.NewLeakDetector())

	first, fired, err := p.Apply(pBalance("s1", 10, true))
	if err != nil || first == nil || first.Confirmed || fired {
		t.Fatalf("first sample: unconfirmed episode, not fired; got incident=%+v fired=%v err=%v", first, fired, err)
	}

	confirmed, fired, err := p.Apply(pBalance("s1", 11, true))
	if err != nil || confirmed == nil || !confirmed.Confirmed || !fired {
		t.Fatalf("first confirmation must fire, got incident=%+v fired=%v err=%v", confirmed, fired, err)
	}
	if _, ok := p.Active("s1"); !ok {
		t.Fatal("confirmed episode should be active")
	}

	// Another watermark advance on the same episode must not fire again.
	again, fired, err := p.Apply(pBalance("s1", 12, true))
	if err != nil || again == nil || fired {
		t.Fatalf("repeat confirmation must not fire, got incident=%+v fired=%v err=%v", again, fired, err)
	}

	// Operator clears the active mirror; residual telemetry still must not re-fire.
	p.Clear("s1")
	if _, fired, _ := p.Apply(pBalance("s1", 13, true)); fired {
		t.Fatal("watermark advance after manual Clear must not re-raise the case")
	}
}

func TestApplyRecoveryClosesActiveAnomaly(t *testing.T) {
	p := NewLeakProtection(hydraulic.NewLeakDetector())

	p.Apply(pBalance("s1", 10, true))
	if _, fired, _ := p.Apply(pBalance("s1", 11, true)); !fired {
		t.Fatal("expected the episode to confirm")
	}

	recovered, fired, err := p.Apply(pBalance("s1", 12, false))
	if err != nil || recovered != nil || fired {
		t.Fatalf("recovery should return no incident and not fire, got %+v fired=%v err=%v", recovered, fired, err)
	}
	if _, ok := p.Active("s1"); ok {
		t.Fatal("recovery should close the active anomaly")
	}

	// A fresh leak after recovery is a new episode and may fire once more.
	p.Apply(pBalance("s1", 13, true))
	next, fired, _ := p.Apply(pBalance("s1", 14, true))
	if next == nil || !fired {
		t.Fatalf("new episode after recovery should fire on confirmation, got %+v fired=%v", next, fired)
	}
}
