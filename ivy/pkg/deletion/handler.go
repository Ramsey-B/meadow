// Package deletion handles entity deletion logic
package deletion

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/Gobusters/ectologger"
	"github.com/Ramsey-B/stem/pkg/tracing"
	"github.com/google/uuid"

	deletionrepo "github.com/Ramsey-B/ivy/internal/repositories/deletion"
	"github.com/Ramsey-B/ivy/internal/repositories/mergedentity"
	"github.com/Ramsey-B/ivy/internal/repositories/stagedentity"
	"github.com/Ramsey-B/ivy/pkg/graph"
	"github.com/Ramsey-B/ivy/pkg/kafka"
	"github.com/Ramsey-B/ivy/pkg/models"
)

// Handler orchestrates deletion logic
type Handler struct {
	logger          ectologger.Logger
	strategyRepo    *deletionrepo.StrategyRepository
	executionRepo   *deletionrepo.ExecutionRepository
	pendingRepo     *deletionrepo.PendingRepository
	stagedRepo      *stagedentity.Repository
	mergedRepo      *mergedentity.Repository
	graphEntity     *graph.EntityService
	kafkaProducer   *kafka.Producer
}

// NewHandler creates a new deletion handler
func NewHandler(
	logger ectologger.Logger,
	strategyRepo *deletionrepo.StrategyRepository,
	executionRepo *deletionrepo.ExecutionRepository,
	pendingRepo *deletionrepo.PendingRepository,
	stagedRepo *stagedentity.Repository,
	mergedRepo *mergedentity.Repository,
	graphEntity *graph.EntityService,
	kafkaProducer *kafka.Producer,
) *Handler {
	return &Handler{
		logger:        logger,
		strategyRepo:  strategyRepo,
		executionRepo: executionRepo,
		pendingRepo:   pendingRepo,
		stagedRepo:    stagedRepo,
		mergedRepo:    mergedRepo,
		graphEntity:   graphEntity,
		kafkaProducer: kafkaProducer,
	}
}

// HandleDeleteMessage handles an explicit delete message from Lotus
func (h *Handler) HandleDeleteMessage(ctx context.Context, msg *models.LotusDeleteMessage) error {
	ctx, span := tracing.StartSpan(ctx, "deletion.Handler.HandleDeleteMessage")
	defer span.End()

	log := h.logger.WithContext(ctx).WithFields(map[string]any{
		"tenant_id":   msg.Source.TenantID,
		"entity_type": msg.EntityType,
		"entity_id":   msg.EntityID,
	})

	log.Debug("Processing explicit delete message")

	// Find the staged entity by source ID
	entity, err := h.stagedRepo.GetBySource(ctx, msg.Source.TenantID, msg.EntityID, msg.Source.Type)
	if err != nil {
		log.WithError(err).Warn("Failed to find entity for deletion")
		return err
	}
	if entity == nil {
		log.Debug("Entity not found, skipping deletion")
		return nil
	}

	// Delete the staged entity
	if err := h.stagedRepo.Delete(ctx, msg.Source.TenantID, entity.ID); err != nil {
		return err
	}

	// Find and update merged entity
	merged, err := h.mergedRepo.GetMergedEntityByStagedID(ctx, msg.Source.TenantID, entity.ID)
	if err != nil {
		log.WithError(err).Warn("Failed to get merged entity")
	}

	if merged != nil {
		// Remove from cluster
		if err := h.mergedRepo.RemoveFromCluster(ctx, msg.Source.TenantID, merged.ID, entity.ID); err != nil {
			log.WithError(err).Warn("Failed to remove from cluster")
		}

		// Check if merged entity still has sources
		members, _ := h.mergedRepo.GetClusterMembers(ctx, msg.Source.TenantID, merged.ID)
		if len(members) == 0 {
			// No more sources, delete merged entity
			if err := h.mergedRepo.SoftDelete(ctx, msg.Source.TenantID, merged.ID); err != nil {
				log.WithError(err).Warn("Failed to soft delete merged entity")
			}

			// Delete from graph
			if h.graphEntity != nil {
				if err := h.graphEntity.Delete(ctx, msg.Source.TenantID, merged.ID, merged.EntityType); err != nil {
					log.WithError(err).Warn("Failed to delete from graph")
				}
			}

			// Emit deletion event
			if h.kafkaProducer != nil {
				h.kafkaProducer.PublishEntityEvent(ctx, &kafka.EntityEvent{
					EventType:  "deleted",
					TenantID:   msg.Source.TenantID,
					EntityID:   merged.ID.String(),
					EntityType: merged.EntityType,
					Version:    merged.Version,
				})
			}
		}
	}

	log.Info("Processed explicit delete")
	return nil
}

