package hydraulic

import "testing"

func leakBalance(station string, watermark uint64, likely bool) Balance {
	return Balance{StationID: station, Watermark: watermark, ImbalanceRatio: 0.4, LeakLikely: likely}
}

func TestObserveDeduplicatesRepeatedWatermark(t *testing.T) {
	d := NewLeakDetector()
	for i := 0; i < 3; i++ {
		incident, recovered, err := d.Observe(leakBalance("st-1", 10, true))
		if err != nil {
			t.Fatalf("observe: %v", err)
		}
		if recovered {
			t.Fatal("leak sample must not report recovery")
		}
		if i == 0 && incident == nil {
			t.Fatal("first leak sample must raise an incident")
		}
		if i > 0 && incident != nil {
			t.Fatalf("duplicate watermark %d must not raise another incident", i)
		}
	}
	if got := len(d.Incidents()); got != 1 {
		t.Fatalf("expected 1 incident after duplicates, got %d", got)
	}
}

func TestObserveIgnoresStaleWatermark(t *testing.T) {
	d := NewLeakDetector()
	if _, _, err := d.Observe(leakBalance("st-1", 10, true)); err != nil {
		t.Fatalf("observe: %v", err)
	}
	incident, recovered, err := d.Observe(leakBalance("st-1", 7, true))
	if err != nil {
		t.Fatalf("observe: %v", err)
	}
	if incident != nil || recovered {
		t.Fatal("stale watermark must be ignored")
	}
	if got := len(d.Incidents()); got != 1 {
		t.Fatalf("expected 1 incident after stale sample, got %d", got)
	}
}

func TestObserveConfirmsOnlyOnHigherWatermarkEvidence(t *testing.T) {
	d := NewLeakDetector()
	first, _, err := d.Observe(leakBalance("st-1", 10, true))
	if err != nil {
		t.Fatalf("observe: %v", err)
	}
	if first.Confirmed {
		t.Fatal("first leak sample must be unconfirmed")
	}
	second, _, err := d.Observe(leakBalance("st-1", 11, true))
	if err != nil {
		t.Fatalf("observe: %v", err)
	}
	if second == nil || !second.Confirmed {
		t.Fatal("consecutive higher-watermark leak must be confirmed")
	}
}

func TestObserveRecoveryClosesAnomalyWithoutNewIncident(t *testing.T) {
	d := NewLeakDetector()
	if _, _, err := d.Observe(leakBalance("st-1", 10, true)); err != nil {
		t.Fatalf("observe: %v", err)
	}
	incident, recovered, err := d.Observe(leakBalance("st-1", 11, false))
	if err != nil {
		t.Fatalf("observe: %v", err)
	}
	if incident != nil {
		t.Fatal("recovery sample must not raise an incident")
	}
	if !recovered {
		t.Fatal("fresh non-leak sample must report recovery")
	}
	if got := len(d.Incidents()); got != 1 {
		t.Fatalf("recovery must not append incidents, got %d", got)
	}
	// A leak after recovery starts a new confirmation chain.
	next, _, err := d.Observe(leakBalance("st-1", 12, true))
	if err != nil {
		t.Fatalf("observe: %v", err)
	}
	if next == nil || next.Confirmed {
		t.Fatal("leak after recovery must be unconfirmed")
	}
}

func TestObserveStaleRecoveryDoesNotCloseAnomaly(t *testing.T) {
	d := NewLeakDetector()
	if _, _, err := d.Observe(leakBalance("st-1", 10, true)); err != nil {
		t.Fatalf("observe: %v", err)
	}
	_, recovered, err := d.Observe(leakBalance("st-1", 10, false))
	if err != nil {
		t.Fatalf("observe: %v", err)
	}
	if recovered {
		t.Fatal("stale duplicate watermark must not count as recovery evidence")
	}
}

func TestObserveRejectsIncompleteIdentity(t *testing.T) {
	d := NewLeakDetector()
	if _, _, err := d.Observe(Balance{StationID: "st-1"}); err == nil {
		t.Fatal("missing watermark must be rejected")
	}
	if _, _, err := d.Observe(Balance{Watermark: 3}); err == nil {
		t.Fatal("missing station must be rejected")
	}
}
