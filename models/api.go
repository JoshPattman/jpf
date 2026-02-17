package models

import (
	"fmt"

	"github.com/JoshPattman/jpf"
)

type ReasoningEffort uint8

const (
	LowReasoning ReasoningEffort = iota
	MediumReasoning
	HighReasoning
	XHighReasoning
)

type Verbosity uint8

const (
	LowVerbosity Verbosity = iota
	MediumVerbosity
	HighVerbosity
)

type APIFormat uint8

const (
	OpenAI APIFormat = iota
	Google
)

type apiModelSettings struct {
	url     string
	stream  *streamCallbacks
	headers map[string]string

	temperature     *float64
	reasoning       *ReasoningEffort
	verbosity       *Verbosity
	topP            *int
	presencePenalty *float64
	prediction      *string
	maxOutput       *int

	jsonSchema map[string]any
}

type streamCallbacks struct {
	onBegin func()
	onText  func(string)
}

type APIModelOpt func(*apiModelSettings)

func WithTemperature(temp float64) APIModelOpt {
	return func(kw *apiModelSettings) { kw.temperature = &temp }
}
func WithReasoningEffort(re ReasoningEffort) APIModelOpt {
	return func(kw *apiModelSettings) { kw.reasoning = &re }
}
func WithVerbosity(vb Verbosity) APIModelOpt {
	return func(kw *apiModelSettings) { kw.verbosity = &vb }
}

func WithTopP(tp int) APIModelOpt {
	return func(kw *apiModelSettings) { kw.topP = &tp }
}
func WithPresencePenalty(p float64) APIModelOpt {
	return func(kw *apiModelSettings) { kw.presencePenalty = &p }
}
func WithPrediction(pred string) APIModelOpt {
	return func(kw *apiModelSettings) { kw.prediction = &pred }
}
func WithMaxOutput(n int) APIModelOpt {
	return func(kw *apiModelSettings) { kw.maxOutput = &n }
}
func WithJSONSchema(schema map[string]any) APIModelOpt {
	return func(kw *apiModelSettings) { kw.jsonSchema = schema }
}
func WithStreamCallbacks(onBegin func(), onText func(string)) APIModelOpt {
	return func(kw *apiModelSettings) { kw.stream = &streamCallbacks{onBegin: onBegin, onText: onText} }
}
func WithHeader(key, value string) APIModelOpt {
	return func(kw *apiModelSettings) {
		if kw.headers == nil {
			kw.headers = make(map[string]string)
		}
		kw.headers[key] = value
	}
}
func WithHeaders(headers map[string]string) APIModelOpt {
	return func(kw *apiModelSettings) {
		if kw.headers == nil {
			kw.headers = make(map[string]string)
		}
		for k, v := range headers {
			kw.headers[k] = v
		}
	}
}
func WithURL(u string) APIModelOpt {
	return func(kw *apiModelSettings) { kw.url = u }
}

func NewAPIModel(format APIFormat, name string, key string, opts ...APIModelOpt) jpf.Model {
	settings := apiModelSettings{
		url:     getDefaultURL(format),
		headers: make(map[string]string),
	}
	for _, opt := range opts {
		opt(&settings)
	}
	switch format {
	case OpenAI:
		return &apiOpenAIModel{name, key, settings}
	case Google:
		return &apiGeminiModel{name, key, settings}
	default:
		panic("unrecognised format")
	}
}

func getDefaultURL(format APIFormat) string {
	switch format {
	case OpenAI:
		return "https://api.openai.com/v1/chat/completions"
	case Google:
		return "https://generativelanguage.googleapis.com/v1beta/models"
	default:
		panic("unrecognised format")
	}
}

func errUnsupportedSetting(settingName string, value any) error {
	return fmt.Errorf("parameter '%s' with value '%v' is unsupported for this model", settingName, value)
}
