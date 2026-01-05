package execution

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/Gobusters/ectoerror/httperror"
	"github.com/Gobusters/ectologger"
	"github.com/google/uuid"

	"github.com/Ramsey-B/orchid/pkg/expressions"
	"github.com/Ramsey-B/orchid/pkg/kafka"
	"github.com/Ramsey-B/orchid/pkg/models"
	"github.com/Ramsey-B/orchid/pkg/repositories"
	"github.com/Ramsey-B/stem/pkg/tracing"
)

// AuthManager interface for auth token management
// Defined here to avoid circular imports
type AuthManager interface {
	GetAuthContext(ctx context.Context, authFlowID uuid.UUID, tenantID uuid.UUID, configID uuid.UUID, config map[string]any) (*AuthContext, error)
	InvalidateToken(ctx context.Context, tenantID, authFlowID, configID uuid.UUID) error
}

var (
	// ErrPlanNotFound is returned when a plan is not found
	ErrPlanNotFound = errors.New("plan not found")

	// ErrConfigNotFound is returned when a config is not found
	ErrConfigNotFound = errors.New("config not found")

	// ErrPlanDisabled is returned when trying to execute a disabled plan
	ErrPlanDisabled = errors.New("plan is disabled")

	// ErrExecutionAborted is returned when an execution is aborted due to abort_when condition
	ErrExecutionAborted = errors.New("execution aborted by abort_when condition")

	// ErrMaxLoopsExceeded is returned when the maximum loop count is exceeded
	ErrMaxLoopsExceeded = errors.New("maximum loop count exceeded")

	// ErrExecutionTimeout is returned when the execution times out
	ErrExecutionTimeout = errors.New("execution timeout exceeded")
)

const (
	// DefaultMaxExecutionTime is the default maximum execution time for a plan
	DefaultMaxExecutionTime = 5 * time.Minute

	// DefaultMaxLoops is the default maximum number of while loop iterations
	DefaultMaxLoops = 1000
)

// isNotFound checks if an error is an HTTP 404 Not Found error
func isNotFound(err error) bool {
	return httperror.IsHTTPError(err) && httperror.GetStatusCode(err) == http.StatusNotFound
}

// PlanExecutorConfig holds configuration for the plan executor
type PlanExecutorConfig struct {
	MaxExecutionTime time.Duration
	MaxLoops         int
	MaxNestingDepth  int
}

// DefaultPlanExecutorConfig returns the default configuration
func DefaultPlanExecutorConfig() PlanExecutorConfig {
	return PlanExecutorConfig{
		MaxExecutionTime: DefaultMaxExecutionTime,
		MaxLoops:         DefaultMaxLoops,
		MaxNestingDepth:  DefaultMaxNestingDepth,
	}
}

// PlanExecutionInput holds the input for plan execution
type PlanExecutionInput struct {
	PlanKey     string
	Integration string
	ConfigID    uuid.UUID
	TenantID    uuid.UUID

	// Optional: override stored context
	ContextOverride map[string]any

	// Optional: parent execution ID for sub-executions
	ParentExecutionID *uuid.UUID
}

// PlanExecutionOutput holds the result of plan execution
type PlanExecutionOutput struct {
	ExecutionID   uuid.UUID
	Status        models.ExecutionStatus
	StartedAt     time.Time
	CompletedAt   time.Time
	Duration      time.Duration
	TotalAPICalls int
	Error         error
	ErrorType     *models.ErrorType
	FinalContext  map[string]any
}

// PlanExecutor orchestrates the execution of plans
type PlanExecutor struct {
	// Repositories
	planRepo       repositories.PlanRepo
	configRepo     repositories.ConfigRepo
	authFlowRepo   repositories.AuthFlowRepo
	contextRepo    repositories.PlanContextRepo
	executionRepo  repositories.PlanExecutionRepo
	statisticsRepo repositories.PlanStatisticsRepo

	// Execution components
	stepExecutor   *StepExecutor
	fanoutExecutor *FanoutExecutor
	evaluator      *expressions.Evaluator
	authManager    AuthManager

	// External services
	kafkaProducer *kafka.Producer

	// Configuration
	config PlanExecutorConfig
	logger ectologger.Logger
}

