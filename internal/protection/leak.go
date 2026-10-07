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
}

func NewLeakProtection(detector *hydraulic.LeakDetector) *LeakProtection {
	return &LeakProtection{detector: detector, active: make(map[string]hydraulic.LeakIncident)}
}

func (p *LeakProtection) Apply(balance hydraulic.Balance) (*hydraulic.LeakIncident, error) {
	incident, recovered, err := p.detector.Observe(balance)
	if err != nil {
		return nil, fmt.Errorf("evaluate leak protection: %w", err)
	}
	if recovered {
		p.Clear(balance.StationID)
		return nil, nil
	}
	if incident == nil {
		return nil, nil
	}
	if incident.Confirmed {
		p.mu.Lock()
		p.active[incident.StationID] = *incident
		p.mu.Unlock()
	}
	return incident, nil
}

func (p *LeakProtection) Active(stationID string) (hydraulic.LeakIncident, bool) {
	p.mu.RLock()
	defer p.mu.RUnlock()
	incident, ok := p.active[stationID]
	return incident, ok
}

func (p *LeakProtection) Clear(stationID string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	delete(p.active, stationID)
}
