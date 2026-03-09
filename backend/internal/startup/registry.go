package startup

import (
	"sort"
	"sync"
	"time"
)

type Tier string

const (
	TierCritical Tier = "critical"
	TierOptional Tier = "optional"
)

type State string

const (
	StatePending  State = "pending"
	StateReady    State = "ready"
	StateDegraded State = "degraded"
	StateFailed   State = "failed"
)

type Phase string

const (
	PhaseStarting     Phase = "starting"
	PhaseReady        Phase = "ready"
	PhaseShuttingDown Phase = "shutting_down"
	PhaseFailed       Phase = "failed"
)

type Service struct {
	Name      string         `json:"name"`
	Tier      Tier           `json:"tier"`
	State     State          `json:"state"`
	Summary   string         `json:"summary"`
	Details   map[string]any `json:"details,omitempty"`
	UpdatedAt time.Time      `json:"updated_at"`
}

type TierCounts struct {
	Total    int `json:"total"`
	Ready    int `json:"ready"`
	Degraded int `json:"degraded"`
	Failed   int `json:"failed"`
	Pending  int `json:"pending"`
}

type Summary struct {
	Phase            Phase      `json:"phase"`
	Status           string     `json:"status"`
	Ready            bool       `json:"ready"`
	StartedAt        time.Time  `json:"started_at"`
	UpdatedAt        time.Time  `json:"updated_at"`
	Critical         TierCounts `json:"critical"`
	Optional         TierCounts `json:"optional"`
	DegradedFeatures []string   `json:"degraded_features,omitempty"`
	Services         []Service  `json:"services"`
}

type Registry struct {
	mu        sync.RWMutex
	phase     Phase
	startedAt time.Time
	updatedAt time.Time
	services  map[string]Service
}

func NewRegistry() *Registry {
	now := time.Now().UTC()
	return &Registry{
		phase:     PhaseStarting,
		startedAt: now,
		updatedAt: now,
		services:  make(map[string]Service),
	}
}

func (r *Registry) Register(name string, tier Tier, summary string, details map[string]any) {
	r.set(name, tier, StatePending, summary, details)
}

func (r *Registry) MarkReady(name string, tier Tier, summary string, details map[string]any) {
	r.set(name, tier, StateReady, summary, details)
}

func (r *Registry) MarkDegraded(name string, tier Tier, summary string, details map[string]any) {
	r.set(name, tier, StateDegraded, summary, details)
}

func (r *Registry) MarkFailed(name string, tier Tier, summary string, details map[string]any) {
	r.set(name, tier, StateFailed, summary, details)
}

func (r *Registry) SetPhase(phase Phase) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.phase = phase
	r.updatedAt = time.Now().UTC()
}

func (r *Registry) Snapshot() Summary {
	r.mu.RLock()
	defer r.mu.RUnlock()

	services := make([]Service, 0, len(r.services))
	for _, service := range r.services {
		services = append(services, cloneService(service))
	}

	sort.Slice(services, func(i, j int) bool {
		if services[i].Tier != services[j].Tier {
			return services[i].Tier < services[j].Tier
		}
		return services[i].Name < services[j].Name
	})

	summary := Summary{
		Phase:     r.phase,
		StartedAt: r.startedAt,
		UpdatedAt: r.updatedAt,
		Services:  services,
	}

	for _, service := range services {
		target := &summary.Optional
		if service.Tier == TierCritical {
			target = &summary.Critical
		}

		target.Total++
		switch service.State {
		case StateReady:
			target.Ready++
		case StateDegraded:
			target.Degraded++
		case StateFailed:
			target.Failed++
		default:
			target.Pending++
		}

		if service.Tier == TierOptional && service.State != StateReady {
			summary.DegradedFeatures = append(summary.DegradedFeatures, service.Name)
		}
	}

	summary.Ready = summary.Phase == PhaseReady &&
		summary.Critical.Pending == 0 &&
		summary.Critical.Degraded == 0 &&
		summary.Critical.Failed == 0

	switch {
	case summary.Phase == PhaseFailed || summary.Critical.Failed > 0:
		summary.Status = "failed"
	case summary.Phase == PhaseShuttingDown:
		summary.Status = "shutting_down"
	case !summary.Ready:
		if summary.Phase == PhaseStarting {
			summary.Status = "starting"
		} else {
			summary.Status = "unhealthy"
		}
	case summary.Optional.Degraded > 0 || summary.Optional.Failed > 0 || summary.Optional.Pending > 0:
		summary.Status = "degraded"
	default:
		summary.Status = "healthy"
	}

	return summary
}

func (r *Registry) set(name string, tier Tier, state State, summary string, details map[string]any) {
	r.mu.Lock()
	defer r.mu.Unlock()

	now := time.Now().UTC()
	r.services[name] = Service{
		Name:      name,
		Tier:      tier,
		State:     state,
		Summary:   summary,
		Details:   cloneMap(details),
		UpdatedAt: now,
	}
	r.updatedAt = now
}

func cloneService(service Service) Service {
	service.Details = cloneMap(service.Details)
	return service
}

func cloneMap(details map[string]any) map[string]any {
	if len(details) == 0 {
		return nil
	}

	cloned := make(map[string]any, len(details))
	for key, value := range details {
		cloned[key] = value
	}
	return cloned
}