// NewPlanExecutor creates a new plan executor
func NewPlanExecutor(
	planRepo repositories.PlanRepo,
	configRepo repositories.ConfigRepo,
	authFlowRepo repositories.AuthFlowRepo,
	contextRepo repositories.PlanContextRepo,
	executionRepo repositories.PlanExecutionRepo,
	statisticsRepo repositories.PlanStatisticsRepo,
	stepExecutor *StepExecutor,
	fanoutExecutor *FanoutExecutor,
	evaluator *expressions.Evaluator,
	authManager AuthManager,
	kafkaProducer *kafka.Producer,
	config PlanExecutorConfig,
	logger ectologger.Logger,
) *PlanExecutor {
	return &PlanExecutor{
		planRepo:       planRepo,
		configRepo:     configRepo,
		authFlowRepo:   authFlowRepo,
		contextRepo:    contextRepo,
		executionRepo:  executionRepo,
		statisticsRepo: statisticsRepo,
		stepExecutor:   stepExecutor,
		fanoutExecutor: fanoutExecutor,
		evaluator:      evaluator,
		authManager:    authManager,
		kafkaProducer:  kafkaProducer,
		config:         config,
		logger:         logger,
	}
}

// Execute executes a plan with the given input
func (e *PlanExecutor) Execute(ctx context.Context, input PlanExecutionInput) (*PlanExecutionOutput, error) {
	ctx, span := tracing.StartSpan(ctx, "PlanExecutor.Execute")
	defer span.End()

	startTime := time.Now()
	output := &PlanExecutionOutput{
		ExecutionID: uuid.New(),
		StartedAt:   startTime,
		Status:      models.ExecutionStatusPending,
	}

	e.logger.WithContext(ctx).Infof("Starting plan execution: plan=%s config=%s execution=%s",
		input.PlanKey, input.ConfigID, output.ExecutionID)

	// Create execution record
	execution := &models.PlanExecution{
		ID:                output.ExecutionID,
		TenantID:          input.TenantID,
		PlanKey:           input.PlanKey,
		ConfigID:          input.ConfigID,
		ParentExecutionID: input.ParentExecutionID,
		Status:            models.ExecutionStatusPending,
	}

	if err := e.executionRepo.Create(ctx, execution); err != nil {
		e.logger.WithContext(ctx).WithError(err).Error("Failed to create execution record")
		output.Error = fmt.Errorf("failed to create execution record: %w", err)
		output.Status = models.ExecutionStatusFailed
		return output, output.Error
	}

	// Emit execution.started lifecycle event (best-effort).
	if e.kafkaProducer != nil {
		_ = e.kafkaProducer.PublishExecutionEvent(ctx, &kafka.ExecutionEventMessage{
			Type:        "execution.started",
			TenantID:    input.TenantID.String(),
			Integration: input.Integration,
			PlanKey:     input.PlanKey,
			ConfigID:    input.ConfigID.String(),
			ExecutionID: output.ExecutionID.String(),
			Status:      "running",
			Timestamp:   startTime.UTC(),
		})
	}

	// Set up execution timeout
	execCtx := ctx
	if e.config.MaxExecutionTime > 0 {
		var cancel context.CancelFunc
		execCtx, cancel = context.WithTimeout(ctx, e.config.MaxExecutionTime)
		defer cancel()
	}

	// Execute the plan
	err := e.executePlan(execCtx, input, output)

	// Complete the execution
	output.CompletedAt = time.Now()
	output.Duration = output.CompletedAt.Sub(startTime)

	if err != nil {
		output.Error = err
		output.Status = models.ExecutionStatusFailed

		// Classify error type
		errorType := classifyError(err)
		output.ErrorType = &errorType

		// Check if it was an abort
		if errors.Is(err, ErrExecutionAborted) {
			output.Status = models.ExecutionStatusAborted
		}

		errorMsg := err.Error()
		if markErr := e.executionRepo.MarkCompleted(ctx, output.ExecutionID, output.Status, &errorMsg, output.ErrorType); markErr != nil {
			e.logger.WithContext(ctx).WithError(markErr).Error("Failed to mark execution as completed")
		}
	} else {
		output.Status = models.ExecutionStatusSuccess
		if markErr := e.executionRepo.MarkCompleted(ctx, output.ExecutionID, output.Status, nil, nil); markErr != nil {
			e.logger.WithContext(ctx).WithError(markErr).Error("Failed to mark execution as completed")
		}
	}

	// Record statistics
	durationMs := int(output.Duration.Milliseconds())
	if statsErr := e.statisticsRepo.RecordExecution(ctx, input.PlanKey, input.ConfigID, output.Status == models.ExecutionStatusSuccess, durationMs); statsErr != nil {
		e.logger.WithContext(ctx).WithError(statsErr).Warn("Failed to record execution statistics")
	}

	if output.TotalAPICalls > 0 {
		if statsErr := e.statisticsRepo.IncrementAPICalls(ctx, input.PlanKey, input.ConfigID, output.TotalAPICalls); statsErr != nil {
			e.logger.WithContext(ctx).WithError(statsErr).Warn("Failed to increment API calls statistics")
		}
	}

	e.logger.WithContext(ctx).Infof("Plan execution completed: execution=%s status=%s duration=%s api_calls=%d",
		output.ExecutionID, output.Status, output.Duration, output.TotalAPICalls)

	// Emit execution.completed lifecycle event (best-effort).
	if e.kafkaProducer != nil {
		status := string(output.Status)
		_ = e.kafkaProducer.PublishExecutionEvent(ctx, &kafka.ExecutionEventMessage{
			Type:        "execution.completed",
			TenantID:    input.TenantID.String(),
			Integration: input.Integration,
			PlanKey:     input.PlanKey,
			ConfigID:    input.ConfigID.String(),
			ExecutionID: output.ExecutionID.String(),
			Status:      status,
			Timestamp:   output.CompletedAt.UTC(),
		})
	}

	return output, err
}

