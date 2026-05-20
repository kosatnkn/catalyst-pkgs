package basic

import (
	"context"
	"fmt"
	"maps"
	"strings"
	"sync"
	"time"

	"github.com/kosatnkn/catalyst-pkgs/infra/readiness"
	"github.com/kosatnkn/catalyst-pkgs/telemetry/log"
)

// Readiness contains the current state of different
// components of the service.
type Readiness struct {
	logger   log.LoggerBasic
	mu       sync.RWMutex
	states   map[string]bool
	checkers map[string]func() (bool, error)
}

// NewReadiness creates a new instance.
func NewReadiness(l log.LoggerBasic) readiness.Readiness {
	return &Readiness{
		logger:   l,
		states:   make(map[string]bool),
		checkers: make(map[string]func() (bool, error)),
	}
}

// SetReadiness updates readiness for a component.
func (r *Readiness) SetReadiness(component string, ready bool) {
	r.mu.Lock()
	prev := r.states[component]
	r.states[component] = ready
	checker := r.checkers[component]
	r.mu.Unlock()

	// only trigger recovery when ready state changes from true to false
	if prev && !ready && checker != nil {
		ctx := context.Background()
		r.logger.Warn(ctx, fmt.Sprintf("Component '%s' is not ready, checking for readiness", component))
		go r.recover(ctx, component, checker)
	}
}

// Ready returns true only if all components are ready.
func (r *Readiness) Ready() bool {
	r.mu.RLock()
	defer r.mu.RUnlock()

	if len(r.states) == 0 {
		return false
	}

	for _, ready := range r.states {
		if !ready {
			return false
		}
	}

	return true
}

// RegisterCheckerFn registers a callback function for the component.
func (r *Readiness) RegisterCheckerFn(component string, checker func() (bool, error)) {
	r.mu.Lock()
	r.checkers[component] = checker
	r.states[component] = true
	r.mu.Unlock()

	r.SetReadiness(component, false)
}

// Snapshot returns current component states.
func (r *Readiness) Snapshot() map[string]bool {
	r.mu.RLock()
	defer r.mu.RUnlock()

	// NOTE: since maps are reference types returning `r.state`
	// will give access to it from the outside.
	// cloning prevents this by returning a copy of the map
	return maps.Clone(r.states)
}

// String returns current component states as a string.
func (r *Readiness) String() string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	parts := make([]string, 0, len(r.states))
	for k, v := range r.states {
		parts = append(parts, fmt.Sprintf("%s: %t", k, v))
	}

	return strings.Join(parts, ", ")
}

// recover tries to recover the component state.
func (r *Readiness) recover(ctx context.Context, component string, checker func() (bool, error)) {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			r.logger.Warn(ctx, fmt.Sprintf("Cancelled readiness for component '%s'", component))
			return

		case <-ticker.C:
			ready, err := checker()
			if err != nil || !ready {
				r.logger.Warn(ctx, fmt.Sprintf("Readiness check fails for component '%s' due to error '%s'", component, err))
				continue
			}

			r.SetReadiness(component, true)
			r.logger.Info(ctx, fmt.Sprintf("Readiness check passed for component '%s'", component))

			return
		}
	}
}
