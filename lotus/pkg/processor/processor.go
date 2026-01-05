package processor

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/Gobusters/ectologger"
	"github.com/Ramsey-B/lotus/pkg/binding"
	"github.com/Ramsey-B/lotus/pkg/kafka"
	"github.com/Ramsey-B/lotus/pkg/mapping"
	"github.com/Ramsey-B/stem/pkg/tracing"
)

func looksLikeEntityOrRelationshipRecord(v any) (map[string]any, bool) {
	obj, ok := v.(map[string]any)
	if !ok || len(obj) == 0 {
		return nil, false
	}
	if _, ok := obj["_entity_type"]; ok {
		return obj, true
	}
	if _, ok := obj["_relationship_type"]; ok {
		return obj, true
	}
	return nil, false
}

func extractBatchRecords(target map[string]any) []map[string]any {
	if target == nil {
		return nil
	}
	out := make([]map[string]any, 0)
	for _, v := range target {
		arr, ok := v.([]any)
		if !ok || len(arr) == 0 {
			continue
		}
		allRecords := true
		batch := make([]map[string]any, 0, len(arr))
		for _, el := range arr {
			obj, ok := looksLikeEntityOrRelationshipRecord(el)
			if !ok {
				allRecords = false
				break
			}
			batch = append(batch, obj)
		}
		if allRecords && len(batch) > 0 {
			out = append(out, batch...)
		}
	}
	return out
}

func (p *Processor) publishMappingError(
	ctx context.Context,
	stage string,
	msg *kafka.ReceivedMessage,
	bindingID string,
	mappingID string,
	mappingVersion int,
	err error,
) {
	if p == nil || p.producer == nil || p.config.ErrorTopic == "" || msg == nil || err == nil {
		return
	}

	tenantID := msg.Headers.TenantID
	planKey := ""
	executionID := ""
	stepPath := ""
	traceID := ""
	spanID := ""
	if msg.OrchidMessage != nil {
		if tenantID == "" {
			tenantID = msg.OrchidMessage.TenantID
		}
		planKey = msg.OrchidMessage.PlanKey
		executionID = msg.OrchidMessage.ExecutionID
		stepPath = msg.OrchidMessage.StepPath
		traceID = msg.OrchidMessage.TraceID
		spanID = msg.OrchidMessage.SpanID
	}

	errorMsg := &kafka.MappedMessage{
		Source: kafka.MessageSource{
			Type:        "lotus",
			TenantID:    tenantID,
			Key:         planKey,
			ExecutionID: executionID,
		},
		BindingID:      bindingID,
		MappingID:      mappingID,
		MappingVersion: mappingVersion,
		Timestamp:      time.Now().UTC(),
		Data: map[string]any{
			"stage":     stage,
			"error":     err.Error(),
			"step_path": stepPath,
			"input": map[string]any{
				"topic":     msg.Topic,
				"partition": msg.Partition,
				"offset":    msg.Offset,
				"key":       string(msg.Key),
				"data":      msg.Data,
			},
		},
		TraceID: traceID,
		SpanID:  spanID,
	}

	if pubErr := p.producer.PublishToTopic(ctx, p.config.ErrorTopic, errorMsg); pubErr != nil {
		p.logger.WithContext(ctx).WithError(pubErr).Error("Failed to publish mapping error message")
	}
}

// MappingLoader loads and caches compiled mapping definitions
type MappingLoader interface {
	// GetCompiledMapping returns a compiled mapping definition by ID
	GetCompiledMapping(ctx context.Context, tenantID, mappingID string) (*mapping.MappingDefinition, error)
}

