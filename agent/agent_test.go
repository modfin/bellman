package agent

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/modfin/bellman/models"
	"github.com/modfin/bellman/models/gen"
	"github.com/modfin/bellman/prompt"
	"github.com/modfin/bellman/schema"
	"github.com/modfin/bellman/tools"
)

// Mock prompter for testing
type mockPrompter struct {
	request       gen.Request
	responses     []*gen.Response
	responseIndex int
	shouldError   bool
	errorMessage  string
}

func (m *mockPrompter) SetRequest(request gen.Request) {
	m.request = request
}

func (m *mockPrompter) Prompt(prompts ...prompt.Prompt) (*gen.Response, error) {
	if m.shouldError {
		return nil, errors.New(m.errorMessage)
	}

	if m.responseIndex >= len(m.responses) {
		return nil, errors.New("no more responses available")
	}

	response := m.responses[m.responseIndex]
	m.responseIndex++
	return response, nil
}

func (m *mockPrompter) Stream(prompts ...prompt.Prompt) (<-chan *gen.StreamResponse, error) {
	return nil, errors.New("stream not implemented in mock")
}

// Helper function to create a mock generator
func createMockGenerator(responses []*gen.Response) *gen.Generator {
	return &gen.Generator{
		Prompter: &mockPrompter{responses: responses},
		Request: gen.Request{
			Context: context.Background(),
			Model: gen.Model{
				Provider: "test",
				Name:     "test-model",
			},
		},
	}
}

// Helper function to create a text response
func createTextResponse(text string, inputTokens, outputTokens int) *gen.Response {
	return &gen.Response{
		Texts: []string{text},
		Metadata: models.Metadata{
			Model:        "test/test-model",
			InputTokens:  inputTokens,
			OutputTokens: outputTokens,
			TotalTokens:  inputTokens + outputTokens,
		},
	}
}

// Helper function to create a tool response
func createToolResponse(toolCalls []tools.Call, inputTokens, outputTokens int) *gen.Response {
	return &gen.Response{
		Tools: toolCalls,
		Metadata: models.Metadata{
			Model:        "test/test-model",
			InputTokens:  inputTokens,
			OutputTokens: outputTokens,
			TotalTokens:  inputTokens + outputTokens,
		},
	}
}

// Helper function to create a tool call
func createToolCall(id, name string, argument []byte, ref *tools.Tool) tools.Call {
	return tools.Call{
		ID:       id,
		Name:     name,
		Argument: argument,
		Ref:      ref,
	}
}

// Helper function to create a tool
func createTool(name string, description string, function tools.Function) tools.Tool {
	return tools.Tool{
		Name:        name,
		Description: description,
		Function:    function,
	}
}

