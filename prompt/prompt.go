package prompt

import "encoding/base64"

type Role string

const UserRole = Role("user")
const AssistantRole = Role("assistant")
const ToolCallRole = Role("tool-call")
const ToolResponseRole = Role("tool-resp")

type Prompt struct {
	Role         Role          `json:"role"`
	Text         string        `json:"text,omitempty"`
	Payload      *Payload      `json:"payload,omitempty"`
	ToolCall     *ToolCall     `json:"tool_call,omitempty"`
	ToolResponse *ToolResponse `json:"tool_response,omitempty"`
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
func AsToolResponse(toolCallID, functionName string, response string) Prompt {
	return Prompt{Role: ToolResponseRole, ToolResponse: &ToolResponse{ToolCallID: toolCallID, Name: functionName, Response: response}}
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
