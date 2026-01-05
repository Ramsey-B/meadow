package processor

import (
	"context"
	"sync"
	"time"

	"github.com/Gobusters/ectologger"
	"github.com/Ramsey-B/lotus/pkg/binding"
	"github.com/Ramsey-B/lotus/pkg/models"
	"github.com/Ramsey-B/stem/pkg/tracing"
)

// BindingRepository defines the interface for loading bindings from storage
type BindingRepository interface {
	ListEnabled(ctx context.Context, tenantID string) ([]*models.Binding, error)
	List(ctx context.Context, tenantID string) ([]*models.Binding, error)
}

// TenantRegistry provides the list of active tenants
type TenantRegistry interface {
	GetActiveTenants(ctx context.Context) ([]string, error)
}

// SimpleTenantRegistry is a basic tenant registry that tracks tenants as they appear
type SimpleTenantRegistry struct {
	tenants map[string]bool
	mu      sync.RWMutex
}

// NewSimpleTenantRegistry creates a new simple tenant registry
func NewSimpleTenantRegistry() *SimpleTenantRegistry {
	return &SimpleTenantRegistry{
		tenants: make(map[string]bool),
	}
}

// AddTenant adds a tenant to the registry
func (r *SimpleTenantRegistry) AddTenant(tenantID string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.tenants[tenantID] = true
}

// RemoveTenant removes a tenant from the registry
func (r *SimpleTenantRegistry) RemoveTenant(tenantID string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.tenants, tenantID)
}

// GetActiveTenants returns all registered tenants
func (r *SimpleTenantRegistry) GetActiveTenants(_ context.Context) ([]string, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	tenants := make([]string, 0, len(r.tenants))
	for t := range r.tenants {
		tenants = append(tenants, t)
	}
	return tenants, nil
}

// BindingLoaderConfig configures the binding loader
type BindingLoaderConfig struct {
	// RefreshInterval is how often to reload bindings from the database
	RefreshInterval time.Duration

	// InitialTenants are tenants to load on startup
	InitialTenants []string
}

// DefaultBindingLoaderConfig returns sensible defaults
func DefaultBindingLoaderConfig() BindingLoaderConfig {
	return BindingLoaderConfig{
		RefreshInterval: 1 * time.Minute,
		InitialTenants:  []string{},
	}
}

// BindingLoader loads and refreshes bindings from the database into the matcher
type BindingLoader struct {
	config         BindingLoaderConfig
	repo           BindingRepository
	tenantRegistry TenantRegistry
	matcher        *binding.Matcher
	logger         ectologger.Logger

	cancel context.CancelFunc
	wg     sync.WaitGroup
	mu     sync.Mutex
}

// NewBindingLoader creates a new binding loader
func NewBindingLoader(
	config BindingLoaderConfig,
	repo BindingRepository,
	tenantRegistry TenantRegistry,
	matcher *binding.Matcher,
	logger ectologger.Logger,
) *BindingLoader {
	return &BindingLoader{
		config:         config,
		repo:           repo,
		tenantRegistry: tenantRegistry,
		matcher:        matcher,
		logger:         logger,
	}
}

// Start begins the binding loader refresh loop
func (l *BindingLoader) Start(ctx context.Context) error {
	l.mu.Lock()
	defer l.mu.Unlock()

	// Load initial bindings for configured tenants
	for _, tenantID := range l.config.InitialTenants {
		if err := l.loadTenantBindings(ctx, tenantID); err != nil {
			l.logger.WithContext(ctx).WithError(err).
				Errorf("Failed to load bindings for initial tenant %s", tenantID)
			// Continue loading other tenants
		}
	}

	// Start refresh loop
	ctx, cancel := context.WithCancel(ctx)
	l.cancel = cancel

	l.wg.Add(1)
	go l.refreshLoop(ctx)

	l.logger.Info("Binding loader started")
	return nil
}

// Stop stops the binding loader
func (l *BindingLoader) Stop() error {
	l.mu.Lock()
	defer l.mu.Unlock()

	if l.cancel != nil {
		l.cancel()
	}
	l.wg.Wait()

	l.logger.Info("Binding loader stopped")
	return nil
}

// LoadTenantBindings loads bindings for a specific tenant on-demand
func (l *BindingLoader) LoadTenantBindings(ctx context.Context, tenantID string) error {
	return l.loadTenantBindings(ctx, tenantID)
}

// refreshLoop periodically refreshes bindings from the database
func (l *BindingLoader) refreshLoop(ctx context.Context) {
	defer l.wg.Done()

	ticker := time.NewTicker(l.config.RefreshInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			l.refreshAllTenants(ctx)
		}
	}
}

// refreshAllTenants refreshes bindings for all known tenants
func (l *BindingLoader) refreshAllTenants(ctx context.Context) {
	ctx, span := tracing.StartSpan(ctx, "BindingLoader.refreshAllTenants")
	defer span.End()

	tenants, err := l.tenantRegistry.GetActiveTenants(ctx)
	if err != nil {
		l.logger.WithContext(ctx).WithError(err).Error("Failed to get active tenants")
		return
	}

	for _, tenantID := range tenants {
		if err := l.loadTenantBindings(ctx, tenantID); err != nil {
			l.logger.WithContext(ctx).WithError(err).
				Errorf("Failed to refresh bindings for tenant %s", tenantID)
		}
	}
}

// loadTenantBindings loads bindings for a tenant from the database into the matcher
func (l *BindingLoader) loadTenantBindings(ctx context.Context, tenantID string) error {
	ctx, span := tracing.StartSpan(ctx, "BindingLoader.loadTenantBindings")
	defer span.End()

	bindings, err := l.repo.ListEnabled(ctx, tenantID)
	if err != nil {
		return err
	}

	l.matcher.LoadBindings(tenantID, bindings)

	l.logger.WithContext(ctx).WithFields(map[string]any{
		"tenant_id": tenantID,
		"count":     len(bindings),
	}).Debug("Loaded bindings for tenant")

	return nil
}

