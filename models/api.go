package models

import (
	"fmt"

	"github.com/JoshPattman/jpf"
)

type ReasoningEffort uint8

const (
	NoneReasoning ReasoningEffort = iota
	LowReasoning
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
	OpenAIChatCompletions APIFormat = iota
	Google
	OpenAIResponses
	Anthropic
)

type apiModelSettings struct {
	url     string
	headers map[string]string

	temperature     *float64
	reasoning       *ReasoningEffort
	verbosity       *Verbosity
	topP            *float64
	presencePenalty *float64
	prediction      *string
	maxOutput       *int
	storeReasoning  bool
}

type APIModelOpt func(*apiModelSettings)

func WithTemperature(temp float64) APIModelOpt {
	return func(kw *apiModelSettings) { kw.temperature = &temp }
}

// WithReasoningEffort sets how hard the model reasons before answering. OpenAI
// (both formats) takes it as a categorical effort; Anthropic and Gemini map it
// onto an extended-thinking token budget. NoneReasoning disables reasoning and
// cannot be combined with WithStoreReasoning.
func WithReasoningEffort(re ReasoningEffort) APIModelOpt {
	return func(kw *apiModelSettings) { kw.reasoning = &re }
}
func WithVerbosity(vb Verbosity) APIModelOpt {
	return func(kw *apiModelSettings) { kw.verbosity = &vb }
}

// WithTopP sets nucleus-sampling top_p. It is a probability in the range 0-1.
func WithTopP(tp float64) APIModelOpt {
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

// WithStoreReasoning makes the model emit and preserve opaque
// reasoning-continuation state (jpf.OpaqueReasoningBlock) on assistant messages,
// so a tool-calling agent keeps the model's chain of thought across steps.
//
// It configures the underlying API to return that state in a stateless,
// portable form: Anthropic extended thinking blocks, OpenAI Responses
// reasoning items with encrypted_content, or Gemini thought signatures. Without
// this option no reasoning is parsed out, even from a reasoning model.
//
// For Anthropic and Gemini it also turns thinking on (at the medium tier unless
// WithReasoningEffort says otherwise), since thinking must be enabled for there
// to be anything to store.
//
// It is not supported by the OpenAI Chat Completions API, which returns no
// reusable reasoning - a model built with that format and this option errors on
// every call.
func WithStoreReasoning() APIModelOpt {
	return func(kw *apiModelSettings) { kw.storeReasoning = true }
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

func NewRemote(format APIFormat, name string, key string, opts ...APIModelOpt) jpf.Model {
	settings := apiModelSettings{
		url:     getDefaultURL(format),
		headers: make(map[string]string),
	}
	for _, opt := range opts {
		opt(&settings)
	}
	switch format {
	case OpenAIChatCompletions:
		return &apiOpenAIModel{name, key, settings}
	case Google:
		return &apiGeminiModel{name, key, settings}
	case OpenAIResponses:
		return &apiOpenAIResponsesModel{name, key, settings}
	case Anthropic:
		return &apiAnthropicModel{name, key, settings}
	default:
		panic("unrecognised format")
	}
}

func getDefaultURL(format APIFormat) string {
	switch format {
	case OpenAIChatCompletions:
		return "https://api.openai.com/v1/chat/completions"
	case Google:
		return "https://generativelanguage.googleapis.com/v1beta/models"
	case OpenAIResponses:
		return "https://api.openai.com/v1/responses"
	case Anthropic:
		return "https://api.anthropic.com/v1/messages"
	default:
		panic("unrecognised format")
	}
}

func errUnsupportedSetting(settingName string, value any) error {
	return fmt.Errorf("parameter '%s' with value '%v' is unsupported for this model", settingName, value)
}

// formatFamilyPrefix is the stable jpf.OpaqueReasoningBlock.FormatFamily prefix
// for an API format. These strings are persisted inside serialised sessions and
// gate whether a stored reasoning block can be replayed - treat them as frozen
// wire identifiers and do not change an existing value.
func formatFamilyPrefix(f APIFormat) string {
	switch f {
	case OpenAIChatCompletions:
		return "openai-chat"
	case Google:
		return "gemini"
	case OpenAIResponses:
		return "openai-responses"
	case Anthropic:
		return "anthropic"
	default:
		panic("unrecognised format")
	}
}

// formatFamily is the FormatFamily a backend stamps onto the reasoning blocks it
// produces, and matches against when deciding which stored blocks it may replay.
func formatFamily(f APIFormat, model string) string {
	return formatFamilyPrefix(f) + "/" + model
}

// replayableReasoning keeps only the blocks whose FormatFamily exactly matches
// family. Blocks produced by a different model or provider - e.g. because the
// session was previously run against another backend - are dropped, since their
// opaque payloads are not valid to send here.
func replayableReasoning(blocks []jpf.OpaqueReasoningBlock, family string) []jpf.OpaqueReasoningBlock {
	out := make([]jpf.OpaqueReasoningBlock, 0, len(blocks))
	for _, b := range blocks {
		if b.FormatFamily == family {
			out = append(out, b)
		}
	}
	return out
}
