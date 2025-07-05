package prompt

import (
	"encoding/base64"
	"reflect"
	"testing"
)

func TestAsAssistant(t *testing.T) {
	p := AsAssistant("hello")
	if p.Role != AssistantRole {
		t.Errorf("expected role %q, got %q", AssistantRole, p.Role)
	}
	if p.Text != "hello" {
		t.Errorf("expected text 'hello', got %q", p.Text)
	}
}

func TestAsUser(t *testing.T) {
	p := AsUser("hi")
	if p.Role != UserRole {
		t.Errorf("expected role %q, got %q", UserRole, p.Role)
	}
	if p.Text != "hi" {
		t.Errorf("expected text 'hi', got %q", p.Text)
	}
}

func TestAsUserWithData(t *testing.T) {
	data := []byte("somedata")
	p := AsUserWithData(MimeApplicationPDF, data)
	if p.Role != UserRole {
		t.Errorf("expected role %q, got %q", UserRole, p.Role)
	}
	if p.Payload == nil {
		t.Fatal("expected payload, got nil")
	}
	if p.Payload.Mime != MimeApplicationPDF {
		t.Errorf("expected mime %q, got %q", MimeApplicationPDF, p.Payload.Mime)
	}
	decoded, err := base64.StdEncoding.DecodeString(p.Payload.Data)
	if err != nil {
		t.Fatalf("failed to decode base64: %v", err)
	}
	if string(decoded) != string(data) {
		t.Errorf("expected data %q, got %q", data, decoded)
	}
}

func TestAsUserWithURI(t *testing.T) {
	uri := "file:///tmp/test.pdf"
	p := AsUserWithURI(MimeApplicationPDF, uri)
	if p.Role != UserRole {
		t.Errorf("expected role %q, got %q", UserRole, p.Role)
	}
	if p.Payload == nil {
		t.Fatal("expected payload, got nil")
	}
	if p.Payload.Mime != MimeApplicationPDF {
		t.Errorf("expected mime %q, got %q", MimeApplicationPDF, p.Payload.Mime)
	}
	if p.Payload.Uri != uri {
		t.Errorf("expected uri %q, got %q", uri, p.Payload.Uri)
	}
}

func TestAsToolCall(t *testing.T) {
	args := []byte(`{"foo":42}`)
	p := AsToolCall("id123", "myfunc", args)
	if p.Role != ToolCallRole {
		t.Errorf("expected role %q, got %q", ToolCallRole, p.Role)
	}
	if p.ToolCall == nil {
		t.Fatal("expected ToolCall, got nil")
	}
	if p.ToolCall.ToolCallID != "id123" {
		t.Errorf("expected ToolCallID 'id123', got %q", p.ToolCall.ToolCallID)
	}
	if p.ToolCall.Name != "myfunc" {
		t.Errorf("expected Name 'myfunc', got %q", p.ToolCall.Name)
	}
	if string(p.ToolCall.Arguments) != string(args) {
		t.Errorf("expected Arguments %q, got %q", args, p.ToolCall.Arguments)
	}
}

func TestAsToolResponse(t *testing.T) {
	p := AsToolResponse("id456", "myfunc2", "result")
	if p.Role != ToolResponseRole {
		t.Errorf("expected role %q, got %q", ToolResponseRole, p.Role)
	}
	if p.ToolResponse == nil {
		t.Fatal("expected ToolResponse, got nil")
	}
	if p.ToolResponse.ToolCallID != "id456" {
		t.Errorf("expected ToolCallID 'id456', got %q", p.ToolResponse.ToolCallID)
	}
	if p.ToolResponse.Name != "myfunc2" {
		t.Errorf("expected Name 'myfunc2', got %q", p.ToolResponse.Name)
	}
	if p.ToolResponse.Response != "result" {
		t.Errorf("expected Response 'result', got %q", p.ToolResponse.Response)
	}
}

func TestMIMEMaps(t *testing.T) {
	// Images
	for k, v := range MIMEImages {
		if !v {
			t.Errorf("expected MIMEImages[%q] to be true", k)
		}
	}
	// Audio
	for k, v := range MIMEAudio {
		if !v {
			t.Errorf("expected MIMEAudio[%q] to be true", k)
		}
	}
	// Video
	for k, v := range MIMEVideo {
		if !v {
			t.Errorf("expected MIMEVideo[%q] to be true", k)
		}
	}
}

func TestPromptStructTags(t *testing.T) {
	p := Prompt{
		Role:         UserRole,
		Text:         "foo",
		Payload:      &Payload{Mime: MimeTextPlain, Data: "bar", Uri: "baz"},
		ToolCall:     &ToolCall{ToolCallID: "id", Name: "n", Arguments: []byte("{}")},
		ToolResponse: &ToolResponse{ToolCallID: "id2", Name: "n2", Response: "r"},
	}
	// Just check that fields are settable and readable
	if p.Role != UserRole || p.Text != "foo" || p.Payload.Mime != MimeTextPlain {
		t.Errorf("Prompt struct fields not set/read correctly")
	}
	if p.ToolCall.Name != "n" || p.ToolResponse.Name != "n2" {
		t.Errorf("ToolCall/ToolResponse fields not set/read correctly")
	}
	// Check reflect tags
	typ := reflect.TypeOf(Prompt{})
	if typ.NumField() != 5 {
		t.Errorf("Prompt struct should have 5 fields, got %d", typ.NumField())
	}
}