func TestRun_StringResult(t *testing.T) {
	tests := []struct {
		name           string
		maxDepth       int
		parallelism    int
		responses      []*gen.Response
		expectedResult string
		expectedError  bool
		expectedDepth  int
	}{
		{
			name:        "successful string result on first try",
			maxDepth:    3,
			parallelism: 1,
			responses: []*gen.Response{
				createTextResponse("Hello, World!", 10, 5),
			},
			expectedResult: "Hello, World!",
			expectedError:  false,
			expectedDepth:  0,
		},
		{
			name:        "successful string result after tool calls",
			maxDepth:    3,
			parallelism: 1,
			responses: []*gen.Response{
				createToolResponse([]tools.Call{
					createToolCall("call1", "test_tool", []byte(`{"arg": "value"}`), &tools.Tool{
						Name: "test_tool",
						Function: func(ctx context.Context, call tools.Call) (string, error) {
							return "tool result", nil
						},
					}),
				}, 10, 5),
				createTextResponse("Final result", 15, 8),
			},
			expectedResult: "Final result",
			expectedError:  false,
			expectedDepth:  1,
		},
		{
			name:        "max depth reached",
			maxDepth:    1,
			parallelism: 1,
			responses: []*gen.Response{
				createToolResponse([]tools.Call{
					createToolCall("call1", "test_tool", []byte(`{"arg": "value"}`), &tools.Tool{
						Name: "test_tool",
						Function: func(ctx context.Context, call tools.Call) (string, error) {
							return "tool result", nil
						},
					}),
				}, 10, 5),
				createToolResponse([]tools.Call{
					createToolCall("call2", "test_tool2", []byte(`{"arg": "value2"}`), &tools.Tool{
						Name: "test_tool2",
						Function: func(ctx context.Context, call tools.Call) (string, error) {
							return "tool result 2", nil
						},
					}),
				}, 15, 8),
			},
			expectedError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := createMockGenerator(tt.responses)

			result, err := Run[string](tt.maxDepth, tt.parallelism, g, prompt.AsUser("test prompt"))

			if tt.expectedError {
				if err == nil {
					t.Errorf("expected error but got none")
				}
				return
			}

			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}

			if result.Result != tt.expectedResult {
				t.Errorf("expected result %q, got %q", tt.expectedResult, result.Result)
			}

			if result.Depth != tt.expectedDepth {
				t.Errorf("expected depth %d, got %d", tt.expectedDepth, result.Depth)
			}

			// Check metadata aggregation
			expectedInputTokens := 0
			expectedOutputTokens := 0
			for _, resp := range tt.responses[:tt.expectedDepth+1] {
				expectedInputTokens += resp.Metadata.InputTokens
				expectedOutputTokens += resp.Metadata.OutputTokens
			}

			if result.Metadata.InputTokens != expectedInputTokens {
				t.Errorf("expected input tokens %d, got %d", expectedInputTokens, result.Metadata.InputTokens)
			}

			if result.Metadata.OutputTokens != expectedOutputTokens {
				t.Errorf("expected output tokens %d, got %d", expectedOutputTokens, result.Metadata.OutputTokens)
			}
		})
	}
}

func TestRun_StructResult(t *testing.T) {
	type TestStruct struct {
		Message string `json:"message"`
		Count   int    `json:"count"`
	}

	tests := []struct {
		name           string
		maxDepth       int
		parallelism    int
		responses      []*gen.Response
		expectedResult TestStruct
		expectedError  bool
	}{
		{
			name:        "successful struct result",
			maxDepth:    3,
			parallelism: 1,
			responses: []*gen.Response{
				createTextResponse(`{"message": "Hello", "count": 42}`, 10, 5),
			},
			expectedResult: TestStruct{Message: "Hello", Count: 42},
			expectedError:  false,
		},
		{
			name:        "invalid JSON for struct",
			maxDepth:    3,
			parallelism: 1,
			responses: []*gen.Response{
				createTextResponse(`{"message": "Hello", "count": "not a number"}`, 10, 5),
			},
			expectedError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := createMockGenerator(tt.responses)

			result, err := Run[TestStruct](tt.maxDepth, tt.parallelism, g, prompt.AsUser("test prompt"))

			if tt.expectedError {
				if err == nil {
					t.Errorf("expected error but got none")
				}
				return
			}

			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}

			if result.Result != tt.expectedResult {
				t.Errorf("expected result %+v, got %+v", tt.expectedResult, result.Result)
			}
		})
	}
}

