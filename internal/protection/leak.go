package protection

import (
	"fmt"
	"sync"

	"heatpulse/internal/hydraulic"
)

type LeakProtection struct {
	detector *hydraulic.LeakDetector
	mu       sync.RWMutex
	active   map[string]hydraulic.LeakIncident
	fired    map[string]string // station -> ID of the episode that already raised protection
}

func NewLeakProtection(detector *hydraulic.LeakDetector) *LeakProtection {
	return &LeakProtection{
		detector: detector,
		active:   make(map[string]hydraulic.LeakIncident),
		fired:    make(map[string]string),
	}
}

// Apply feeds the balance to the detector and mirrors its open, confirmed
// episodes. It returns the current episode (nil for duplicate telemetry and
// for recovery samples) and a newlyConfirmed flag that fires exactly once per
// episode, on its unconfirmed -> confirmed transition, so callers raise
// protection actions once per leak instead of once per watermark.
func (p *LeakProtection) Apply(balance hydraulic.Balance) (incident *hydraulic.LeakIncident, newlyConfirmed bool, err error) {
	incident, err = p.detector.Observe(balance)
	if err != nil {
		return nil, false, fmt.Errorf("evaluate leak protection: %w", err)
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	if incident == nil {
		// Recovery (or duplicate) sample: on recovery the detector has closed
		// the episode, so drop the active anomaly instead of leaving a stale
		// case or building a new one.
		if !balance.LeakLikely {
			delete(p.active, balance.StationID)
			delete(p.fired, balance.StationID)
		}
		return nil, false, nil
	}
	if incident.Confirmed {
		p.active[incident.StationID] = *incident
		// Fire only for the first confirmation of this specific episode;
		// later watermark advances (and a manual Clear of the active mirror)
		// must not raise the same protection case again.
		newlyConfirmed = p.fired[incident.StationID] != incident.ID
		if newlyConfirmed {
			p.fired[incident.StationID] = incident.ID
		}
	}
	return incident, newlyConfirmed, nil
}

func (p *LeakProtection) Active(stationID string) (hydraulic.LeakIncident, bool) {
	p.mu.RLock()
	defer p.mu.RUnlock()
	incident, ok := p.active[stationID]
	return incident, ok
}

// Clear drops the active anomaly mirror for the station (used when an operator
// resolves the incident). The detector's episode and the fired-episode guard
// are left intact, so residual telemetry cannot re-raise the same case.
func (p *LeakProtection) Clear(stationID string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	delete(p.active, stationID)
}
