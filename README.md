# Bellman

It tries to be unified interface to interact with LLMs and embedding models.
In particular, it seeks to make it easier to switch between models and vendors
along woth lowering tha barrier to get started.
Bellman supports  `VertexAI/Gemini`, `OpenAI`, `Anthropic`, `VoyageAI` and `Ollama`

Bellman consists of two parts. The library and the service.
The go library enables you to interact with the different LLM vendors directly while the service,
`bellmand` creates a proxy service that lets you connect to all providers with one api key.

Bellman supports the common things that we expect in modern llm models.
Chat, Structured, Tools and binary input.

## Why

This project was built to the lack of official sdk/clients for the major players along with the
slight differences in API.
It also became clear when we started to play around with different LLMs in our projects that the
differences, while slight, had implications and for each new model introduced it became an overhead.
There are other projects out there, like go version of langchain, that deals with some of it.
But having one proxy to hadle all types of models made things alot easier for us to iterate over
problems, models and solutions.

## The Service

`bellmand` is a simple web service that implements the bellman library and exposes it through a http api

The easiest way to get started is to simply run it as a docker service.

### Prerequisite

- Docker being installed
- API Keys to Anthropic, OpenAI, VertexAI(Google Gemini) and/or VoyageAI
- Installing Ollama, https://ollama.com/ (very cool project imho)

### Run

```sh
## Help / Man
docker run --rm -it modfin/bellman --help  

## Example 
docker run --rm -d modfin/bellman \
  --prometheus-metrics-basic-auth="user:pass"
  --ollama-url=http://localhost:11434 \
  --openai-key="$(cat ./credentials/openai-api-key.txt)" \
  --anthropic-key="$(cat ./credentials/anthropic-api-key.txt)" \
  --voyageai-key="$(cat ./credentials/voyageai-api-key.txt)" \
  --google-credential="$(cat ./credentials/google-service-account.json)" \
  --google-project=your-google-project \
  --google-region=europe-north1 \
  --api-key=qwerty 
```

This will start the bellmand service that proxies requests to the model you define in the request.

## The Library

### Installation

```bash
go get github.com/modfin/bellman
```

## Usage

The library provides clients for Anthropic, Ollama, OpenAI, VertexAI, VoyageAI and Bellmand itself.

All the clients implement the same interfaces, `gen.Generator` and `embed.Embeder`,
and can there for be used interchangeably.

```go 
client, err := anthropic.New(...)
client, err := ollama.New(...)
client, err := openai.New(...)
client, err := vertexai.New(...)
client, err := voyageai.New(...)
client, err := bellman.New(...)
```

## bellman.New()

The benefit of using the bellman client,
when you are running `bellmand`,
is that we can interchangeably use any model that we wish to interact with.

```go
client, err := bellman.New(...)
llm := client.Generator()
res, err := llm.Model(openai.GenModel_gpt4o_mini).
    Prompt(
        prompt.AsUser("What company made you?"),
    )
fmt.Println(res, err)
// OpenAI

res, err := llm.Model(vertexai.GenModel_gemini_1_5_flash).
    Prompt(
        prompt.AsUser("What company made you?"),
    )
fmt.Println(res, err)
// Google

// or even a custom model that you created yourself (trained) 
// or a new model that is not in the library yet
model := gen.Model{
    Provider: vertexai.Provider,
    Name:     "gemini-2.0-flash-exp",
    Config:   map[string]interface{}{"region": "us-central1"},
}
res, err := llm.Model(model).
    Prompt(
        prompt.AsUser("What company made you?"),
    )
fmt.Println(res, err)
// Google


```

## Prompting

Just normal conversation mode

```go
res, err := openai.New(apiKy).Generator().
    Model(openai.GenModel_gpt4o_mini).
    Prompt(
        prompt.AsUser("What is the distance to the moon?"),
    )
if err != nil {
    log.Fatalf("Prompt() error = %v", err)
}

awnser, err := res.AsText()

fmt.Println(awnser, err)
// The average distance from Earth to the Moon is approximately 384,400 kilometers 
// (about 238,855 miles). This distance can vary slightly because the Moon's orbit
// is elliptical, ranging from about 363,300 km (225,623 miles) at its closest 
// (perigee) to 405,500 km (251,966 miles) at its farthest (apogee). <nil>
```

