# Bellman

Unified LLM Interface for vertexai/gemini, openai and anthropic

## Prerequisites

- A valid API key for each of the supported models (OpenAI, Anthropic, VertexAI/Gemini, VoyageAI)

## Installation

```bash
go get github.com/modfin/bellman
```


## Usage

## Prompting 

Just normal conversation mode

```go 

llm := openai.New(apiKey).Generator()
res, err := llm.
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

llm := openai.New(apiKey).Generator()
res, err := llm.
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

llm := openai.New(apiKey).Generator()
res, err := llm.
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

a supporting lib with transforming your go struct to json schema is provided. `github.com/modfin/bellman/schema`

```go

type Quote struct {
    Character string `json:"character"`
    Quote     string `json:"quote"`
}
type Responese struct {
    Quote []Quote `json:"quotes"`
}


llm := vertexai.New(googleConfig).Generator()
res, err := llm.
    Model(vertexai.GenModel_gemini_1_5_pro).
    Output(Responese{}).
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
       tools.WithCallback(func(jsondata string) error {
           var arg Args
           err := json.Unmarshal([]byte(jsondata), &arg)
           if err != nil {
               return err
           }
       }),
   )
   ```

2. Use the tool in a prompt:
   ```go
      llm := anthopic.New(apiKey).Generator()
      res, err := llm.
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
          prompt.AsUserWithData(prompt.MimeImageJPEG, bytes.NewBuffer(data)),
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

   pdf := os.Open("path/to/pdf")
   if err != nil {
      t.Fatalf("could not decode image %v", err)
   }

   llm := anthopic.New(apiKey).Generator()
   
   res, err := llm.
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

## RAG Example

Supporter lib for "automated" RAG (Retrieval-Augmented Generation) is supported by Gemini, OpenAI and Anthropic.

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
   tools.WithCallback(func(jsondata string) (string, error) {
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
   tools.WithCallback(func(jsondata string) (string, error) {
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

res, err := rag.Run[Result](5, llm, prompt.AsUser("Get me the price of Volvo B"))
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
// assistant:  tool function call: __bellman__rag_result_callback with argument: {"price":123.45,"stock_id":98765}



```


## License

This project is licensed under the MIT License. See the `LICENSE` file for details.