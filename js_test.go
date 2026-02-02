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
	"github.com/modfin/bellman/prompt"
	"github.com/modfin/bellman/services/vertexai"
	"github.com/modfin/bellman/services/vllm"
	"github.com/modfin/bellman/tools"
)

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