// ProcessorConfig configures the message processor
type ProcessorConfig struct {
	// WorkerCount is the number of parallel processing workers
	WorkerCount int

	// ProcessTimeout is the timeout for processing a single message
	ProcessTimeout time.Duration

	// EnableMetrics enables Prometheus metrics
	EnableMetrics bool

	// ErrorTopic, when set, causes Lotus to emit mapping/processing errors to this Kafka topic.
	// If empty, Lotus will only log errors.
	ErrorTopic string

	// PassthroughTopic, when set, causes Lotus to forward lifecycle events (like execution.completed)
	// from Orchid to this topic without mapping.
	PassthroughTopic string
}

// DefaultProcessorConfig returns a ProcessorConfig with sensible defaults
func DefaultProcessorConfig() ProcessorConfig {
	return ProcessorConfig{
		WorkerCount:      4,
		ProcessTimeout:   30 * time.Second,
		EnableMetrics:    true,
		ErrorTopic:       "",
		PassthroughTopic: "",
	}
}

// TenantLoader loads bindings for a tenant on-demand
type TenantLoader interface {
	LoadTenantBindings(ctx context.Context, tenantID string) error
}

// TenantTracker tracks which tenants have been loaded
type TenantTracker interface {
	AddTenant(tenantID string)
	GetActiveTenants(ctx context.Context) ([]string, error)
}

// Processor processes incoming messages through the mapping pipeline
type Processor struct {
	config        ProcessorConfig
	matcher       *binding.Matcher
	mappingLoader MappingLoader
	producer      *kafka.Producer
	logger        ectologger.Logger

	// Optional: dynamic tenant loading
	tenantLoader  TenantLoader
	tenantTracker TenantTracker
	loadedTenants map[string]bool
	tenantMu      sync.RWMutex

	// Metrics
	messagesProcessed int64
	messagesMatched   int64
	messagesFailed    int64
	mu                sync.Mutex
}

// NewProcessor creates a new message processor
func NewProcessor(
	config ProcessorConfig,
	matcher *binding.Matcher,
	mappingLoader MappingLoader,
	producer *kafka.Producer,
	logger ectologger.Logger,
) *Processor {
	return &Processor{
		config:        config,
		matcher:       matcher,
		mappingLoader: mappingLoader,
		producer:      producer,
		logger:        logger,
		loadedTenants: make(map[string]bool),
	}
}

// SetTenantLoader sets the tenant loader for dynamic binding loading
func (p *Processor) SetTenantLoader(loader TenantLoader, tracker TenantTracker) {
	p.tenantLoader = loader
	p.tenantTracker = tracker
}

// ProcessResult contains the result of processing a message
type ProcessResult struct {
	BindingID      string
	MappingID      string
	MappingVersion int
	Success        bool
	Error          error
	Duration       time.Duration
}