## System Prompting

Just normal conversation mode

```go
res, err := openai.New(apiKey).Generator().
    Model(openai.GenModel_gpt4o_mini).
    System("You are a expert movie quoter and lite fo finish peoples sentences with a movie reference").
    Prompt(
        prompt.AsUser("Who are you going to call?"),
    )
if err != nil {
    log.Fatalf("Prompt() error = %v", err)
}

awnser, err := res.AsText()

fmt.Println(awnser, err)
// Ghostbusters! <nil>
```

## General Configuration

Setting things like temperature, max tokens, top p, and stop secuences

```go
res, err := openai.New(apiKey).Generator().
    Model(openai.GenModel_gpt4o_mini).
    Temperature(0.5).
    MaxTokens(100).
    TopP(0.9). // should really not be used with temperature
    StopAt(".", "!", "?").
    Prompt(
        prompt.AsUser("Write me a 2 paragraph text about gophers"),
    )
if err != nil {
    log.Fatalf("Prompt() error = %v", err)
}

awnser, err := res.AsText()

fmt.Println(awnser, err)
// Gophers are small, 
// burrowing rodents belonging to the family Geomyidae, 
// primarily found in North America
```

## Structured Output

From many models, you can now specify a schema that you want the models to output.

A supporting library that can transforming your go struct to json schema is provided. `github.com/modfin/bellman/schema`

There are a few different annotations that you can use on your golang structs to enrich the corresponding json schema.

| Annotation             | Description                                                                                                              | Supported        |
|------------------------|--------------------------------------------------------------------------------------------------------------------------|------------------|
| json-description       | A description of the field, overrides the default description value                                                      | *                |
| json-type              | The type of the field, overrides the default type value                                                                  | *                |
| json-enum              | A list of possible values for the field. Can be used with: slices, string, number, integer, boolean                      | *                |
| json-maximum           | The maximum value for the field. Can be used with: number, integer                                                       | VertexAI, OpenAI |
| json-minimum           | The minimum value for the field. Can be used with: number, integer                                                       | VertexAI, OpenAI |
| json-exclusive-maximum | The exclusive maximum value for the field. Can be used with: number, integer                                             | VertexAI, OpenAI |
| json-exclusive-minimum | The exclusive minimum value for the field. Can be used with: number, integer                                             | VertexAI, OpenAI |
| json-max-items         | The maximum number of items in the array. Can be used with: slices                                                       | VertexAI, OpenAI |
| json-min-items         | The minimum number of items in the array. Can be used with: slices                                                       | VertexAI, OpenAI |
| json-max-length        | The maximum length of the string. Can be used with: string                                                               | OpenAI         |
| json-min-length        | The minimum length of the string. Can be used with: string                                                               | OpenAI         |
| json-format            | Format of a string, one of: date-time, time, date, duration, email, hostname, ipv4, ipv6, uuid. Can be used with: string | VertexAI, OpenAI |
| json-pattern           | Regex pattern of a string. Can be used with: string                                                                      | OpenAI           |

```go
type Quote struct {
   Character string `json:"character"`
   Quote     string `json:"quote"`
}
type Responese struct {
   Quotes []Quote `json:"quotes"`
}


llm := vertexai.New(googleConfig).Generator()
res, err := llm.
    Model(vertexai.GenModel_gemini_1_5_pro).
    Output(schema.From(Responese{})).
    Prompt(
        prompt.AsUser("give me 3 quotes from different characters in Hamlet"),
    )
if err != nil {
    log.Fatalf("Prompt() error = %v", err)
}

awnser, err := res.AsText() // will return the json of the struct
fmt.Println(awnser, err)
//{
//  "quotes": [
//    {
//      "character": "Hamlet",
//      "quote": "To be or not to be, that is the question."
//    },
//    {
//      "character": "Polonius",
//      "quote": "This above all: to thine own self be true."
//    },
//    {
//      "character": "Queen Gertrude",
//      "quote": "The lady doth protest too much, methinks."
//    }
//  ]
//}  <nil>

var result Result
err := res.Unmarshal(&result) // Just a shorthand to marshal it into your struct
fmt.Println(result, err)
// {[
//      {Hamlet To be or not to be, that is the question.} 
//      {Polonius This above all: to thine own self be true.} 
//      {Queen Gertrude The lady doth protest too much, methinks.}
// ]} <nil>
```

