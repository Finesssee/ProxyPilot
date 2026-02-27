package translator

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/tidwall/gjson"
)

// TestCase represents a single translation test case.
type TestCase struct {
	Name           string   // Descriptive name for the test case
	Input          []byte   // Input payload
	ExpectedOutput []byte   // Expected output payload (optional, for exact matching)
	FromFormat     Format   // Source format
	ToFormat       Format   // Target format
	Model          string   // Model name to use for translation
	Stream         bool     // Whether this is a streaming request
	ShouldFail     bool     // Whether the translation should fail
	SkipValidation bool     // Skip schema validation of output
	ValidateFields []string // Fields to validate in output (if ExpectedOutput is nil)
}

// TestResult contains the result of running a test case.
type TestResult struct {
	Name    string
	Passed  bool
	Error   error
	Input   []byte
	Output  []byte
	Details string
}

// TestSuite represents a collection of test cases.
type TestSuite struct {
	Name        string
	Description string
	Cases       []TestCase
}

// RunTestCases executes a slice of test cases against the default registry.
func RunTestCases(t *testing.T, cases []TestCase) {
	t.Helper()
	RunTestCasesWithRegistry(t, Default(), cases)
}

// RunTestCasesWithRegistry executes a slice of test cases against a specific registry.
func RunTestCasesWithRegistry(t *testing.T, registry *Registry, cases []TestCase) {
	t.Helper()

	for _, tc := range cases {
		tc := tc // capture range variable
		t.Run(tc.Name, func(t *testing.T) {
			result := runSingleTestCase(registry, tc)
			if !result.Passed {
				t.Errorf("Test case %q failed: %v\nDetails: %s", tc.Name, result.Error, result.Details)
			}
		})
	}
}

// runSingleTestCase executes a single test case and returns the result.
func runSingleTestCase(registry *Registry, tc TestCase) TestResult {
	result := TestResult{
		Name:  tc.Name,
		Input: tc.Input,
	}

	// Perform translation
	output := registry.TranslateRequest(tc.FromFormat, tc.ToFormat, tc.Model, tc.Input, tc.Stream)
	result.Output = output

	// Check if translation was expected to fail
	if tc.ShouldFail {
		// For fail cases, we check if the output equals input (passthrough means no translator)
		if string(output) == string(tc.Input) {
			result.Passed = true
			return result
		}
		result.Error = fmt.Errorf("expected translation to fail, but it succeeded")
		return result
	}

	// Validate output schema if not skipped
	if !tc.SkipValidation {
		if err := ValidateSchema(tc.ToFormat, output); err != nil {
			result.Error = fmt.Errorf("output validation failed: %w", err)
			result.Details = fmt.Sprintf("Output: %s", truncateForTest(output, 500))
			return result
		}
	}

	// If ExpectedOutput is provided, do exact comparison
	if tc.ExpectedOutput != nil {
		if !jsonEqual(output, tc.ExpectedOutput) {
			result.Error = fmt.Errorf("output does not match expected")
			result.Details = fmt.Sprintf("Expected: %s\nGot: %s",
				truncateForTest(tc.ExpectedOutput, 500),
				truncateForTest(output, 500))
			return result
		}
	}

	// If ValidateFields is provided, check those specific fields
	if len(tc.ValidateFields) > 0 {
		outputParsed := gjson.ParseBytes(output)
		for _, field := range tc.ValidateFields {
			if !outputParsed.Get(field).Exists() {
				result.Error = fmt.Errorf("expected field %q not found in output", field)
				result.Details = fmt.Sprintf("Output: %s", truncateForTest(output, 500))
				return result
			}
		}
	}

	result.Passed = true
	return result
}

// RunTestSuite executes a test suite against the default registry.
func RunTestSuite(t *testing.T, suite TestSuite) {
	t.Helper()
	t.Run(suite.Name, func(t *testing.T) {
		if suite.Description != "" {
			t.Logf("Suite: %s - %s", suite.Name, suite.Description)
		}
		RunTestCases(t, suite.Cases)
	})
}