// executePlan performs the actual plan execution
func (e *PlanExecutor) executePlan(ctx context.Context, input PlanExecutionInput, output *PlanExecutionOutput) error {
	ctx, span := tracing.StartSpan(ctx, "PlanExecutor.executePlan")
	defer span.End()

	// Mark execution as started
	if err := e.executionRepo.MarkStarted(ctx, output.ExecutionID); err != nil {
		return fmt.Errorf("failed to mark execution as started: %w", err)
	}
	output.Status = models.ExecutionStatusRunning

	// Load plan
	plan, err := e.planRepo.GetByKey(ctx, input.PlanKey)
	if err != nil {
		if isNotFound(err) {
			return ErrPlanNotFound
		}
		return fmt.Errorf("failed to load plan: %w", err)
	}

	if !plan.Enabled {
		return ErrPlanDisabled
	}

	// Load config
	config, err := e.configRepo.GetByID(ctx, input.ConfigID)
	if err != nil {
		if isNotFound(err) {
			return ErrConfigNotFound
		}
		return fmt.Errorf("failed to load config: %w", err)
	}

	// Parse plan definition
	planDef, err := e.parsePlanDefinition(plan)
	if err != nil {
		return fmt.Errorf("failed to parse plan definition: %w", err)
	}

	// Load or initialize context
	storedContext, err := e.loadContext(ctx, input.PlanKey, input.ConfigID)
	if err != nil && !isNotFound(err) {
		return fmt.Errorf("failed to load context: %w", err)
	}

	// Build execution context
	execCtx := NewExecutionContext()
	if storedContext != nil {
		execCtx.Context = storedContext
	}
	if input.ContextOverride != nil {
		for k, v := range input.ContextOverride {
			execCtx.Context[k] = v
		}
	}

	// Set config values
	if config.Values.Data != nil {
		execCtx.WithConfig(config.Values.Data)
	}

	// Set metadata
	execCtx.WithMeta(&ExecutionMeta{
		TenantID:    input.TenantID.String(),
		PlanKey:     input.PlanKey,
		ConfigID:    input.ConfigID.String(),
		ExecutionID: output.ExecutionID.String(),
		StepPath:    "root",
	})
	// Attach stable plan key (if provided in plan_definition JSON).
	if execCtx.Meta != nil {
		execCtx.Meta.PlanKey = planDef.Key
	}

	// Execute auth flow if specified
	if planDef.Step.AuthFlowID != "" {
		authFlowID, parseErr := uuid.Parse(planDef.Step.AuthFlowID)
		if parseErr != nil {
			return fmt.Errorf("invalid auth_flow_id: %w", parseErr)
		}

		e.logger.WithContext(ctx).Debugf("Executing auth flow %s", authFlowID)

		authCtx, authErr := e.authManager.GetAuthContext(ctx, authFlowID, input.TenantID, input.ConfigID, execCtx.Config)
		if authErr != nil {
			return fmt.Errorf("auth flow failed: %w", authErr)
		}

		execCtx.WithAuth(authCtx)
		e.logger.WithContext(ctx).Debug("Auth context obtained successfully")
	}

	// Apply plan definition limits
	maxLoops := e.config.MaxLoops
	if planDef.MaxExecutionSeconds > 0 {
		// Use plan's timeout if it's more restrictive
		planTimeout := time.Duration(planDef.MaxExecutionSeconds) * time.Second
		deadline, ok := ctx.Deadline()
		if !ok || time.Until(deadline) > planTimeout {
			var cancel context.CancelFunc
			ctx, cancel = context.WithTimeout(ctx, planTimeout)
			defer cancel()
		}
	}

	maxNesting := e.config.MaxNestingDepth
	if planDef.MaxNestingDepth > 0 && planDef.MaxNestingDepth < maxNesting {
		maxNesting = planDef.MaxNestingDepth
	}

	// Build execution options with rate limits
	execOpts := &ExecuteOptions{
		TenantID:      input.TenantID,
		IntegrationID: plan.IntegrationID,
		ConfigID:      input.ConfigID,
		RateLimits:    planDef.RateLimits,
		MaxRateWait:   60 * time.Second,
	}

	// Execute the main step (with optional while loop)
	step := &planDef.Step
	apiCalls, err := e.executeStepWithLoop(ctx, step, execCtx, maxLoops, input, output, execOpts)
	output.TotalAPICalls = apiCalls

	if err != nil {
		return err
	}

	// Save final context
	output.FinalContext = execCtx.Context
	if err := e.saveContext(ctx, input.PlanKey, input.ConfigID, execCtx.Context); err != nil {
		e.logger.WithContext(ctx).WithError(err).Warn("Failed to save execution context")
	}

	return nil
}

