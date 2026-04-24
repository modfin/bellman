package vertexai

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"sync/atomic"
	"time"

	"github.com/modfin/bellman/models"
	"github.com/modfin/bellman/models/gen"
	"github.com/modfin/bellman/prompt"
	"github.com/modfin/bellman/tools"
)

var requestNo int64

type generator struct {
	google  *Google
	request gen.Request
}

func (g *generator) SetRequest(config gen.Request) {
	g.request = config
}
func (g *generator) Stream(prompts ...prompt.Prompt) (<-chan *gen.StreamResponse, error) {

	g.request.Stream = true
	resp, model, err := g.prompt(prompts...)
	if err != nil {
		return nil, fmt.Errorf("could not make http request for prompt, %w", err)
	}

	reqc := atomic.AddInt64(&requestNo, 1)
	g.google.log("[gen] request",
		"request", reqc,
		"model", g.request.Model.FQN(),
		"tools", len(g.request.Tools) > 0,
		"tool_choice", g.request.ToolConfig != nil,
		"output_schema", g.request.OutputSchema != nil,
		"system_prompt", g.request.SystemPrompt != "",
		"temperature", g.request.Temperature,
		"top_p", g.request.TopP,
		"max_tokens", g.request.MaxTokens,
		"stop_sequences", g.request.StopSequences,
		"url", model.url,
	)

	if resp.StatusCode != http.StatusOK {
		b, err := io.ReadAll(resp.Body)
		return nil, errors.Join(fmt.Errorf("unexpected status code, %d, err: {%s}, for url: {%s} ", resp.StatusCode, string(b), model.url), err)
	}

	reader := bufio.NewReader(resp.Body)

	stream := make(chan *gen.StreamResponse)

	go func() {
		defer resp.Body.Close()
		defer close(stream)

		defer func() {
			stream <- &gen.StreamResponse{
				Type: gen.TYPE_EOF,
			}
		}()

		// Track the first tool call of this response. Gemini may emit the
		// thoughtSignature for a turn in a separate "closure" part (empty
		// text, no functionCall) arriving in a later SSE chunk; per the
		// docs only the first functionCall part carries the signature, so
		// we attach late-arriving closure signatures to the first tool
		// call's ID so the handler can merge them by ID.
		var firstToolCallID string
		var firstToolCallName string
		var firstToolCallRef *tools.Tool

		for {
			line, _, err := reader.ReadLine()
			if err != nil {
				// If there's an error, check if it's EOF (end of stream)
				if errors.Is(err, http.ErrBodyReadAfterClose) {
					log.Println("SSE stream closed by server (Read after close).")
					break
				}
				log.Printf("Error reading from stream: %v", err)
				break // Exit the loop on any other error
			}

			if len(line) == 0 {
				continue
			}
			if !bytes.HasPrefix(line, []byte("data: ")) {
				stream <- &gen.StreamResponse{
					Type:    gen.TYPE_ERROR,
					Content: "expected 'data' header from sse",
				}
				break
			}
			line = line[6:] // removing header

			//fmt.Println("line", string(line))
			var ss geminiStreamingResponse
			err = json.Unmarshal(line, &ss)
			if err != nil {
				log.Printf("could not unmarshal chunk, %v", err)
				break
			}

			if len(ss.Candidates) == 0 {
				stream <- &gen.StreamResponse{
					Type:    gen.TYPE_ERROR,
					Content: "there where no candidates in response",
				}
			}
			candidate := ss.Candidates[0]

			role := prompt.AssistantRole
			if candidate.Content.Role == "user" {
				role = prompt.UserRole
			}
			t := time.Now().UnixNano()
			for idx, part := range candidate.Content.Parts {
				var sig []byte
				if part.ThoughtSignature != nil && *part.ThoughtSignature != "" {
					sig = []byte(*part.ThoughtSignature)
				}
				// Only process parts with actual visible text content here.
				// Empty-text parts (part.Text == "" or nil) fall through to the
				// closure-signature handler below, which attaches any signature
				// to the first tool call of the turn — empty assistant text
				// parts sent on replay are rejected by the API.
				if part.Text != nil && *part.Text != "" {
					if part.Thought != nil && *part.Thought {
						stream <- &gen.StreamResponse{
							Type:    gen.TYPE_THINKING_DELTA,
							Role:    role,
							Index:   candidate.Index,
							Content: *part.Text,
						}
						if len(sig) > 0 {
							stream <- &gen.StreamResponse{
								Type:  gen.TYPE_BLOCK,
								Role:  role,
								Index: candidate.Index,
								Block: new(prompt.AsThinking(*part.Text, sig, "")),
							}
						}
						continue
					}
					stream <- &gen.StreamResponse{
						Type:    gen.TYPE_DELTA,
						Role:    role,
						Index:   candidate.Index,
						Content: *part.Text,
					}
					// Gemini may attach a thoughtSignature to plain assistant text
					// parts too — emit a finalized block so streaming consumers can
					// capture the signature for replay.
					if len(sig) > 0 {
						stream <- &gen.StreamResponse{
							Type:  gen.TYPE_BLOCK,
							Role:  role,
							Index: candidate.Index,
							Block: new(prompt.AsAssistantWithReplay(*part.Text, sig)),
						}
					}
				}
				if part.FunctionCall != nil {
					f := part.FunctionCall
					arg, err := json.Marshal(f.Args)
					if err != nil {
						stream <- &gen.StreamResponse{
							Type:    gen.TYPE_ERROR,
							Content: fmt.Sprintf("could not marshal tool call arguments, %v", err),
						}
						continue
					}
					id := fmt.Sprintf("%d-%d", t, idx)
					ref := model.toolBelt[f.Name]
					if firstToolCallID == "" {
						firstToolCallID = id
						firstToolCallName = f.Name
						firstToolCallRef = ref
					}
					stream <- &gen.StreamResponse{
						Type:  gen.TYPE_DELTA,
						Role:  prompt.ToolCallRole,
						Index: candidate.Index,
						ToolCall: &tools.Call{
							Name:     f.Name,
							Argument: arg,
							ID:       id,
							Replay:   sig,
							Ref:      ref,
						},
					}
					continue
				}

				// Closure signature part: Gemini may return the turn's
				// thoughtSignature in a later part with empty text and no
				// functionCall. Attach it to the first tool call of this turn
				// via a second delta that the handler merges by ID, or (if the
				// turn had no tool call) preserve it as a thinking block.
				textIsEmpty := part.Text == nil || *part.Text == ""
				if textIsEmpty && part.FunctionCall == nil && len(sig) > 0 {
					if firstToolCallID != "" {
						stream <- &gen.StreamResponse{
							Type:  gen.TYPE_DELTA,
							Role:  prompt.ToolCallRole,
							Index: candidate.Index,
							ToolCall: &tools.Call{
								Name:   firstToolCallName,
								ID:     firstToolCallID,
								Replay: sig,
								Ref:    firstToolCallRef,
							},
						}
					} else {
						stream <- &gen.StreamResponse{
							Type:  gen.TYPE_BLOCK,
							Role:  role,
							Index: candidate.Index,
							Block: new(prompt.AsThinking("", sig, "")),
						}
					}
				}

			}
			if ss.UsageMetadata.TotalTokenCount > 0 {
				thinkingTokens := ss.UsageMetadata.ThoughtsTokenCount
				outputTokens := ss.UsageMetadata.CandidatesTokenCount
				stream <- &gen.StreamResponse{
					Type: gen.TYPE_METADATA,
					Metadata: &models.Metadata{
						Model:          ss.ModelVersion,
						InputTokens:    ss.UsageMetadata.PromptTokenCount,
						OutputTokens:   outputTokens,
						ThinkingTokens: thinkingTokens,
						TotalTokens:    ss.UsageMetadata.PromptTokenCount + outputTokens + thinkingTokens,
					},
				}
			}

			if len(candidate.FinishReason) > 0 {
				break
			}

		}

	}()
	return stream, nil
}