// LoadTestCasesFromJSON loads test cases from a JSON file.
// The JSON format should be an array of test case objects.
func LoadTestCasesFromJSON(path string) ([]TestCase, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read test file: %w", err)
	}

	var rawCases []struct {
		Name           string          `json:"name"`
		Input          json.RawMessage `json:"input"`
		ExpectedOutput json.RawMessage `json:"expected_output,omitempty"`
		FromFormat     string          `json:"from_format"`
		ToFormat       string          `json:"to_format"`
		Model          string          `json:"model,omitempty"`
		Stream         bool            `json:"stream,omitempty"`
		ShouldFail     bool            `json:"should_fail,omitempty"`
		SkipValidation bool            `json:"skip_validation,omitempty"`
		ValidateFields []string        `json:"validate_fields,omitempty"`
	}

	if err := json.Unmarshal(data, &rawCases); err != nil {
		return nil, fmt.Errorf("failed to parse test file: %w", err)
	}

	cases := make([]TestCase, len(rawCases))
	for i, rc := range rawCases {
		cases[i] = TestCase{
			Name:           rc.Name,
			Input:          rc.Input,
			ExpectedOutput: rc.ExpectedOutput,
			FromFormat:     Format(rc.FromFormat),
			ToFormat:       Format(rc.ToFormat),
			Model:          rc.Model,
			Stream:         rc.Stream,
			ShouldFail:     rc.ShouldFail,
			SkipValidation: rc.SkipValidation,
			ValidateFields: rc.ValidateFields,
		}
	}

	return cases, nil
}

// LoadTestSuiteFromJSON loads a test suite from a JSON file.
func LoadTestSuiteFromJSON(path string) (*TestSuite, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read test suite file: %w", err)
	}

	var rawSuite struct {
		Name        string          `json:"name"`
		Description string          `json:"description,omitempty"`
		Cases       json.RawMessage `json:"cases"`
	}

	if err := json.Unmarshal(data, &rawSuite); err != nil {
		return nil, fmt.Errorf("failed to parse test suite file: %w", err)
	}

	cases, err := parseRawCases(rawSuite.Cases)
	if err != nil {
		return nil, err
	}

	return &TestSuite{
		Name:        rawSuite.Name,
		Description: rawSuite.Description,
		Cases:       cases,
	}, nil
}

// LoadTestCasesFromDir loads all JSON test case files from a directory.
func LoadTestCasesFromDir(dir string) ([]TestCase, error) {
	var allCases []TestCase

	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("failed to read test directory: %w", err)
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		if filepath.Ext(entry.Name()) != ".json" {
			continue
		}

		path := filepath.Join(dir, entry.Name())
		cases, err := LoadTestCasesFromJSON(path)
		if err != nil {
			return nil, fmt.Errorf("failed to load %s: %w", entry.Name(), err)
		}
		allCases = append(allCases, cases...)
	}

	return allCases, nil
}

// parseRawCases parses raw JSON cases into TestCase structs.
func parseRawCases(data json.RawMessage) ([]TestCase, error) {
	var rawCases []struct {
		Name           string          `json:"name"`
		Input          json.RawMessage `json:"input"`
		ExpectedOutput json.RawMessage `json:"expected_output,omitempty"`
		FromFormat     string          `json:"from_format"`
		ToFormat       string          `json:"to_format"`
		Model          string          `json:"model,omitempty"`
		Stream         bool            `json:"stream,omitempty"`
		ShouldFail     bool            `json:"should_fail,omitempty"`
		SkipValidation bool            `json:"skip_validation,omitempty"`
		ValidateFields []string        `json:"validate_fields,omitempty"`
	}

	if err := json.Unmarshal(data, &rawCases); err != nil {
		return nil, fmt.Errorf("failed to parse cases: %w", err)
	}

	cases := make([]TestCase, len(rawCases))
	for i, rc := range rawCases {
		cases[i] = TestCase{
			Name:           rc.Name,
			Input:          rc.Input,
			ExpectedOutput: rc.ExpectedOutput,
			FromFormat:     Format(rc.FromFormat),
			ToFormat:       Format(rc.ToFormat),
			Model:          rc.Model,
			Stream:         rc.Stream,
			ShouldFail:     rc.ShouldFail,
			SkipValidation: rc.SkipValidation,
			ValidateFields: rc.ValidateFields,
		}
	}

	return cases, nil
}