## Tools

The Bellman library allows you to define and use tools in your prompts.
Here is an example of how to define and use a tool:

1. Define a tool:
   ```go
    type Args struct {
         Name string `json:"name"`
    }
   
    getQuote := tools.NewTool("get_quote",
       tools.WithDescription(
            "a function to get a quote from a person or character in Hamlet",
       ),
       tools.WithArgSchema(Args{}),
       tools.WithCallback(func(jsondata string) (string, error) {
           var arg Args
           err := json.Unmarshal([]byte(jsondata), &arg)
           if err != nil {
               return "",err
           }
           return dao.GetQuoateFrom(arg.Name)
       }),
   )
   ```

2. Use the tool in a prompt:
   ```go
   res, err := anthopic.New(apiKey).Generator().
       Model(anthropic.GenModel_3_5_haiku_latest)).
       System("You are a Shakespeare quote generator").
       Tools(getQuote).
       // Configure a specific too to be used, or the setting for it
       Tool(tools.RequiredTool). 
       Prompt(
           prompt.AsUser("Give me 3 quotes from different characters"),
       )

   if err != nil {
       log.Fatalf("Prompt() error = %v", err)
   }

   // Evaluate with callback function
   err = res.Eval()
   if err != nil {
       log.Fatalf("Eval() error = %v", err)
   }
   
   
   // or Evaluate your self
   
   tools, err := res.Tools()
   if err != nil {
         log.Fatalf("Tools() error = %v", err)
   }
   
   for _, tool := range tools {
       log.Printf("Tool: %s", tool.Name)
       switch tool.Name {
          // ....
       }
   }
   
   ```

## Binary Data

Images is supported by Gemini, OpenAI and Anthropic.\
PDFs is only supported by Gemini and Anthropic

#### Image

```go 

image := "/9j/4AAQSkZJRgABAQEBLAEsAAD//g......gM4OToWbsBg5mGu0veCcRZO6f0EjK5Jv5X/AP/Z"
data, err := base64.StdEncoding.DecodeString(image)
if err != nil {
    t.Fatalf("could not decode image %v", err)
}
res, err := llm.
    Prompt(
        prompt.AsUserWithData(prompt.MimeImageJPEG, data),
        prompt.AsUser("Describe the image to me"),
    )

if err != nil {
    t.Fatalf("Prompt() error = %v", err)
}
fmt.Println(res.AsText())
// The image contains the word "Hot!" in red text. The text is centered on a white background. 
// The exclamation point is after the word.  The image is a simple and straightforward 
// depiction of the word "hot." <nil>

```

#### PDF

```go
pdf, err := os.ReadFile("path/to/pdf")
if err != nil {
    t.Fatalf("could open file, %v", err)
}

res, err := anthopic.New(apiKey).Generator().
    Prompt(
        prompt.AsUserWithData(prompt.MimeApplicationPDF, pdf),
        prompt.AsUser("Describe to me what is in the PDF"),
    )

if err != nil {
    t.Fatalf("Prompt() error = %v", err)
}
fmt.Println(res.AsText())
// The image contains the word "Hot!" in red text. The text is centered on a white background. 
// The exclamation point is after the word.  The image is a simple and straightforward 
// depiction of the word "hot." <nil>

```

## Reasoning

Control reasoning by setting the budget for the reasoning tokens. Determine whether to return the reasoning data or not.
The default thinking/reasoning behaviour is different depending on the model you are using.

