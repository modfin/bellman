package bellman

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"math"
	"os"
	"testing"

	"github.com/dop251/goja"
	"github.com/joho/godotenv"
	"github.com/modfin/bellman/agent"
	"github.com/modfin/bellman/prompt"
	"github.com/modfin/bellman/services/openai"
	"github.com/modfin/bellman/services/vertexai"
	"github.com/modfin/bellman/services/vllm"
	"github.com/modfin/bellman/tools"
)

func TestMockPTC(t *testing.T) {
	// get env vars
	err := godotenv.Load()
	if err != nil {
		log.Fatal("Error loading .env file")
	}
	bellmanUrl := os.Getenv("BELLMAN_URL")
	bellmanToken := os.Getenv("BELLMAN_TOKEN")

	// setup goja
	vm := goja.New()
	vm.Set("CONFIG", map[string]string{
		"token": bellmanToken,
		"url":   bellmanUrl,
	})
	vm.Set("goLog", func(msg string) { // enables logging to go env
		fmt.Printf("[JS-LOG]: %s\n", msg)
	})

	// define go func "tools" for JS use
	askBellman := func(url, token, userMessage string) string {
		client := New(url, Key{Name: "test", Token: token})
		llm := client.Generator()
		res, _ := llm.Model(openai.GenModel_gpt4o_mini).Temperature(1).
			Prompt(
				//prompt.AsUser(userMessage),
				prompt.AsUser("tell me a (very short) Bellman joke in English (Swedish Bellman joke)!"),
			)
		text, _ := res.AsText()
		return text
	}
	// add the "tool"
	vm.Set("askBellman", askBellman)
	// add another "tool"
	Sum := func(a, b int) int {
		return a + b
	}
	vm.Set("Sum", Sum)

	// define PTC tool, input args and func
	type Args struct {
		Code string `json:"code" json-description:"Executable JavaScript source code ONLY. Should use available JS Functions (Environment). Do not include explanations or reasoning."`
	}

	var codeExecutor tools.Function = func(ctx context.Context, call tools.Call) (string, error) {
		// extract tool-call arguments
		var arg Args
		err := json.Unmarshal(call.Argument, &arg)
		if err != nil {
			return "", err
		}

		//fmt.Println("##### JS exec code:\n", arg.Code)

		// run JS code TODO: time limit for loops?
		res, err := vm.RunString(arg.Code)
		if err != nil {
			return fmt.Sprintf(`{"error": %q}`, err.Error()), fmt.Errorf(`{"error": %q}`, err.Error())
		}

		// marshall res into valid JSON
		jsonBytes, err := json.Marshal(res.Export())
		if err != nil {
			return "", err
		}

		return string(jsonBytes), nil
	}

	codeExecution := tools.NewTool("code_execution",
		tools.WithDescription(
			"MANDATORY: You must write executable JavaScript code. "+
				"Executes JS code. Environment has: Sum(a,b), askBellman(url,token,prompt), and CONFIG object. "+
				"Combine all required tool calls into ONE script and return an object with all results.",
		),
		tools.WithArgSchema(Args{}),
		tools.WithFunction(codeExecutor),
	)

	const systemPrompt = `## Role
You are a Financial Assistant. Today is 2026-02-03.

## Capabilities
You solve complex logic by writing JavaScript code for the code_execution tool.

## Rules for code_execution
1. CALL LIMIT: You may call code_execution ONLY ONCE per turn. 
2. LOGIC: Perform all calculations and multi-tool logic INSIDE the JS script. Before calling code_execution, plan how to combine all tasks into a single script. You are penalized for making more than one tool call.
3. RETURN: The JS script MUST end with an object containing all final data.
4. FORMAT: Do not use console.log for final data; the last evaluated expression is the return value.
6. SYNTHESIS: Once you have the result from code_execution, you have everything you need. 
7. TERMINATION: Do not call the tool again with the same or modified code.
8. SYNC: Do not use async functions unless specified.

## Example JS Script Input
({
  joke: askBellman(CONFIG.url, CONFIG.token, ""),
  total: Sum(123, 456)
})`

	// create Bellman llm and run agent
	client := New(bellmanUrl, Key{Name: "test", Token: bellmanToken})
	llm := client.Generator().Model(openai.GenModel_gpt4o_mini).
		System(systemPrompt).
		SetTools(codeExecution).Temperature(0)

	const useGemini = true // quick-swap provider (gemini separate agent implementation)
	if useGemini {
		llm = llm.Model(vertexai.GenModel_gemini_2_5_flash_latest)
	}

	// prompt, expected result --> <bad-bellman-joke> and 2222222211
	userPrompt := "Tell me a Bellman joke and sum the numbers 1234567890 and 0987654321."
	type Result struct {
		Text string `json:"text" json-description:"The final natural text answer to the user's request."`
	}
	var res *agent.Result[Result]
	switch llm.Request.Model.Provider {
	case vertexai.Provider:
		res, err = agent.RunWithToolsOnly[Result](10, 0, llm, prompt.AsUser(userPrompt))
	default:
		res, err = agent.Run[Result](10, 0, llm, prompt.AsUser(userPrompt))
	}

	if err != nil {
		log.Fatalf("Prompt() error = %v", err)
	}

	// pretty print
	fmt.Printf("==== %s ====\n", res.Metadata.Model)
	fmt.Printf("==== Result after %d calls ====\n", res.Depth)
	fmt.Printf("%+v\n", res.Result.Text)
	fmt.Printf("==== Conversation ====\n")

	for _, p := range res.Prompts {
		switch p.Role {
		case prompt.ToolCallRole:
			fmt.Printf("%s: %s\n", p.Role, *p.ToolCall)
		case prompt.ToolResponseRole:
			fmt.Printf("%s: %s\n", p.Role, *p.ToolResponse)
		default:
			fmt.Printf("%s: %s\n", p.Role, p.Text)
		}
	}
}

