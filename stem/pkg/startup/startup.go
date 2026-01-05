package startup

import (
	"context"
	"fmt"
	"time"

	"github.com/Gobusters/ectologger"
)

type StartupDependency interface {
	GetName() string
	DependsOn() []string
	Start(ctx context.Context) error
	Stop(ctx context.Context) error
}

type StartupStatus int

const (
	StartupStatusPending StartupStatus = iota
	StartupStatusStarted
	StartupStatusStopped
	StartupStatusFailed
)

type Startup struct {
	dependencies map[string]StartupDependency
	logger       ectologger.Logger
	statuses     map[string]StartupStatus
	attempt      int
	maxAttempts  int
}

func NewStartup[T any](logger ectologger.Logger, maxAttempts int) *Startup {
	return &Startup{
		logger:       logger,
		dependencies: make(map[string]StartupDependency),
		statuses:     make(map[string]StartupStatus),
		maxAttempts:  maxAttempts,
	}
}

func (s *Startup) AddDependency(dependency StartupDependency) {
	s.dependencies[dependency.GetName()] = dependency
}

func (s *Startup) Start(ctx context.Context) error {
	s.attempt = 0
	var lastErr error

	// Fibonacci backoff sequence
	a, b := 1, 1
	for s.attempt < s.maxAttempts {
		s.attempt++
		s.logger.WithField("attempt", s.attempt).Infof("Beginning startup attempt %d", s.attempt)

		success := true
		for _, dependency := range s.dependencies {
			err := s.startDependency(ctx, dependency)
			if err != nil {
				s.logger.WithError(err).Errorf("Startup dependency '%s' attempt %d failed", dependency.GetName(), s.attempt)
				lastErr = err
				success = false
				break
			}
		}

		if success {
			return nil
		}

		if s.attempt >= s.maxAttempts {
			return fmt.Errorf("startup failed after %d attempts: %w", s.attempt, lastErr)
		}

		// Calculate next fibonacci number for backoff
		waitTime := time.Duration(a) * time.Second
		s.logger.Infof("Retrying in %d seconds (attempt %d/%d)", a, s.attempt, s.maxAttempts)

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(waitTime):
			// Continue with next attempt
		}

		// Update fibonacci sequence
		a, b = b, a+b
	}

	return nil
}

func (s *Startup) startDependency(ctx context.Context, dependency StartupDependency) error {
	if s.statuses[dependency.GetName()] == StartupStatusStarted {
		return nil
	}

	for _, dependencyName := range dependency.DependsOn() {
		if s.statuses[dependencyName] != StartupStatusStarted {
			err := s.startDependency(ctx, s.dependencies[dependencyName])
			if err != nil {
				return err
			}
		}
	}

	s.logger.WithField("dependency", dependency.GetName()).Infof("Starting dependency '%s'", dependency.GetName())
	s.statuses[dependency.GetName()] = StartupStatusPending
	if err := dependency.Start(ctx); err != nil {
		s.statuses[dependency.GetName()] = StartupStatusFailed
		s.logger.WithError(err).WithField("dependency", dependency.GetName()).Errorf("Failed to start dependency '%s'", dependency.GetName())
		return err
	}
	s.statuses[dependency.GetName()] = StartupStatusStarted
	return nil
}

func (s *Startup) Stop(ctx context.Context) error {
	// Get dependencies as slice and reverse
	deps := make([]StartupDependency, 0, len(s.dependencies))
	for _, dep := range s.dependencies {
		deps = append(deps, dep)
	}
	// Reverse the slice
	for i, j := 0, len(deps)-1; i < j; i, j = i+1, j-1 {
		deps[i], deps[j] = deps[j], deps[i]
	}

	for _, dependency := range deps {
		err := s.stopDependency(ctx, dependency)
		if err != nil {
			return err
		}
	}
	return nil
}

func (s *Startup) stopDependency(ctx context.Context, dependency StartupDependency) error {
	s.logger.WithField("dependency", dependency.GetName()).Infof("Stopping dependency '%s'", dependency.GetName())
	if err := dependency.Stop(ctx); err != nil {
		s.logger.WithError(err).WithField("dependency", dependency.GetName()).Errorf("Failed to stop dependency '%s'", dependency.GetName())
		return err
	}

	s.logger.WithField("dependency", dependency.GetName()).Infof("Dependency '%s' stopped", dependency.GetName())
	s.statuses[dependency.GetName()] = StartupStatusStopped

	for _, dependencyName := range dependency.DependsOn() {
		if s.statuses[dependencyName] != StartupStatusStopped {
			err := s.stopDependency(ctx, s.dependencies[dependencyName])
			if err != nil {
				return err
			}
		}
	}
	return nil
}