// executeStepWithLoop executes a step, handling while loops
func (e *PlanExecutor) executeStepWithLoop(
	ctx context.Context,
	step *models.Step,
	execCtx *ExecutionContext,
	maxLoops int,
	input PlanExecutionInput,
	output *PlanExecutionOutput,
	execOpts *ExecuteOptions,
) (int, error) {
	ctx, span := tracing.StartSpan(ctx, "PlanExecutor.executeStepWithLoop")
	defer span.End()

	loopCount := 0
	totalAPICalls := 0

	for {
		// Check context cancellation
		select {
		case <-ctx.Done():
			if ctx.Err() == context.DeadlineExceeded {
				return totalAPICalls, ErrExecutionTimeout
			}
			return totalAPICalls, ctx.Err()
		default:
		}

		// Check max loops
		loopCount++
		if loopCount > maxLoops {
			return totalAPICalls, ErrMaxLoopsExceeded
		}

		// Update metadata
		if execCtx.Meta != nil {
			execCtx.Meta.LoopCount = loopCount
		}

		// Execute the step with rate limiting
		result, err := e.stepExecutor.ExecuteWithOptions(ctx, step, execCtx, execOpts)
		if err != nil {
			return totalAPICalls, fmt.Errorf("step execution failed: %w", err)
		}

		totalAPICalls++

		hasFanout := len(step.SubSteps) > 0 && step.IterateOver != ""
		hasSubStepsNoFanout := len(step.SubSteps) > 0 && step.IterateOver == ""
		hasIterateOnly := len(step.SubSteps) == 0 && step.IterateOver != ""

		// Check for abort
		if result.ShouldAbort {
			return totalAPICalls, ErrExecutionAborted
		}

		// Check for break (exit while loop)
		if result.ShouldBreak {
			e.logger.WithContext(ctx).Debug("Breaking out of while loop due to break_when condition")
			break
		}

		// Handle sub-steps with fanout
		if hasFanout {
			fanoutResult, fanoutErr := e.fanoutExecutor.ExecuteWithOptions(ctx, step, execCtx, 0, execOpts)
			if fanoutErr != nil {
				return totalAPICalls, fmt.Errorf("fanout execution failed: %w", fanoutErr)
			}

			totalAPICalls += fanoutResult.TotalItems

			// Standard emission: 1 Kafka message per step execution, response_body is ALWAYS an array.
			// For fanout steps, response_body = []enriched items (each item has sub_step outputs appended as fields).
			items := make([]any, 0, len(fanoutResult.Results))
			forceError := false
			forceAbort := false
			for _, itemRes := range fanoutResult.Results {
				if itemRes == nil || itemRes.Context == nil {
					continue
				}
				// If any sub-step tripped ignore_on/abort_on, route the whole page to error topic.
				if itemRes.Context.Context != nil {
					if v, ok := itemRes.Context.Context["fanout_policy_error"].(bool); ok && v {
						forceError = true
					}
					if v, ok := itemRes.Context.Context["fanout_policy_abort"].(bool); ok && v {
						forceAbort = true
						forceError = true
					}
				}
				items = append(items, buildEnrichedFanoutPayload(itemRes.Context))
			}
			if emitErr := e.emitStepBatchToKafka(ctx, input, output, step, result, items, forceError); emitErr != nil {
				e.logger.WithContext(ctx).WithError(emitErr).Warn("Failed to emit response to Kafka")
			}

			if fanoutResult.AbortTriggered {
				return totalAPICalls, ErrExecutionAborted
			}
			if forceAbort {
				return totalAPICalls, ErrExecutionAborted
			}
		} else if hasSubStepsNoFanout {
			// Sub-steps without iterate_over: execute them once and append their response bodies as fields
			// onto each "item" in the main response.
			subOutputs := make(map[string]any)
			forceError := false
			forceAbort := false
			for subIdx, subStep := range step.SubSteps {
				subRes, subErr := e.stepExecutor.ExecuteWithOptions(ctx, &subStep, execCtx, execOpts)
				if subErr != nil {
					return totalAPICalls, fmt.Errorf("sub_step execution failed: %w", subErr)
				}
				totalAPICalls++

				if subRes != nil && subRes.Response != nil {
					status := subRes.Response.StatusCode
					if containsStatus(subStep.AbortOn, status) {
						forceAbort = true
						forceError = true
					} else if containsStatus(subStep.IgnoreOn, status) {
						forceError = true
					}
				}

				if subRes != nil && subRes.Response != nil && subRes.Response.BodyJSON != nil {
					key := subStep.ID
					if key == "" {
						key = fmt.Sprintf("sub_step_%d", subIdx)
					}
					subOutputs[key] = subRes.Response.BodyJSON
				}
				if subRes != nil && subRes.ShouldAbort {
					return totalAPICalls, ErrExecutionAborted
				}
			}

			items := buildItemsFromResponse(result)
			for _, it := range items {
				if m, ok := it.(map[string]any); ok && m != nil {
					for k, v := range subOutputs {
						m[k] = v
					}
				}
			}
			if emitErr := e.emitStepBatchToKafka(ctx, input, output, step, result, items, forceError); emitErr != nil {
				e.logger.WithContext(ctx).WithError(emitErr).Warn("Failed to emit response to Kafka")
			}
			if forceAbort {
				return totalAPICalls, ErrExecutionAborted
			}
		} else if hasIterateOnly {
			// Standard list-extraction: if iterate_over is provided, response_body is the evaluated item slice,
			// even when there are no sub_steps.
			items, evalErr := e.evaluator.EvaluateSlice(step.IterateOver, execCtx.ToMap())
			if evalErr != nil {
				return totalAPICalls, fmt.Errorf("failed to evaluate iterate_over: %w", evalErr)
			}
			if emitErr := e.emitStepBatchToKafka(ctx, input, output, step, result, items, false); emitErr != nil {
				e.logger.WithContext(ctx).WithError(emitErr).Warn("Failed to emit response to Kafka")
			}
		} else {
			// Non-fanout: emit the main response as an array (object wrapped as [object], arrays passed through).
			items := buildItemsFromResponse(result)
			if emitErr := e.emitStepBatchToKafka(ctx, input, output, step, result, items, false); emitErr != nil {
				e.logger.WithContext(ctx).WithError(emitErr).Warn("Failed to emit response to Kafka")
			}
		}

		// Status policy: abort_on aborts the plan after emitting to the error topic.
		if result != nil && result.Response != nil && containsStatus(step.AbortOn, result.Response.StatusCode) {
			return totalAPICalls, ErrExecutionAborted
		}

		// Check while condition for looping
		if step.While != "" {
			shouldContinue, whileErr := e.stepExecutor.EvaluateWhile(ctx, step, execCtx.ToMap())
			if whileErr != nil {
				return totalAPICalls, fmt.Errorf("while condition evaluation failed: %w", whileErr)
			}

			if !shouldContinue {
				e.logger.WithContext(ctx).Debug("Exiting while loop: condition is false")
				break
			}

			// Store previous response for next iteration
			if result.Response != nil {
				execCtx.WithPrev(&ResponseContext{
					StatusCode: result.Response.StatusCode,
					Headers:    result.Response.Headers,
					Body:       result.Response.BodyJSON,
				})
			}
		} else {
			// No while condition, execute only once
			break
		}
	}

	return totalAPICalls, nil
}