```go
res, err := anthropic.New(apiKey).Generator().
    Model(anthropic.GenModel_4_0_sonnet_20250514).
    MaxTokens(3000).
    ThinkingBudget(2000). // the budget for reasoning tokens, set to 0 to disable reasoning if supported by the selected model
    IncludeThinkingParts(true). // if available, includes the reasoning parts in the response (will be summaries for some models)
    Prompt(
        prompt.AsUser("What is 27 * 453?"),
    )
if err != nil {
    log.Fatalf("Prompt() error = %v", err)
}

answer, err := res.AsText()

fmt.Println(awnser, err)
```

## Agent Example

Supporter lib for simple agentic tasks

```go
type GetQuoteArg struct {
    StockId int `json:"stock_id" json-description:"the id of a stock for which  quote to get"`
}
type Search struct {
    Name string `json:"name" json-description:"the name of a stock being looked for"`
}

getQuote := tools.NewTool("get_quote",
    tools.WithDescription("a function get a stock quote based on stock id"),
    tools.WithArgSchema(GetQuoteArg{}),
    tools.WithCallback(func (jsondata string) (string, error) {
        var arg GetQuoteArg
        err := json.Unmarshal([]byte(jsondata), &arg)
        if err != nil {
            return "", err
        }
         return `{"stock_id": ` + strconv.Itoa(arg.StockId) + `,"price": 123.45}`, nil
    }),
)

getStock := tools.NewTool("get_stock",
    tools.WithDescription("a function a stock based on name"),
    tools.WithArgSchema(Search{}),
    tools.WithCallback(func (jsondata string) (string, error) {
        var arg GetQuoteArg
        err := json.Unmarshal([]byte(jsondata), &arg)
        if err != nil {
            return "", err
        }
        return `{"stock_id": 98765}`, nil
    }),
)


type Result struct {
    StockId int     `json:"stock_id"`
    Price   float64 `json:"price"`
}

llm := anthopic.New(apiKey).Generator()
llm = llm.SetTools(getQuote, getStock)

res, err := agent.Run[Result](5, llm, prompt.AsUser("Get me the price of Volvo B"))
if err != nil {
    t.Fatalf("Prompt() error = %v", err)
}

fmt.Printf("==== Result after %d calls ====\n", res.Depth)
fmt.Printf("%+v\n", res.Result)
fmt.Printf("==== Conversation ====\n")

for _, p := range res.Promps {
    fmt.Printf("%s: %s\n", p.Role, p.Text)
}

// ==== Result after 2 calls ====
// {StockId:98765 Price:123.45}
// ==== Conversation ====
// user:       Get me the price of Volvo B
// assistant:  tool function call: get_stock with argument: {"name":"Volvo B"}
// user:       result: get_stock => {"stock_id": 98765}
// assistant:  tool function call: get_quote with argument: {"stock_id":98765}
// user:       result: get_quote => {"stock_id": 98765,"price": 123.45}
// assistant:  tool function call: __return_result_tool__ with argument: {"price":123.45,"stock_id":98765}
```

## Embeddings

Bellman integrates with most the embeddig models aswell as the llms that is provided by the supported
providers. There is also a VoyageAI, voyageai.com, that only really deals with embeddings

```go
client := bellman_client := bellman.New(...)

res, err := client.Embed(embed.Request{
    Model: vertexai.EmbedModel_text_005.WithType(embed.TypeDocument),
    Text:  "The document to embed",
})

fmt.Println(res.AsFloat32())
// [-0.06821047514677048 -0.00014664272021036595 0.011814368888735771 ....
```

### Type

Some embeddings models support specific types of input.

Eg.
[VertexAI](https://cloud.google.com/vertex-ai/generative-ai/docs/embeddings/task-types#retrieve_information_from_texts)
and [VoyageAI](https://docs.voyageai.com/docs/embeddings)

The API allows you to define what type of text you are sending.
For example `embed.TypeDocument` for initial embedding and `embed.TypeQuery`
for getting a vector that is to be compared

## License

This project is licensed under the MIT License. See the `LICENSE` file for details.
