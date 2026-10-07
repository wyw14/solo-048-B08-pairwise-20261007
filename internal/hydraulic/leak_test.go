package hydraulic

import "testing"

func leakBalance(stationID string, watermark uint64, leak bool) Balance {
	ratio := 0.0
	if leak {
		ratio = 0.5
	}
	return Balance{StationID: stationID, Watermark: watermark, ImbalanceRatio: ratio, LeakLikely: leak}
}

func TestObserveDeduplicatesSameWatermark(t *testing.T) {
	d := NewLeakDetector()

	first, err := d.Observe(leakBalance("s1", 10, true))
	if err != nil || first == nil || first.Confirmed {
		t.Fatalf("first observation should be an unconfirmed episode, got %+v err %v", first, err)
	}

	// Repeated telemetry at the same watermark must not append another case.
	repeat, err := d.Observe(leakBalance("s1", 10, true))
	if err != nil || repeat != nil {
		t.Fatalf("same-watermark sample should be ignored, got %+v err %v", repeat, err)
	}
	if incidents := d.Incidents(); len(incidents) != 1 {
		t.Fatalf("expected 1 incident, got %d: %+v", len(incidents), incidents)
	}
}

func TestObserveHigherWatermarkConfirmsOpenEpisode(t *testing.T) {
	d := NewLeakDetector()

	d.Observe(leakBalance("s1", 10, true))
	confirmed, err := d.Observe(leakBalance("s1", 11, true))
	if err != nil || confirmed == nil || !confirmed.Confirmed {
		t.Fatalf("higher watermark should confirm the episode, got %+v err %v", confirmed, err)
	}

	// Confirmation happens in place: same case ID, still one incident.
	incidents := d.Incidents()
	if len(incidents) != 1 {
		t.Fatalf("expected 1 incident after confirmation, got %d: %+v", len(incidents), incidents)
	}
	if incidents[0].Watermark != 11 || incidents[0].Ratio != 0.5 {
		t.Fatalf("open episode should carry the latest evidence, got %+v", incidents[0])
	}

	// Further advances update the same episode, never create new cases.
	again, _ := d.Observe(leakBalance("s1", 12, true))
	if again == nil || !again.Confirmed || again.ID != incidents[0].ID {
		t.Fatalf("watermark advance should update the same episode %q, got %+v", incidents[0].ID, again)
	}
	if incidents := d.Incidents(); len(incidents) != 1 {
		t.Fatalf("expected still 1 incident, got %d", len(incidents))
	}
}

func TestObserveRecoveryClosesOpenEpisode(t *testing.T) {
	d := NewLeakDetector()

	d.Observe(leakBalance("s1", 10, true))
	d.Observe(leakBalance("s1", 11, true))

	recovered, err := d.Observe(leakBalance("s1", 12, false))
	if err != nil || recovered != nil {
		t.Fatalf("recovery sample should return no incident, got %+v err %v", recovered, err)
	}

	incidents := d.Incidents()
	if len(incidents) != 1 || !incidents[0].Closed || incidents[0].ClosedAt.IsZero() {
		t.Fatalf("the open episode should be closed, got %+v", incidents)
	}

	// A stale retransmission at an already-seen watermark must not reopen it.
	if stale, _ := d.Observe(leakBalance("s1", 11, true)); stale != nil {
		t.Fatalf("stale leak sample after recovery should be ignored, got %+v", stale)
	}
	if incidents := d.Incidents(); len(incidents) != 1 {
		t.Fatalf("stale sample must not create a case, got %d", len(incidents))
	}

	// A genuinely new leak after recovery opens a fresh case.
	next, err := d.Observe(leakBalance("s1", 13, true))
	if err != nil || next == nil || next.Confirmed {
		t.Fatalf("new leak should open an unconfirmed episode, got %+v err %v", next, err)
	}
	if incidents := d.Incidents(); len(incidents) != 2 {
		t.Fatalf("expected a new episode after recovery, got %d incidents", len(incidents))
	}
}

func TestObserveIsolatesStations(t *testing.T) {
	d := NewLeakDetector()
	d.Observe(leakBalance("s1", 10, true))
	d.Observe(leakBalance("s2", 10, true))
	if incidents := d.Incidents(); len(incidents) != 2 {
		t.Fatalf("each station gets its own episode, got %d", len(incidents))
	}
	d.Observe(leakBalance("s1", 11, false))
	incidents := d.Incidents()
	if !incidents[0].Closed || incidents[1].Closed {
		t.Fatalf("only s1's episode should close, got %+v", incidents)
	}
}