// parsePlanDefinition parses the plan definition from the plan model
func (e *PlanExecutor) parsePlanDefinition(plan *models.Plan) (*models.PlanDefinition, error) {
	if plan.PlanDefinition.Data == nil {
		return nil, errors.New("plan definition is empty")
	}

	// Serialize and deserialize to get proper PlanDefinition struct
	data, err := json.Marshal(plan.PlanDefinition.Data)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal plan definition: %w", err)
	}

	var planDef models.PlanDefinition
	if err := json.Unmarshal(data, &planDef); err != nil {
		return nil, fmt.Errorf("failed to unmarshal plan definition: %w", err)
	}

	// Apply defaults
	if planDef.MaxExecutionSeconds <= 0 {
		planDef.MaxExecutionSeconds = 300 // 5 minutes
	}
	if planDef.MaxNestingDepth <= 0 {
		planDef.MaxNestingDepth = DefaultMaxNestingDepth
	}

	return &planDef, nil
}

// loadContext loads the stored context for a plan/config combination
func (e *PlanExecutor) loadContext(ctx context.Context, planKey string, configID uuid.UUID) (map[string]any, error) {
	planContext, err := e.contextRepo.GetByPlanAndConfig(ctx, planKey, configID)
	if err != nil {
		return nil, err
	}

	if planContext.ContextData.Data != nil {
		return planContext.ContextData.Data, nil
	}

	return make(map[string]any), nil
}

