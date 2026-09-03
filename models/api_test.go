package models

import (
	"testing"

	"github.com/JoshPattman/jpf"
)

func TestAPIModelOpts(t *testing.T) {
	var settings apiModelSettings
	WithReasoningEffort(HighReasoning)(&settings)
	if settings.reasoning == nil || *settings.reasoning != HighReasoning {
		t.Fatalf("got %+v", settings)
	}

	WithVerbosity(HighVerbosity)(&settings)
	if settings.verbosity == nil || *settings.verbosity != HighVerbosity {
		t.Fatalf("got %+v", settings)
	}

	WithTopP(5)(&settings)
	if settings.topP == nil || *settings.topP != 5 {
		t.Fatalf("got %+v", settings)
	}

	WithPresencePenalty(0.5)(&settings)
	if settings.presencePenalty == nil || *settings.presencePenalty != 0.5 {
		t.Fatalf("got %+v", settings)
	}

	WithPrediction("pred")(&settings)
	if settings.prediction == nil || *settings.prediction != "pred" {
		t.Fatalf("got %+v", settings)
	}

	WithMaxOutput(100)(&settings)
	if settings.maxOutput == nil || *settings.maxOutput != 100 {
		t.Fatalf("got %+v", settings)
	}

	WithURL("http://example.com")(&settings)
	if settings.url != "http://example.com" {
		t.Fatalf("got %+v", settings)
	}
}

func TestWithHeader(t *testing.T) {
	var settings apiModelSettings
	WithHeader("A", "1")(&settings)
	WithHeader("B", "2")(&settings)
	if settings.headers["A"] != "1" || settings.headers["B"] != "2" {
		t.Fatalf("got %+v", settings.headers)
	}
}

func TestWithHeaders(t *testing.T) {
	var settings apiModelSettings
	WithHeader("A", "1")(&settings)
	WithHeaders(map[string]string{"B": "2", "C": "3"})(&settings)
	if settings.headers["A"] != "1" || settings.headers["B"] != "2" || settings.headers["C"] != "3" {
		t.Fatalf("got %+v", settings.headers)
	}
}

func TestGetDefaultURL(t *testing.T) {
	tests := []struct {
		format APIFormat
		want   string
	}{
		{OpenAIChatCompletions, "https://api.openai.com/v1/chat/completions"},
		{Google, "https://generativelanguage.googleapis.com/v1beta/models"},
		{OpenAIResponses, "https://api.openai.com/v1/responses"},
	}
	for _, tt := range tests {
		if got := getDefaultURL(tt.format); got != tt.want {
			t.Fatalf("got %q want %q", got, tt.want)
		}
	}
}

func TestGetDefaultURLPanicsOnUnknownFormat(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Fatal("expected a panic for an unrecognised format")
		}
	}()
	getDefaultURL(APIFormat(255))
}

func TestErrUnsupportedSetting(t *testing.T) {
	err := errUnsupportedSetting("temperature", 0.5)
	if err == nil || !contains(err.Error(), "temperature") {
		t.Fatalf("got %v", err)
	}
}

func TestNewRemoteConstructsCorrectModelType(t *testing.T) {
	tests := []struct {
		format APIFormat
		check  func(jpf.Model) bool
	}{
		{OpenAIChatCompletions, func(m jpf.Model) bool { _, ok := m.(*apiOpenAIModel); return ok }},
		{Google, func(m jpf.Model) bool { _, ok := m.(*apiGeminiModel); return ok }},
		{OpenAIResponses, func(m jpf.Model) bool { _, ok := m.(*apiOpenAIResponsesModel); return ok }},
	}
	for _, tt := range tests {
		model := NewRemote(tt.format, "name", "key")
		if !tt.check(model) {
			t.Fatalf("unexpected model type for format %v: %T", tt.format, model)
		}
	}
}

func TestNewRemotePanicsOnUnknownFormat(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Fatal("expected a panic for an unrecognised format")
		}
	}()
	NewRemote(APIFormat(255), "name", "key")
}
