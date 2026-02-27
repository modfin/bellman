package test

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"testing"
	"time"

	"github.com/dop251/goja"
	"github.com/joho/godotenv"
	"github.com/modfin/bellman"
	"github.com/modfin/bellman/agent"
	"github.com/modfin/bellman/models/embed"
	"github.com/modfin/bellman/models/gen"
	"github.com/modfin/bellman/prompt"
	"github.com/modfin/bellman/services/anthropic"
	"github.com/modfin/bellman/services/openai"
	"github.com/modfin/bellman/services/vertexai"
	"github.com/modfin/bellman/tools"
	"github.com/modfin/bellman/tools/ptc"
	"github.com/wizenheimer/comet"
)

func TestToolman(t *testing.T) {
	// get env vars
	err := godotenv.Load("../../../.env")
	if err != nil {
		log.Fatal("Error loading .env file")
	}
	bellmanUrl := os.Getenv("BELLMAN_URL")
	bellmanToken := os.Getenv("BELLMAN_TOKEN")

	allTools := GetMockBellmanTools(true)
	models := []gen.Model{openai.GenModel_gpt4o_mini, vertexai.GenModel_gemini_2_5_flash_latest, anthropic.GenModel_3_haiku_20240307}
	models = []gen.Model{openai.GenModel_gpt5_mini_latest}

	// create Bellman llm and run agent
	client := bellman.New(bellmanUrl, bellman.Key{Name: "test", Token: bellmanToken})
	llm := client.Generator().System("# Role\nYou are a helpful LLM assistant.").
		SetTools(allTools...).SetPTCLanguage(tools.JavaScript).ThinkingBudget(100) //.Temperature(0)

	userPrompt := "1. Do you know what PTC is (programmatic tool calling), and how LLMs call tools? If yes; answer me which tool at your disposal is PTC. If no; why not?"
	userPrompt += "2. Predict the future, 3. convert 69 usd to sek, and then 4. generate a secret password. "
	//userPrompt += "Also, solve this problem: " + "Find the integer between 1 and 1,000 that produces the longest Collatz sequence. " +
	//	"Rules:\n 1. Start with any number n.\n 2. If n is even, divide by 2.\n 3. If n is odd, multiply by 3 and add 1.\n 4. Repeat until n becomes 1." +
	//	"\nReturn the starting number and the length of its sequence."
	userPrompt += "also, 5. get me the stock info (price/details) for saab, ericsson, and telia."

	// stopwatch
	start := time.Now()

	// run all models
	for _, m := range models {
		// swap model
		llm = llm.Model(m)

		var res *agent.Result[Result]
		switch llm.Request.Model.Provider {
		case vertexai.Provider:
			res, err = agent.RunWithToolsOnly[Result](10, 0, llm, prompt.AsUser(userPrompt))
		case anthropic.Provider:
			// haiku does not support temperature=0
			llm.Temperature(1)
			res, err = agent.Run[Result](10, 0, llm, prompt.AsUser(userPrompt))
		default:
			res, err = agent.Run[Result](10, 0, llm, prompt.AsUser(userPrompt))
		}

		if err != nil {
			log.Printf("Prompt() error = %v", err)
		} else {
			for i, m := range res.Prompts {
				fmt.Printf("prompt %v: { role: %v, text: %v, tool_call: %v, tool_response: %v }\n", i, m.Role, m.Text, m.ToolCall, m.ToolResponse)
			}
			// pretty print
			prettyPrint(res)
			elapsed := time.Since(start)
			fmt.Printf("Prompt took %s", elapsed)
		}
	}
}