// saveContext saves the execution context to the database
func (e *PlanExecutor) saveContext(ctx context.Context, planKey string, configID uuid.UUID, contextData map[string]any) error {
	planContext := &models.PlanContext{
		PlanKey:  planKey,
		ConfigID: configID,
	}
	planContext.ContextData.Data = contextData

	return e.contextRepo.Upsert(ctx, planContext)
}

// emitToKafka emits the step response to Kafka
func (e *PlanExecutor) emitStepBatchToKafka(
	ctx context.Context,
	input PlanExecutionInput,
	output *PlanExecutionOutput,
	step *models.Step,
	result *StepResult,
	items []any,
	forceError bool,
) error {
	if e.kafkaProducer == nil || step == nil {
		return nil
	}

	if step.EmitToKafka != nil && !*step.EmitToKafka {
		return nil
	}

	if result == nil || result.Response == nil {
		return nil
	}

	// Always emit response_body as a JSON array.
	body, _ := json.Marshal(items)

	stepPath := "root"
	if result.Context != nil && result.Context.Meta != nil && result.Context.Meta.StepPath != "" {
		stepPath = result.Context.Meta.StepPath
	}

	msg := &kafka.APIResponseMessage{
		TenantID:    input.TenantID.String(),
		Integration: input.Integration,
		PlanKey:     input.PlanKey,
		ConfigID:    input.ConfigID.String(),
		ExecutionID: output.ExecutionID.String(),
		StepPath:    stepPath,
		Timestamp:   time.Now(),
		RequestURL: func() string {
			if result != nil && result.RequestURL != "" {
				return result.RequestURL
			}
			return step.URL
		}(),
		RequestMethod: func() string {
			if result != nil && result.RequestMethod != "" {
				return result.RequestMethod
			}
			return step.Method
		}(),
		StatusCode:      result.Response.StatusCode,
		ResponseBody:    body,
		ResponseHeaders: result.Response.Headers,
		ResponseSize:    result.Response.ContentLength,
		DurationMs:      result.ExecutionTime.Milliseconds(),
	}

	// Status policy routing: abort_on and ignore_on go to error topic.
	status := result.Response.StatusCode
	if forceError || result.ShouldIgnore || containsStatus(step.AbortOn, status) || containsStatus(step.IgnoreOn, status) {
		return e.kafkaProducer.PublishError(ctx, msg)
	}

	return e.kafkaProducer.Publish(ctx, msg)
}

