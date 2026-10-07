package hydraulic

import (
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
)

type LeakIncident struct {
	ID        string    `json:"id"`
	StationID string    `json:"station_id"`
	Watermark uint64    `json:"watermark"`
	Ratio     float64   `json:"ratio"`
	Confirmed bool      `json:"confirmed"`
	At        time.Time `json:"at"`
}

type LeakDetector struct {
	mu        sync.Mutex
	last      map[string]Balance
	incidents []LeakIncident
}

func NewLeakDetector() *LeakDetector {
	return &LeakDetector{last: make(map[string]Balance), incidents: make([]LeakIncident, 0)}
}

// Observe records a hydraulic balance for a station. Samples are deduplicated
// by watermark: only a strictly higher watermark than the last observed one
// counts as new evidence, repeated or stale telemetry is ignored entirely.
// It returns the newly raised incident, or recovered=true when a fresh sample
// shows the imbalance has cleared and the current anomaly should be closed.
func (d *LeakDetector) Observe(balance Balance) (incident *LeakIncident, recovered bool, err error) {
	if balance.StationID == "" || balance.Watermark == 0 {
		return nil, false, fmt.Errorf("balance identity is incomplete")
	}
	d.mu.Lock()
	defer d.mu.Unlock()
	previous, seen := d.last[balance.StationID]
	if seen && balance.Watermark <= previous.Watermark {
		return nil, false, nil
	}
	d.last[balance.StationID] = balance
	if !balance.LeakLikely {
		return nil, true, nil
	}
	confirmed := seen && previous.LeakLikely
	newIncident := LeakIncident{
		ID: uuid.NewString(), StationID: balance.StationID, Watermark: balance.Watermark,
		Ratio: balance.ImbalanceRatio, Confirmed: confirmed, At: time.Now().UTC(),
	}
	d.incidents = append(d.incidents, newIncident)
	return &newIncident, false, nil
}

func (d *LeakDetector) Incidents() []LeakIncident {
	d.mu.Lock()
	defer d.mu.Unlock()
	return append([]LeakIncident(nil), d.incidents...)
}
