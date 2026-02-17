package models

import (
	"time"

	"github.com/JoshPattman/jpf"
)

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
	X                jpf.Role
	TransformContent func(string) string
}
type WithSalt struct{ X string }

type WithReasoningAs struct {
	X                jpf.Role
	TransformContent func(string) string
}

type WithStreamResponse struct {
	// Called when the stream begins, may be called multiple times if retries occur.
	OnBegin func()
	// Called when a new chunk of text is received.
	OnText func(string)
}

func TransformByPrefix(prefix string) func(string) string {
	return func(s string) string {
		return prefix + s
	}
}