func TestAgent(t *testing.T) {
	// get env vars
	err := godotenv.Load()
	if err != nil {
		log.Fatal("Error loading .env file")
	}

	bellmanUrl := os.Getenv("BELLMAN_URL")
	bellmanToken := os.Getenv("BELLMAN_TOKEN")

	// define go func for JS use
	askBellman := func(url, token, userMessage string) string {
		client := New(url, Key{Name: "test", Token: token})
		llm := client.Generator()
		res, _ := llm.Model(vllm.GenModel_gpt_oss_20b).
			Prompt(
				prompt.AsUser(userMessage),
			)
		text, _ := res.AsText()
		return text
	}

	// goja stuff
	vm := goja.New()
	vm.Set("CONFIG", map[string]string{
		"token": bellmanToken,
		"url":   bellmanUrl,
	})
	vm.Set("askBellman", askBellman)
	vm.Set("goLog", func(msg string) {
		fmt.Printf("[JS-LOG]: %s\n", msg)
	})

	script := `
		//goLog("Asking Bellman...");
		var result = askBellman(CONFIG.url, CONFIG.token, "What company made you?");
		//goLog("Answer is: " + result);
		result; // Return the result to Go
	`

	_, err = vm.RunString(script)
	if err != nil {
		panic(err)
	}

	//fmt.Printf("Final value returned to Go: %v\n", val.Export())

	// define tool
	type Args struct {
		Name string `json:"name" json-description:"the name of the company"`
	}

	var fun tools.Function = func(ctx context.Context, call tools.Call) (string, error) {
		var arg Args
		err := json.Unmarshal(call.Argument, &arg)
		if err != nil {
			return "", err
		}
		return `{"result": 6969696969}`, nil
	}

	getEarnings := tools.NewTool("get_earnings",
		tools.WithDescription(
			"a function to get company earnings by name.",
		),
		tools.WithArgSchema(Args{}),
		tools.WithFunction(fun),
	)

	client := New(bellmanUrl, Key{Name: "test", Token: bellmanToken})

	//llm := client.Generator().Model(vertexai.GenModel_gemini_2_5_flash_latest).
	llm := client.Generator().Model(openai.GenModel_gpt4o_mini).
		SetTools(getEarnings).System("You are a financial assistant. For context, today is 2026-02-02. You can get company earnings by name using the get_earnings() tool.")

	// prompt
	userPrompt := "What are the earnings for company 'LKAB'?"
	type Result struct {
		Result int `json:"result"`
	}
	var res *agent.Result[Result]
	switch llm.Request.Model.Provider {
	case vertexai.Provider:
		res, err = agent.RunWithToolsOnly[Result](10, 1, llm, prompt.AsUser(userPrompt))
	default:
		res, err = agent.RunWithToolsOnly[Result](10, 1, llm, prompt.AsUser(userPrompt))
	}

	if err != nil {
		log.Fatalf("Prompt() error = %v", err)
	}

	// pretty print
	fmt.Printf("==== Result after %d calls ====\n", res.Depth)
	fmt.Printf("%+v\n", res.Result)
	fmt.Printf("==== Conversation ====\n")

	for _, p := range res.Prompts {
		fmt.Printf("%s: %s\n", p.Role, p.Text)
	}
}