// ProcessMessage processes a message through all matching bindings
func (p *Processor) ProcessMessage(ctx context.Context, msg *kafka.ReceivedMessage) ([]ProcessResult, error) {
	ctx, span := tracing.StartSpan(ctx, "processor.ProcessMessage")
	defer span.End()

	results := make([]ProcessResult, 0)

	// Orchid page-batch mode: if response_body is a JSON array, split into per-item messages
	// so bindings/mappings can treat response_body as a single record.
	if msg != nil && msg.OrchidMessage != nil {
		var arr []any
		if err := json.Unmarshal(msg.OrchidMessage.ResponseBody, &arr); err == nil && arr != nil {
			// If Orchid emitted an empty batch (response_body: []), there are no records to map.
			// Avoid running bindings/mappings against an empty slice, which produces noisy type errors
			// like "expected type string but got []interface {}" for fields like response_body.id.
			if len(arr) == 0 {
				return results, nil
			}

			all := make([]ProcessResult, 0)
			for i, item := range arr {
				// Deep copy the data map and replace response_body with the item
				dataBytes, _ := json.Marshal(msg.Data)
				var dataCopy map[string]any
				_ = json.Unmarshal(dataBytes, &dataCopy)
				dataCopy["response_body"] = item

				itemMsg := *msg
				itemMsg.Data = dataCopy
				if itemMsg.OrchidMessage != nil {
					base := itemMsg.OrchidMessage.StepPath
					if base == "" {
						base = "root.fanout"
					}
					itemMsg.OrchidMessage.StepPath = fmt.Sprintf("%s[%d]", base, i)
					itemMsg.OrchidMessage.ResponseBody, _ = json.Marshal(item)
				}

				r, err := p.ProcessMessage(ctx, &itemMsg)
				if err != nil {
					return append(all, results...), err
				}
				all = append(all, r...)
			}
			return all, nil
		}
	}

	// Get tenant ID
	tenantID := msg.Headers.TenantID
	if tenantID == "" && msg.OrchidMessage != nil {
		tenantID = msg.OrchidMessage.TenantID
	}

	// Ensure tenant bindings are loaded
	if tenantID != "" {
		if err := p.ensureTenantLoaded(ctx, tenantID); err != nil {
			p.logger.WithContext(ctx).WithError(err).
				Warnf("Failed to load bindings for tenant %s", tenantID)
			// Continue anyway - we'll just not match any bindings
		}
	}

	// Find matching bindings
	matches := p.matcher.Match(msg)
	if len(matches) == 0 {
		return results, nil
	}

	p.incrementMatched()

	// Process each matching binding
	for _, match := range matches {
		start := time.Now()
		result := ProcessResult{
			BindingID: match.Binding.ID,
			MappingID: match.Binding.MappingID,
		}

		// Load compiled mapping
		tenantID := msg.Headers.TenantID
		if tenantID == "" && msg.OrchidMessage != nil {
			tenantID = msg.OrchidMessage.TenantID
		}

		mappingDef, err := p.mappingLoader.GetCompiledMapping(ctx, tenantID, match.Binding.MappingID)
		if err != nil {
			result.Error = fmt.Errorf("failed to load mapping %s: %w", match.Binding.MappingID, err)
			p.publishMappingError(ctx, "load_mapping", msg, match.Binding.ID, match.Binding.MappingID, 0, result.Error)
			result.Duration = time.Since(start)
			results = append(results, result)
			p.incrementFailed()
			continue
		}

		result.MappingVersion = mappingDef.Version

		// Debug: log source data structure
		p.logger.WithContext(ctx).WithFields(map[string]any{
			"source_data": msg.Data,
			"mapping_id":  mappingDef.ID,
		}).Debug("Executing mapping with source data")

		// Execute mapping using pooled execution
		mappingResult, err := mappingDef.ExecuteMappingPooled(msg.Data)
		if err != nil {
			result.Error = fmt.Errorf("mapping execution failed: %w", err)
			p.publishMappingError(ctx, "execute_mapping", msg, match.Binding.ID, match.Binding.MappingID, mappingDef.Version, result.Error)
			result.Duration = time.Since(start)
			results = append(results, result)
			p.incrementFailed()
			continue
		}

		// If the mapping produced batch records (arrays of entity/relationship objects),
		// emit one Kafka message per record.
		batchRecords := extractBatchRecords(mappingResult.TargetRaw)
		if len(batchRecords) > 0 {
			for _, rec := range batchRecords {
				outputMsg := &kafka.MappedMessage{
					Source: kafka.MessageSource{
						Type:        "orchid",
						Integration: msg.OrchidMessage.Integration,
						ConfigID:    msg.OrchidMessage.ConfigID,
						TenantID:    tenantID,
					},
					BindingID:      match.Binding.ID,
					MappingID:      match.Binding.MappingID,
					MappingVersion: mappingDef.Version,
					Timestamp:      time.Now().UTC(),
					Data:           rec,
				}
				// Add tracing info
				if msg.OrchidMessage != nil {
					outputMsg.Source.Key = msg.OrchidMessage.PlanKey
					outputMsg.Source.ExecutionID = msg.OrchidMessage.ExecutionID
					outputMsg.TraceID = msg.OrchidMessage.TraceID
					outputMsg.SpanID = msg.OrchidMessage.SpanID
				}

				outputTopic := match.Binding.OutputTopic
				if outputTopic == "" {
					result.Error = fmt.Errorf("binding %s has no output_topic configured", match.Binding.ID)
					p.publishMappingError(ctx, "output_topic_missing", msg, match.Binding.ID, match.Binding.MappingID, mappingDef.Version, result.Error)
					result.Duration = time.Since(start)
					results = append(results, result)
					p.incrementFailed()
					continue
				}

				if err := p.producer.PublishToTopic(ctx, outputTopic, outputMsg); err != nil {
					result.Error = fmt.Errorf("failed to publish output: %w", err)
					p.publishMappingError(ctx, "publish_output", msg, match.Binding.ID, match.Binding.MappingID, mappingDef.Version, result.Error)
					result.Duration = time.Since(start)
					results = append(results, result)
					p.incrementFailed()
					continue
				}
			}

			// Release mapping back to pool
			mapping.ReleaseMapping(mappingResult)
			result.Success = true
			result.Duration = time.Since(start)
			results = append(results, result)
			p.incrementProcessed()
			continue
		}

		// Debug: log mapping result
		p.logger.WithContext(ctx).WithFields(map[string]any{
			"target_raw":          mappingResult.TargetRaw,
			"source_field_values": mappingResult.SourceFieldValues,
			"mapping_id":          mappingDef.ID,
		}).Debug("Mapping execution result")

		// Create output message
		outputMsg := &kafka.MappedMessage{
			Source: kafka.MessageSource{
				Type:     "orchid",
				TenantID: tenantID,
			},
			BindingID:      match.Binding.ID,
			MappingID:      match.Binding.MappingID,
			MappingVersion: mappingDef.Version,
			Timestamp:      time.Now().UTC(),
			Data:           mappingResult.TargetRaw,
		}

		// Add tracing info, integration, and config ID
		if msg.OrchidMessage != nil {
			outputMsg.Source.Integration = msg.OrchidMessage.Integration
			outputMsg.Source.Key = msg.OrchidMessage.PlanKey
			outputMsg.Source.ConfigID = msg.OrchidMessage.ConfigID
			outputMsg.Source.ExecutionID = msg.OrchidMessage.ExecutionID
			outputMsg.TraceID = msg.OrchidMessage.TraceID
			outputMsg.SpanID = msg.OrchidMessage.SpanID
		}

		// Publish to Kafka - use binding's output topic
		outputTopic := match.Binding.OutputTopic
		if outputTopic == "" {
			result.Error = fmt.Errorf("binding %s has no output_topic configured", match.Binding.ID)
			p.publishMappingError(ctx, "output_topic_missing", msg, match.Binding.ID, match.Binding.MappingID, mappingDef.Version, result.Error)
			result.Duration = time.Since(start)
			results = append(results, result)
			p.incrementFailed()
			// Release mapping back to pool
			mapping.ReleaseMapping(mappingResult)
			continue
		}

		if err := p.producer.PublishToTopic(ctx, outputTopic, outputMsg); err != nil {
			result.Error = fmt.Errorf("failed to publish output: %w", err)
			p.publishMappingError(ctx, "publish_output", msg, match.Binding.ID, match.Binding.MappingID, mappingDef.Version, result.Error)
			result.Duration = time.Since(start)
			results = append(results, result)
			p.incrementFailed()
			// Release mapping back to pool
			mapping.ReleaseMapping(mappingResult)
			continue
		}

		// Release mapping back to pool
		mapping.ReleaseMapping(mappingResult)

		result.Success = true
		result.Duration = time.Since(start)
		results = append(results, result)
		p.incrementProcessed()
	}

	return results, nil
}

