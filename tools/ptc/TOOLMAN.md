# Toolman

Toolman introduces Programmatic Tool-Calling (PTC), allowing LLMs to write and execute code (e.g., JavaScript) to interact with tools. This enables complex logic, loops, and data manipulation that standard single-turn tool calling struggles with.

## Usage

To use PTC, you need to define your tools and set the PTC language on the generator.

```go
// Define your Bellman tools and schemas as usual, then set PTC property to true and (optional) add response schema 
type Args struct {
    Name string `json:"name"`
}

type Response struct {
	Quotes []Quote `json:"quotes"`
}

type Quote struct {
   Character string `json:"character"`
   Quote string `json:"quote"`
}

ptcTool := tools.NewTool("get_quote",
   tool.WithPTC(true), // <-- Enable PTC
   tools.WithDescription(
      "a function to get a quote from a person or character in Hamlet", 
	  ),
   tool.WithArgSchema(args{}),
   tools.WithFunction(func(ctx context.Context, call tools.Call) (string, error) {
         var arg Args
         err := json.Unmarshal(call.Argument, &arg)
         if err != nil {
            return "",err
         }
         var res Response // tool response schema
         res, err = dao.GetQuoateFrom(arg.Name)
         return res, err
      }),
   tool.WithResponseSchema[Response](), // <-- set response schema type (same as tool response)
)

// Append to tool list (other tools can be both PTC and non-PTC)
tools = append(tools, ptcTool)

// Initialize llm
llm := client.Generator().
   Model(openai.GenModel_gpt4o).
   SetTools(tools). // set tools as usual

llm, err := llm.ActivatePTC(ptc.JavaScript) // <-- Activate PTC (on enabled tools) and select language
if err != nil {
   log.Fatalf("ActivatePTC() error = %v", err)
}

res, err := llm.Prompt(prompt.AsUser("Give me 3 quotes from different characters"))

if err != nil {
   log.Fatalf("Prompt() error = %v", err)
}

// Evaluate with callback function
err = res.Eval()
if err != nil {
   log.Fatalf("Eval() error = %v", err)
}

```

For documentation on how to use Bellman, please refer to the Bellman [README.md](../../README.md).

### Custom PTC System Prompt

When guiding the model for using PTC, a PTC system fragment is added to the system prompt.
The default PTC system fragment can be overwritten:
```go
//TODO
```

### Statefulness

The code execution runtime is stateful! it is up to the developer to utilize or destroy state in a practical manner.
Statefulness is often practical even for stateless LLM APIs,and standard for LLMs code execution sandboxes.

The runtime can be reset by recreating it:
```go
//TODO
```

## Benchmarking

Toolman includes a benchmarking suite to evaluate LLM performance on tool-calling tasks, specifically focusing on PTC capabilities.

Benchmarks can run both with PTC enabled and disabled.

### Berkeley Function Calling Leaderboard (BFCL)

We provide an implementation to run the BFCL benchmark using Toolman's PTC engine. The benchmark is best run from the Toolman branch in the BFCL repo, see [?]().

#### Running the Benchmark Server

The benchmark server acts as an adapter between the BFCL evaluation scripts and the Bellman/Toolman API.

1. Navigate to the benchmark directory:
   ```bash
   cd tools/ptc/bench
   ```
2. Set up your environment variables in a `.env` file:
   ```env
   BELLMAN_URL=http://localhost:8080
   BELLMAN_TOKEN=your_token
   ```
3. Run the server:
   ```bash
   go run main.go

The BFCL server endpoint can be accessed at: `http://localhost:8080/bfcl`.

#### Debug

To debug BFCL benchmark using Toolman, a simple UI is provided. When the BFCL benchmark server is started; the debug UI can be accessed at: `http://localhost:8080/debug`.
