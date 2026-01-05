package binding

import (
	"strings"
	"sync"

	"github.com/Ramsey-B/lotus/pkg/kafka"
	"github.com/Ramsey-B/lotus/pkg/models"
)

// Matcher matches incoming messages to bindings
type Matcher struct {
	bindings map[string][]*models.Binding // tenant_id -> bindings
	mu       sync.RWMutex
}

// NewMatcher creates a new binding matcher
func NewMatcher() *Matcher {
	return &Matcher{
		bindings: make(map[string][]*models.Binding),
	}
}

// LoadBindings loads bindings for a tenant
func (m *Matcher) LoadBindings(tenantID string, bindings []*models.Binding) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Only load enabled bindings
	enabled := make([]*models.Binding, 0, len(bindings))
	for _, b := range bindings {
		if b.IsEnabled {
			enabled = append(enabled, b)
		}
	}

	m.bindings[tenantID] = enabled
}

// RemoveTenant removes all bindings for a tenant
func (m *Matcher) RemoveTenant(tenantID string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.bindings, tenantID)
}

// UpdateBinding updates or adds a single binding
func (m *Matcher) UpdateBinding(binding *models.Binding) {
	m.mu.Lock()
	defer m.mu.Unlock()

	tenantBindings := m.bindings[binding.TenantID]

	// Find and update or append
	found := false
	for i, b := range tenantBindings {
		if b.ID == binding.ID {
			if binding.IsEnabled {
				tenantBindings[i] = binding
			} else {
				// Remove disabled binding
				tenantBindings = append(tenantBindings[:i], tenantBindings[i+1:]...)
			}
			found = true
			break
		}
	}

	if !found && binding.IsEnabled {
		tenantBindings = append(tenantBindings, binding)
	}

	m.bindings[binding.TenantID] = tenantBindings
}

// RemoveBinding removes a binding
func (m *Matcher) RemoveBinding(tenantID, bindingID string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	tenantBindings := m.bindings[tenantID]
	for i, b := range tenantBindings {
		if b.ID == bindingID {
			m.bindings[tenantID] = append(tenantBindings[:i], tenantBindings[i+1:]...)
			return
		}
	}
}

// MatchResult contains a matched binding and its priority
type MatchResult struct {
	Binding *models.Binding
	Score   int // Higher score = more specific match
}

// Match finds all bindings that match the given message
func (m *Matcher) Match(msg *kafka.ReceivedMessage) []*MatchResult {
	m.mu.RLock()
	defer m.mu.RUnlock()

	// Get tenant ID from message
	tenantID := msg.Headers.TenantID
	if tenantID == "" && msg.OrchidMessage != nil {
		tenantID = msg.OrchidMessage.TenantID
	}

	if tenantID == "" {
		return nil
	}

	tenantBindings := m.bindings[tenantID]
	if len(tenantBindings) == 0 {
		return nil
	}

	results := make([]*MatchResult, 0)

	for _, binding := range tenantBindings {
		if score := m.matchBinding(binding, msg); score > 0 {
			results = append(results, &MatchResult{
				Binding: binding,
				Score:   score,
			})
		}
	}

	return results
}

// MatchFirst finds the first (highest scoring) binding that matches
func (m *Matcher) MatchFirst(msg *kafka.ReceivedMessage) *models.Binding {
	results := m.Match(msg)
	if len(results) == 0 {
		return nil
	}

	// Find highest scoring match
	best := results[0]
	for _, r := range results[1:] {
		if r.Score > best.Score {
			best = r
		}
	}

	return best.Binding
}

// matchBinding checks if a binding matches a message and returns a score
func (m *Matcher) matchBinding(binding *models.Binding, msg *kafka.ReceivedMessage) int {
	filter := binding.Filter
	score := 1 // Base score for matching tenant

	// Get message data
	var planKey string
	var integration string
	var statusCode int
	var stepPath string
	var requestURL string

	if msg.OrchidMessage != nil {
		planKey = msg.OrchidMessage.PlanKey
		integration = msg.OrchidMessage.Integration
		statusCode = msg.OrchidMessage.StatusCode
		stepPath = msg.OrchidMessage.StepPath
		requestURL = msg.OrchidMessage.RequestURL
	} else if msg.Data != nil {
		if v, ok := msg.Data["plan_key"].(string); ok {
			planKey = v
		}
		if v, ok := msg.Data["integration"].(string); ok {
			integration = v
		}
		if v, ok := msg.Data["status_code"].(float64); ok {
			statusCode = int(v)
		}
		if v, ok := msg.Data["step_path"].(string); ok {
			stepPath = v
		}
		if v, ok := msg.Data["request_url"].(string); ok {
			requestURL = v
		}
	}

	if integration != "" && integration != filter.Integration {
		return 0 // No match
	}

	// Check plan ID filter
	if len(filter.Keys) > 0 {
		found := false
		for _, pid := range filter.Keys {
			if pid == planKey || (planKey != "" && pid == planKey) {
				found = true
				break
			}
		}
		if !found {
			return 0 // No match
		}
		score += 10 // Strong match for specific plan
	}

	// Check status code filter
	if len(filter.StatusCodes) > 0 {
		found := false
		for _, sc := range filter.StatusCodes {
			if sc == statusCode {
				found = true
				break
			}
		}
		if !found {
			return 0 // No match
		}
		score += 5
	}

	// Check status code range
	if filter.MinStatusCode > 0 && statusCode < filter.MinStatusCode {
		return 0
	}
	if filter.MaxStatusCode > 0 && statusCode > filter.MaxStatusCode {
		return 0
	}
	if filter.MinStatusCode > 0 || filter.MaxStatusCode > 0 {
		score += 2
	}

	// Check step path prefix
	if filter.StepPathPrefix != "" {
		if !strings.HasPrefix(stepPath, filter.StepPathPrefix) {
			return 0 // No match
		}
		score += 5
	}

	// Check request URL contains
	if filter.RequestURLContains != "" {
		if !strings.Contains(requestURL, filter.RequestURLContains) {
			return 0
		}
		score += 5
	}

	return score
}

// BindingCount returns the total number of bindings loaded
func (m *Matcher) BindingCount() int {
	m.mu.RLock()
	defer m.mu.RUnlock()

	count := 0
	for _, bindings := range m.bindings {
		count += len(bindings)
	}
	return count
}

// TenantCount returns the number of tenants with bindings
func (m *Matcher) TenantCount() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.bindings)
}
