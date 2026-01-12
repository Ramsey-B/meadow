package runner

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"gopkg.in/yaml.v3"
)

// TestDefinition represents a YAML test file
type TestDefinition struct {
	Name        string                   `yaml:"name"`
	Description string                   `yaml:"description"`
	Services    []string                 `yaml:"services"`
	Setup       []map[string]interface{} `yaml:"setup"`
	Steps       []map[string]interface{} `yaml:"steps"`
	Cleanup     []map[string]interface{} `yaml:"cleanup"`
}

// Config holds the configuration for running tests
type Config struct {
	TestFiles    []string
	DryRun       bool
	Verbose      bool
	ShowFailures bool // Show failure details without verbose step output
	ReportFormat string
	Parallel     int // Number of parallel workers (0 = sequential)

	// Service URLs
	OrchidURL string
	LotusURL  string
	IvyURL    string
	MocksURL  string

	// Kafka configuration
	KafkaBrokers []string

	// Test configuration
	TestTenant string
}

// Result holds the test execution results
type Result struct {
	Total  int
	Passed int
	Failed int
	Tests  []TestResult
}

// TestResult holds results for a single test
type TestResult struct {
	Name     string
	FilePath string
	Passed   bool
	Error    string
}

// Run executes the test suite
func Run(config Config) (*Result, error) {
	result := &Result{
		Tests: make([]TestResult, 0),
		Total: len(config.TestFiles),
	}

	// Use parallel execution if configured
	if config.Parallel > 1 {
		return runParallel(config, result)
	}

	return runSequential(config, result)
}

// runSequential executes tests one at a time
func runSequential(config Config, result *Result) (*Result, error) {
	ctx := context.Background()
	testCtx := NewTestContext(ctx, config)

	// Load helpers (fixtures and templates)
	if err := loadHelpers(testCtx); err != nil {
		return nil, fmt.Errorf("failed to load helpers: %w", err)
	}

	// Run each test file
	for _, file := range config.TestFiles {
		testResult := runSingleTest(config, file)
		if testResult.Passed {
			result.Passed++
		} else {
			result.Failed++
		}
		result.Tests = append(result.Tests, testResult)
	}

	return result, nil
}

// runParallel executes tests concurrently with a worker pool
func runParallel(config Config, result *Result) (*Result, error) {
	numWorkers := config.Parallel
	if numWorkers > len(config.TestFiles) {
		numWorkers = len(config.TestFiles)
	}

	// Create channels
	jobs := make(chan string, len(config.TestFiles))
	results := make(chan TestResult, len(config.TestFiles))

	// Start workers
	var wg sync.WaitGroup
	for i := 0; i < numWorkers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for file := range jobs {
				testResult := runSingleTest(config, file)
				results <- testResult
			}
		}()
	}

	// Send jobs
	for _, file := range config.TestFiles {
		jobs <- file
	}
	close(jobs)

	// Wait for workers and close results
	go func() {
		wg.Wait()
		close(results)
	}()

	// Collect results
	for testResult := range results {
		if testResult.Passed {
			result.Passed++
		} else {
			result.Failed++
		}
		result.Tests = append(result.Tests, testResult)
	}

	return result, nil
}

// runSingleTest executes a single test file
func runSingleTest(config Config, file string) TestResult {
	ctx := context.Background()
	testCtx := NewTestContext(ctx, config)

	// Always clean up tenant data at the end, regardless of success/failure
	defer testCtx.CleanupTenant()

	testResult := TestResult{
		FilePath: file,
	}

	// Load helpers
	if err := loadHelpers(testCtx); err != nil {
		testResult.Passed = false
		testResult.Error = fmt.Sprintf("Failed to load helpers: %v", err)
		fmt.Printf("✗ FAILED: %s\n", file)
		return testResult
	}

	test, err := loadTest(file)
	if err != nil {
		testResult.Passed = false
		testResult.Error = fmt.Sprintf("Failed to load test: %v", err)
		fmt.Printf("✗ FAILED: %s\n", file)
		if config.Verbose || config.ShowFailures {
			fmt.Printf("  Error: %v\n\n", err)
		}
		return testResult
	}

	testResult.Name = test.Name

	if config.DryRun {
		fmt.Printf("✓ [DRY-RUN] %s (%s)\n", test.Name, file)
		testResult.Passed = true
		return testResult
	}

	// Run the test
	fmt.Printf("▶ Running: %s\n", test.Name)
	if test.Description != "" && config.Verbose {
		fmt.Printf("  Description: %s\n", test.Description)
	}

	if err := runTest(testCtx, test); err != nil {
		fmt.Printf("✗ FAILED: %s\n", test.Name)
		if config.Verbose || config.ShowFailures {
			fmt.Printf("  Error: %v\n\n", err)
		}
		testResult.Passed = false
		testResult.Error = err.Error()
	} else {
		fmt.Printf("✓ PASSED: %s\n", test.Name)
		testResult.Passed = true
	}

	return testResult
}

