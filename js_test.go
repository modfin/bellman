package bellman

import (
	"fmt"
	"log"
	"math"
	"os"
	"testing"

	"github.com/dop251/goja"
	"github.com/joho/godotenv"
	"github.com/modfin/bellman/prompt"
	"github.com/modfin/bellman/services/vllm"
)

func TestToolman(t *testing.T) {
	client := New("BELLMAN_URL", Key{Name: "test", Token: "BELLMAN_TOKEN"})
	llm := client.Generator()
	res, err := llm.Model(vllm.GenModel_gpt_oss_20b).
		Prompt(
			prompt.AsUser("What company made you?"),
		)
	fmt.Println(res, err)

	// another prompt
	model := llm.Model(vllm.GenModel_gpt_oss_20b)
	res, err = model.Prompt(prompt.AsUser("Tell me a joke"))
	fmt.Println(res, err)
}

func TestJSLLM(t *testing.T) {
	err := godotenv.Load()
	if err != nil {
		log.Fatal("Error loading .env file")
	}

	bellmanUrl := os.Getenv("BELLMAN_URL")
	bellmanToken := os.Getenv("BELLMAN_TOKEN")

	askBellman := func(userMessage string) string {
		client := New("BELLMAN_URL", Key{Name: "test", Token: "BELLMAN_TOKEN"})
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
		var result = askBellman("What company made you?");
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
