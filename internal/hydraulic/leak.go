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
	Closed    bool      `json:"closed"`
	At        time.Time `json:"at"`
	ClosedAt  time.Time `json:"closed_at,omitempty"`
}

type LeakDetector struct {
	mu        sync.Mutex
	last      map[string]Balance
	openIndex map[string]int // station -> index of the open episode in incidents
	incidents []LeakIncident
}

func NewLeakDetector() *LeakDetector {
	return &LeakDetector{
		last:      make(map[string]Balance),
		openIndex: make(map[string]int),
		incidents: make([]LeakIncident, 0),
	}
}

func (d *LeakDetector) Observe(balance Balance) (*LeakIncident, error) {
	if balance.StationID == "" || balance.Watermark == 0 {
		return nil, fmt.Errorf("balance identity is incomplete")
	}
	d.mu.Lock()
	defer d.mu.Unlock()

	previous, seen := d.last[balance.StationID]
	d.last[balance.StationID] = balance

	// A recovered balance closes the open episode instead of opening a new case.
	if !balance.LeakLikely {
		d.close(balance.StationID)
		return nil, nil
	}

	// A leak episode is already open for this station.
	if idx, ok := d.openIndex[balance.StationID]; ok {
		current := d.incidents[idx]
		// Only a strictly higher watermark counts as new evidence; repeated or
		// stale telemetry at the same watermark must not add duplicate cases.
		if balance.Watermark <= current.Watermark {
			return nil, nil
		}
		current.Watermark = balance.Watermark
		current.Ratio = balance.ImbalanceRatio
		current.Confirmed = true
		current.At = time.Now().UTC()
		d.incidents[idx] = current
		incident := current
		return &incident, nil
	}

	// No open episode: a sample at or below the highest watermark already seen
	// is a retransmission, not the start of a new leak.
	if seen && balance.Watermark <= previous.Watermark {
		return nil, nil
	}

	// First suspicious balance of a new episode stays unconfirmed until a
	// higher watermark corroborates it.
	incident := LeakIncident{
		ID: uuid.NewString(), StationID: balance.StationID, Watermark: balance.Watermark,
		Ratio: balance.ImbalanceRatio, Confirmed: false, At: time.Now().UTC(),
	}
	d.incidents = append(d.incidents, incident)
	d.openIndex[balance.StationID] = len(d.incidents) - 1
	return &incident, nil
}

// close marks the station's open episode closed. Caller must hold d.mu.
func (d *LeakDetector) close(stationID string) {
	idx, ok := d.openIndex[stationID]
	if !ok {
		return
	}
	d.incidents[idx].Closed = true
	d.incidents[idx].ClosedAt = time.Now().UTC()
	delete(d.openIndex, stationID)
}

func (d *LeakDetector) Incidents() []LeakIncident {
	d.mu.Lock()
	defer d.mu.Unlock()
	return append([]LeakIncident(nil), d.incidents...)
}