// loadTest loads a test definition from a YAML file
func loadTest(filePath string) (*TestDefinition, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read file: %w", err)
	}

	var test TestDefinition
	if err := yaml.Unmarshal(data, &test); err != nil {
		return nil, fmt.Errorf("failed to parse YAML: %w", err)
	}

	return &test, nil
}

// loadHelpers loads fixtures and templates from the helpers directory
func loadHelpers(testCtx *TestContext) error {
	helpersDir := "tests/helpers"

	// Check if helpers directory exists
	if _, err := os.Stat(helpersDir); os.IsNotExist(err) {
		// No helpers directory, that's OK
		return nil
	}

	// Load fixtures.yaml
	fixturesPath := filepath.Join(helpersDir, "fixtures.yaml")
	if _, err := os.Stat(fixturesPath); err == nil {
		data, err := os.ReadFile(fixturesPath)
		if err != nil {
			return fmt.Errorf("failed to read fixtures: %w", err)
		}

		var fixturesFile struct {
			Fixtures map[string]interface{} `yaml:"fixtures"`
		}
		if err := yaml.Unmarshal(data, &fixturesFile); err != nil {
			return fmt.Errorf("failed to parse fixtures: %w", err)
		}

		testCtx.LoadFixtures(fixturesFile.Fixtures)
	}

	// Load templates.yaml
	templatesPath := filepath.Join(helpersDir, "templates.yaml")
	if _, err := os.Stat(templatesPath); err == nil {
		data, err := os.ReadFile(templatesPath)
		if err != nil {
			return fmt.Errorf("failed to read templates: %w", err)
		}

		var templatesFile struct {
			Templates map[string]interface{} `yaml:"templates"`
		}
		if err := yaml.Unmarshal(data, &templatesFile); err != nil {
			return fmt.Errorf("failed to parse templates: %w", err)
		}

		testCtx.LoadTemplates(templatesFile.Templates)
	}

	return nil
}

// runTest executes a single test
func runTest(testCtx *TestContext, test *TestDefinition) error {
	// Run setup steps
	for i, step := range test.Setup {
		if err := executeStep(testCtx, step, fmt.Sprintf("setup[%d]", i)); err != nil {
			return fmt.Errorf("setup failed at step %d: %w", i, err)
		}
	}

	// Run test steps
	for i, step := range test.Steps {
		if err := executeStep(testCtx, step, fmt.Sprintf("step[%d]", i)); err != nil {
			// Run cleanup even on failure
			runCleanup(testCtx, test.Cleanup)
			return fmt.Errorf("test failed at step %d: %w", i, err)
		}
	}

	// Run cleanup
	if err := runCleanup(testCtx, test.Cleanup); err != nil {
		return fmt.Errorf("cleanup failed: %w", err)
	}

	return nil
}

// runCleanup runs cleanup steps (always runs all, even if one fails)
func runCleanup(testCtx *TestContext, cleanup []map[string]interface{}) error {
	var firstErr error
	for i, step := range cleanup {
		if err := executeStep(testCtx, step, fmt.Sprintf("cleanup[%d]", i)); err != nil {
			if firstErr == nil {
				firstErr = err
			}
			testCtx.Error("Cleanup step %d failed: %v", i, err)
		}
	}
	return firstErr
}
