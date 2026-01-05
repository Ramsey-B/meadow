package execution

import (
	"context"
	"fmt"
	"sync"

	"github.com/Gobusters/ectologger"
	"github.com/Ramsey-B/orchid/pkg/expressions"
	"github.com/Ramsey-B/orchid/pkg/models"
)

const (
	// DefaultConcurrency is the default number of concurrent sub-step executions
	DefaultConcurrency = 50

	// DefaultMaxNestingDepth is the default maximum nesting depth
	DefaultMaxNestingDepth = 5
)

// FanoutResult holds the results of a fanout execution
type FanoutResult struct {
	Results        []*StepResult
	Errors         []error
	TotalItems     int
	SuccessCount   int
	FailureCount   int
	AbortTriggered bool
}

// FanoutExecutor handles sub-step fanout execution
type FanoutExecutor struct {
	stepExecutor *StepExecutor
	evaluator    *expressions.Evaluator
	logger       ectologger.Logger
	maxNesting   int
}

// NewFanoutExecutor creates a new fanout executor
func NewFanoutExecutor(
	stepExecutor *StepExecutor,
	evaluator *expressions.Evaluator,
	logger ectologger.Logger,
	maxNesting int,
) *FanoutExecutor {
	if maxNesting <= 0 {
		maxNesting = DefaultMaxNestingDepth
	}
	return &FanoutExecutor{
		stepExecutor: stepExecutor,
		evaluator:    evaluator,
		logger:       logger,
		maxNesting:   maxNesting,
	}
}

// Execute executes sub-steps for each item in iterate_over
func (f *FanoutExecutor) Execute(
	ctx context.Context,
	step *models.Step,
	execCtx *ExecutionContext,
	currentNesting int,
) (*FanoutResult, error) {
	return f.ExecuteWithOptions(ctx, step, execCtx, currentNesting, nil)
}

// ExecuteWithOptions executes sub-steps with rate limiting options
func (f *FanoutExecutor) ExecuteWithOptions(
	ctx context.Context,
	step *models.Step,
	execCtx *ExecutionContext,
	currentNesting int,
	execOpts *ExecuteOptions,
) (*FanoutResult, error) {
	// Check nesting depth
	if currentNesting >= f.maxNesting {
		return nil, fmt.Errorf("maximum nesting depth exceeded: %d (max %d)", currentNesting, f.maxNesting)
	}

	// Evaluate iterate_over to get items
	if step.IterateOver == "" {
		return nil, fmt.Errorf("iterate_over is required for fanout")
	}

	data := execCtx.ToMap()
	items, err := f.evaluator.EvaluateSlice(step.IterateOver, data)
	if err != nil {
		return nil, fmt.Errorf("failed to evaluate iterate_over: %w", err)
	}

	if len(items) == 0 {
		f.logger.WithContext(ctx).Debug("No items to iterate over")
		return &FanoutResult{
			Results:      make([]*StepResult, 0),
			TotalItems:   0,
			SuccessCount: 0,
			FailureCount: 0,
		}, nil
	}

	result := &FanoutResult{
		Results:    make([]*StepResult, len(items)),
		Errors:     make([]error, 0),
		TotalItems: len(items),
	}

	// Determine concurrency
	concurrency := step.Concurrency
	if concurrency <= 0 {
		concurrency = DefaultConcurrency
	}
	if concurrency > len(items) {
		concurrency = len(items)
	}

	f.logger.WithContext(ctx).Infof("Executing fanout: %d items with concurrency %d", len(items), concurrency)

	// Create worker pool
	itemChan := make(chan indexedItem, len(items))
	resultChan := make(chan indexedResult, len(items))

	// Start workers
	var wg sync.WaitGroup
	workerCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go f.worker(workerCtx, &wg, step, execCtx, currentNesting, execOpts, itemChan, resultChan)
	}

	// Send items to workers
	go func() {
		for i, item := range items {
			select {
			case <-workerCtx.Done():
				return
			case itemChan <- indexedItem{index: i, item: item}:
			}
		}
		close(itemChan)
	}()

	// Collect results
	go func() {
		wg.Wait()
		close(resultChan)
	}()

	// Process results
	for res := range resultChan {
		result.Results[res.index] = res.result

		if res.err != nil {
			result.Errors = append(result.Errors, res.err)
			result.FailureCount++
		} else {
			result.SuccessCount++
		}

		// Check for abort
		if res.result != nil && res.result.ShouldAbort {
			result.AbortTriggered = true
			cancel() // Stop other workers
			f.logger.WithContext(ctx).Warn("Fanout aborted due to abort_when condition")
			break
		}
	}

	return result, nil
}