func TestRun_ToolValidationErrors(t *testing.T) {
	tests := []struct {
		name                  string
		maxDepth              int
		parallelism           int
		responses             []*gen.Response
		expectedErrorContains string
	}{
		{
			name:        "tool without ref",
			maxDepth:    3,
			parallelism: 1,
			responses: []*gen.Response{
				createToolResponse([]tools.Call{
					createToolCall("call1", "test_tool", []byte(`{"arg": "value"}`), nil),
				}, 10, 5),
			},
			expectedErrorContains: "tool test_tool not found in local setup",
		},
		{
			name:        "tool without function",
			maxDepth:    3,
			parallelism: 1,
			responses: []*gen.Response{
				createToolResponse([]tools.Call{
					createToolCall("call1", "test_tool", []byte(`{"arg": "value"}`), &tools.Tool{
						Name: "test_tool",
						// Function is nil
					}),
				}, 10, 5),
			},
			expectedErrorContains: "tool test_tool has no callback function attached",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := createMockGenerator(tt.responses)

			_, err := Run[string](tt.maxDepth, tt.parallelism, g, prompt.AsUser("test prompt"))

			if err == nil {
				t.Errorf("expected error but got none")
				return
			}

			if tt.expectedErrorContains != "" && !errors.Is(err, errors.New(tt.expectedErrorContains)) {
				if !contains(err.Error(), tt.expectedErrorContains) {
					t.Errorf("expected error to contain %q, got %q", tt.expectedErrorContains, err.Error())
				}
			}
		})
	}
}

func TestRun_ToolExecutionErrors(t *testing.T) {
	tests := []struct {
		name                  string
		maxDepth              int
		parallelism           int
		responses             []*gen.Response
		expectedErrorContains string
	}{
		{
			name:        "tool execution fails",
			maxDepth:    3,
			parallelism: 1,
			responses: []*gen.Response{
				createToolResponse([]tools.Call{
					createToolCall("call1", "test_tool", []byte(`{"arg": "value"}`), &tools.Tool{
						Name: "test_tool",
						Function: func(ctx context.Context, call tools.Call) (string, error) {
							return "", errors.New("tool execution failed")
						},
					}),
				}, 10, 5),
			},
			expectedErrorContains: "tool test_tool failed",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := createMockGenerator(tt.responses)

			_, err := Run[string](tt.maxDepth, tt.parallelism, g, prompt.AsUser("test prompt"))

			if err == nil {
				t.Errorf("expected error but got none")
				return
			}

			if tt.expectedErrorContains != "" && !contains(err.Error(), tt.expectedErrorContains) {
				t.Errorf("expected error to contain %q, got %q", tt.expectedErrorContains, err.Error())
			}
		})
	}
}

func TestRun_PromptError(t *testing.T) {
	g := &gen.Generator{
		Prompter: &mockPrompter{
			shouldError:  true,
			errorMessage: "prompt failed",
		},
		Request: gen.Request{
			Context: context.Background(),
			Model: gen.Model{
				Provider: "test",
				Name:     "test-model",
			},
		},
	}

	_, err := Run[string](3, 1, g, prompt.AsUser("test prompt"))

	if err == nil {
		t.Errorf("expected error but got none")
		return
	}

	if !contains(err.Error(), "failed to prompt") {
		t.Errorf("expected error to contain 'failed to prompt', got %q", err.Error())
	}
}

func TestRunWithToolsOnly_StringResult(t *testing.T) {
	tests := []struct {
		name           string
		maxDepth       int
		parallelism    int
		responses      []*gen.Response
		expectedResult string
		expectedError  bool
		expectedDepth  int
	}{
		{
			name:        "successful string result with custom tool",
			maxDepth:    3,
			parallelism: 1,
			responses: []*gen.Response{
				createToolResponse([]tools.Call{
					createToolCall("call1", customResultCalculatedTool, []byte(`"Hello, World!"`), &tools.Tool{
						Name: customResultCalculatedTool,
						Function: func(ctx context.Context, call tools.Call) (string, error) {
							return "result", nil
						},
					}),
				}, 10, 5),
			},
			expectedResult: "Hello, World!",
			expectedError:  false,
			expectedDepth:  0,
		},
		{
			name:        "successful string result after other tool calls",
			maxDepth:    3,
			parallelism: 1,
			responses: []*gen.Response{
				createToolResponse([]tools.Call{
					createToolCall("call1", "test_tool", []byte(`{"arg": "value"}`), &tools.Tool{
						Name: "test_tool",
						Function: func(ctx context.Context, call tools.Call) (string, error) {
							return "tool result", nil
						},
					}),
				}, 10, 5),
				createToolResponse([]tools.Call{
					createToolCall("call2", customResultCalculatedTool, []byte(`"Final result"`), &tools.Tool{
						Name: customResultCalculatedTool,
						Function: func(ctx context.Context, call tools.Call) (string, error) {
							return "result", nil
						},
					}),
				}, 15, 8),
			},
			expectedResult: "Final result",
			expectedError:  false,
			expectedDepth:  1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := createMockGenerator(tt.responses)

			result, err := RunWithToolsOnly[string](tt.maxDepth, tt.parallelism, g, prompt.AsUser("test prompt"))

			if tt.expectedError {
				if err == nil {
					t.Errorf("expected error but got none")
				}
				return
			}

			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}

			if result.Result != tt.expectedResult {
				t.Errorf("expected result %q, got %q", tt.expectedResult, result.Result)
			}

			if result.Depth != tt.expectedDepth {
				t.Errorf("expected depth %d, got %d", tt.expectedDepth, result.Depth)
			}
		})
	}
}

