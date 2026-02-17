package models

import (
	"time"

	"github.com/JoshPattman/jpf"
)

type WithMessagePrefix struct{ X string }
type WithDelay struct{ X time.Duration }
type WithReasoningPrompt struct{ X string }
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

func failedResponse() jpf.ModelResponse {
	failedUsage := jpf.Usage{FailedCalls: 1}
	return jpf.ModelResponse{Usage: failedUsage}
}

func failedResponseAfter(usage jpf.Usage) jpf.ModelResponse {
	failedUsage := jpf.Usage{FailedCalls: 1}.Add(usage)
	return jpf.ModelResponse{Usage: failedUsage}
}