type indexedItem struct {
	index int
	item  any
}

type indexedResult struct {
	index  int
	result *StepResult
	err    error
}

func (f *FanoutExecutor) worker(
	ctx context.Context,
	wg *sync.WaitGroup,
	step *models.Step,
	baseCtx *ExecutionContext,
	nestingLevel int,
	execOpts *ExecuteOptions,
	items <-chan indexedItem,
	results chan<- indexedResult,
) {
	defer wg.Done()

	for item := range items {
		select {
		case <-ctx.Done():
			return
		default:
		}

		// Clone context for this item
		itemCtx := baseCtx.Clone()
		itemCtx.WithItem(item.item, item.index)

		// Update metadata
		if itemCtx.Meta != nil {
			itemCtx.Meta.NestingLevel = nestingLevel + 1
			// Provide a stable prefix Lotus can filter on (e.g., "root.fanout[0]")
			itemCtx.Meta.StepPath = fmt.Sprintf("%s.fanout[%d]", itemCtx.Meta.StepPath, item.index)
		}

		// Execute sub-steps sequentially for this item
		var lastResult *StepResult
		var lastErr error

		for subIdx, subStep := range step.SubSteps {
			// Check for nested fanout
			if subStep.IterateOver != "" && len(subStep.SubSteps) > 0 {
				fanoutResult, err := f.ExecuteWithOptions(ctx, &subStep, itemCtx, nestingLevel+1, execOpts)
				if err != nil {
					lastErr = err
					break
				}
				if fanoutResult.AbortTriggered {
					lastResult = &StepResult{ShouldAbort: true}
					break
				}
				continue
			}

			// Execute regular step with rate limiting
			result, err := f.stepExecutor.ExecuteWithOptions(ctx, &subStep, itemCtx, execOpts)
			if err != nil {
				lastErr = err
				lastResult = result
				break
			}

			lastResult = result

			// Track sub-step status policy outcomes so the parent step can route the batch to error topic / abort.
			if result != nil && result.Response != nil {
				status := result.Response.StatusCode
				if containsStatus(subStep.AbortOn, status) {
					if itemCtx.Context == nil {
						itemCtx.Context = make(map[string]any)
					}
					itemCtx.Context["fanout_policy_error"] = true
					itemCtx.Context["fanout_policy_abort"] = true
				} else if containsStatus(subStep.IgnoreOn, status) {
					if itemCtx.Context == nil {
						itemCtx.Context = make(map[string]any)
					}
					itemCtx.Context["fanout_policy_error"] = true
				}
			}

			// Capture sub-step response bodies so PlanExecutor can emit an "enriched fanout item" message later.
			// This is per-item only; it is NOT persisted to plan_contexts.
			if result != nil && result.Response != nil && result.Response.BodyJSON != nil {
				if itemCtx.Context == nil {
					itemCtx.Context = make(map[string]any)
				}
				fanout, ok := itemCtx.Context["fanout"].(map[string]any)
				if !ok || fanout == nil {
					fanout = make(map[string]any)
					itemCtx.Context["fanout"] = fanout
				}
				key := subStep.ID
				if key == "" {
					key = fmt.Sprintf("sub_step_%d", subIdx)
				}
				fanout[key] = result.Response.BodyJSON
			}

			// Handle abort
			if result.ShouldAbort {
				break
			}

			// Update context for next sub-step
			if result.Response != nil {
				itemCtx.WithResponse(result.Response)
			}
		}

		results <- indexedResult{
			index:  item.index,
			result: lastResult,
			err:    lastErr,
		}
	}
}

// ExtractIterateItems extracts items from iterate_over expression
func (f *FanoutExecutor) ExtractIterateItems(step *models.Step, data map[string]any) ([]any, error) {
	if step.IterateOver == "" {
		return nil, nil
	}
	return f.evaluator.EvaluateSlice(step.IterateOver, data)
}