// jsonEqual compares two JSON byte slices for semantic equality.
func jsonEqual(a, b []byte) bool {
	var aVal, bVal interface{}
	if err := json.Unmarshal(a, &aVal); err != nil {
		return false
	}
	if err := json.Unmarshal(b, &bVal); err != nil {
		return false
	}

	aBytes, err := json.Marshal(aVal)
	if err != nil {
		return false
	}
	bBytes, err := json.Marshal(bVal)
	if err != nil {
		return false
	}

	return string(aBytes) == string(bBytes)
}

// truncateForTest truncates a byte slice for test output.
func truncateForTest(data []byte, maxLen int) string {
	if len(data) <= maxLen {
		return string(data)
	}
	return string(data[:maxLen]) + "...[truncated]"
}

// AssertTranslation is a helper for simple translation assertions in tests.
func AssertTranslation(t *testing.T, from, to Format, input []byte, validateFields ...string) {
	t.Helper()

	output := TranslateRequest(from, to, "", input, false)
	if output == nil {
		t.Fatalf("Translation %s -> %s returned nil", from, to)
	}

	if err := ValidateSchema(to, output); err != nil {
		t.Errorf("Output validation failed: %v\nOutput: %s", err, truncateForTest(output, 500))
	}

	if len(validateFields) > 0 {
		parsed := gjson.ParseBytes(output)
		for _, field := range validateFields {
			if !parsed.Get(field).Exists() {
				t.Errorf("Expected field %q not found in output", field)
			}
		}
	}
}

// AssertRoundtrip is a helper for roundtrip translation assertions in tests.
func AssertRoundtrip(t *testing.T, format Format, input []byte) {
	t.Helper()

	preserved, err := TestRoundtrip(format, input)
	if err != nil {
		t.Fatalf("Roundtrip test failed: %v", err)
	}
	if !preserved {
		result, _ := TestRoundtripDetailed(format, FormatOpenAI, input)
		if result != nil {
			t.Errorf("Roundtrip did not preserve data: %v", result.Differences)
		} else {
			t.Error("Roundtrip did not preserve data")
		}
	}
}

// NewTestCase creates a new TestCase with common defaults.
func NewTestCase(name string, from, to Format, input []byte) TestCase {
	return TestCase{
		Name:       name,
		Input:      input,
		FromFormat: from,
		ToFormat:   to,
	}
}

// WithExpectedOutput sets the expected output for a test case.
func (tc TestCase) WithExpectedOutput(output []byte) TestCase {
	tc.ExpectedOutput = output
	return tc
}

// WithModel sets the model for a test case.
func (tc TestCase) WithModel(model string) TestCase {
	tc.Model = model
	return tc
}

// WithStream sets the stream flag for a test case.
func (tc TestCase) WithStream(stream bool) TestCase {
	tc.Stream = stream
	return tc
}

// WithShouldFail marks a test case as expected to fail.
func (tc TestCase) WithShouldFail() TestCase {
	tc.ShouldFail = true
	return tc
}

// WithSkipValidation skips output schema validation for a test case.
func (tc TestCase) WithSkipValidation() TestCase {
	tc.SkipValidation = true
	return tc
}

// WithValidateFields sets fields to validate in the output.
func (tc TestCase) WithValidateFields(fields ...string) TestCase {
	tc.ValidateFields = fields
	return tc
}
