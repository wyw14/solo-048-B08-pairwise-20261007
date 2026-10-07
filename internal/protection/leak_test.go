package protection

import (
	"testing"

	"heatpulse/internal/hydraulic"
)

func leakBalance(station string, watermark uint64, likely bool) hydraulic.Balance {
	return hydraulic.Balance{StationID: station, Watermark: watermark, ImbalanceRatio: 0.4, LeakLikely: likely}
}

func TestApplyActivatesConfirmedIncident(t *testing.T) {
	p := NewLeakProtection(hydraulic.NewLeakDetector())
	if _, err := p.Apply(leakBalance("st-1", 10, true)); err != nil {
		t.Fatalf("apply: %v", err)
	}
	if _, ok := p.Active("st-1"); ok {
		t.Fatal("unconfirmed incident must not become active")
	}
	incident, err := p.Apply(leakBalance("st-1", 11, true))
	if err != nil {
		t.Fatalf("apply: %v", err)
	}
	if incident == nil || !incident.Confirmed {
		t.Fatal("second higher-watermark leak must be confirmed")
	}
	active, ok := p.Active("st-1")
	if !ok || active.Watermark != 11 {
		t.Fatalf("expected active incident at watermark 11, got %+v (ok=%v)", active, ok)
	}
}

func TestApplyDuplicateTelemetryDoesNotRaiseNewIncident(t *testing.T) {
	p := NewLeakProtection(hydraulic.NewLeakDetector())
	first, err := p.Apply(leakBalance("st-1", 10, true))
	if err != nil {
		t.Fatalf("apply: %v", err)
	}
	if first == nil {
		t.Fatal("first leak sample must raise an incident")
	}
	dup, err := p.Apply(leakBalance("st-1", 10, true))
	if err != nil {
		t.Fatalf("apply: %v", err)
	}
	if dup != nil {
		t.Fatal("duplicate telemetry must not raise another incident")
	}
}

func TestApplyRecoveryClosesActiveAnomaly(t *testing.T) {
	p := NewLeakProtection(hydraulic.NewLeakDetector())
	if _, err := p.Apply(leakBalance("st-1", 10, true)); err != nil {
		t.Fatalf("apply: %v", err)
	}
	if _, err := p.Apply(leakBalance("st-1", 11, true)); err != nil {
		t.Fatalf("apply: %v", err)
	}
	if _, ok := p.Active("st-1"); !ok {
		t.Fatal("expected active anomaly before recovery")
	}
	incident, err := p.Apply(leakBalance("st-1", 12, false))
	if err != nil {
		t.Fatalf("apply: %v", err)
	}
	if incident != nil {
		t.Fatal("recovery sample must not raise an incident")
	}
	if _, ok := p.Active("st-1"); ok {
		t.Fatal("recovery sample must close the active anomaly")
	}
}