// MessageHandler returns a kafka.MessageHandler for use with the consumer
func (p *Processor) MessageHandler() kafka.MessageHandler {
	return func(ctx context.Context, msg *kafka.ReceivedMessage) error {
		// Passthrough Orchid lifecycle events (execution.*) so Ivy can enforce execution-based deletion.
		if p != nil && p.producer != nil && p.config.PassthroughTopic != "" && msg != nil && msg.Data != nil {
			if t, ok := msg.Data["type"].(string); ok && len(t) > 10 && t[:10] == "execution." {
				tenantID := msg.Headers.TenantID
				planKey := msg.Headers.PlanKey
				execID := msg.Headers.ExecutionID
				if msg.OrchidMessage != nil {
					if tenantID == "" {
						tenantID = msg.OrchidMessage.TenantID
					}
					if planKey == "" {
						planKey = msg.OrchidMessage.PlanKey
					}
					if execID == "" {
						execID = msg.OrchidMessage.ExecutionID
					}
				}
				key := tenantID + ":" + execID
				headers := map[string]string{
					"tenant_id":    tenantID,
					"plan_key":     planKey,
					"execution_id": execID,
				}
				// Optional header for quick detection (Ivy also parses body)
				headers["type"] = t

				if err := p.producer.PublishRawToTopic(ctx, p.config.PassthroughTopic, key, headers, msg.Value); err != nil {
					p.logger.WithContext(ctx).WithError(err).Warn("Failed to passthrough execution event")
				}
				return nil
			}
		}

		results, err := p.ProcessMessage(ctx, msg)
		if err != nil {
			return err
		}

		// Log any processing errors (but don't fail the message)
		for _, r := range results {
			if r.Error != nil {
				p.logger.WithContext(ctx).WithError(r.Error).
					Errorf("Failed to process binding %s", r.BindingID)
			}
		}

		return nil
	}
}