func TestTool(t *testing.T) {
	// get env vars
	err := godotenv.Load()
	if err != nil {
		log.Fatal("Error loading .env file")
	}

	bellmanUrl := os.Getenv("BELLMAN_URL")
	bellmanToken := os.Getenv("BELLMAN_TOKEN")

	// define go func for JS use
	askBellman := func(url, token, userMessage string) string {
		client := New(url, Key{Name: "test", Token: token})
		llm := client.Generator()
		res, _ := llm.Model(vllm.GenModel_gpt_oss_20b).
			Prompt(
				prompt.AsUser(userMessage),
			)
		text, _ := res.AsText()
		return text
	}

	// goja stuff
	vm := goja.New()
	vm.Set("CONFIG", map[string]string{
		"token": bellmanToken,
		"url":   bellmanUrl,
	})
	vm.Set("askBellman", askBellman)
	vm.Set("goLog", func(msg string) {
		fmt.Printf("[JS-LOG]: %s\n", msg)
	})

	script := `
		//goLog("Asking Bellman...");
		var result = askBellman(CONFIG.url, CONFIG.token, "What company made you?");
		//goLog("Answer is: " + result);
		result; // Return the result to Go
	`

	_, err = vm.RunString(script)
	if err != nil {
		panic(err)
	}

	//fmt.Printf("Final value returned to Go: %v\n", val.Export())

	// define tool
	type Args struct {
		Name string `json:"name"`
	}

	var fun tools.Function = func(ctx context.Context, call tools.Call) (string, error) {
		var arg Args
		err := json.Unmarshal(call.Argument, &arg)
		if err != nil {
			return "", err
		}
		return arg.Name, nil
	}

	getEarnings := tools.NewTool("get_earnings",
		tools.WithDescription(
			"a function to get company earnings by name",
		),
		tools.WithArgSchema(Args{}),
		tools.WithFunction(fun),
	)

	client := New(bellmanUrl, Key{Name: "test", Token: bellmanToken})
	llm := client.Generator().ThinkingBudget(0)
	res, err := llm.Model(vertexai.GenModel_gemini_2_5_flash_lite_latest).
		System("You are a financial assistant. You can get company earnings by name using the get_earnings() tool.").
		//SetTools(getEarnings).
		AddTools(getEarnings).
		SetToolConfig(tools.RequiredTool).
		Prompt(
			prompt.AsUser("What are the earnings for company 'LKAB'?"),
		)

	if err != nil {
		log.Fatalf("Prompt() error = %v", err)
	}

	err = res.Eval(context.Background()) // TODO: needs context?
	if err != nil {
		log.Fatalf("Eval() error = %v", err)
	}

	// print res?
	text, _ := res.AsText()
	fmt.Println("final response: ", text, res)

	for _, tool := range res.Tools {
		response, _ := tool.Ref.Function(context.Background(), tool)
		fmt.Println(response)
	}
}