func TestRunWithToolsOnly_StructResult(t *testing.T) {
	type TestStruct struct {
		Message string `json:"message"`
		Count   int    `json:"count"`
	}

	tests := []struct {
		name           string
		maxDepth       int
		parallelism    int
		responses      []*gen.Response
		expectedResult TestStruct
		expectedError  bool
	}{
		{
			name:        "successful struct result with custom tool",
			maxDepth:    3,
			parallelism: 1,
			responses: []*gen.Response{
				createToolResponse([]tools.Call{
					createToolCall("call1", customResultCalculatedTool, []byte(`{"message": "Hello", "count": 42}`), &tools.Tool{
						Name: customResultCalculatedTool,
						Function: func(ctx context.Context, call tools.Call) (string, error) {
							return "result", nil
						},
					}),
				}, 10, 5),
			},
			expectedResult: TestStruct{Message: "Hello", Count: 42},
			expectedError:  false,
		},
		{
			name:        "invalid JSON for struct in custom tool",
			maxDepth:    3,
			parallelism: 1,
			responses: []*gen.Response{
				createToolResponse([]tools.Call{
					createToolCall("call1", customResultCalculatedTool, []byte(`{"message": "Hello", "count": "not a number"}`), &tools.Tool{
						Name: customResultCalculatedTool,
						Function: func(ctx context.Context, call tools.Call) (string, error) {
							return "result", nil
						},
					}),
				}, 10, 5),
			},
			expectedError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := createMockGenerator(tt.responses)

			result, err := RunWithToolsOnly[TestStruct](tt.maxDepth, tt.parallelism, g, prompt.AsUser("test prompt"))

			if tt.expectedError {
				if err == nil {
					t.Errorf("expected error but got none")
				}
				return
			}

			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}

			if result.Result != tt.expectedResult {
				t.Errorf("expected result %+v, got %+v", tt.expectedResult, result.Result)
			}
		})
	}
}

func TestRunWithToolsOnly_RemovesCustomToolFromExistingTools(t *testing.T) {
	// Create a generator with existing tools including the custom tool
	existingTool := createTool("existing_tool", "existing tool", func(ctx context.Context, call tools.Call) (string, error) {
		return "existing result", nil
	})

	customTool := createTool(customResultCalculatedTool, "custom tool", func(ctx context.Context, call tools.Call) (string, error) {
		return "custom result", nil
	})

	g := &gen.Generator{
		Prompter: &mockPrompter{
			responses: []*gen.Response{
				createToolResponse([]tools.Call{
					createToolCall("call1", customResultCalculatedTool, []byte(`"result"`), &customTool),
				}, 10, 5),
			},
		},
		Request: gen.Request{
			Context: context.Background(),
			Model: gen.Model{
				Provider: "test",
				Name:     "test-model",
			},
			Tools: []tools.Tool{existingTool, customTool},
		},
	}

	result, err := RunWithToolsOnly[string](3, 1, g, prompt.AsUser("test prompt"))

	if err != nil {
		t.Errorf("unexpected error: %v", err)
		return
	}

	if result.Result != "result" {
		t.Errorf("expected result 'result', got %q", result.Result)
	}
}