// buildEnrichedFanoutPayload merges the iterated item object with captured sub-step bodies.
func buildEnrichedFanoutPayload(itemCtx *ExecutionContext) map[string]any {
	payload := map[string]any{}
	if itemCtx != nil {
		if userObj, ok := itemCtx.Item.(map[string]any); ok && userObj != nil {
			for k, v := range userObj {
				payload[k] = v
			}
		} else if itemCtx.Item != nil {
			payload["user"] = itemCtx.Item
		}

		if itemCtx.Context != nil {
			if fanout, ok := itemCtx.Context["fanout"].(map[string]any); ok && fanout != nil {
				for k, v := range fanout {
					payload[k] = v
				}
			}
		}
	}
	return payload
}

func buildItemsFromResponse(result *StepResult) []any {
	if result == nil || result.Response == nil {
		return []any{}
	}

	switch v := result.Response.BodyJSON.(type) {
	case []any:
		return v
	case map[string]any:
		return []any{v}
	case nil:
		return []any{}
	default:
		// Scalar or other type: wrap into an object so sub_step fields can be appended consistently
		return []any{map[string]any{"value": v}}
	}
}

func containsStatus(list []int, status int) bool {
	for _, s := range list {
		if s == status {
			return true
		}
	}
	return false
}

// classifyError determines the error type for a given error
func classifyError(err error) models.ErrorType {
	if err == nil {
		return models.ErrorTypeTransient
	}

	// Check for specific error types
	if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, ErrExecutionTimeout) {
		return models.ErrorTypeTransient
	}

	if errors.Is(err, ErrMaxLoopsExceeded) {
		return models.ErrorTypePermanent
	}

	if errors.Is(err, ErrPlanNotFound) || errors.Is(err, ErrConfigNotFound) || errors.Is(err, ErrPlanDisabled) {
		return models.ErrorTypePermanent
	}

	if errors.Is(err, ErrExecutionAborted) {
		return models.ErrorTypePermanent
	}

	// Default to transient (can be retried)
	return models.ErrorTypeTransient
}