func TestBellman(t *testing.T) {
	err := godotenv.Load()
	if err != nil {
		log.Fatal("Error loading .env file")
	}

	bellmanUrl := os.Getenv("BELLMAN_URL")
	bellmanToken := os.Getenv("BELLMAN_TOKEN")

	client := New(bellmanUrl, Key{Name: "test", Token: bellmanToken})
	llm := client.Generator().ThinkingBudget(0)
	res, err := llm.Model(vllm.GenModel_gpt_oss_20b).
		Prompt(
			prompt.AsUser("What company made you?"),
		)
	text, _ := res.AsText()
	fmt.Println(text)

	// another prompt
	model := llm.Model(vllm.GenModel_gpt_oss_20b)
	res, err = model.Prompt(prompt.AsUser("Tell me a joke"))
	text, _ = res.AsText()
	fmt.Println(text)
}

func TestJSLLM(t *testing.T) {
	err := godotenv.Load()
	if err != nil {
		log.Fatal("Error loading .env file")
	}

	bellmanUrl := os.Getenv("BELLMAN_URL")
	bellmanToken := os.Getenv("BELLMAN_TOKEN")

	askBellman := func(url, token, userMessage string) string {
		client := New(url, Key{Name: "test", Token: token})
		llm := client.Generator()
		res, _ := llm.Model(vllm.GenModel_gpt_oss_20b).
			Prompt(
				prompt.AsUser(userMessage),
			)
		text, _ := res.AsText()
		return text
	}

	vm := goja.New()
	vm.Set("CONFIG", map[string]string{
		"token": bellmanToken,
		"url":   bellmanUrl,
	})
	vm.Set("askBellman", askBellman)
	vm.Set("goLog", func(msg string) {
		fmt.Printf("[JS-LOG]: %s\n", msg)
	})

	script := `
		goLog("Asking Bellman...");
		var result = askBellman(CONFIG.url, CONFIG.token, "What company made you?");
		goLog("Answer is: " + result);
		result; // Return the result to Go
	`

	val, err := vm.RunString(script)
	if err != nil {
		panic(err)
	}

	fmt.Printf("Final value returned to Go: %v\n", val.Export())
}

func TestJS(t *testing.T) {
	// 1. Load the .env file
	err := godotenv.Load()
	if err != nil {
		log.Fatal("Error loading .env file")
	}

	// 2. Access variables using the standard 'os' package
	apiToken := os.Getenv("API_TOKEN")
	fmt.Println(apiToken)

	vm := goja.New()

	//set env var in js runtime
	vm.Set("CONFIG", map[string]string{
		"token": apiToken,
	})

	// 1. Define a Go function
	// You can use standard Go types; goja handles the conversion!
	goCalculateHypotenuse := func(a, b float64) float64 {
		return math.Sqrt(a*a + b*b)
	}

	// 2. Register the function in the JS VM
	// We are mapping the Go variable to a JS name "getHypotenuse"
	vm.Set("getHypotenuse", goCalculateHypotenuse)

	// 3. Register a more complex function (e.g., a logger)
	vm.Set("goLog", func(msg string) {
		fmt.Printf("[JS-LOG]: %s\n", msg)
	})

	// Now JS can use it!
	script := `goLog("JS received token: " + CONFIG.token);`
	_, err = vm.RunString(script)

	// 4. Run JS code that calls these Go functions
	script = `
		goLog("Starting calculation...");
		var result = getHypotenuse(3, 4);
		goLog("Result is: " + result);
		result; // Return the result to Go
	`

	val, err := vm.RunString(script)
	if err != nil {
		panic(err)
	}

	fmt.Printf("Final value returned to Go: %v\n", val.Export())
}
