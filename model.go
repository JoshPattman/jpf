package jpf

import (
	"context"
	"slices"
	"time"
)

// Usage defines how many tokens were used when making calls to LLMs.
type Usage struct {
	InputTokens     int
	OutputTokens    int
	SuccessfulCalls int
	FailedCalls     int
}

func (u Usage) Add(u2 Usage) Usage {
	return Usage{
		u.InputTokens + u2.InputTokens,
		u.OutputTokens + u2.OutputTokens,
		u.SuccessfulCalls + u2.SuccessfulCalls,
		u.FailedCalls + u2.FailedCalls,
	}
}

type ModelResponse struct {
	// Extra messages that are not the final response,
	// but were used to build up the final response.
	// For example, reasoning messages.
	AuxiliaryMessages []Message
	// The primary response to the users query.
	// Usually the only response that matters.
	PrimaryMessage Message
	// The usage of making this call.
	// This may be the sum of multiple LLM calls.
	Usage Usage
}

// Utility to allow you to return the usage but 0 value messages when an error occurs.
func (r ModelResponse) OnlyUsage() ModelResponse {
	return ModelResponse{Usage: r.Usage}
}

// Utility to include another usage object in this response object
func (r ModelResponse) IncludingUsage(u Usage) ModelResponse {
	return ModelResponse{
		AuxiliaryMessages: slices.Clone(r.AuxiliaryMessages),
		PrimaryMessage:    r.PrimaryMessage,
		Usage:             r.Usage.Add(u),
	}
}

// Model defines an interface to an LLM.
type Model interface {
	// Responds to a set of input messages.
	Respond(context.Context, []Message) (ModelResponse, error)
}

// ReasoningEffort defines how hard a reasoning model should think.
type ReasoningEffort uint8

const (
	LowReasoning ReasoningEffort = iota
	MediumReasoning
	HighReasoning
)

type Verbosity uint8

const (
	LowVerbosity Verbosity = iota
	MediumVerbosity
	HighVerbosity
)

type WithMessagePrefix struct{ X string }
type WithDelay struct{ X time.Duration }
type WithTemperature struct{ X float64 }
type WithReasoningEffort struct{ X ReasoningEffort }
type WithURL struct{ X string }
type WithHTTPHeader struct {
	K string
	V string
}
type WithReasoningPrompt struct{ X string }
type WithVerbosity struct{ X Verbosity }
type WithTopP struct{ X int }
type WithPresencePenalty struct{ X float64 }
type WithPrediction struct{ X string }
type WithJsonSchema struct{ X map[string]any }
type WithMaxOutputTokens struct{ X int }
type WithSystemAs struct {
	X                Role
	TransformContent func(string) string
}
type WithSalt struct{ X string }

type WithReasoningAs struct {
	X                Role
	TransformContent func(string) string
}

func TransformByPrefix(prefix string) func(string) string {
	return func(s string) string {
		return prefix + s
	}
}