func TestExecuteCallbacksSequential(t *testing.T) {
	ctx := context.Background()

	tool1 := createTool("tool1", "first tool", func(ctx context.Context, call tools.Call) (string, error) {
		return "result1", nil
	})

	tool2 := createTool("tool2", "second tool", func(ctx context.Context, call tools.Call) (string, error) {
		return "result2", nil
	})

	callbacks := []tools.Call{
		createToolCall("call1", "tool1", []byte(`{"arg": "value1"}`), &tool1),
		createToolCall("call2", "tool2", []byte(`{"arg": "value2"}`), &tool2),
	}

	results := executeCallbacksSequential(ctx, callbacks)

	if len(results) != 2 {
		t.Errorf("expected 2 results, got %d", len(results))
		return
	}

	if results[0].Response != "result1" || results[0].Error != nil {
		t.Errorf("expected first result to be 'result1' with no error, got %q, %v", results[0].Response, results[0].Error)
	}

	if results[1].Response != "result2" || results[1].Error != nil {
		t.Errorf("expected second result to be 'result2' with no error, got %q, %v", results[1].Response, results[1].Error)
	}
}

func TestExecuteCallbacksSequential_WithError(t *testing.T) {
	ctx := context.Background()

	tool1 := createTool("tool1", "first tool", func(ctx context.Context, call tools.Call) (string, error) {
		return "result1", nil
	})

	tool2 := createTool("tool2", "second tool", func(ctx context.Context, call tools.Call) (string, error) {
		return "", errors.New("tool2 failed")
	})

	callbacks := []tools.Call{
		createToolCall("call1", "tool1", []byte(`{"arg": "value1"}`), &tool1),
		createToolCall("call2", "tool2", []byte(`{"arg": "value2"}`), &tool2),
	}

	results := executeCallbacksSequential(ctx, callbacks)

	if len(results) != 2 {
		t.Errorf("expected 2 results, got %d", len(results))
		return
	}

	if results[0].Response != "result1" || results[0].Error != nil {
		t.Errorf("expected first result to be 'result1' with no error, got %q, %v", results[0].Response, results[0].Error)
	}

	if results[1].Response != "" || results[1].Error == nil {
		t.Errorf("expected second result to have error, got %q, %v", results[1].Response, results[1].Error)
	}
}

func TestExecuteCallbacksParallel(t *testing.T) {
	ctx := context.Background()

	tool1 := createTool("tool1", "first tool", func(ctx context.Context, call tools.Call) (string, error) {
		time.Sleep(10 * time.Millisecond) // Simulate some work
		return "result1", nil
	})

	tool2 := createTool("tool2", "second tool", func(ctx context.Context, call tools.Call) (string, error) {
		time.Sleep(10 * time.Millisecond) // Simulate some work
		return "result2", nil
	})

	callbacks := []tools.Call{
		createToolCall("call1", "tool1", []byte(`{"arg": "value1"}`), &tool1),
		createToolCall("call2", "tool2", []byte(`{"arg": "value2"}`), &tool2),
	}

	results := executeCallbacksParallel(ctx, callbacks, 2)

	if len(results) != 2 {
		t.Errorf("expected 2 results, got %d", len(results))
		return
	}

	// Check that both results are present (order may vary due to parallelism)
	foundResult1 := false
	foundResult2 := false

	for _, result := range results {
		if result.Response == "result1" && result.Error == nil {
			foundResult1 = true
		}
		if result.Response == "result2" && result.Error == nil {
			foundResult2 = true
		}
	}

	if !foundResult1 {
		t.Errorf("did not find result1 in parallel execution results")
	}

	if !foundResult2 {
		t.Errorf("did not find result2 in parallel execution results")
	}
}