func (g *generator) Prompt(prompts ...prompt.Prompt) (*gen.Response, error) {
	resp, model, err := g.prompt(prompts...)
	if err != nil {
		return nil, fmt.Errorf("could not make http request for prompt, %w", err)
	}

	reqc := atomic.AddInt64(&requestNo, 1)
	g.google.log("[gen] request",
		"request", reqc,
		"model", g.request.Model.FQN(),
		"tools", len(g.request.Tools) > 0,
		"tool_choice", g.request.ToolConfig != nil,
		"output_schema", g.request.OutputSchema != nil,
		"system_prompt", g.request.SystemPrompt != "",
		"temperature", g.request.Temperature,
		"top_p", g.request.TopP,
		"max_tokens", g.request.MaxTokens,
		"stop_sequences", g.request.StopSequences,
		"thinking_budget", g.request.ThinkingBudget != nil,
		"thinking_parts", g.request.ThinkingParts != nil,
		"url", model.url,
	)

	if resp.StatusCode != http.StatusOK {
		b, err := io.ReadAll(resp.Body)
		return nil, errors.Join(fmt.Errorf("unexpected status code, %d, err: {%s}, for url: {%s} ", resp.StatusCode, string(b), model.url), err)
	}

	defer resp.Body.Close()
	var respModel geminiResponse
	err = json.NewDecoder(resp.Body).Decode(&respModel)
	if err != nil {
		return nil, fmt.Errorf("could not decode google response, %w", err)
	}

	if len(respModel.Candidates) == 0 {
		return nil, fmt.Errorf("no candidates in response")
	}
	if len(respModel.Candidates[0].Content.Parts) == 0 {
		return nil, fmt.Errorf("no parts in response")
	}

	res := &gen.Response{
		Metadata: models.Metadata{
			Model: g.request.Model.FQN(),
		},
	}
	thinkingTokens := respModel.UsageMetadata.ThoughtsTokenCount
	outputTokens := respModel.UsageMetadata.CandidatesTokenCount
	res.Metadata.InputTokens = respModel.UsageMetadata.PromptTokenCount
	res.Metadata.OutputTokens = outputTokens
	res.Metadata.ThinkingTokens = thinkingTokens
	res.Metadata.TotalTokens = respModel.UsageMetadata.PromptTokenCount + outputTokens + thinkingTokens
	for _, c := range respModel.Candidates {
		for _, p := range c.Content.Parts {
			var sig []byte
			if p.ThoughtSignature != nil && *p.ThoughtSignature != "" {
				sig = []byte(*p.ThoughtSignature)
			}
			if p.Thought != nil && *p.Thought {
				res.Thinking = append(res.Thinking, p.Text)
				res.Turn = append(res.Turn, prompt.AsThinking(p.Text, sig, ""))
				continue
			}
			if len(p.Text) > 0 {
				res.Texts = append(res.Texts, p.Text)
				// Gemini may attach thoughtSignature to plain assistant text parts
				// too (the "save state" on the final visible output). Emit the
				// part as its own assistant prompt so the signature travels with
				// the text that was signed.
				res.Turn = append(res.Turn, prompt.AsAssistantWithReplay(p.Text, sig))
			}

			if len(p.Text) == 0 && len(p.FunctionCall.Name) > 0 { // Tool calls
				f := p.FunctionCall
				arg, err := json.Marshal(f.Args)
				if err != nil {
					return nil, fmt.Errorf("could not marshal google request, %w", err)
				}
				res.Tools = append(res.Tools, tools.Call{
					Name:     f.Name,
					Argument: arg,
					Replay:   sig,
					Ref:      model.toolBelt[f.Name],
				})

			}

		}
	}

	g.google.log("[gen] response",
		"request", reqc,
		"model", g.request.Model.FQN(),
		"token-input", res.Metadata.InputTokens,
		"token-output", res.Metadata.OutputTokens,
		"token-thinking", res.Metadata.ThinkingTokens,
		"token-total", res.Metadata.TotalTokens,
	)

	return res, nil
}
func (g *generator) prompt(prompts ...prompt.Prompt) (*http.Response, genRequest, error) {

	//https://cloud.google.com/vertex-ai/generative-ai/docs/model-reference/inference

	mode := "generateContent"
	if g.request.Stream {
		mode = "streamGenerateContent?alt=sse"
	}

	if g.request.Model.Name == "" {
		return nil, genRequest{}, errors.New("model is required")
	}

	model := genRequest{
		Contents: []genRequestContent{},
		GenerationConfig: &genConfig{
			MaxOutputTokens:  g.request.MaxTokens,
			TopP:             g.request.TopP,
			TopK:             g.request.TopK,
			Temperature:      g.request.Temperature,
			StopSequences:    g.request.StopSequences,
			FrequencyPenalty: g.request.FrequencyPenalty,
			PresencePenalty:  g.request.PresencePenalty,
		},
	}

	if g.request.SystemPrompt != "" {
		model.SystemInstruction = &genRequestContent{
			Role: "system", // does not take role into account, it can be anything?
			Parts: []genRequestContentPart{
				{
					Text: g.request.SystemPrompt,
				},
			},
		}
	}

	// Adding output schema to model
	if g.request.OutputSchema != nil {
		ct := "application/json"
		model.GenerationConfig.ResponseMimeType = &ct
		model.GenerationConfig.ResponseSchema = fromBellmanSchema(g.request.OutputSchema)
	}

	// Adding tools to model

	model.toolBelt = map[string]*tools.Tool{}
	if len(g.request.Tools) > 0 {
		model.Tools = []genTool{{FunctionDeclaration: []genToolFunc{}}}
		for _, t := range g.request.Tools {
			model.Tools[0].FunctionDeclaration = append(model.Tools[0].FunctionDeclaration, genToolFunc{
				Name:        t.Name,
				Description: t.Description,
				Parameters:  fromBellmanSchema(t.ArgumentSchema),
			})
			model.toolBelt[t.Name] = &t
		}
	}

	// Dealing with SetToolConfig request
	if g.request.ToolConfig != nil {
		model.ToolConfig = &genToolConfig{
			GoogleFunctionCallingConfig: genFunctionCallingConfig{
				Mode: "ANY",
			},
		}
		// https://cloud.google.com/vertex-ai/generative-ai/docs/model-reference/function-calling#functioncallingconfig
		switch g.request.ToolConfig.Name {
		case tools.NoTool.Name:
			model.ToolConfig.GoogleFunctionCallingConfig.Mode = "NONE"
		case tools.AutoTool.Name:
			model.ToolConfig.GoogleFunctionCallingConfig.Mode = "AUTO"
		case tools.RequiredTool.Name:
			model.ToolConfig.GoogleFunctionCallingConfig.Mode = "ANY"
		default:
			model.ToolConfig.GoogleFunctionCallingConfig.Mode = "ANY"
			model.ToolConfig.GoogleFunctionCallingConfig.AllowedFunctionNames = []string{g.request.ToolConfig.Name}
		}
	}

	if g.request.ThinkingBudget != nil || g.request.ThinkingParts != nil {
		model.GenerationConfig.ThinkingConfig = &thinkingConfig{}
	}
	if g.request.ThinkingBudget != nil {
		model.GenerationConfig.ThinkingConfig.ThinkingBudget = g.request.ThinkingBudget
	}
	if g.request.ThinkingParts != nil {
		model.GenerationConfig.ThinkingConfig.IncludeThoughts = g.request.ThinkingParts
	}

	// Append a part into the current Content of the given role, merging with
	// the previous Content when it already has the same role. Gemini requires
	// thinking/text/functionCall parts from one model turn to live under a
	// single `model` Content so thoughtSignature validation can match them up.
	appendPart := func(role string, part genRequestContentPart) {
		if n := len(model.Contents); n > 0 && model.Contents[n-1].Role == role {
			model.Contents[n-1].Parts = append(model.Contents[n-1].Parts, part)
			return
		}
		model.Contents = append(model.Contents, genRequestContent{Role: role, Parts: []genRequestContentPart{part}})
	}

	for _, p := range prompts {
		switch p.Role {
		case prompt.ToolResponseRole:
			if p.ToolResponse == nil {
				return nil, model, fmt.Errorf("ToolResponse is required for role tool response")
			}
			appendPart("tool", genRequestContentPart{
				FunctionResponse: &functionResponse{
					Name: p.ToolResponse.Name,
					Response: struct {
						Name    string `json:"name,omitempty"`
						Content any    `json:"content,omitempty"`
					}{Name: p.ToolResponse.Name, Content: p.ToolResponse.Response},
				},
			})
		case prompt.ToolCallRole:
			if p.ToolCall == nil {
				return nil, model, fmt.Errorf("ToolCall is required for role tool call")
			}
			var jsonArguments map[string]any
			err := json.Unmarshal(p.ToolCall.Arguments, &jsonArguments)
			if err != nil {
				return nil, model, fmt.Errorf("failed to unmarshal tool call arguments: %w", err)
			}
			part := genRequestContentPart{
				FunctionCall: &functionCall{Name: p.ToolCall.Name, Args: jsonArguments},
			}
			if len(p.Replay) > 0 {
				part.ThoughtSignature = string(p.Replay)
			}
			appendPart("model", part)
		case prompt.ThinkingRole:
			if p.Thinking == nil {
				continue
			}
			part := genRequestContentPart{
				Text:    p.Thinking.Text,
				Thought: true,
			}
			if len(p.Replay) > 0 {
				part.ThoughtSignature = string(p.Replay)
			}
			appendPart("model", part)
		default: // prompt.UserRole, prompt.AssistantRole
			role := "user"
			if p.Role == prompt.AssistantRole {
				role = "model"
			}
			part := genRequestContentPart{Text: p.Text}
			if p.Role == prompt.AssistantRole && len(p.Replay) > 0 {
				part.ThoughtSignature = string(p.Replay)
			}
			if p.Payload != nil {
				if len(p.Payload.Data) > 0 {
					part.InlineData = &inlineData{
						MimeType: p.Payload.Mime,
						Data:     p.Payload.Data,
					}
				}
				if len(p.Payload.Uri) > 0 {
					part.InlineData = nil
					part.FileData = &fileData{
						MimeType: p.Payload.Mime,
						FileUri:  p.Payload.Uri,
					}
				}
			}
			appendPart(role, part)
		}
	}

	region := g.google.config.Region
	project := g.google.config.Project
	if len(g.request.Model.Config) > 0 {
		cfg := g.request.Model.Config

		r, ok := cfg["region"].(string)
		if ok {
			region = r
		}
		p, ok := cfg["project"].(string)
		if ok {
			project = p
		}
	}

	if !modelNamePattern.MatchString(g.request.Model.Name) {
		return nil, model, fmt.Errorf("model name %s contains invalid characters, only [\\w.-]+ is allowed", g.request.Model.Name)
	}
	if !regionPattern.MatchString(region) {
		return nil, model, fmt.Errorf("region %q contains invalid characters, only (global)|([a-z]+-[a-z]+[1-9][0-9]*) or global is allowed", region)
	}

	if !projectIdPattern.MatchString(project) {
		return nil, model, fmt.Errorf("project %q contains invalid characters, only [a-z]([a-z0-9-]{4,28}[a-z0-9])? is allowed", project)
	}

	model.url = fmt.Sprintf("https://%s-aiplatform.googleapis.com/v1/projects/%s/locations/%s/publishers/google/models/%s:%s",
		region, project, region, g.request.Model.Name, mode)

	// Support for global region, which should decrease risk for 429 rate limit
	// https://cloud.google.com/vertex-ai/generative-ai/docs/provisioned-throughput/error-code-429#troubleshoot-dynamic-shared-quota
	if region == "global" {
		model.url = fmt.Sprintf("https://aiplatform.googleapis.com/v1/projects/%s/locations/global/publishers/google/models/%s:%s",
			project, g.request.Model.Name, mode)
	}

	body, err := json.Marshal(model)
	if err != nil {
		return nil, model, fmt.Errorf("could not marshal google request, %w", err)
	}

	ctx := g.request.Context
	if ctx == nil {
		ctx = context.Background()
	}
	req, err := http.NewRequestWithContext(ctx, "POST", model.url, bytes.NewReader(body))
	if err != nil {
		return nil, model, fmt.Errorf("could not create google request, %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := g.google.client.Do(req)

	if err != nil {
		return nil, model, fmt.Errorf("could not post google request, %w", err)
	}
	return resp, model, nil
}