// ensureTenantLoaded checks if a tenant's bindings are loaded and loads them if not
func (p *Processor) ensureTenantLoaded(ctx context.Context, tenantID string) error {
	// Check if already loaded
	p.tenantMu.RLock()
	loaded := p.loadedTenants[tenantID]
	p.tenantMu.RUnlock()

	if loaded {
		return nil
	}

	// Need to load - check if we have a tenant loader
	if p.tenantLoader == nil {
		return nil // No loader configured, skip
	}

	// Double-check with write lock
	p.tenantMu.Lock()
	defer p.tenantMu.Unlock()

	if p.loadedTenants[tenantID] {
		return nil // Another goroutine loaded it
	}

	// Load bindings for this tenant
	if err := p.tenantLoader.LoadTenantBindings(ctx, tenantID); err != nil {
		return err
	}

	// Register with tenant tracker
	if p.tenantTracker != nil {
		p.tenantTracker.AddTenant(tenantID)
	}

	p.loadedTenants[tenantID] = true

	p.logger.WithContext(ctx).WithFields(map[string]any{
		"tenant_id": tenantID,
	}).Info("Loaded bindings for new tenant")

	return nil
}

// Metrics methods

func (p *Processor) incrementProcessed() {
	p.mu.Lock()
	p.messagesProcessed++
	p.mu.Unlock()
}

func (p *Processor) incrementMatched() {
	p.mu.Lock()
	p.messagesMatched++
	p.mu.Unlock()
}

func (p *Processor) incrementFailed() {
	p.mu.Lock()
	p.messagesFailed++
	p.mu.Unlock()
}

// Stats returns processor statistics
type Stats struct {
	MessagesProcessed int64
	MessagesMatched   int64
	MessagesFailed    int64
	BindingsLoaded    int
	TenantsActive     int
}

func (p *Processor) Stats() Stats {
	p.mu.Lock()
	defer p.mu.Unlock()

	return Stats{
		MessagesProcessed: p.messagesProcessed,
		MessagesMatched:   p.messagesMatched,
		MessagesFailed:    p.messagesFailed,
		BindingsLoaded:    p.matcher.BindingCount(),
		TenantsActive:     p.matcher.TenantCount(),
	}
}