func TestVectorSearch(t *testing.T) {
	// get env vars
	err := godotenv.Load()
	if err != nil {
		log.Fatal("Error loading .env file")
	}
	bellmanUrl := os.Getenv("BELLMAN_URL")
	bellmanToken := os.Getenv("BELLMAN_TOKEN")

	allTools := GetMockBellmanTools(true)

	// create Bellman client
	client := bellman.New(bellmanUrl, bellman.Key{Name: "test", Token: bellmanToken})

	// embed all tool descr and index
	log.Printf("embedding tool descriptions")
	dimension := 1536
	vecIdx, _ := comet.NewFlatIndex(dimension, comet.Cosine)
	bmIdx := comet.NewBM25SearchIndex()
	hybrid := comet.NewHybridSearchIndex(vecIdx, bmIdx, nil)
	for _, t := range allTools {
		res, _ := client.Embed(embed.NewSingleRequest(
			context.Background(),
			openai.EmbedModel_text3_small.WithType(embed.TypeDocument),
			t.Description,
		))
		log.Printf("Tokens used: %v - cost: $%v\n", res.Metadata.TotalTokens, float64(res.Metadata.TotalTokens)/(800*62_500))

		vec, _ := res.SingleAsFloat32()
		//node := comet.NewVectorNode(vec)
		//vecIdx.Add(*node)
		hybrid.Add(vec, t.Description, nil)
	}

	// query index with cosine sim
	query := "i have japanese currency and want to exchange to dollar"
	log.Printf(" ---- search query: %s", query)
	res, _ := client.Embed(embed.NewSingleRequest(
		context.Background(),
		openai.EmbedModel_text3_small.WithType(embed.TypeQuery),
		query,
	))
	q, _ := res.SingleAsFloat32()
	//results, _ := idx.NewSearch().
	//	WithQuery(q).
	//	WithK(5).
	//	Execute()

	// Search hybrid
	results, err := hybrid.NewSearch().
		WithVector(q).
		WithText(query).
		WithFusionKind(comet.ReciprocalRankFusion). // Combine rankings
		WithK(2).
		Execute()

	// Return relevant documents
	for _, result := range results {
		log.Printf("Found: %s (similarity: %.4f)",
			//allTools[result.GetId()-1].Description, 1-result.GetScore())
			allTools[result.GetId()-1].Description, result.GetScore())
	}
}

func TestAutoPTC(t *testing.T) {
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

	// define bellman tools
	mockTools := GetMockBellmanTools(true)
	//regularTools, PTCTools := ptc.AdaptToolsToPTC(vm, mockTools, gen.JavaScript)
	//allTools := append(regularTools, PTCTools...)

	// define system prompt
	const systemPrompt = `## Role
You are a Financial Assistant. Today is 2026-02-03.

## Capabilities
You solve complex logic by writing JavaScript code for the code_execution tool.`

	// create Bellman llm and run agent
	client := bellman.New(bellmanUrl, bellman.Key{Name: "test", Token: bellmanToken})
	llm := client.Generator().Model(openai.GenModel_gpt4o_mini).
		System(systemPrompt).
		SetTools(mockTools...).Temperature(0)

	const useGemini = true // quick-swap provider (gemini separate agent implementation)
	if useGemini {
		llm = llm.Model(vertexai.GenModel_gemini_2_5_flash_latest)
	}

	// prompt, expected result --> <bad-bellman-joke> and 2222222211
	userPrompt := "Predict the future, convert 69 usd to sek, and then generate a secret password."

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

	for i, m := range res.Prompts {
		fmt.Printf("prompt %v: %v\n", i, m)
	}

	// pretty print
	prettyPrint(res)
}

type Result struct {
	Text string `json:"text" json-description:"The final natural text answer to the user's request."`
}

func prettyPrint(res *agent.Result[Result]) {
	fmt.Printf("\n==== %s ====\n", res.Metadata.Model)
	fmt.Printf("==== Result after %d calls ====\n", res.Depth)
	fmt.Printf("%+v\n", res.Result.Text)
	fmt.Printf("==== Used %d tokens ====\n", res.Metadata.TotalTokens)
	costLow := (float64(res.Metadata.InputTokens)*0.15 + float64(res.Metadata.OutputTokens)*0.60) / 1_000_000.0
	costHigh := (float64(res.Metadata.InputTokens)*0.30 + float64(res.Metadata.OutputTokens)*2.50) / 1_000_000.0
	fmt.Printf("Input/output tokens: %d / %d\n", res.Metadata.InputTokens, res.Metadata.OutputTokens)
	fmt.Printf("approx. $%.4f - $%.4f\n", costLow, costHigh)
	fmt.Printf("Thinking tokens: %d\n", res.Metadata.ThinkingTokens)
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
		client := bellman.New(url, bellman.Key{Name: "test", Token: token})
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

	codeExecution := tools.NewTool(ptc.CodeExecutionToolName,
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
	client := bellman.New(bellmanUrl, bellman.Key{Name: "test", Token: bellmanToken})
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