// HandleExecutionCompleted handles an execution completed event from Orchid
func (h *Handler) HandleExecutionCompleted(ctx context.Context, msg *models.ExecutionCompletedMessage) error {
	ctx, span := tracing.StartSpan(ctx, "deletion.Handler.HandleExecutionCompleted")
	defer span.End()

	log := h.logger.WithContext(ctx).WithFields(map[string]any{
		"tenant_id":    msg.TenantID,
		"plan_id":      msg.PlanID,
		"execution_id": msg.ExecutionID,
	})

	log.Debug("Processing execution completed event")

	// Record the execution for each entity type
	for entityType, count := range msg.EntityCounts {
		tracking := &models.ExecutionTracking{
			TenantID:    msg.TenantID,
			ExecutionID: msg.ExecutionID,
			PlanID:      msg.PlanID,
			EntityType:  entityType,
			EntityCount: count,
			StartedAt:   msg.StartedAt,
			CompletedAt: msg.CompletedAt,
		}

		if err := h.executionRepo.RecordExecution(ctx, tracking); err != nil {
			log.WithError(err).WithFields(map[string]any{"entity_type": entityType}).Warn("Failed to record execution")
		}
	}

	// Process each entity type for execution-based deletions
	sourceType := fmt.Sprintf("orchid:%s", msg.PlanID)

	for entityType := range msg.EntityCounts {
		strategy, err := h.strategyRepo.GetBySource(ctx, msg.TenantID, sourceType, entityType)
		if err != nil {
			log.WithError(err).Warn("Failed to get deletion strategy")
			continue
		}

		if strategy == nil || strategy.Strategy != models.DeletionStrategyExecutionBased || !strategy.Enabled {
			continue
		}

		// Find entities from this plan that were not seen in this execution
		if err := h.processExecutionBasedDeletions(ctx, msg.TenantID, msg.PlanID, msg.ExecutionID, entityType, strategy); err != nil {
			log.WithError(err).Warn("Failed to process execution-based deletions")
		}
	}

	log.Info("Processed execution completed event")
	return nil
}

// processExecutionBasedDeletions handles deletion logic for execution-based strategy
func (h *Handler) processExecutionBasedDeletions(ctx context.Context, tenantID, planID, executionID, entityType string, strategy *models.DeletionStrategy) error {
	ctx, span := tracing.StartSpan(ctx, "deletion.Handler.processExecutionBasedDeletions")
	defer span.End()

	log := h.logger.WithContext(ctx).WithFields(map[string]any{
		"tenant_id":    tenantID,
		"plan_id":      planID,
		"execution_id": executionID,
		"entity_type":  entityType,
	})

	// Find entities that were NOT seen in this execution but belong to this plan
	// These are entities where last_seen_execution != current execution
	query := `
		SELECT id, tenant_id, entity_type, source_id, source_type, data, fingerprint, created_at, updated_at
		FROM staged_entities
		WHERE tenant_id = $1
		  AND entity_type = $2
		  AND source_type = $3
		  AND is_deleted = false
		  AND (last_seen_execution IS NULL OR last_seen_execution != $4)
	`
	sourceType := fmt.Sprintf("orchid:%s", planID)

	var missingEntities []models.StagedEntity
	if err := h.stagedRepo.SelectRaw(ctx, &missingEntities, query, tenantID, entityType, sourceType, executionID); err != nil {
		return err
	}

	if len(missingEntities) == 0 {
		log.Debug("No missing entities found")
		return nil
	}

	log.WithFields(map[string]any{"count": len(missingEntities)}).Info("Found entities missing from execution")

	// Schedule deletions with grace period
	scheduledFor := time.Now().UTC().Add(time.Duration(strategy.GracePeriodHours) * time.Hour)

	for _, entity := range missingEntities {
		// Find merged entity if exists
		var mergedID *uuid.UUID
		merged, _ := h.mergedRepo.GetMergedEntityByStagedID(ctx, tenantID, entity.ID)
		if merged != nil {
			mergedID = &merged.ID
		}

		pending := &models.PendingDeletion{
			TenantID:       tenantID,
			StagedEntityID: entity.ID,
			MergedEntityID: mergedID,
			EntityType:     entityType,
			Reason:         models.DeletionStrategyExecutionBased,
			ScheduledFor:   scheduledFor,
		}

		if err := h.pendingRepo.Create(ctx, pending); err != nil {
			log.WithError(err).WithFields(map[string]any{"entity_id": entity.ID}).Warn("Failed to schedule deletion")
		}
	}

	return nil
}

