package prompt

import "encoding/base64"

type Role string

const UserRole = Role("user")
const AssistantRole = Role("assistant")
const ToolCallRole = Role("tool-call")
const ToolResponseRole = Role("tool-resp")
const ThinkingRole = Role("thinking")

type Prompt struct {
	Role         Role          `json:"role"`
	Text         string        `json:"text,omitempty"`
	Payload      *Payload      `json:"payload,omitempty"`
	ToolCall     *ToolCall     `json:"tool_call,omitempty"`
	ToolResponse *ToolResponse `json:"tool_response,omitempty"`

	Thinking *ThinkingContent `json:"thinking,omitempty"`

	// Replay is opaque per-provider bytes that must be echoed back verbatim
	// on the next request for the provider to accept the turn. Depending on
	// provider and role this is a signature (Anthropic thinking MAC, Gemini
	// thoughtSignature) or a ciphertext blob (Anthropic redacted_thinking
	// data, OpenAI reasoning.encrypted_content). Callers shouldn't inspect
	// or construct it — just round-trip the value from Response.Turn.
	Replay []byte `json:"replay,omitempty"`
}

// ThinkingContent is the payload attached to a Prompt with Role==ThinkingRole.
// The opaque replay bytes live on the wrapping Prompt.Replay field.
type ThinkingContent struct {
	Text     string `json:"text,omitempty"`
	ID       string `json:"id,omitempty"`
	Redacted bool   `json:"redacted,omitempty"`
}

type Payload struct {
	Mime string `json:"mime_type"`
	Data string `json:"data"`
	Uri  string `json:"uri"`
}

type ToolCall struct {
	ToolCallID string `json:"id,omitempty"`
	Name       string `json:"name"`
	Arguments  []byte `json:"arguments"`
}
type ToolResponse struct {
	ToolCallID string `json:"id,omitempty"`
	Name       string `json:"name"`
	Response   string `json:"content"`
}

func AsAssistant(text string) Prompt {
	return Prompt{Role: AssistantRole, Text: text}
}
func AsAssistantWithReplay(text string, replay []byte) Prompt {
	return Prompt{Role: AssistantRole, Text: text, Replay: replay}
}
func AsUser(text string) Prompt {
	return Prompt{Role: UserRole, Text: text}
}
func AsUserWithData(mime string, data []byte) Prompt {
	return Prompt{Role: UserRole, Payload: &Payload{Mime: mime, Data: base64.StdEncoding.EncodeToString(data)}}
}
func AsUserWithURI(mime string, uri string) Prompt {
	return Prompt{Role: UserRole, Payload: &Payload{Mime: mime, Uri: uri}}
}
func AsToolCall(toolCallID, functionName string, functionArg []byte) Prompt {
	return Prompt{Role: ToolCallRole, ToolCall: &ToolCall{ToolCallID: toolCallID, Name: functionName, Arguments: functionArg}}
}
func AsToolCallWithReplay(toolCallID, functionName string, functionArg, replay []byte) Prompt {
	return Prompt{
		Role:     ToolCallRole,
		Replay:   replay,
		ToolCall: &ToolCall{ToolCallID: toolCallID, Name: functionName, Arguments: functionArg},
	}
}
func AsToolResponse(toolCallID, functionName string, response string) Prompt {
	return Prompt{Role: ToolResponseRole, ToolResponse: &ToolResponse{ToolCallID: toolCallID, Name: functionName, Response: response}}
}
func AsThinking(text string, replay []byte, id string) Prompt {
	return Prompt{
		Role:     ThinkingRole,
		Replay:   replay,
		Thinking: &ThinkingContent{Text: text, ID: id},
	}
}

// AsRedactedThinking builds a thinking prompt whose content the provider has
// chosen not to disclose. data is the opaque ciphertext blob the provider
// returned (Anthropic's redacted_thinking.data); it must be echoed back
// verbatim via Prompt.Replay for the turn to be accepted.
func AsRedactedThinking(data []byte) Prompt {
	return Prompt{
		Role:     ThinkingRole,
		Replay:   data,
		Thinking: &ThinkingContent{Redacted: true},
	}
}

const MimeApplicationPDF = "application/pdf"
const MimeTextPlain = "text/plain"

const MimeAudioMPEG = "audio/mpeg"
const MimeAudioMP3 = "audio/mp3"
const MimeAudioWAV = "audio/wav"

const MimeImagePNG = "image/png"
const MimeImageJPEG = "image/jpeg"
const MimeImageWebp = "image/webp"

const MimeVideoMOV = "video/mov"
const MimeVideoMPEG = "video/mpeg"
const MimeVideoMP4 = "video/mp4"
const MimeVideoMPG = "video/mpg"
const MimeVideoAVI = "video/avi"
const MimeVideoWMV = "video/wmv"
const MimeVideoMPEGS = "video/mpegps"
const MimeVideoFLV = "video/flv"

var MIMEImages map[string]bool = map[string]bool{
	MimeImagePNG:  true,
	MimeImageJPEG: true,
	MimeImageWebp: true,
}
var MIMEAudio map[string]bool = map[string]bool{
	MimeAudioMPEG: true,
	MimeAudioMP3:  true,
	MimeAudioWAV:  true,
}
var MIMEVideo map[string]bool = map[string]bool{
	MimeVideoMOV:   true,
	MimeVideoMPEG:  true,
	MimeVideoMP4:   true,
	MimeVideoMPG:   true,
	MimeVideoAVI:   true,
	MimeVideoWMV:   true,
	MimeVideoMPEGS: true,
	MimeVideoFLV:   true,
}