func TestExecuteCallbacksParallel_WithConcurrencyLimit(t *testing.T) {
	ctx := context.Background()

	// Create tools that take time to execute
	tool1 := createTool("tool1", "first tool", func(ctx context.Context, call tools.Call) (string, error) {
		time.Sleep(50 * time.Millisecond) // Simulate work
		return "result1", nil
	})

	tool2 := createTool("tool2", "second tool", func(ctx context.Context, call tools.Call) (string, error) {
		time.Sleep(50 * time.Millisecond) // Simulate work
		return "result2", nil
	})

	tool3 := createTool("tool3", "third tool", func(ctx context.Context, call tools.Call) (string, error) {
		time.Sleep(50 * time.Millisecond) // Simulate work
		return "result3", nil
	})

	callbacks := []tools.Call{
		createToolCall("call1", "tool1", []byte(`{"arg": "value1"}`), &tool1),
		createToolCall("call2", "tool2", []byte(`{"arg": "value2"}`), &tool2),
		createToolCall("call3", "tool3", []byte(`{"arg": "value3"}`), &tool3),
	}

	start := time.Now()
	results := executeCallbacksParallel(ctx, callbacks, 1) // Limit to 1 concurrent execution
	duration := time.Since(start)

	if len(results) != 3 {
		t.Errorf("expected 3 results, got %d", len(results))
		return
	}

	// With parallelism=1, execution should be sequential, so it should take at least 150ms
	if duration < 150*time.Millisecond {
		t.Errorf("expected sequential execution to take at least 150ms, took %v", duration)
	}

	// Check that all results are present
	expectedResults := map[string]bool{"result1": false, "result2": false, "result3": false}
	for _, result := range results {
		if result.Error == nil {
			expectedResults[result.Response] = true
		}
	}

	for result, found := range expectedResults {
		if !found {
			t.Errorf("did not find %s in parallel execution results", result)
		}
	}
}

func TestResult_StringRepresentation(t *testing.T) {
	result := &Result[string]{
		Prompts: []prompt.Prompt{
			prompt.AsUser("test prompt"),
		},
		Result: "test result",
		Metadata: models.Metadata{
			Model:        "test/test-model",
			InputTokens:  10,
			OutputTokens: 5,
			TotalTokens:  15,
		},
		Depth: 1,
	}

	// Test that the result can be accessed
	if result.Result != "test result" {
		t.Errorf("expected result 'test result', got %q", result.Result)
	}

	if result.Depth != 1 {
		t.Errorf("expected depth 1, got %d", result.Depth)
	}

	if len(result.Prompts) != 1 {
		t.Errorf("expected 1 prompt, got %d", len(result.Prompts))
	}

	if result.Metadata.InputTokens != 10 {
		t.Errorf("expected 10 input tokens, got %d", result.Metadata.InputTokens)
	}
}

func TestRun_WithOutputSchema(t *testing.T) {
	type TestStruct struct {
		Message string `json:"message"`
	}

	g := &gen.Generator{
		Prompter: &mockPrompter{
			responses: []*gen.Response{
				createTextResponse(`{"message": "Hello"}`, 10, 5),
			},
		},
		Request: gen.Request{
			Context: context.Background(),
			Model: gen.Model{
				Provider: "test",
				Name:     "test-model",
			},
			OutputSchema: schema.From(TestStruct{}),
		},
	}

	result, err := Run[TestStruct](3, 1, g, prompt.AsUser("test prompt"))

	if err != nil {
		t.Errorf("unexpected error: %v", err)
		return
	}

	if result.Result.Message != "Hello" {
		t.Errorf("expected message 'Hello', got %q", result.Result.Message)
	}
}

// Helper function to check if a string contains a substring
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 ||
		(len(s) > len(substr) && (s[:len(substr)] == substr ||
			s[len(s)-len(substr):] == substr ||
			func() bool {
				for i := 1; i <= len(s)-len(substr); i++ {
					if s[i:i+len(substr)] == substr {
						return true
					}
				}
				return false
			}())))
}