// ProcessPendingDeletions processes all due pending deletions
func (h *Handler) ProcessPendingDeletions(ctx context.Context, tenantID string, batchSize int) (int, error) {
	ctx, span := tracing.StartSpan(ctx, "deletion.Handler.ProcessPendingDeletions")
	defer span.End()

	log := h.logger.WithContext(ctx).WithFields(map[string]any{
		"tenant_id":  tenantID,
		"batch_size": batchSize,
	})

	pending, err := h.pendingRepo.GetDue(ctx, tenantID, batchSize)
	if err != nil {
		return 0, err
	}

	if len(pending) == 0 {
		return 0, nil
	}

	log.WithFields(map[string]any{"count": len(pending)}).Debug("Processing pending deletions")

	processed := 0
	for _, p := range pending {
		if err := h.executePendingDeletion(ctx, &p); err != nil {
			log.WithError(err).WithFields(map[string]any{"id": p.ID}).Warn("Failed to execute deletion")
			continue
		}

		if err := h.pendingRepo.MarkExecuted(ctx, p.ID); err != nil {
			log.WithError(err).WithFields(map[string]any{"id": p.ID}).Warn("Failed to mark deletion executed")
		}

		processed++
	}

	log.WithFields(map[string]any{"processed": processed}).Info("Processed pending deletions")
	return processed, nil
}

// executePendingDeletion executes a single pending deletion
func (h *Handler) executePendingDeletion(ctx context.Context, pending *models.PendingDeletion) error {
	ctx, span := tracing.StartSpan(ctx, "deletion.Handler.executePendingDeletion")
	defer span.End()

	// Mark staged entity as deleted
	if err := h.stagedRepo.Delete(ctx, pending.TenantID, pending.StagedEntityID); err != nil {
		return err
	}

	// Handle merged entity
	if pending.MergedEntityID != nil {
		// Remove from cluster
		h.mergedRepo.RemoveFromCluster(ctx, pending.TenantID, *pending.MergedEntityID, pending.StagedEntityID)

		// Check remaining members
		members, _ := h.mergedRepo.GetClusterMembers(ctx, pending.TenantID, *pending.MergedEntityID)
		if len(members) == 0 {
			// Delete merged entity
			merged, _ := h.mergedRepo.Get(ctx, pending.TenantID, *pending.MergedEntityID)
			if merged != nil {
				h.mergedRepo.SoftDelete(ctx, pending.TenantID, merged.ID)

				// Delete from graph
				if h.graphEntity != nil {
					h.graphEntity.Delete(ctx, pending.TenantID, merged.ID, merged.EntityType)
				}

				// Emit event
				if h.kafkaProducer != nil {
					h.kafkaProducer.PublishEntityEvent(ctx, &kafka.EntityEvent{
						EventType:  "deleted",
						TenantID:   pending.TenantID,
						EntityID:   merged.ID.String(),
						EntityType: merged.EntityType,
						Version:    merged.Version,
					})
				}
			}
		}
	}

	return nil
}

// UpdateLastSeenExecution updates the last_seen_execution for entities in a message
func (h *Handler) UpdateLastSeenExecution(ctx context.Context, tenantID, executionID string, entityID uuid.UUID) error {
	ctx, span := tracing.StartSpan(ctx, "deletion.Handler.UpdateLastSeenExecution")
	defer span.End()

	query := `UPDATE staged_entities SET last_seen_execution = $1 WHERE tenant_id = $2 AND id = $3`
	_, err := h.stagedRepo.ExecRaw(ctx, query, executionID, tenantID, entityID)
	if err != nil {
		return err
	}

	// Cancel any pending deletion for this entity
	h.pendingRepo.CancelByEntityID(ctx, tenantID, entityID)

	return nil
}

// ParseDeleteMessage parses a Lotus message to check if it's a delete instruction
func ParseDeleteMessage(data []byte) (*models.LotusDeleteMessage, bool) {
	var msg struct {
		Action string `json:"action"`
	}

	if err := json.Unmarshal(data, &msg); err != nil {
		return nil, false
	}

	if msg.Action != "delete" {
		return nil, false
	}

	var deleteMsg models.LotusDeleteMessage
	if err := json.Unmarshal(data, &deleteMsg); err != nil {
		return nil, false
	}

	return &deleteMsg, true
}

// ParseExecutionCompletedMessage parses an execution completed event
func ParseExecutionCompletedMessage(data []byte) (*models.ExecutionCompletedMessage, error) {
	var msg models.ExecutionCompletedMessage
	if err := json.Unmarshal(data, &msg); err != nil {
		return nil, err
	}
	return &msg, nil
}

